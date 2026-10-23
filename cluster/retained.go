// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package cluster

import (
	"encoding/binary"
	"strings"
	"sync/atomic"
	"time"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

const (
	defaultTombstoneTTL = time.Hour * 24
	retainedFlagDeleted = 1 << 0
)

// retainVersion orders changes to a topic's retained message across the cluster. A later
// time wins, with the node name breaking ties, so every node keeps the same message.
type retainVersion struct {
	Time int64  // unix nanoseconds, on the node where the change was published
	Node string // the node where the change was published
}

// after returns true if v is a later change than o.
func (v retainVersion) after(o retainVersion) bool {
	return v.Time > o.Time || v.Time == o.Time && v.Node > o.Node
}

// storedVersion is the version of a retained message with no recorded change, such as one
// loaded from storage or retained before the node joined the cluster.
func storedVersion(pk packets.Packet) retainVersion {
	return retainVersion{Time: pk.Created * int64(time.Second)}
}

// retainEntry records the latest change to a topic's retained message. Deletions are kept
// as tombstones, so an older copy of the message arriving later can't restore it.
type retainEntry struct {
	version retainVersion
	deleted bool
	at      time.Time // when a tombstone was recorded
}

// nextRetainTime returns a unix nanosecond time for a local change, later than any
// previous local change.
func nextRetainTime(clock *atomic.Int64) int64 {
	for {
		prev := clock.Load()
		next := max(time.Now().UnixNano(), prev+1)
		if clock.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// encodeRetained returns a retained message change frame. pk is the retained packet, or
// for a deletion, any packet with the topic name.
func encodeRetained(v retainVersion, deleted bool, pk packets.Packet) ([]byte, error) {
	b := binary.AppendVarint(nil, v.Time)
	b = appendString(b, v.Node)

	if deleted {
		b = append(b, retainedFlagDeleted, pk.ProtocolVersion)
		return encodeFrame(frameRetained, appendString(b, pk.TopicName)), nil
	}

	raw, err := encodePacket(pk)
	if err != nil {
		return nil, err
	}

	b = append(b, 0, pk.ProtocolVersion)
	return encodeFrame(frameRetained, append(b, encodePublish(pk, raw, nil)...)), nil
}

// decodeRetained decodes a retained message change frame body.
func decodeRetained(b []byte) (v retainVersion, deleted bool, pk packets.Packet, err error) {
	t, n := binary.Varint(b)
	if n <= 0 {
		return v, false, pk, ErrMalformedFrame
	}
	v.Time = t

	v.Node, b, err = readString(b[n:])
	if err != nil {
		return v, false, pk, err
	}

	if len(b) < 2 {
		return v, false, pk, ErrMalformedFrame
	}
	deleted = b[0]&retainedFlagDeleted > 0
	version := b[1]
	b = b[2:]

	if deleted {
		pk.FixedHeader = packets.FixedHeader{Type: packets.Publish, Retain: true}
		pk.TopicName, b, err = readString(b)
		if err == nil && len(b) > 0 {
			err = ErrMalformedFrame
		}
	} else {
		pk, _, err = decodePublish(b)
	}

	pk.ProtocolVersion = version // expiry of retained messages depends on the publisher's version
	return v, deleted, pk, err
}

// OnRetainMessage replicates a retained message published to this node.
func (c *Cluster) OnRetainMessage(cl *mqtt.Client, pk packets.Packet, r int64) {
	if strings.HasPrefix(pk.TopicName, mqtt.SysPrefix) {
		return
	}

	v := retainVersion{Time: nextRetainTime(&c.retainClock), Node: c.opts.NodeName}
	deleted := len(pk.Payload) == 0

	c.retainMu.Lock()
	defer c.retainMu.Unlock()

	c.retained[pk.TopicName] = retainEntry{version: v, deleted: deleted, at: time.Now()}

	// The server has already retained the message, but a change from another node may
	// have replaced it since, so make sure the local message matches the recorded version.
	c.server.Topics.RetainMessage(pk.Copy(false))
	c.countRetained()

	frame, err := encodeRetained(v, deleted, pk)
	if err != nil {
		c.log.Warn("failed to encode retained message", "error", err, "topic", pk.TopicName)
		return
	}

	c.peersMu.RLock()
	defer c.peersMu.RUnlock()
	for _, p := range c.peers {
		if p.send(frame) {
			c.metrics.retainedSent.Add(1)
		} else {
			c.metrics.linkResyncs.Add(1)
			p.resync() // the snapshot sent on reconnecting includes this change
		}
	}
}

// applyRetained applies a retained message change from another node, if it is later than
// the change this node has.
func (c *Cluster) applyRetained(v retainVersion, deleted bool, pk packets.Packet) bool {
	if strings.HasPrefix(pk.TopicName, mqtt.SysPrefix) || !mqtt.IsValidFilter(pk.TopicName, true) {
		return false
	}

	c.retainMu.Lock()
	defer c.retainMu.Unlock()

	cur, ok := c.retained[pk.TopicName]
	if !ok {
		if existing, has := c.server.Topics.Retained.Get(pk.TopicName); has {
			cur, ok = retainEntry{version: storedVersion(existing)}, true
		}
	}

	if ok && !v.after(cur.version) {
		c.metrics.retainedIgnored.Add(1)
		return false
	}
	c.metrics.retainedApplied.Add(1)

	c.retained[pk.TopicName] = retainEntry{version: v, deleted: deleted, at: time.Now()}
	if deleted {
		pk.Payload = nil
	}
	c.server.Topics.RetainMessage(pk)
	c.countRetained()
	return true
}

// countRetained updates the server's retained message count. c.retainMu must be held.
func (c *Cluster) countRetained() {
	atomic.StoreInt64(&c.server.Info.Retained, int64(c.server.Topics.Retained.Len()))
}

// retainedSnapshot returns frames containing every retained message and tombstone.
func (c *Cluster) retainedSnapshot() [][]byte {
	c.retainMu.Lock()
	defer c.retainMu.Unlock()

	var frames [][]byte
	for topic, pk := range c.server.Topics.Retained.GetAll() {
		if strings.HasPrefix(topic, mqtt.SysPrefix) {
			continue
		}

		v := storedVersion(pk)
		if e, ok := c.retained[topic]; ok && !e.deleted {
			v = e.version
		}

		if frame, err := encodeRetained(v, false, pk); err == nil {
			frames = append(frames, frame)
		}
	}

	for topic, e := range c.retained {
		if e.deleted {
			frame, _ := encodeRetained(e.version, true, packets.Packet{TopicName: topic})
			frames = append(frames, frame)
		}
	}

	return frames
}

// expireTombstones forgets tombstones older than the tombstone ttl, and versions of
// retained messages which have expired.
func (c *Cluster) expireTombstones(now time.Time) {
	c.retainMu.Lock()
	defer c.retainMu.Unlock()
	for topic, e := range c.retained {
		if e.deleted {
			if now.Sub(e.at) > c.opts.RetainedTombstoneTTL {
				delete(c.retained, topic)
			}
			continue
		}

		if _, ok := c.server.Topics.Retained.Get(topic); !ok {
			delete(c.retained, topic)
		}
	}
}

// tombstoneLoop periodically expires tombstones.
func (c *Cluster) tombstoneLoop() {
	defer c.wg.Done()
	interval := min(c.opts.RetainedTombstoneTTL/10, time.Minute)
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-c.done:
			return
		case now := <-t.C:
			c.expireTombstones(now)
		}
	}
}
