// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package cluster

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/hashicorp/raft"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

var (
	ErrDraining     = errors.New("cluster node is draining")
	ErrNoDrainPeers = errors.New("no other cluster node can take over sessions")
)

// DrainOptions configures draining a node.
type DrainOptions struct {
	// Rate is the number of clients disconnected per second, so that they don't all
	// reconnect to other nodes at once. Zero disconnects every client immediately.
	Rate int
}

// DrainResult reports the outcome of draining a node.
type DrainResult struct {
	Disconnected int // clients disconnected
	Migrated     int // offline sessions moved to other nodes
	Failed       int // offline sessions which could not be moved, and remain on this node
}

// Drain prepares the node to be stopped without losing sessions. The node refuses new
// connections (with reason code 0x9C, use another server), hands over registry leadership,
// and disconnects its clients so they reconnect to other nodes, which take their sessions
// over. Clients are disconnected without sending their will messages, as they haven't
// failed. Persistent sessions left offline on the node are then moved to other nodes.
// Stop the server once Drain returns; the node keeps forwarding messages until then.
func (c *Cluster) Drain(ctx context.Context, opts DrainOptions) (DrainResult, error) {
	var res DrainResult
	if !c.started.Load() || c.raft == nil {
		return res, ErrNoQuorum
	}

	c.draining.Store(true)
	if err := c.ml.UpdateNode(c.opts.WriteTimeout); err != nil { // tell other nodes not to send sessions here
		c.log.Warn("failed to announce draining", "error", err)
	}

	if c.raft.State() == raft.Leader {
		if err := c.raft.LeadershipTransfer().Error(); err != nil {
			c.log.Warn("failed to transfer registry leadership", "error", err)
		}
	}

	c.log.Info("draining cluster node")

	var interval time.Duration
	if opts.Rate > 0 {
		interval = time.Second / time.Duration(opts.Rate)
	}

	for _, cl := range c.server.Clients.GetAll() {
		if cl.Net.Inline || cl.Closed() {
			continue
		}

		if interval > 0 && res.Disconnected > 0 {
			select {
			case <-ctx.Done():
				return res, ctx.Err()
			case <-time.After(interval):
			}
		}

		c.relocate(cl)
		res.Disconnected++
		c.metrics.drainDisconnected.Add(1)
	}

	// Wait for the disconnected clients' sessions to be either ended or left offline.
	for !c.allClientsClosed() {
		select {
		case <-ctx.Done():
			return res, ctx.Err()
		case <-time.After(time.Millisecond * 20):
		}
	}

	var offline []string
	for _, cl := range c.server.Clients.GetAll() {
		if !cl.Net.Inline && persistent(cl) {
			offline = append(offline, cl.ID)
		}
	}

	targets := c.drainTargets()
	if len(offline) > 0 && len(targets) == 0 {
		res.Failed = len(offline)
		return res, ErrNoDrainPeers
	}

	for i, id := range offline {
		if ctx.Err() != nil {
			res.Failed += len(offline) - i
			return res, ctx.Err()
		}

		// Try each peer in turn, as one may be unable to claim sessions (just restarted, say).
		var err error
		for attempt := range targets {
			if err = c.migrate(ctx, targets[(i+attempt)%len(targets)], id); err == nil || ctx.Err() != nil {
				break
			}
		}
		if err != nil {
			c.log.Warn("failed to move session while draining", "client", id, "error", err)
			res.Failed++
			c.metrics.drainMigrateFailure.Add(1)
			continue
		}
		res.Migrated++
		c.metrics.drainMigrated.Add(1)
	}

	c.log.Info("drained cluster node", "disconnected", res.Disconnected, "migrated", res.Migrated, "failed", res.Failed)
	return res, nil
}

// Draining returns true if the node is draining.
func (c *Cluster) Draining() bool {
	return c.draining.Load()
}

// relocate disconnects a healthy client so that it reconnects to another node. The will
// message is cleared first, as the client hasn't failed.
func (c *Cluster) relocate(cl *mqtt.Client) {
	atomic.StoreUint32(&cl.Properties.Will.Flag, 0)
	_ = c.server.DisconnectClient(cl, packets.ErrUseAnotherServer)
}

// persistent returns true if a client's session outlives its connection.
func persistent(cl *mqtt.Client) bool {
	if cl.Properties.ProtocolVersion == 5 {
		return cl.Properties.Props.SessionExpiryInterval > 0
	}
	return !cl.Properties.Clean
}

func (c *Cluster) allClientsClosed() bool {
	for _, cl := range c.server.Clients.GetAll() {
		if !cl.Net.Inline && !cl.Closed() {
			return false
		}
	}
	return true
}

// drainTargets returns the peers which can take over sessions.
func (c *Cluster) drainTargets() []string {
	c.peersMu.RLock()
	defer c.peersMu.RUnlock()
	var names []string
	for name, p := range c.peers {
		if !p.draining.Load() {
			names = append(names, name)
		}
	}
	return names
}

// migrate asks a peer to take over an offline session, if it is still here and offline.
func (c *Cluster) migrate(ctx context.Context, peer, id string) error {
	if cl, ok := c.server.Clients.Get(id); !ok || !cl.Closed() {
		return nil // the client has reconnected, and the session moved with it
	}

	addr := c.peerAddr(peer)
	if addr == "" {
		return errPeerUnknown
	}

	ctx, cancel := context.WithTimeout(ctx, c.opts.ClaimTimeout*2+c.takeoverTimeout())
	defer cancel()
	body := appendString(nil, id)
	body = appendString(body, c.opts.NodeName)
	_, err := c.call(ctx, addr, frameAdoptRequest, body)
	return err
}

// handleAdopt takes over an offline session from a draining node. The session is only
// claimed if the draining node still owns it (or nobody does), so a session which has
// moved on with its client is left alone.
func (c *Cluster) handleAdopt(id, from string) error {
	if c.draining.Load() {
		return ErrDraining
	}

	if _, ok := c.server.Clients.Get(id); ok {
		return nil // the session is already here
	}

	e, ok := c.sessions.startIfAbsent(id)
	if !ok {
		return nil // a connection for the session is being admitted here
	}
	abort := func() {
		e.markReady()
		c.sessions.remove(id, e)
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.opts.ClaimTimeout)
	defer cancel()

	res, err := c.claimIf(ctx, id, from)
	if err != nil {
		abort()
		return err
	}

	if res.Epoch == 0 {
		abort()
		return nil // the session has moved on
	}
	e.setEpoch(res.Epoch)
	c.metrics.claims.Add(1)

	release := func() {
		abort()
		go c.release(id, res.Epoch)
	}

	if res.Prev.Node == c.opts.NodeName {
		release() // this node held the claim, but no longer has the session
		return nil
	}

	state, err := c.takeover(from, id, res.Epoch, false)
	if errors.Is(err, errNotHeld) {
		err = nil // the session has gone
	}
	if err != nil || state == nil {
		release()
		return err
	}

	cl, err := c.server.RestoreSession(state.Client, state.Subscriptions, state.Inflight)
	if errors.Is(err, mqtt.ErrSessionActive) {
		abort() // the client has connected here meanwhile, under a newer claim
		return nil
	}
	if err != nil {
		release()
		return err
	}

	c.sessions.setClient(id, e, cl)
	e.markReady()
	c.metrics.sessionsTakenOver.Add(1)
	return nil
}

// OnPacketRead refuses packets from clients while the node is out of contact with the
// registry for longer than the lease, disconnecting them. This closes the gap between the
// lease expiring and the fence loop disconnecting clients, for example after the process
// has been paused.
func (c *Cluster) OnPacketRead(cl *mqtt.Client, pk packets.Packet) (packets.Packet, error) {
	if cl.Net.Inline || pk.FixedHeader.Type == packets.Connect || // connections are refused when admitted
		time.Since(c.lastContact()) <= c.opts.LeaseTimeout {
		return pk, nil
	}

	c.relocate(cl)
	c.metrics.clientsFenced.Add(1)
	return pk, packets.ErrRejectPacket
}
