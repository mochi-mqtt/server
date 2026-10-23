// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package cluster

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/mochi-mqtt/server/v2/packets"
	"github.com/stretchr/testify/require"
)

func TestFrameRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	buf.Write(encodeFrame(frameRouteAdd, []byte("a/b/#")))
	buf.Write(encodeFrame(frameRoutes, appendStrings(nil, []string{"a", "b/+", "$share/g/c"})))

	r := bufio.NewReader(&buf)
	ft, body, err := readFrame(r)
	require.NoError(t, err)
	require.Equal(t, frameRouteAdd, ft)
	require.Equal(t, "a/b/#", string(body))

	ft, body, err = readFrame(r)
	require.NoError(t, err)
	require.Equal(t, frameRoutes, ft)
	ss, rest, err := readStrings(body)
	require.NoError(t, err)
	require.Empty(t, rest)
	require.Equal(t, []string{"a", "b/+", "$share/g/c"}, ss)
}

func TestReadFrameTooLarge(t *testing.T) {
	r := bufio.NewReader(bytes.NewReader([]byte{frameRoutes, 0xff, 0xff, 0xff, 0xff}))
	_, _, err := readFrame(r)
	require.ErrorIs(t, err, ErrFrameTooLarge)
}

func TestReadStringsMalformed(t *testing.T) {
	_, _, err := readStrings([]byte{0x05, 0x01, 'a'}) // claims 5 strings
	require.ErrorIs(t, err, ErrMalformedFrame)

	_, _, err = readStrings([]byte{0x01, 0x09, 'a'}) // string claims 9 bytes
	require.ErrorIs(t, err, ErrMalformedFrame)
}

func TestPublishRoundTrip(t *testing.T) {
	pk := packets.Packet{
		FixedHeader: packets.FixedHeader{Type: packets.Publish, Qos: 1, Retain: true},
		TopicName:   "a/b/c",
		Payload:     []byte("hello mochi"),
		PacketID:    42,
		Origin:      "client-1",
		Created:     1700000000,
		Expiry:      1700000060,
		Properties: packets.Properties{
			ContentType:           "text/plain",
			ResponseTopic:         "reply/to",
			CorrelationData:       []byte("corr"),
			MessageExpiryInterval: 60,
			TopicAlias:            3,
			TopicAliasFlag:        true,
			User:                  []packets.UserProperty{{Key: "k", Val: "v"}},
		},
		ProtocolVersion: 4, // properties are kept regardless of the publisher's version
	}

	raw, err := encodePacket(pk)
	require.NoError(t, err)

	out, shared, err := decodePublish(encodePublish(pk, raw, []string{"$share/g/a/b/c"}))
	require.NoError(t, err)
	require.Equal(t, []string{"$share/g/a/b/c"}, shared)
	require.Equal(t, pk.TopicName, out.TopicName)
	require.Equal(t, pk.Payload, out.Payload)
	require.Equal(t, pk.FixedHeader.Qos, out.FixedHeader.Qos)
	require.True(t, out.FixedHeader.Retain)
	require.Equal(t, pk.Origin, out.Origin)
	require.Equal(t, pk.Created, out.Created)
	require.Equal(t, pk.Expiry, out.Expiry)
	require.Equal(t, "text/plain", out.Properties.ContentType)
	require.Equal(t, "reply/to", out.Properties.ResponseTopic)
	require.Equal(t, []byte("corr"), out.Properties.CorrelationData)
	require.Equal(t, pk.Properties.User, out.Properties.User)
	require.Zero(t, out.Properties.TopicAlias) // aliases don't cross connections
}

func TestPublishQos1WithoutPacketID(t *testing.T) {
	pk := packets.Packet{
		FixedHeader: packets.FixedHeader{Type: packets.Publish, Qos: 1},
		TopicName:   "a/b",
		Payload:     []byte("x"),
	}

	raw, err := encodePacket(pk) // inline client publishes have no packet id
	require.NoError(t, err)
	out, shared, err := decodePublish(encodePublish(pk, raw, nil))
	require.NoError(t, err)
	require.Empty(t, shared)
	require.Equal(t, "a/b", out.TopicName)
}

func TestDecodePublishMalformed(t *testing.T) {
	pk := packets.Packet{
		FixedHeader: packets.FixedHeader{Type: packets.Publish},
		TopicName:   "a/b",
		Payload:     []byte("payload"),
	}
	raw, err := encodePacket(pk)
	require.NoError(t, err)
	b := encodePublish(pk, raw, nil)

	_, _, err = decodePublish(b[:len(b)-2]) // truncated payload
	require.ErrorIs(t, err, ErrMalformedFrame)

	_, _, err = decodePublish(nil)
	require.ErrorIs(t, err, ErrMalformedFrame)
}

func TestPublishFrame(t *testing.T) {
	pk := packets.Packet{
		FixedHeader: packets.FixedHeader{Type: packets.Publish},
		TopicName:   "a/b",
		Payload:     []byte("payload"),
		Origin:      "o",
	}
	raw, err := encodePacket(pk)
	require.NoError(t, err)

	shared := []string{"$share/g/a/b", "$share/h/a/#"}
	require.Equal(t, encodeFrame(framePublish, encodePublish(pk, raw, shared)), publishFrame(pk, raw, shared))
	require.Equal(t, encodeFrame(framePublish, encodePublish(pk, raw, nil)), publishFrame(pk, raw, nil))
}
