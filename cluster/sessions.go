// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package cluster

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/raft"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

// session is a client session owned by this node, under a registry claim.
type session struct {
	client      *mqtt.Client
	epoch       atomic.Uint64 // zero until the claim is committed
	settled     chan struct{} // closed once the claim has been committed, or has failed
	ready       chan struct{} // closed once the session is established, or its admission fails
	settledOnce sync.Once
	readyOnce   sync.Once

	// Guarded by sessions.mu: superseded is set when a newer claim has been seen, so an
	// admission in progress must refuse its client, and admitted once an admission has
	// passed that check, so the session must be taken over once it is established.
	superseded bool
	admitted   bool
	replaced   *session // the session this admission replaced, restored if it fails
}

// setEpoch records the session's committed claim.
func (s *session) setEpoch(epoch uint64) {
	s.epoch.Store(epoch)
	s.settledOnce.Do(func() { close(s.settled) })
}

func (s *session) markReady() {
	s.settledOnce.Do(func() { close(s.settled) })
	s.readyOnce.Do(func() { close(s.ready) })
}

// waitSettled waits until the session's claim is known, or d has passed.
func (s *session) waitSettled(d time.Duration) {
	select {
	case <-s.settled:
	case <-time.After(d):
	}
}

// wait waits until the session is ready, or d has passed.
func (s *session) wait(d time.Duration) {
	select {
	case <-s.ready:
	case <-time.After(d):
	}
}

// sessions tracks the sessions owned by this node, by client id.
type sessions struct {
	mu sync.Mutex
	m  map[string]*session
}

// start records a session being admitted for a client, replacing any older session
// with the same id (which the server will take over locally).
func (s *sessions) start(cl *mqtt.Client) *session {
	return s.startID(cl.ID, cl)
}

// startID records a session being admitted for a client id. cl may be nil until the
// session's client is known.
func (s *sessions) startID(id string, cl *mqtt.Client) *session {
	e := &session{client: cl, settled: make(chan struct{}), ready: make(chan struct{})}
	s.mu.Lock()
	e.replaced = s.m[id]
	s.m[id] = e
	s.mu.Unlock()
	return e
}

// abandon forgets a session whose admission failed, restoring the session it replaced.
func (s *sessions) abandon(id string, e *session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m[id] != e {
		return
	}

	if e.replaced != nil {
		s.m[id] = e.replaced
		return
	}
	delete(s.m, id)
}

// startIfAbsent records a session being adopted for a client id, unless this node already
// has a session for it.
func (s *sessions) startIfAbsent(id string) (*session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[id]; ok {
		return nil, false
	}

	e := &session{settled: make(chan struct{}), ready: make(chan struct{})}
	s.m[id] = e
	return e, true
}

// setClient sets the client of a session, if it is still the current one for its id.
func (s *sessions) setClient(id string, e *session, cl *mqtt.Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m[id] == e {
		e.client = cl
	}
}

// get returns the current session for a client id.
func (s *sessions) get(id string) *session {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[id]
}

// forClient returns the current session for a client, if it belongs to that client.
func (s *sessions) forClient(cl *mqtt.Client) *session {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := s.m[cl.ID]; ok && e.client == cl && cl != nil {
		return e
	}
	return nil
}

// remove forgets a session, if it is still the current one for its id.
func (s *sessions) remove(id string, e *session) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m[id] == e {
		delete(s.m, id)
		return true
	}
	return false
}

// admit marks a session as admitted, unless it has been superseded or replaced.
func (s *sessions) admit(id string, e *session) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e.superseded || s.m[id] != e {
		return false
	}
	e.admitted = true
	return true
}

// supersede marks a session as superseded by a newer claim.
func (s *sessions) supersede(e *session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e.superseded = true
}

func (s *sessions) len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.m)
}

// OnSessionAdmit claims a connecting client's session in the registry, and takes the
// session over from its previous owner if that was another node.
func (c *Cluster) OnSessionAdmit(cl *mqtt.Client, pk packets.Packet) error {
	if cl.Net.Inline {
		return nil
	}

	if c.draining.Load() {
		c.metrics.connectionsRefused.Add(1)
		return packets.ErrUseAnotherServer
	}

	if c.raft == nil || !c.inContact() {
		c.metrics.connectionsRefused.Add(1)
		return packets.ErrServerUnavailable
	}

	e := c.sessions.start(cl)
	abort := func() {
		e.markReady()
		c.sessions.abandon(cl.ID, e)
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.opts.ClaimTimeout)
	defer cancel()

	res, err := c.claim(ctx, cl.ID)
	if err != nil {
		abort()
		c.metrics.claimFailures.Add(1)
		c.metrics.connectionsRefused.Add(1)
		c.log.Warn("refused connection: failed to claim session", "client", cl.ID, "error", err)
		return packets.ErrServerUnavailable
	}
	e.setEpoch(res.Epoch)
	c.metrics.claims.Add(1)

	_, heldHere := c.server.Clients.Get(cl.ID)
	if res.Prev.Node == "" || res.Prev.Node == c.opts.NodeName && heldHere {
		if !c.sessions.admit(cl.ID, e) {
			abort()
			c.metrics.connectionsRefused.Add(1)
			return packets.ErrServerUnavailable
		}
		return nil // a new session, or one the server will take over locally
	}

	if err := c.takeoverFrom(cl, e, res, pk.Connect.Clean); err != nil {
		return err
	}

	// A newer claim may have taken the session over while this one was being admitted.
	// Any session state already restored stays here, offline, for the newer claim to take.
	if !c.sessions.admit(cl.ID, e) {
		abort()
		c.metrics.connectionsRefused.Add(1)
		c.log.Warn("refused connection: session claimed again during admission", "client", cl.ID)
		return packets.ErrServerUnavailable
	}
	return nil
}

// takeoverFrom takes a connecting client's session over from its previous owner. If that
// node doesn't hold the session (its claim was committed after it gave up on it), the
// owners before it are tried in turn. If any of them are unreachable, the node waits out
// their lease before admitting the client.
func (c *Cluster) takeoverFrom(cl *mqtt.Client, e *session, res claimResult, clean bool) error {
	abort := func() {
		e.markReady()
		c.sessions.abandon(cl.ID, e)
	}

	var unreachable []string
	for _, prev := range append([]owner{res.Prev}, res.Older...) {
		if prev.Node == "" {
			continue
		}
		if prev.Node == c.opts.NodeName {
			if _, ok := c.server.Clients.Get(cl.ID); ok {
				break // the server takes over a session held here itself
			}
			continue // a claim made here which was never taken up
		}

		state, err := c.takeover(prev.Node, cl.ID, res.Epoch, clean)
		switch {
		case errors.Is(err, errNotHeld):
			continue

		case errors.Is(err, errStaleClaim):
			abort()
			c.metrics.connectionsRefused.Add(1)
			c.log.Warn("refused connection: session claimed again elsewhere", "client", cl.ID, "owner", prev.Node)
			return packets.ErrServerUnavailable

		case err != nil:
			unreachable = append(unreachable, prev.Node)
			c.log.Debug("session owner unreachable", "client", cl.ID, "owner", prev.Node, "error", err)
			continue
		}

		if state != nil && !clean {
			if _, err := c.server.RestoreSession(state.Client, state.Subscriptions, state.Inflight); err != nil {
				abort()
				c.metrics.connectionsRefused.Add(1)
				c.log.Warn("refused connection: failed to restore session", "client", cl.ID, "error", err)
				return packets.ErrServerUnavailable
			}
			c.metrics.sessionsTakenOver.Add(1)
			c.log.Debug("session taken over", "client", cl.ID, "from", prev.Node, "epoch", res.Epoch)
		}
		break
	}

	if len(unreachable) == 0 {
		return nil
	}

	// An unreachable owner may still have the client connected. It disconnects its clients
	// once it loses contact with the registry for the lease timeout, and drops sessions it
	// knows have been claimed elsewhere, so waiting out the lease ensures the client is never
	// connected to both nodes.
	c.metrics.ownerUnreachable.Add(1)
	c.log.Warn("session owner unreachable, waiting for its lease to expire",
		"client", cl.ID, "owners", unreachable, "wait", c.leaseWait())
	select {
	case <-time.After(c.leaseWait()):
	case <-c.done:
		abort()
		return packets.ErrServerUnavailable
	}

	// The owner may only have seemed unreachable, so make sure no newer claim has been
	// made meanwhile. This reads through the registry log, so it can't be stale.
	ctx, cancel := context.WithTimeout(context.Background(), c.opts.ClaimTimeout)
	current, err := c.confirm(ctx, cl.ID, res.Epoch)
	cancel()
	if err != nil || !current {
		abort()
		c.metrics.connectionsRefused.Add(1)
		c.log.Warn("refused connection: session claimed again while waiting", "client", cl.ID, "error", err)
		return packets.ErrServerUnavailable
	}
	return nil
}

// OnSessionEstablished marks a session as ready to be taken over by another node. If
// another node took the session over while the client was being admitted, the client is
// disconnected straight away.
func (c *Cluster) OnSessionEstablished(cl *mqtt.Client, pk packets.Packet) {
	if cl.Net.Inline {
		return
	}

	if e := c.sessions.forClient(cl); e != nil {
		e.markReady()
		return
	}

	if !cl.IsTakenOver() {
		c.log.Warn("session taken over during admission", "client", cl.ID)
		_ = c.server.DisconnectClient(cl, packets.ErrSessionTakenOver)
	}
}

// OnDisconnect releases the claim on a session which has ended.
func (c *Cluster) OnDisconnect(cl *mqtt.Client, err error, expire bool) {
	if expire && !cl.IsTakenOver() {
		c.endSession(cl)
	}
}

// OnClientExpired releases the claim on a session which has expired.
func (c *Cluster) OnClientExpired(cl *mqtt.Client) {
	c.endSession(cl)
}

// endSession forgets a client's session and releases its claim.
func (c *Cluster) endSession(cl *mqtt.Client) {
	e := c.sessions.forClient(cl)
	if e == nil || !c.sessions.remove(cl.ID, e) {
		return
	}

	e.markReady()
	if epoch := e.epoch.Load(); epoch > 0 {
		go c.release(cl.ID, epoch) // returns when the cluster stops
	}
}

// handleTakeover gives up a session to a node which has claimed it at epoch.
func (c *Cluster) handleTakeover(id string, epoch uint64, clean bool) (*sessionState, error) {
	if e := c.sessions.get(id); e != nil {
		// A connection may be being admitted here. If its claim is newer, the requester
		// has been superseded. Otherwise, let the admission finish so that its session is
		// taken over, rather than missed and established after the requester's.
		e.waitSettled(c.opts.ClaimTimeout * 2)
		if e.epoch.Load() > epoch {
			return nil, errStaleClaim
		}

		// An admission in progress now refuses its client; one already admitted is
		// taken over once established.
		c.sessions.supersede(e)
		select {
		case <-e.ready:
		case <-time.After(c.admissionWait()):
			c.log.Warn("taking over a session still being admitted", "client", id, "epoch", epoch)
		}
		c.sessions.remove(id, e)
	}

	cl, subs, inflight, ok := c.server.ExtractSession(id)
	if !ok {
		return nil, errNotHeld
	}
	c.metrics.sessionsHandedOver.Add(1)
	if clean {
		return nil, nil
	}

	c.log.Debug("session handed over", "client", id, "epoch", epoch)
	return &sessionState{Client: cl, Subscriptions: subs, Inflight: inflight}, nil
}

// onClaim is called by the registry when any claim is applied. If another node has
// claimed a session this node holds under an older claim, the session is dropped after a
// grace period, unless the other node has taken it over directly by then. This covers the
// cases where both nodes can reach the registry but not each other, and where the newer
// claim followed one which was never taken up.
func (c *Cluster) onClaim(id string, claim owner, prev owner) {
	if claim.Node == c.opts.NodeName {
		return
	}

	e := c.sessions.get(id)
	if e == nil {
		if _, ok := c.server.Clients.Get(id); ok {
			go c.dropOrphan(id) // a session held here without a claim
		}
		return
	}

	go func() {
		select {
		case <-time.After(c.opts.ClaimTimeout):
		case <-c.done:
			return
		}

		e.waitSettled(c.opts.ClaimTimeout * 2)
		if e.epoch.Load() >= claim.Epoch {
			return // reclaimed by this node
		}

		c.sessions.supersede(e)
		e.wait(c.admissionWait())
		if !c.sessions.remove(id, e) && c.sessions.get(id) != nil {
			return // taken over, or claimed again by this node
		}

		// Drop the session, including any state an abandoned admission left behind.
		if _, _, _, ok := c.server.ExtractSession(id); !ok {
			return
		}
		c.metrics.sessionsSuperseded.Add(1)
		c.log.Warn("dropped session claimed by another node", "client", id, "owner", claim.Node)
	}()
}

// lastContact returns the last time this node is known to have been in contact with a
// registry quorum: now if it is the leader (a leader steps down if it loses contact with a
// quorum), otherwise when it last heard from the leader.
func (c *Cluster) lastContact() time.Time {
	if c.raft == nil {
		return time.Time{}
	}

	seen := time.Now().UnixNano()
	if c.raft.State() != raft.Leader {
		seen = c.raft.LastContact().UnixNano()
	}

	for {
		prev := c.contact.Load()
		if seen <= prev || c.contact.CompareAndSwap(prev, seen) {
			return time.Unix(0, max(prev, seen))
		}
	}
}

// sweepSuperseded drops any local session which the registry shows another node has since
// claimed. It is called when the registry is restored from a snapshot, which doesn't apply
// the individual claims.
func (c *Cluster) sweepSuperseded() {
	c.sessions.mu.Lock()
	ids := make([]string, 0, len(c.sessions.m))
	for id := range c.sessions.m {
		ids = append(ids, id)
	}
	c.sessions.mu.Unlock()

	for _, id := range ids {
		if o, ok := c.registry.owner(id); ok && o.Node != c.opts.NodeName {
			c.onClaim(id, o, owner{Node: c.opts.NodeName})
		}
	}
}

// dropOrphan drops a session held here without a claim, after a grace period for it to be
// taken over directly, if the registry still shows another node owns it.
func (c *Cluster) dropOrphan(id string) {
	select {
	case <-time.After(c.opts.ClaimTimeout):
	case <-c.done:
		return
	}

	if c.sessions.get(id) != nil {
		return // admitted again here meanwhile
	}

	if o, ok := c.registry.owner(id); ok && o.Node != c.opts.NodeName {
		if _, _, _, ok := c.server.ExtractSession(id); ok {
			c.metrics.sessionsSuperseded.Add(1)
			c.log.Warn("dropped orphaned session claimed by another node", "client", id, "owner", o.Node)
		}
	}
}

// reconcileSessions drops any session held here without a claim, which the registry
// shows another node owns: state left behind by an admission which was superseded, or a
// session loaded from storage which has since moved.
func (c *Cluster) reconcileSessions() {
	for _, cl := range c.server.Clients.GetAll() {
		if cl.Net.Inline || c.sessions.get(cl.ID) != nil {
			continue
		}

		o, ok := c.registry.owner(cl.ID)
		if !ok || o.Node == c.opts.NodeName {
			continue
		}

		if _, _, _, ok := c.server.ExtractSession(cl.ID); ok {
			c.metrics.sessionsSuperseded.Add(1)
			c.log.Info("dropped orphaned session owned by another node", "client", cl.ID, "owner", o.Node)
		}
	}
}

// inContact returns true if this node can currently reach the registry leader.
func (c *Cluster) inContact() bool {
	if c.raft == nil {
		return false
	}

	if _, leader := c.raft.LeaderWithID(); leader == "" {
		return false
	}
	return time.Since(c.lastContact()) < c.opts.LeaseTimeout
}

// leaseCheckInterval is how often a node checks whether it is in contact with the registry.
func (c *Cluster) leaseCheckInterval() time.Duration {
	return c.opts.LeaseTimeout / 10
}

// admissionWait is the longest a connection can take to be admitted, once its claim is
// committed: taking the session over, or waiting out an unreachable owner's lease.
func (c *Cluster) admissionWait() time.Duration {
	return c.leaseWait() + c.opts.ClaimTimeout
}

// takeoverTimeout bounds a takeover request, which may wait for an admission in progress.
func (c *Cluster) takeoverTimeout() time.Duration {
	return c.opts.ClaimTimeout*3 + c.admissionWait()
}

// leaseWait is how long a node must wait before it can be sure an unreachable node has
// disconnected its clients. A node cut off while leading the registry counts itself in
// contact until it steps down, so that time is included too.
func (c *Cluster) leaseWait() time.Duration {
	return c.opts.LeaseTimeout + c.leaderLease + c.leaseCheckInterval()*2
}

// fenceLoop disconnects this node's clients whenever it has been out of contact with the
// registry for longer than the lease timeout, as other nodes may now own their sessions.
func (c *Cluster) fenceLoop() {
	defer c.wg.Done()
	t := time.NewTicker(c.leaseCheckInterval())
	defer t.Stop()

	for i := 0; ; i++ {
		select {
		case <-c.done:
			return
		case <-t.C:
		}

		if time.Since(c.lastContact()) <= c.opts.LeaseTimeout {
			if i%10 == 0 && c.inContact() {
				c.reconcileSessions()
			}
			continue
		}

		n := 0
		for _, cl := range c.server.Clients.GetAll() {
			if !cl.Net.Inline && !cl.Closed() {
				c.relocate(cl)
				n++
			}
		}
		c.metrics.clientsFenced.Add(int64(n))

		if n > 0 {
			c.log.Warn("lost contact with the session registry, disconnected clients", "clients", n)
		}
	}
}

// reconcileLoop keeps the raft configuration in step with gossip membership.
func (c *Cluster) reconcileLoop() {
	defer c.wg.Done()
	t := time.NewTicker(reconcileInterval)
	defer t.Stop()
	for {
		select {
		case <-c.done:
			return
		case <-t.C:
			c.reconcileRaft()
		}
	}
}
