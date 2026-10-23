// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package cluster

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
)

var errPartitioned = errors.New("raft stream partitioned")

// Registry commands, applied through raft.
const (
	cmdClaim   byte = iota + 1 // body: client id, node
	cmdRelease                 // body: client id, node, epoch
	cmdClaimIf                 // body: client id, node, expected owner
	cmdConfirm                 // body: client id, node, epoch
)

// owner records which node holds a client's session, and the epoch of its claim.
type owner struct {
	Node  string `json:"node"`
	Epoch uint64 `json:"epoch"`
}

// claimResult is the outcome of claiming a client's session.
type claimResult struct {
	Epoch uint64  // the new claim's epoch
	Prev  owner   // the previous owner, if any
	Older []owner // owners before the previous one, most recent first
}

// historyLength is how many previous owners of a session the registry remembers. A claim
// can be committed even though its claimant saw it fail (if the leader changed while it
// was being made), and then the claimant never held the session, so a takeover may need to
// look past it to the node which did.
const historyLength = 4

func encodeClaim(id, node string) []byte {
	b := appendString([]byte{cmdClaim}, id)
	return appendString(b, node)
}

// encodeClaimIf claims a session only if it is unowned, or owned by expected.
func encodeClaimIf(id, node, expected string) []byte {
	b := appendString([]byte{cmdClaimIf}, id)
	b = appendString(b, node)
	return appendString(b, expected)
}

// encodeConfirm checks, through the log, that node still owns a session at epoch.
func encodeConfirm(id, node string, epoch uint64) []byte {
	b := appendString([]byte{cmdConfirm}, id)
	b = appendString(b, node)
	return binary.AppendUvarint(b, epoch)
}

func encodeRelease(id, node string, epoch uint64) []byte {
	b := appendString([]byte{cmdRelease}, id)
	b = appendString(b, node)
	return binary.AppendUvarint(b, epoch)
}

// registry is a raft state machine recording which node owns each client's session.
// Each claim's epoch is its raft log index, so epochs increase strictly, and a node
// holding an older epoch for a session knows it has been superseded.
type registry struct {
	mu      sync.RWMutex
	owners  map[string]owner
	history map[string][]owner // previous owners, most recent first

	// onClaim is called whenever a claim is applied, and onRestore whenever the registry
	// is replaced by a snapshot. They must not block.
	onClaim   func(id string, claim owner, prev owner)
	onRestore func()
}

func newRegistry() *registry {
	return &registry{owners: map[string]owner{}, history: map[string][]owner{}}
}

// owner returns the current owner of a client's session.
func (r *registry) owner(id string) (owner, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	o, ok := r.owners[id]
	return o, ok
}

// Apply applies a registry command. A claim returns a claimResult, with a zero epoch if a
// conditional claim was refused, and a release returns true if the claim was released.
func (r *registry) Apply(l *raft.Log) any {
	if len(l.Data) == 0 {
		return nil
	}

	id, b, err := readString(l.Data[1:])
	if err != nil {
		return nil
	}

	node, b, err := readString(b)
	if err != nil {
		return nil
	}

	switch l.Data[0] {
	case cmdClaim, cmdClaimIf:
		claim := owner{Node: node, Epoch: l.Index}
		r.mu.Lock()
		prev := r.owners[id]
		if l.Data[0] == cmdClaimIf {
			expected, _, err := readString(b)
			if err != nil || prev.Node != "" && prev.Node != expected {
				r.mu.Unlock()
				return claimResult{Prev: prev} // the session has moved on
			}
		}
		older := r.history[id]
		if prev.Node != "" {
			h := append([]owner{prev}, older...)
			r.history[id] = h[:min(len(h), historyLength)]
		}
		r.owners[id] = claim
		r.mu.Unlock()

		if r.onClaim != nil {
			r.onClaim(id, claim, prev)
		}
		return claimResult{Epoch: claim.Epoch, Prev: prev, Older: append([]owner(nil), older...)}

	case cmdConfirm:
		epoch, n := binary.Uvarint(b)
		if n <= 0 {
			return false
		}

		r.mu.RLock()
		defer r.mu.RUnlock()
		cur, ok := r.owners[id]
		return ok && cur.Node == node && cur.Epoch == epoch

	case cmdRelease:
		epoch, n := binary.Uvarint(b)
		if n <= 0 {
			return false
		}

		r.mu.Lock()
		defer r.mu.Unlock()
		if cur, ok := r.owners[id]; ok && cur.Node == node && cur.Epoch == epoch {
			delete(r.owners, id)
			delete(r.history, id)
			return true
		}
		return false // the session has since been claimed again
	}

	return nil
}

// registryState is the registry as persisted in a snapshot.
type registryState struct {
	Owners  map[string]owner   `json:"owners"`
	History map[string][]owner `json:"history,omitempty"`
}

// Snapshot returns a copy of the registry for raft to persist.
func (r *registry) Snapshot() (raft.FSMSnapshot, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	state := registryState{
		Owners:  make(map[string]owner, len(r.owners)),
		History: make(map[string][]owner, len(r.history)),
	}
	for id, o := range r.owners {
		state.Owners[id] = o
	}
	for id, h := range r.history {
		state.History[id] = append([]owner(nil), h...)
	}
	return &registrySnapshot{state: state}, nil
}

// Restore replaces the registry with a snapshot.
func (r *registry) Restore(rc io.ReadCloser) error {
	defer rc.Close()
	var state registryState
	if err := json.NewDecoder(rc).Decode(&state); err != nil {
		return err
	}
	if state.Owners == nil {
		state.Owners = map[string]owner{}
	}
	if state.History == nil {
		state.History = map[string][]owner{}
	}

	r.mu.Lock()
	r.owners = state.Owners
	r.history = state.History
	r.mu.Unlock()

	if r.onRestore != nil {
		r.onRestore()
	}
	return nil
}

type registrySnapshot struct {
	state registryState
}

func (s *registrySnapshot) Persist(sink raft.SnapshotSink) error {
	if err := json.NewEncoder(sink).Encode(s.state); err != nil {
		_ = sink.Cancel()
		return err
	}
	return sink.Close()
}

func (s *registrySnapshot) Release() {}

// raftStream is a raft transport stream layer over a TCP listener, which advertises an
// address other nodes can reach (the listener may be bound to all interfaces). It can be
// partitioned, cutting the node off from raft traffic, for testing.
type raftStream struct {
	net.Listener
	advertise net.Addr

	mu          sync.Mutex
	partitioned bool
	conns       map[net.Conn]struct{}
}

func (s *raftStream) Addr() net.Addr {
	return s.advertise
}

func (s *raftStream) Accept() (net.Conn, error) {
	for {
		conn, err := s.Listener.Accept()
		if err != nil || s.track(conn) {
			return conn, err
		}
	}
}

func (s *raftStream) Dial(addr raft.ServerAddress, timeout time.Duration) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", string(addr), timeout)
	if err != nil {
		return nil, err
	}

	if !s.track(conn) {
		return nil, errPartitioned
	}
	return conn, nil
}

// track records a connection, or closes it and returns false if the stream is partitioned.
func (s *raftStream) track(conn net.Conn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.partitioned {
		_ = conn.Close()
		return false
	}

	if s.conns == nil {
		s.conns = map[net.Conn]struct{}{}
	}
	s.conns[conn] = struct{}{}
	return true
}

// partition closes all raft connections, and refuses new ones.
func (s *raftStream) partition() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.partitioned = true
	for conn := range s.conns {
		_ = conn.Close()
	}
	s.conns = nil
}

// newRaftLogger returns an hclog logger which writes to a structured logger. Raft's
// routine info messages (elections, configuration changes) are logged at debug level.
func newRaftLogger(log *slog.Logger) hclog.Logger {
	return hclog.New(&hclog.LoggerOptions{
		Name:        "raft",
		Level:       hclog.Info,
		Output:      &raftLog{log: log},
		DisableTime: true,
	})
}

type raftLog struct {
	log *slog.Logger
}

func (w *raftLog) Write(b []byte) (int, error) {
	msg := strings.TrimSpace(string(b))
	switch {
	case strings.HasPrefix(msg, "[ERROR]"):
		w.log.Error(msg)
	case strings.HasPrefix(msg, "[WARN]"):
		w.log.Warn(msg)
	default:
		w.log.Debug(msg)
	}
	return len(b), nil
}

// reconcileRaft makes the raft configuration match the gossip membership: nodes which
// have joined are added as voters, and nodes which have left or failed are removed.
// Only the leader can change the configuration, so other nodes do nothing.
func (c *Cluster) reconcileRaft() {
	if c.raft.State() != raft.Leader {
		return
	}

	future := c.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		return
	}

	servers := map[raft.ServerID]raft.ServerAddress{}
	for _, s := range future.Configuration().Servers {
		servers[s.ID] = s.Address
	}

	alive := map[raft.ServerID]raft.ServerAddress{}
	for name, m := range c.memberSnapshot() {
		if !m.ok {
			continue
		}
		alive[raft.ServerID(name)] = raft.ServerAddress(net.JoinHostPort(m.addr, itoa(m.meta.Raft)))
	}

	for id, addr := range alive {
		if cur, ok := servers[id]; ok && cur == addr {
			continue
		}

		if err := c.raft.AddVoter(id, addr, 0, c.opts.ClaimTimeout).Error(); err != nil {
			c.log.Warn("failed to add raft voter", "peer", id, "error", err)
			continue
		}
		c.log.Info("added raft voter", "peer", id, "addr", addr)
	}

	for id := range servers {
		if _, ok := alive[id]; ok || string(id) == c.opts.NodeName {
			continue
		}

		if err := c.raft.RemoveServer(id, 0, c.opts.ClaimTimeout).Error(); err != nil {
			c.log.Warn("failed to remove raft voter", "peer", id, "error", err)
			continue
		}
		c.log.Info("removed raft voter", "peer", id)
	}
}
