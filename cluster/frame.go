// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package cluster

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"

	"github.com/mochi-mqtt/server/v2/packets"
)

// Frame types sent over a peer link. Each link carries frames from one node (the
// dialer) to another, in order: a hello, a full route snapshot, then any mix of
// route deltas and forwarded publishes.
const (
	frameHello       byte = iota + 1 // body: sending node name
	frameRoutes                      // body: the sending node's complete set of route filters
	frameRouteAdd                    // body: a filter the sending node now has subscribers for
	frameRouteDelete                 // body: a filter the sending node no longer has subscribers for
	framePublish                     // body: a forwarded publish packet, see encodePublish
	frameRetained                    // body: a change to a retained message, see encodeRetained
)

const (
	frameHeaderSize = 5         // 1 byte type + 4 byte body length
	maxFrameSize    = 268435460 // the largest mqtt packet plus publish metadata
)

var (
	ErrFrameTooLarge  = errors.New("cluster frame exceeds maximum size")
	ErrMalformedFrame = errors.New("malformed cluster frame")
)

// encodeFrame returns a frame of type t containing body.
func encodeFrame(t byte, body []byte) []byte {
	b := make([]byte, frameHeaderSize+len(body))
	b[0] = t
	binary.BigEndian.PutUint32(b[1:frameHeaderSize], uint32(len(body)))
	copy(b[frameHeaderSize:], body)
	return b
}

// readFrame reads the next frame from r.
func readFrame(r *bufio.Reader) (t byte, body []byte, err error) {
	var h [frameHeaderSize]byte
	if _, err = io.ReadFull(r, h[:]); err != nil {
		return 0, nil, err
	}

	n := binary.BigEndian.Uint32(h[1:])
	if n > maxFrameSize {
		return 0, nil, ErrFrameTooLarge
	}

	body = make([]byte, n)
	if _, err = io.ReadFull(r, body); err != nil {
		return 0, nil, err
	}

	return h[0], body, nil
}

// appendString appends a length-prefixed string to b.
func appendString(b []byte, s string) []byte {
	b = binary.AppendUvarint(b, uint64(len(s)))
	return append(b, s...)
}

// readString reads a length-prefixed string from b, returning the remainder.
func readString(b []byte) (string, []byte, error) {
	n, k := binary.Uvarint(b)
	if k <= 0 || n > uint64(len(b)-k) {
		return "", nil, ErrMalformedFrame
	}

	end := k + int(n)
	return string(b[k:end]), b[end:], nil
}

// appendStrings appends a count-prefixed list of strings to b.
func appendStrings(b []byte, ss []string) []byte {
	b = binary.AppendUvarint(b, uint64(len(ss)))
	for _, s := range ss {
		b = appendString(b, s)
	}
	return b
}

// readStrings reads a count-prefixed list of strings from b, returning the remainder.
func readStrings(b []byte) ([]string, []byte, error) {
	n, k := binary.Uvarint(b)
	if k <= 0 || n > uint64(len(b)-k) { // every string takes at least one byte
		return nil, nil, ErrMalformedFrame
	}

	b = b[k:]
	ss := make([]string, 0, n)
	for i := uint64(0); i < n; i++ {
		var s string
		var err error
		s, b, err = readString(b)
		if err != nil {
			return nil, nil, err
		}
		ss = append(ss, s)
	}

	return ss, b, nil
}

// encodePacket encodes a publish packet for forwarding. Packets are always encoded
// as mqtt v5 so that properties survive the hop, regardless of the publisher's version.
func encodePacket(pk packets.Packet) ([]byte, error) {
	out := pk
	out.ProtocolVersion = 5
	out.Mods.AllowResponseInfo = true
	out.Properties.TopicAlias = 0 // aliases are per-connection [MQTT-3.3.2-10]
	out.Properties.TopicAliasFlag = false
	if out.FixedHeader.Qos > 0 && out.PacketID == 0 {
		out.PacketID = 1 // required on the wire; receivers assign their own packet ids
	}

	var buf bytes.Buffer
	if err := out.PublishEncode(&buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// encodePublish returns a publish frame body. raw is the packet encoded with
// encodePacket, and shared lists the shared subscription filters which the
// receiving node should serve.
func encodePublish(pk packets.Packet, raw []byte, shared []string) []byte {
	return appendPublish(make([]byte, 0, len(raw)+len(pk.Origin)+32), pk, raw, shared)
}

func appendPublish(b []byte, pk packets.Packet, raw []byte, shared []string) []byte {
	b = binary.AppendVarint(b, pk.Created)
	b = binary.AppendVarint(b, pk.Expiry)
	b = appendString(b, pk.Origin)
	b = appendStrings(b, shared)
	return append(b, raw...)
}

// publishFrame returns a complete publish frame, with a single allocation.
func publishFrame(pk packets.Packet, raw []byte, shared []string) []byte {
	n := len(raw) + len(pk.Origin) + 32
	for _, s := range shared {
		n += len(s) + binary.MaxVarintLen64
	}

	b := make([]byte, frameHeaderSize, frameHeaderSize+n)
	b[0] = framePublish
	b = appendPublish(b, pk, raw, shared)
	binary.BigEndian.PutUint32(b[1:frameHeaderSize], uint32(len(b)-frameHeaderSize))
	return b
}

// decodePublish decodes a publish frame body.
func decodePublish(b []byte) (pk packets.Packet, shared []string, err error) {
	created, k := binary.Varint(b)
	if k <= 0 {
		return pk, nil, ErrMalformedFrame
	}
	b = b[k:]

	expiry, k := binary.Varint(b)
	if k <= 0 {
		return pk, nil, ErrMalformedFrame
	}
	b = b[k:]

	origin, b, err := readString(b)
	if err != nil {
		return pk, nil, err
	}

	shared, b, err = readStrings(b)
	if err != nil {
		return pk, nil, err
	}

	if len(b) < 2 {
		return pk, nil, ErrMalformedFrame
	}

	if err = pk.FixedHeader.Decode(b[0]); err != nil {
		return pk, nil, err
	}

	if pk.FixedHeader.Type != packets.Publish {
		return pk, nil, ErrMalformedFrame
	}

	remaining, n, err := packets.DecodeLength(bytes.NewReader(b[1:]))
	if err != nil {
		return pk, nil, err
	}

	body := b[1+n:]
	if len(body) != remaining {
		return pk, nil, ErrMalformedFrame
	}

	pk.FixedHeader.Remaining = remaining
	pk.ProtocolVersion = 5
	if err = pk.PublishDecode(body); err != nil {
		return pk, nil, err
	}

	pk.Origin = origin
	pk.Created = created
	pk.Expiry = expiry

	return pk, shared, nil
}
