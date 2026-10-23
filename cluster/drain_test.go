// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package cluster

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

func TestDrainMovesSessions(t *testing.T) {
	nodes := newCluster(t, "n1", "n2", "n3")
	persistent := connectOptions{persistent: true, expiry: 60}

	watcher := connect(t, nodes[1], "watcher", nil)
	watcher.subscribe("will/#", 0)
	waitRoutes(t, nodes[0], "n2", "will/#")

	// A connected persistent client with a will, a connected clean client, and an offline
	// persistent session.
	o := persistent
	o.will = &packets.ConnectParams{WillTopic: "will/p", WillPayload: []byte("p died")}
	p, _ := connectWith(t, nodes[0], "p", o)
	p.subscribe("p/#", 1)
	clean := connect(t, nodes[0], "clean", nil)
	off, _ := connectWith(t, nodes[0], "off", persistent)
	off.subscribe("off/#", 1)
	_ = off.conn.Close()
	require.Eventually(t, func() bool { return !liveSession(nodes[0], "off") }, waitFor, tick)

	kicked := make(chan byte, 2)
	for _, c := range []*client{p, clean} {
		go func(c *client) {
			code, _ := c.disconnected(waitFor)
			kicked <- code
		}(c)
	}

	res, err := nodes[0].cluster.Drain(context.Background(), DrainOptions{})
	require.NoError(t, err)
	require.Equal(t, DrainResult{Disconnected: 2, Migrated: 2}, res)
	require.Equal(t, packets.ErrUseAnotherServer.Code, <-kicked)
	require.Equal(t, packets.ErrUseAnotherServer.Code, <-kicked)

	// Nothing is left on n1, and the persistent sessions are owned by other nodes.
	for _, cl := range nodes[0].server.Clients.GetAll() {
		require.True(t, cl.Net.Inline, "client %s left on drained node", cl.ID)
	}
	for _, id := range []string{"p", "off"} {
		require.Eventually(t, func() bool {
			o, ok := nodes[1].cluster.registry.owner(id)
			return ok && o.Node != "n1"
		}, waitFor, tick)
	}
	_, ok := watcher.receive(time.Millisecond * 200)
	require.False(t, ok, "drained clients must not send their wills")

	// Messages for the offline session are queued on its new node, and the client gets
	// them when it reconnects anywhere.
	waitRoutes(t, nodes[2], "n1")
	pub := connect(t, nodes[2], "pub", nil)
	pub.publish("off/x", []byte("queued while moving"), 1)
	back, ack := connectWith(t, nodes[1], "off", persistent)
	require.True(t, ack.SessionPresent)
	pk, ok := back.receive(waitFor)
	require.True(t, ok)
	require.Equal(t, []byte("queued while moving"), pk.Payload)

	// The drained node refuses new connections, and other nodes know it is draining.
	_, ack, err = dial(t, nodes[0], "late", connectOptions{})
	require.NoError(t, err)
	require.Equal(t, packets.ErrUseAnotherServer.Code, ack.ReasonCode)
	require.Eventually(t, func() bool {
		for _, n := range nodes[1:] {
			for _, peer := range n.cluster.Stats().Peers {
				if peer.Name == "n1" && !peer.Draining {
					return false
				}
			}
		}
		return true
	}, waitFor, tick)
	require.NotContains(t, nodes[1].cluster.drainTargets(), "n1")
	require.NotEqual(t, "n1", nodes[1].cluster.Leader())

	stats := nodes[0].cluster.Stats()
	require.True(t, stats.Draining)
	require.EqualValues(t, 2, stats.DrainDisconnected)
	require.EqualValues(t, 2, stats.DrainMigrated)
	require.EqualValues(t, 2, stats.SessionsHandedOver)
}

func TestDrainRate(t *testing.T) {
	nodes := newCluster(t, "n1", "n2")
	for _, id := range []string{"a", "b", "c"} {
		c := connect(t, nodes[0], id, nil)
		go func() { _, _ = c.disconnected(waitFor) }()
	}

	start := time.Now()
	res, err := nodes[0].cluster.Drain(context.Background(), DrainOptions{Rate: 10})
	require.NoError(t, err)
	require.Equal(t, 3, res.Disconnected)
	require.GreaterOrEqual(t, time.Since(start), time.Millisecond*200) // three clients at ten a second
}

func TestDrainCancelled(t *testing.T) {
	nodes := newCluster(t, "n1", "n2")
	for _, id := range []string{"a", "b", "c"} {
		c := connect(t, nodes[0], id, nil)
		go func() { _, _ = c.disconnected(waitFor) }()
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*50)
	defer cancel()
	res, err := nodes[0].cluster.Drain(ctx, DrainOptions{Rate: 1})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Equal(t, 1, res.Disconnected)
}

func TestDrainWithoutPeers(t *testing.T) {
	nodes := newCluster(t, "n1")
	c, _ := connectWith(t, nodes[0], "stuck", connectOptions{persistent: true, expiry: 60})
	_ = c.conn.Close()
	require.Eventually(t, func() bool { return !liveSession(nodes[0], "stuck") }, waitFor, tick)

	res, err := nodes[0].cluster.Drain(context.Background(), DrainOptions{})
	require.ErrorIs(t, err, ErrNoDrainPeers)
	require.Equal(t, 1, res.Failed)
}

func TestAdoptRefusedWhileDraining(t *testing.T) {
	nodes := newCluster(t, "n1")
	nodes[0].cluster.draining.Store(true)
	require.ErrorIs(t, nodes[0].cluster.handleAdopt("x", "n2"), ErrDraining)
}

func TestAdoptWithoutSession(t *testing.T) {
	nodes := newCluster(t, "n1", "n2")
	require.NoError(t, nodes[1].cluster.handleAdopt("nobody", "n1"))
	waitOwner(t, nodes, "nobody", "") // the claim is released again
}

func TestPacketGateFencesStaleClients(t *testing.T) {
	nodes := newCluster(t, "n1", "n2", "n3")
	c := nodes[0].cluster
	cl := nodes[0].server.NewClient(nil, "test", "gated", false)
	publish := packets.Packet{FixedHeader: packets.FixedHeader{Type: packets.Publish}, TopicName: "a"}

	_, err := c.OnPacketRead(cl, publish)
	require.NoError(t, err)

	isolate(t, nodes[0])
	require.Eventually(t, func() bool {
		_, err := c.OnPacketRead(cl, publish)
		return err != nil
	}, waitFor, tick)

	_, err = c.OnPacketRead(cl, publish)
	require.ErrorIs(t, err, packets.ErrRejectPacket)
	require.True(t, cl.Closed())

	// Connect packets are handled when the connection is admitted.
	_, err = c.OnPacketRead(nodes[0].server.NewClient(nil, "test", "new", false), packets.Packet{
		FixedHeader: packets.FixedHeader{Type: packets.Connect},
	})
	require.NoError(t, err)

	// Inline clients are never fenced.
	inline := nodes[0].server.NewClient(nil, mqtt.LocalListener, mqtt.InlineClientId, true)
	_, err = c.OnPacketRead(inline, publish)
	require.NoError(t, err)
}

func TestBootstrapWhenSeedsUnreachable(t *testing.T) {
	opts := testOptions("first", "127.0.0.1:1")
	opts.Bootstrap = true
	opts.Memberlist.TCPTimeout = time.Millisecond * 200
	first := newNodeWithOptions(t, opts)
	require.Eventually(t, first.cluster.inContact, waitFor, tick)
	connect(t, first, "client", nil)

	second := newNode(t, "second", first.cluster.Addr())
	require.Eventually(t, func() bool { return second.cluster.inContact() && raftVoters(second) == 2 }, waitFor, tick)
	connect(t, second, "client2", nil)
}

func TestAdoptLeavesMovedSessionAlone(t *testing.T) {
	nodes := newCluster(t, "n1", "n2", "n3")
	live, _ := connectWith(t, nodes[1], "moved", connectOptions{persistent: true, expiry: 60})
	go func() { _, _ = live.disconnected(waitFor) }()
	waitOwner(t, nodes, "moved", "n2")
	o, _ := nodes[1].cluster.registry.owner("moved")

	// A draining node asks for a session which has since moved, to another node...
	require.NoError(t, nodes[2].cluster.handleAdopt("moved", "n1"))
	// ...or to the adopting node itself.
	require.NoError(t, nodes[1].cluster.handleAdopt("moved", "n1"))

	time.Sleep(nodes[0].cluster.opts.ClaimTimeout * 2) // long enough for a superseded session to be dropped
	require.True(t, liveSession(nodes[1], "moved"))
	require.NotNil(t, nodes[1].cluster.sessions.get("moved"))
	for _, n := range nodes {
		got, ok := n.cluster.registry.owner("moved")
		require.True(t, ok)
		require.Equal(t, o, got, "the claim on %s changed", n.cluster.Name())
	}
}
