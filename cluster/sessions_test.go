// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package cluster

import (
	"context"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/require"

	"github.com/mochi-mqtt/server/v2/hooks/storage"
	"github.com/mochi-mqtt/server/v2/packets"
)

// isolate cuts a node off from the cluster without leaving, as if it had crashed or been
// partitioned, while its mqtt server and clients keep running.
func isolate(t *testing.T, n *node) {
	t.Helper()
	c := n.cluster
	require.NoError(t, c.raft.Shutdown().Error())
	c.gossipDown.Store(true)
	require.NoError(t, c.ml.Shutdown())
	_ = c.ln.Close()

	c.peersMu.Lock()
	for name, p := range c.peers {
		p.stop()
		delete(c.peers, name)
	}
	c.peersMu.Unlock()

	c.inboundMu.Lock()
	for _, conn := range c.inbound {
		_ = conn.Close()
	}
	c.inboundMu.Unlock()
}

// liveSession returns true if a node has the client connected.
func liveSession(n *node, id string) bool {
	cl, ok := n.server.Clients.Get(id)
	return ok && !cl.Closed()
}

// waitOwner waits until every node's registry records owner as the client's owner.
func waitOwner(t *testing.T, nodes []*node, id, owner string) {
	t.Helper()
	require.Eventually(t, func() bool {
		for _, n := range nodes {
			o, ok := n.cluster.registry.owner(id)
			if owner == "" && ok || owner != "" && o.Node != owner {
				return false
			}
		}
		return true
	}, waitFor, tick, "owner of %s", id)
}

// disconnected waits for the server to disconnect a client, returning the reason code.
func (c *client) disconnected(d time.Duration) (byte, bool) {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		pk, ok := c.read(time.Until(deadline))
		if !ok {
			return 0, false
		}
		if pk.FixedHeader.Type == packets.Disconnect {
			return pk.ReasonCode, true
		}
	}
	return 0, false
}

func TestSessionMovesBetweenNodes(t *testing.T) {
	nodes := newCluster(t, "n1", "n2", "n3")
	o := connectOptions{persistent: true, expiry: 60}

	c1, ack := connectWith(t, nodes[0], "mover", o)
	require.False(t, ack.SessionPresent)
	c1.subscribe("s/#", 1)
	waitRoutes(t, nodes[2], "n1", "s/#")
	waitOwner(t, nodes, "mover", "n1")

	// The client goes away, and a message is queued for its session on n1.
	_ = c1.conn.Close()
	require.Eventually(t, func() bool { return !liveSession(nodes[0], "mover") }, waitFor, tick)
	pub := connect(t, nodes[2], "pub", nil)
	pub.publish("s/x", []byte("while away"), 1)
	require.Eventually(t, func() bool {
		cl, ok := nodes[0].server.Clients.Get("mover")
		return ok && cl.State.Inflight.Len() == 1
	}, waitFor, tick)

	// It reconnects to n2, which takes over the session from n1.
	c2, ack := connectWith(t, nodes[1], "mover", o)
	require.True(t, ack.SessionPresent)

	pk, ok := c2.receive(waitFor)
	require.True(t, ok)
	require.Equal(t, "s/x", pk.TopicName)
	require.Equal(t, []byte("while away"), pk.Payload)

	_, ok = nodes[0].server.Clients.Get("mover")
	require.False(t, ok)
	waitOwner(t, nodes, "mover", "n2")
	waitRoutes(t, nodes[2], "n1")
	waitRoutes(t, nodes[2], "n2", "s/#")

	// The subscription moved with it.
	pub.publish("s/y", []byte("after the move"), 1)
	pk, ok = c2.receive(waitFor)
	require.True(t, ok)
	require.Equal(t, []byte("after the move"), pk.Payload)
}

func TestSessionLiveTakeover(t *testing.T) {
	nodes := newCluster(t, "n1", "n2")
	o := connectOptions{persistent: true, expiry: 60}

	c1, _ := connectWith(t, nodes[0], "dup", o)
	c1.subscribe("live/#", 0)

	type result struct {
		code byte
		ok   bool
	}
	kicked := make(chan result, 1)
	go func() { // the takeover writes a disconnect to the old connection
		code, ok := c1.disconnected(waitFor)
		kicked <- result{code, ok}
	}()

	_, ack := connectWith(t, nodes[1], "dup", o)
	require.True(t, ack.SessionPresent)

	r := <-kicked
	require.True(t, r.ok)
	require.Equal(t, packets.ErrSessionTakenOver.Code, r.code)
	require.False(t, liveSession(nodes[0], "dup"))
	require.True(t, liveSession(nodes[1], "dup"))
}

func TestSessionCleanStartDiscardsRemoteSession(t *testing.T) {
	nodes := newCluster(t, "n1", "n2")

	c1, _ := connectWith(t, nodes[0], "fresh", connectOptions{persistent: true, expiry: 60})
	c1.subscribe("old/#", 0)
	waitRoutes(t, nodes[1], "n1", "old/#")
	_ = c1.conn.Close()
	require.Eventually(t, func() bool { return !liveSession(nodes[0], "fresh") }, waitFor, tick)

	c2, ack := connectWith(t, nodes[1], "fresh", connectOptions{})
	require.False(t, ack.SessionPresent)
	_, ok := nodes[0].server.Clients.Get("fresh")
	require.False(t, ok)
	waitRoutes(t, nodes[1], "n1")

	pub := connect(t, nodes[0], "pub", nil)
	pub.publish("old/topic", []byte("x"), 0)
	_, ok = c2.receive(time.Millisecond * 200)
	require.False(t, ok)
}

func TestSessionConcurrentConnectsOneWinner(t *testing.T) {
	nodes := newCluster(t, "n1", "n2", "n3")

	var wg sync.WaitGroup
	for _, n := range nodes {
		wg.Add(1)
		go func(n *node) {
			defer wg.Done()
			c, _, err := dial(t, n, "racer", connectOptions{persistent: true, expiry: 60})
			if err == nil {
				go func() { _, _ = io.Copy(io.Discard, c.conn) }() // accept a takeover's disconnect
			}
		}(n)
	}
	wg.Wait()

	live := func() (count int, at string) {
		for _, n := range nodes {
			if liveSession(n, "racer") {
				count++
				at = n.cluster.Name()
			}
		}
		return
	}

	require.Eventually(t, func() bool { count, _ := live(); return count == 1 }, waitFor, tick)
	require.Never(t, func() bool { count, _ := live(); return count != 1 }, time.Millisecond*300, tick)

	_, winner := live()
	waitOwner(t, nodes, "racer", winner)
}

func TestSessionReleasedOnEnd(t *testing.T) {
	nodes := newCluster(t, "n1", "n2")

	c := connect(t, nodes[0], "brief", nil)
	waitOwner(t, nodes, "brief", "n1")

	_ = c.conn.Close() // a clean session ends on disconnect
	waitOwner(t, nodes, "brief", "")
	require.Zero(t, nodes[0].cluster.sessions.len())
}

func TestSessionStaleTakeoverRefused(t *testing.T) {
	nodes := newCluster(t, "n1")
	connect(t, nodes[0], "keeper", nil)
	o, ok := nodes[0].cluster.registry.owner("keeper")
	require.True(t, ok)

	_, err := nodes[0].cluster.handleTakeover("keeper", o.Epoch-1, false)
	require.ErrorIs(t, err, errStaleClaim)
	require.True(t, liveSession(nodes[0], "keeper"))
}

func TestSessionDelayedWillCancelledOnRemoteResume(t *testing.T) {
	nodes := newCluster(t, "n1", "n2", "n3")

	sub := connect(t, nodes[2], "watcher", nil)
	sub.subscribe("will/#", 0)
	waitRoutes(t, nodes[0], "n3", "will/#")

	o := connectOptions{
		persistent: true,
		expiry:     60,
		will: &packets.ConnectParams{
			WillTopic:      "will/flaky",
			WillPayload:    []byte("gone"),
			WillProperties: packets.Properties{WillDelayInterval: 1},
		},
	}
	c1, _ := connectWith(t, nodes[0], "flaky", o)
	_ = c1.conn.Close() // the will is scheduled for a second from now
	require.Eventually(t, func() bool { return !liveSession(nodes[0], "flaky") }, waitFor, tick)

	connectWith(t, nodes[1], "flaky", o) // the session resumes elsewhere in time

	_, ok := sub.receive(time.Millisecond * 1500)
	require.False(t, ok, "delayed will should not be sent")
}

func TestSessionRefusedWithoutQuorum(t *testing.T) {
	nodes := newCluster(t, "n1", "n2", "n3")
	existing := connect(t, nodes[0], "existing", nil)

	isolate(t, nodes[1])
	isolate(t, nodes[2])

	// The remaining node can't reach a quorum, so it disconnects its clients...
	code, ok := existing.disconnected(waitFor)
	require.True(t, ok)
	require.Equal(t, packets.ErrUseAnotherServer.Code, code)

	// ...and refuses new ones.
	_, ack, err := dial(t, nodes[0], "newcomer", connectOptions{})
	require.NoError(t, err)
	require.Equal(t, packets.ErrServerUnavailable.Code, ack.ReasonCode)
	require.False(t, liveSession(nodes[0], "newcomer"))
}

func TestSessionFencedWhenOwnerIsolated(t *testing.T) {
	nodes := newCluster(t, "n1", "n2", "n3")
	o := connectOptions{persistent: true, expiry: 60}

	old, _ := connectWith(t, nodes[0], "fenced", o)
	waitOwner(t, nodes, "fenced", "n1")

	// n1 is cut off, but its server and the client's connection to it keep running.
	isolate(t, nodes[0])

	var oldClosed time.Time
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = old.disconnected(waitFor * 2)
		oldClosed = time.Now()
	}()

	// The client also connects to n2. n2 can't reach n1 to take the session over, so it
	// waits until n1 must have disconnected the client.
	start := time.Now()
	_, ack := connectWith(t, nodes[1], "fenced", o)
	admitted := time.Now()
	require.False(t, ack.SessionPresent) // n1's session state couldn't be recovered
	require.GreaterOrEqual(t, admitted.Sub(start), nodes[1].cluster.opts.LeaseTimeout)

	<-done
	require.False(t, liveSession(nodes[0], "fenced"))
	require.True(t, oldClosed.Before(admitted), "old connection closed at %s, new admitted at %s",
		oldClosed.Sub(start), admitted.Sub(start))
	waitOwner(t, nodes[1:], "fenced", "n2")
}

func TestSessionDroppedWhenOwnerUnreachableButInContact(t *testing.T) {
	nodes := newCluster(t, "n1", "n2", "n3")
	o := connectOptions{persistent: true, expiry: 60}

	// Claims are made through the leader, so n1 mustn't be the leader.
	if nodes[0].cluster.Leader() == "n1" {
		require.NoError(t, nodes[0].cluster.raft.LeadershipTransfer().Error())
	}
	require.Eventually(t, func() bool {
		leader := nodes[0].cluster.Leader()
		return leader != "" && leader != "n1" && nodes[0].cluster.inContact()
	}, waitFor, tick)

	old, _ := connectWith(t, nodes[0], "split", o)
	waitOwner(t, nodes, "split", "n1")

	// Other nodes can no longer reach n1 directly, but n1 is still in the registry quorum.
	_ = nodes[0].cluster.ln.Close()

	var oldClosed time.Time
	var oldCode byte
	done := make(chan struct{})
	go func() {
		defer close(done)
		oldCode, _ = old.disconnected(waitFor * 2)
		oldClosed = time.Now()
	}()

	start := time.Now()
	connectWith(t, nodes[1], "split", o)
	admitted := time.Now()

	// n1 sees n2's newer claim and drops its own session before n2 admits the client.
	<-done
	require.Equal(t, packets.ErrSessionTakenOver.Code, oldCode)
	require.True(t, nodes[0].cluster.inContact())
	require.False(t, liveSession(nodes[0], "split"))
	require.True(t, oldClosed.Before(admitted), "old connection closed at %s, new admitted at %s",
		oldClosed.Sub(start), admitted.Sub(start))
}

func TestSessionFencedWhenOwnerPartitioned(t *testing.T) {
	nodes := newCluster(t, "n1", "n2", "n3")
	o := connectOptions{persistent: true, expiry: 60}

	if nodes[0].cluster.Leader() == "n1" {
		require.NoError(t, nodes[0].cluster.raft.LeadershipTransfer().Error())
	}
	require.Eventually(t, func() bool {
		leader := nodes[0].cluster.Leader()
		return leader != "" && leader != "n1" && nodes[0].cluster.inContact()
	}, waitFor, tick)

	old, _ := connectWith(t, nodes[0], "cutoff", o)
	waitOwner(t, nodes, "cutoff", "n1")

	// n1's raft keeps running as a follower, but can't reach the leader, and other nodes
	// can't reach n1 to take the session over.
	nodes[0].cluster.stream.partition()
	_ = nodes[0].cluster.ln.Close()

	var oldClosed time.Time
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = old.disconnected(waitFor * 2)
		oldClosed = time.Now()
	}()

	start := time.Now()
	connectWith(t, nodes[1], "cutoff", o)
	admitted := time.Now()

	<-done
	require.False(t, liveSession(nodes[0], "cutoff"))
	require.True(t, oldClosed.Before(admitted), "old connection closed at %s, new admitted at %s",
		oldClosed.Sub(start), admitted.Sub(start))
}

func TestSessionSweptAfterRegistryRestore(t *testing.T) {
	nodes := newCluster(t, "n1")
	c, _ := connectWith(t, nodes[0], "stale", connectOptions{persistent: true, expiry: 60})
	go func() { _, _ = c.disconnected(waitFor * 2) }()
	o, _ := nodes[0].cluster.registry.owner("stale")

	// As if a snapshot showed the session had moved to n2 while this node was away.
	nodes[0].cluster.registry.mu.Lock()
	nodes[0].cluster.registry.owners["stale"] = owner{Node: "n2", Epoch: o.Epoch + 10}
	nodes[0].cluster.registry.mu.Unlock()
	nodes[0].cluster.sweepSuperseded()

	require.Eventually(t, func() bool {
		_, ok := nodes[0].server.Clients.Get("stale")
		return !ok
	}, waitFor, tick)
	require.EqualValues(t, 1, nodes[0].cluster.Stats().SessionsSuperseded)
}

func TestSessionTakeoverWaitsForAdmissionInProgress(t *testing.T) {
	nodes := newCluster(t, "n1", "n2", "n3")
	o := connectOptions{persistent: true, expiry: 60}

	// The client is connected to n3, which other nodes then can't reach.
	first, _ := connectWith(t, nodes[2], "racer", o)
	go func() { _, _ = first.disconnected(waitFor * 2) }()
	waitOwner(t, nodes, "racer", "n3")
	_ = nodes[2].cluster.ln.Close()

	// The client connects to n1, which has to wait out n3's lease...
	type result struct {
		c       *client
		ack     packets.Packet
		err     error
		closed  time.Time
		reason  byte
		kickedd bool
	}
	slow := make(chan result, 1)
	go func() {
		c, ack, err := dial(t, nodes[0], "racer", o)
		r := result{c: c, ack: ack, err: err}
		if err == nil && ack.ReasonCode == 0 {
			r.reason, r.kickedd = c.disconnected(waitFor * 2)
			r.closed = time.Now()
		}
		slow <- r
	}()
	time.Sleep(time.Millisecond * 300)

	// ...and meanwhile connects to n2, which takes the session over from n1. n1 must let
	// its own admission finish and hand that session over, rather than miss it.
	_, ack := connectWith(t, nodes[1], "racer", o)
	admitted := time.Now()
	require.Equal(t, packets.CodeSuccess.Code, ack.ReasonCode)

	r := <-slow
	require.NoError(t, r.err)
	if r.ack.ReasonCode == 0 {
		require.True(t, r.kickedd, "n1's connection was never disconnected")
		require.Equal(t, packets.ErrSessionTakenOver.Code, r.reason)
		require.True(t, r.closed.Before(admitted), "n1's connection closed %s after n2 admitted the client", r.closed.Sub(admitted))
	}

	require.Eventually(t, func() bool {
		return !liveSession(nodes[0], "racer") && liveSession(nodes[1], "racer")
	}, waitFor, tick)
	waitOwner(t, nodes[:2], "racer", "n2")
}

func TestSessionFencedWhenLeaderPartitioned(t *testing.T) {
	// Slower raft timings, so that the cut off leader takes a while to step down.
	slow := func(name string, seeds ...string) Options {
		o := testOptions(name, seeds...)
		o.Raft.HeartbeatTimeout = time.Millisecond * 500
		o.Raft.ElectionTimeout = time.Millisecond * 500
		o.Raft.LeaderLeaseTimeout = time.Millisecond * 500
		return o
	}
	nodes := []*node{newNodeWithOptions(t, slow("n1"))}
	for _, name := range []string{"n2", "n3"} {
		nodes = append(nodes, newNodeWithOptions(t, slow(name, nodes[0].cluster.Addr())))
	}
	require.Eventually(t, func() bool {
		for _, n := range nodes {
			if !n.cluster.inContact() || raftVoters(n) != 3 {
				return false
			}
		}
		return nodes[0].cluster.Leader() == "n1"
	}, waitFor*2, tick)

	o := connectOptions{persistent: true, expiry: 60}
	old, _ := connectWith(t, nodes[0], "leaders", o)
	waitOwner(t, nodes, "leaders", "n1")

	// The leader is cut off, but keeps leading until raft notices.
	nodes[0].cluster.stream.partition()
	_ = nodes[0].cluster.ln.Close()

	var oldClosed time.Time
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = old.disconnected(waitFor * 2)
		oldClosed = time.Now()
	}()

	require.Eventually(t, func() bool { // a new leader is elected by the others
		l := nodes[1].cluster.Leader()
		return l != "" && l != "n1"
	}, waitFor, tick)

	connectWith(t, nodes[1], "leaders", o)
	admitted := time.Now()

	<-done
	require.True(t, oldClosed.Before(admitted), "old connection closed %s after the new one was admitted",
		oldClosed.Sub(admitted))
}

func TestSessionConcurrentLeaseWaitsOneWinner(t *testing.T) {
	nodes := newCluster(t, "n1", "n2", "n3", "n4")
	o := connectOptions{persistent: true, expiry: 60}

	first, _ := connectWith(t, nodes[2], "waiter", o)
	go func() { _, _ = first.disconnected(waitFor * 2) }()
	waitOwner(t, nodes, "waiter", "n3")

	// Requests to n3 hang, as if it were partitioned, so n2 waits for its request to time
	// out before waiting out n3's lease.
	addr := nodes[2].cluster.ln.Addr().String()
	_ = nodes[2].cluster.ln.Close()
	hole, err := net.Listen("tcp", addr)
	require.NoError(t, err)
	t.Cleanup(func() { _ = hole.Close() })
	go func() {
		for {
			conn, err := hole.Accept()
			if err != nil {
				return
			}
			t.Cleanup(func() { _ = conn.Close() })
		}
	}()

	// n4's view of the cluster is stale, so it thinks n2 is unreachable too.
	c4 := nodes[3].cluster
	c4.peersMu.Lock()
	c4.peers["n2"].stop()
	delete(c4.peers, "n2")
	c4.peersMu.Unlock()

	type result struct {
		c         *client
		ack       packets.Packet
		err       error
		connected time.Time
		closed    chan time.Time
	}
	dialAt := func(n *node) chan result {
		out := make(chan result, 1)
		go func() {
			c, ack, err := dial(t, n, "waiter", o)
			r := result{c: c, ack: ack, err: err, connected: time.Now(), closed: make(chan time.Time, 1)}
			out <- r
			if err == nil && ack.ReasonCode == 0 {
				_, _ = c.disconnected(waitFor * 2)
				r.closed <- time.Now()
			}
		}()
		return out
	}

	// Both admissions wait out a lease: n2's for n3, and n4's for n2. Whichever claim is
	// newer must win, and the other must never be live at the same time.
	on2 := dialAt(nodes[1])
	time.Sleep(time.Millisecond * 200)
	on4 := dialAt(nodes[3])
	r2, r4 := <-on2, <-on4
	require.NoError(t, r2.err)
	require.NoError(t, r4.err)

	ok2, ok4 := r2.ack.ReasonCode == 0, r4.ack.ReasonCode == 0
	require.True(t, ok2 || ok4, "neither node admitted the client")
	if ok2 && ok4 {
		first, second := r2, r4
		if r4.connected.Before(r2.connected) {
			first, second = r4, r2
		}
		select {
		case closed := <-first.closed:
			require.True(t, closed.Before(second.connected), "both nodes admitted the client, for %s", closed.Sub(second.connected))
		case <-time.After(waitFor):
			t.Fatal("both nodes admitted the client, and the first was never disconnected")
		}
	}

	require.Eventually(t, func() bool {
		return liveSession(nodes[1], "waiter") != liveSession(nodes[3], "waiter")
	}, waitFor, tick, "exactly one of n2 and n4 should hold the session")
}

// ghostClaim commits a claim for a node which never takes the session up, as happens when a
// claim is committed by a new leader after its claimant saw it fail.
func ghostClaim(t *testing.T, nodes []*node, id, owner string) {
	t.Helper()
	for _, n := range nodes {
		if n.cluster.raft.State() == raft.Leader {
			ctx, cancel := context.WithTimeout(context.Background(), waitFor)
			defer cancel()
			_, err := n.cluster.applyLocal(ctx, encodeClaim(id, owner))
			require.NoError(t, err)
			waitOwner(t, nodes, id, owner)
			return
		}
	}
	t.Fatal("no leader")
}

func TestSessionTakeoverLooksPastGhostClaim(t *testing.T) {
	nodes := newCluster(t, "n1", "n2", "n3")
	o := connectOptions{persistent: true, expiry: 60}

	holder, _ := connectWith(t, nodes[0], "ghosted", o)
	holder.subscribe("ghost/#", 1)
	kicked := make(chan byte, 1)
	go func() {
		code, _ := holder.disconnected(waitFor * 2)
		kicked <- code
	}()
	ghostClaim(t, nodes, "ghosted", "n3")

	// n2 asks n3, which doesn't hold the session, and then n1, which does.
	_, ack := connectWith(t, nodes[1], "ghosted", o)
	require.True(t, ack.SessionPresent)
	require.Equal(t, packets.ErrSessionTakenOver.Code, <-kicked)
	require.False(t, liveSession(nodes[0], "ghosted"))
}

func TestSessionGhostClaimDropsHolder(t *testing.T) {
	nodes := newCluster(t, "n1", "n2", "n3")
	holder, _ := connectWith(t, nodes[0], "ghosted", connectOptions{persistent: true, expiry: 60})
	go func() { _, _ = holder.disconnected(waitFor * 2) }()

	// A newer claim by another node, even one never taken up, supersedes n1's session.
	ghostClaim(t, nodes, "ghosted", "n3")
	require.Eventually(t, func() bool {
		_, ok := nodes[0].server.Clients.Get("ghosted")
		return !ok
	}, waitFor, tick)
}

func TestSessionGhostClaimHolderPartitioned(t *testing.T) {
	nodes := newCluster(t, "n1", "n2", "n3")
	o := connectOptions{persistent: true, expiry: 60}

	if nodes[0].cluster.Leader() == "n1" {
		require.NoError(t, nodes[0].cluster.raft.LeadershipTransfer().Error())
	}
	require.Eventually(t, func() bool {
		l := nodes[0].cluster.Leader()
		return l != "" && l != "n1" && nodes[0].cluster.inContact()
	}, waitFor, tick)

	holder, _ := connectWith(t, nodes[0], "ghosted", o)
	var closed time.Time
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = holder.disconnected(waitFor * 3)
		closed = time.Now()
	}()

	// n1 is cut off just as a claim it never sees is committed for n3.
	nodes[0].cluster.stream.partition()
	_ = nodes[0].cluster.ln.Close()
	ghostClaim(t, nodes[1:], "ghosted", "n3")

	// n2 finds n3 doesn't hold the session, and must wait for n1 to disconnect its client.
	connectWith(t, nodes[1], "ghosted", o)
	admitted := time.Now()
	<-done
	require.True(t, closed.Before(admitted), "n1's connection closed %s after n2 admitted the client", closed.Sub(admitted))
}

func TestSessionOrphansReconciled(t *testing.T) {
	nodes := newCluster(t, "n1", "n2")

	// A session left here without a claim, as by a superseded admission.
	_, err := nodes[0].server.RestoreSession(
		storage.Client{ID: "orphan", ProtocolVersion: 5, Properties: storage.ClientProperties{SessionExpiryInterval: 60}},
		[]storage.Subscription{{Client: "orphan", Filter: "o/#"}}, nil)
	require.NoError(t, err)
	ghostClaim(t, nodes, "orphan", "n2")

	nodes[0].cluster.reconcileSessions()
	_, ok := nodes[0].server.Clients.Get("orphan")
	require.False(t, ok)

	// Sessions without a claim elsewhere are kept.
	_, err = nodes[0].server.RestoreSession(storage.Client{ID: "kept", ProtocolVersion: 5}, nil, nil)
	require.NoError(t, err)
	nodes[0].cluster.reconcileSessions()
	_, ok = nodes[0].server.Clients.Get("kept")
	require.True(t, ok)
}

func TestSessionTakeoverLooksPastOwnGhostClaim(t *testing.T) {
	nodes := newCluster(t, "n1", "n2", "n3")
	o := connectOptions{persistent: true, expiry: 60}

	holder, _ := connectWith(t, nodes[1], "self-ghost", o)
	kicked := make(chan byte, 1)
	go func() {
		code, _ := holder.disconnected(waitFor * 2)
		kicked <- code
	}()

	// n3 made a claim which it never took up, then the client connects to n3.
	ghostClaim(t, nodes, "self-ghost", "n3")
	_, ack := connectWith(t, nodes[2], "self-ghost", o)
	admitted := time.Now()
	require.True(t, ack.SessionPresent, "n3 should have taken the session over from n2")

	select {
	case code := <-kicked:
		require.Equal(t, packets.ErrSessionTakenOver.Code, code)
	case <-time.After(waitFor):
		t.Fatal("n2 kept the session")
	}
	require.False(t, liveSession(nodes[1], "self-ghost"))
	_ = admitted
}
