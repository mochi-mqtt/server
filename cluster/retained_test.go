// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package cluster

import (
	"bytes"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/mochi-mqtt/server/v2/packets"
)

func (c *client) publishRetained(topic string, payload []byte) {
	pk := packets.Packet{
		FixedHeader:     packets.FixedHeader{Type: packets.Publish, Qos: 1, Retain: true},
		ProtocolVersion: 5,
		TopicName:       topic,
		Payload:         payload,
		PacketID:        c.nextID(),
	}
	var buf bytes.Buffer
	require.NoError(c.t, pk.PublishEncode(&buf))
	c.write(buf.Bytes())
	c.expect(packets.Puback)
}

// waitRetained waits until every node has payload retained for topic, or no retained
// message if payload is nil.
func waitRetained(t *testing.T, nodes []*node, topic string, payload []byte) {
	t.Helper()
	require.Eventually(t, func() bool {
		for _, n := range nodes {
			pk, ok := n.server.Topics.Retained.Get(topic)
			if payload == nil && ok || payload != nil && (!ok || !bytes.Equal(pk.Payload, payload)) {
				return false
			}
		}
		return true
	}, waitFor, tick, "retained %s", topic)
}

func TestRetainedVersionOrder(t *testing.T) {
	require.True(t, retainVersion{Time: 2, Node: "a"}.after(retainVersion{Time: 1, Node: "z"}))
	require.True(t, retainVersion{Time: 1, Node: "b"}.after(retainVersion{Time: 1, Node: "a"}))
	require.False(t, retainVersion{Time: 1, Node: "a"}.after(retainVersion{Time: 1, Node: "a"}))
	require.False(t, retainVersion{Time: 1, Node: "z"}.after(retainVersion{Time: 2, Node: "a"}))
}

func TestRetainedFrameRoundTrip(t *testing.T) {
	pk := packets.Packet{
		FixedHeader:     packets.FixedHeader{Type: packets.Publish, Qos: 1, Retain: true},
		TopicName:       "r/a",
		Payload:         []byte("hello"),
		Origin:          "pub",
		Created:         1700000000,
		Expiry:          1700003600,
		ProtocolVersion: 4,
	}
	v := retainVersion{Time: 1700000000123456789, Node: "n1"}

	frame, err := encodeRetained(v, false, pk)
	require.NoError(t, err)
	require.Equal(t, frameRetained, frame[0])
	gotV, deleted, got, err := decodeRetained(frame[frameHeaderSize:])
	require.NoError(t, err)
	require.Equal(t, v, gotV)
	require.False(t, deleted)
	require.Equal(t, "r/a", got.TopicName)
	require.Equal(t, []byte("hello"), got.Payload)
	require.True(t, got.FixedHeader.Retain)
	require.Equal(t, byte(4), got.ProtocolVersion) // a v3 publisher's message must not expire like a v5 one
	require.Equal(t, pk.Expiry, got.Expiry)
	require.Equal(t, pk.Origin, got.Origin)

	frame, err = encodeRetained(v, true, packets.Packet{TopicName: "r/b"})
	require.NoError(t, err)
	gotV, deleted, got, err = decodeRetained(frame[frameHeaderSize:])
	require.NoError(t, err)
	require.Equal(t, v, gotV)
	require.True(t, deleted)
	require.Equal(t, "r/b", got.TopicName)
	require.Empty(t, got.Payload)

	_, _, _, err = decodeRetained(nil)
	require.ErrorIs(t, err, ErrMalformedFrame)
	_, _, _, err = decodeRetained(frame[frameHeaderSize : len(frame)-1])
	require.ErrorIs(t, err, ErrMalformedFrame)
}

func TestRetainedLatestChangeWins(t *testing.T) {
	n := newNode(t, "n1")
	c := n.cluster
	set := func(payload string) packets.Packet {
		return packets.Packet{
			FixedHeader: packets.FixedHeader{Type: packets.Publish, Retain: true},
			TopicName:   "r/x",
			Payload:     []byte(payload),
		}
	}

	require.True(t, c.applyRetained(retainVersion{Time: 10, Node: "n2"}, false, set("ten")))
	require.False(t, c.applyRetained(retainVersion{Time: 5, Node: "n3"}, false, set("five")))
	waitRetained(t, []*node{n}, "r/x", []byte("ten"))

	// A deletion leaves a tombstone, so an older change can't restore the message.
	require.True(t, c.applyRetained(retainVersion{Time: 20, Node: "n2"}, true, packets.Packet{TopicName: "r/x"}))
	waitRetained(t, []*node{n}, "r/x", nil)
	require.False(t, c.applyRetained(retainVersion{Time: 15, Node: "n3"}, false, set("fifteen")))
	waitRetained(t, []*node{n}, "r/x", nil)

	require.True(t, c.applyRetained(retainVersion{Time: 30, Node: "n3"}, false, set("thirty")))
	waitRetained(t, []*node{n}, "r/x", []byte("thirty"))

	// $SYS topics and invalid topics are never replicated.
	require.False(t, c.applyRetained(retainVersion{Time: 40, Node: "n2"}, false, packets.Packet{TopicName: "$SYS/x", Payload: []byte("x")}))
	require.False(t, c.applyRetained(retainVersion{Time: 40, Node: "n2"}, false, packets.Packet{TopicName: "a/+", Payload: []byte("x")}))
}

func TestRetainedStoredMessageVersion(t *testing.T) {
	n := newNode(t, "n1")
	c := n.cluster

	// A message retained without a recorded change, as if loaded from storage.
	n.server.Topics.RetainMessage(packets.Packet{
		FixedHeader: packets.FixedHeader{Type: packets.Publish, Retain: true},
		TopicName:   "r/stored",
		Payload:     []byte("stored"),
		Created:     100,
	})

	older := retainVersion{Time: 99 * int64(time.Second), Node: "n2"}
	require.False(t, c.applyRetained(older, true, packets.Packet{TopicName: "r/stored"}))
	newer := retainVersion{Time: 101 * int64(time.Second), Node: "n2"}
	require.True(t, c.applyRetained(newer, true, packets.Packet{TopicName: "r/stored"}))
	waitRetained(t, []*node{n}, "r/stored", nil)
}

func TestRetainedTombstonesExpire(t *testing.T) {
	n := newNode(t, "n1")
	c := n.cluster
	require.True(t, c.applyRetained(retainVersion{Time: 1, Node: "n2"}, true, packets.Packet{TopicName: "r/gone"}))
	require.True(t, c.applyRetained(retainVersion{Time: 1, Node: "n2"}, false, packets.Packet{TopicName: "r/kept", Payload: []byte("x")}))

	c.expireTombstones(time.Now())
	require.Len(t, c.retained, 2)

	c.expireTombstones(time.Now().Add(c.opts.RetainedTombstoneTTL + time.Second))
	require.Len(t, c.retained, 1)

	n.server.Topics.Retained.Delete("r/kept") // as when a retained message expires
	c.expireTombstones(time.Now())
	require.Empty(t, c.retained)
}

func TestRetainedReplicated(t *testing.T) {
	nodes := newCluster(t, "n1", "n2", "n3")

	pub := connect(t, nodes[0], "pub", nil)
	pub.publishRetained("r/a", []byte("hello"))
	waitRetained(t, nodes, "r/a", []byte("hello"))
	require.Eventually(t, func() bool { // the count includes the node's own $SYS topics
		return atomic.LoadInt64(&nodes[2].server.Info.Retained) == int64(nodes[2].server.Topics.Retained.Len())
	}, waitFor, tick)

	// A new subscriber on another node receives it.
	sub := connect(t, nodes[2], "sub", nil)
	sub.subscribe("r/#", 1)
	pk, ok := sub.receive(waitFor)
	require.True(t, ok)
	require.Equal(t, "r/a", pk.TopicName)
	require.Equal(t, []byte("hello"), pk.Payload)
	require.True(t, pk.FixedHeader.Retain)

	// Deleting it on any node deletes it everywhere.
	pub2 := connect(t, nodes[1], "pub2", nil)
	pub2.publishRetained("r/a", nil)
	waitRetained(t, nodes, "r/a", nil)
}

func TestRetainedInlineAndWillReplicated(t *testing.T) {
	nodes := newCluster(t, "n1", "n2")

	require.NoError(t, nodes[0].server.Publish("r/inline", []byte("inline"), true, 0))
	waitRetained(t, nodes, "r/inline", []byte("inline"))

	dying := connect(t, nodes[0], "dying", &packets.ConnectParams{
		WillTopic:   "r/will",
		WillPayload: []byte("goodbye"),
		WillRetain:  true,
	})
	_ = dying.conn.Close()
	waitRetained(t, nodes, "r/will", []byte("goodbye"))
}

func TestRetainedConcurrentChangesConverge(t *testing.T) {
	nodes := newCluster(t, "n1", "n2", "n3")

	pubs := make([]*client, len(nodes))
	for i, n := range nodes {
		pubs[i] = connect(t, n, "pub"+n.cluster.Name(), nil)
	}

	for round := 0; round < 5; round++ {
		for i, p := range pubs {
			p.publishRetained("r/race", []byte{byte('a' + i), byte('0' + round)})
		}
	}

	require.Eventually(t, func() bool {
		first, ok := nodes[0].server.Topics.Retained.Get("r/race")
		if !ok {
			return false
		}
		for _, n := range nodes[1:] {
			pk, ok := n.server.Topics.Retained.Get("r/race")
			if !ok || !bytes.Equal(pk.Payload, first.Payload) {
				return false
			}
		}
		return true
	}, waitFor, tick)

	pk, _ := nodes[0].server.Topics.Retained.Get("r/race")
	require.Equal(t, byte('4'), pk.Payload[1]) // the last round wins
}

func TestRetainedSnapshotOnJoin(t *testing.T) {
	nodes := newCluster(t, "n1", "n2")

	pub := connect(t, nodes[0], "pub", nil)
	pub.publishRetained("r/kept", []byte("kept"))
	pub.publishRetained("r/deleted", []byte("soon gone"))
	pub.publishRetained("r/deleted", nil)
	waitRetained(t, nodes, "r/kept", []byte("kept"))

	n3 := newNode(t, "n3", nodes[0].cluster.Addr())
	waitRetained(t, []*node{n3}, "r/kept", []byte("kept"))
	require.Eventually(t, func() bool {
		n3.cluster.retainMu.Lock()
		defer n3.cluster.retainMu.Unlock()
		e, ok := n3.cluster.retained["r/deleted"]
		return ok && e.deleted
	}, waitFor, tick, "tombstone replicated")
}

func TestRetainedSurvivesNodeLeaving(t *testing.T) {
	nodes := newCluster(t, "n1", "n2", "n3")

	pub := connect(t, nodes[2], "pub", nil)
	pub.publishRetained("r/orphan", []byte("still here"))
	waitRetained(t, nodes, "r/orphan", []byte("still here"))

	nodes[2].close()

	sub := connect(t, nodes[0], "sub", nil)
	sub.subscribe("r/orphan", 0)
	pk, ok := sub.receive(waitFor)
	require.True(t, ok)
	require.Equal(t, []byte("still here"), pk.Payload)
}
