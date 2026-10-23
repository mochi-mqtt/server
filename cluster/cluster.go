// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

// Package cluster joins multiple mochi mqtt servers into a single logical broker.
//
// Nodes discover each other and detect failures using gossip (hashicorp/memberlist).
// Each node announces the topic filters its clients subscribe to, and a publish is
// forwarded once to every other node with matching subscribers, over a persistent
// TCP link per peer. Shared subscription groups receive each message once across the
// cluster: a local group member is preferred, otherwise one node with a member is
// chosen at random.
//
// Session ownership is strongly consistent. A raft-replicated registry records which
// node owns each client id. When a client connects, its node claims the id, and if
// another node owned it, takes the session (subscriptions and inflight messages) over
// from that node, so a client can reconnect to any node. A client is never connected to
// two nodes at once: nodes which lose contact with a registry quorum disconnect their
// clients, and new connections are refused until contact is restored.
//
// Retained messages are replicated to every node. Each change is versioned by the time
// and node it was published on, and every node keeps the latest version, so concurrent
// changes to a topic's retained message converge.
//
// Routes are eventually consistent: a message published immediately after a client
// subscribes on another node may not reach that client. When a peer can't keep up and
// its queue is full, qos 0 publishes are dropped, while qos 1 and 2 publishes wait for
// room, which slows their publishers down. Forwarding is otherwise best effort: messages
// are lost if a link fails mid-write, and dropped if a peer is unreachable.
package cluster

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/raft"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

const (
	defaultForwardAddr   = ":7947"
	defaultRaftAddr      = ":7948"
	defaultPeerQueueSize = 8192
	defaultDialTimeout   = time.Second * 5
	defaultWriteTimeout  = time.Second * 10
	defaultClaimTimeout  = time.Second * 5
	defaultLeaseTimeout  = time.Second * 10
	leaveTimeout         = time.Second * 5
	reconcileInterval    = time.Millisecond * 500
)

var (
	ErrNodeNameRequired = errors.New("cluster node name is required")
	ErrAlreadyStarted   = errors.New("cluster already started")
	ErrJoinFailed       = errors.New("failed to join any cluster seed")
)

// Options contains the configuration for a cluster node.
type Options struct {
	// NodeName uniquely identifies this node in the cluster. Required.
	NodeName string

	// BindAddr and BindPort are the address and port used for gossip. If zero, the
	// values from Memberlist are used (by default 0.0.0.0:7946).
	BindAddr string
	BindPort int

	// AdvertiseAddr and AdvertisePort are the gossip address and port announced to other
	// nodes, if different from the bind address. Other nodes also connect to the
	// advertised address to open peer links and for raft.
	AdvertiseAddr string
	AdvertisePort int

	// ForwardAddr is the address to listen on for peer links. Defaults to ":7947". Use
	// port 0 to listen on any free port.
	ForwardAddr string

	// RaftAddr is the address to listen on for the session registry's raft traffic.
	// Defaults to ":7948". Use port 0 to listen on any free port.
	RaftAddr string

	// Seeds are the gossip addresses ("host:port") of existing nodes to join. A node
	// started without seeds bootstraps a new cluster, so only the very first node should
	// be started without them; restart it with seeds of the other nodes.
	Seeds []string

	// Bootstrap creates a new cluster if none of the Seeds can be joined, rather than
	// failing to start. Set it on the first node only, so that the first node can be given
	// the other nodes as seeds and restarted with the same configuration. A node started
	// with Bootstrap while partitioned from the rest of the cluster forms a second cluster.
	Bootstrap bool

	// PeerQueueSize is the number of frames which may wait to be sent to each peer. When
	// it's full, forwarded qos 0 publishes are dropped, and qos 1 and 2 publishes wait for
	// room, for up to WriteTimeout. Defaults to 8192.
	PeerQueueSize int

	// DialTimeout and WriteTimeout bound connecting to and writing to a peer link.
	DialTimeout  time.Duration
	WriteTimeout time.Duration

	// ClaimTimeout bounds claiming a session in the registry, and taking it over from
	// another node, when a client connects. Defaults to 5 seconds.
	ClaimTimeout time.Duration

	// LeaseTimeout is how long a node may be out of contact with a registry quorum before
	// it disconnects its clients. A client reconnecting to another node while its previous
	// node is unreachable waits slightly longer than this. Defaults to 10 seconds, and is
	// raised to at least twice ClaimTimeout.
	LeaseTimeout time.Duration

	// RetainedTombstoneTTL is how long a node remembers that a retained message was
	// deleted, so that an older copy from a node which missed the deletion can't restore
	// it. A node which rejoins after longer than this may restore deleted retained
	// messages. Defaults to 24 hours.
	RetainedTombstoneTTL time.Duration

	// Memberlist is an optional base gossip configuration. Defaults to
	// memberlist.DefaultLANConfig(). Its Name, Delegate, Events and Logger are overwritten.
	Memberlist *memberlist.Config

	// Raft is an optional base raft configuration. Defaults to raft.DefaultConfig(). Its
	// LocalID and Logger are overwritten.
	Raft *raft.Config

	// RaftLogStore, RaftStableStore and RaftSnapshotStore persist the session registry.
	// They default to in-memory stores, in which case a node rejoining the cluster recovers
	// the registry from the other nodes, but the registry is lost if every node restarts.
	RaftLogStore      raft.LogStore
	RaftStableStore   raft.StableStore
	RaftSnapshotStore raft.SnapshotStore
}

func (o *Options) ensureDefaults() {
	if o.ForwardAddr == "" {
		o.ForwardAddr = defaultForwardAddr
	}

	if o.RaftAddr == "" {
		o.RaftAddr = defaultRaftAddr
	}

	if o.PeerQueueSize <= 0 {
		o.PeerQueueSize = defaultPeerQueueSize
	}

	if o.DialTimeout <= 0 {
		o.DialTimeout = defaultDialTimeout
	}

	if o.WriteTimeout <= 0 {
		o.WriteTimeout = defaultWriteTimeout
	}

	if o.ClaimTimeout <= 0 {
		o.ClaimTimeout = defaultClaimTimeout
	}

	if o.LeaseTimeout <= 0 {
		o.LeaseTimeout = defaultLeaseTimeout
	}

	if o.RetainedTombstoneTTL <= 0 {
		o.RetainedTombstoneTTL = defaultTombstoneTTL
	}

	if o.LeaseTimeout < o.ClaimTimeout*2 { // a superseded node must drop its sessions within the lease
		o.LeaseTimeout = o.ClaimTimeout * 2
	}
}

// Cluster is a server hook which connects the server to other nodes in a cluster.
type Cluster struct {
	mqtt.HookBase
	opts   Options
	server *mqtt.Server
	log    *slog.Logger

	localMu sync.Mutex // guards local, and orders route deltas with snapshots
	local   localRoutes

	remote *remoteRoutes

	peersMu sync.RWMutex
	peers   map[string]*peer

	membersMu sync.Mutex
	members   map[string]member // gossip members, including this node

	inboundMu sync.Mutex
	inbound   map[string]net.Conn

	retainMu    sync.Mutex // guards retained, and orders changes to the retained messages
	retained    map[string]retainEntry
	retainClock atomic.Int64

	registry    *registry
	sessions    *sessions
	raft        *raft.Raft
	leaderLease time.Duration // how long a leader cut off from a quorum may keep leading
	stream      *raftStream
	contact     atomic.Int64 // unix nanoseconds of the last known contact with a registry quorum

	ml          *memberlist.Memberlist
	ln          net.Listener
	raftLn      net.Listener
	forwardPort int
	raftPort    int
	done        chan struct{}
	started     atomic.Bool
	stopping    atomic.Bool
	gossipDown  atomic.Bool // memberlist panics if it leaves after shutting down
	stopOnce    sync.Once
	wg          sync.WaitGroup

	draining atomic.Bool
	metrics  metrics
}

// New returns a new cluster hook for a server. Add it to the server with AddHook,
// then call Start to join the cluster.
func New(server *mqtt.Server, opts Options) (*Cluster, error) {
	if opts.NodeName == "" {
		return nil, ErrNodeNameRequired
	}

	opts.ensureDefaults()
	c := &Cluster{
		opts:     opts,
		server:   server,
		log:      server.Log.With("node", opts.NodeName),
		local:    localRoutes{},
		remote:   newRemoteRoutes(),
		peers:    map[string]*peer{},
		members:  map[string]member{},
		inbound:  map[string]net.Conn{},
		retained: map[string]retainEntry{},
		registry: newRegistry(),
		sessions: &sessions{m: map[string]*session{}},
		done:     make(chan struct{}),
	}
	c.registry.onClaim = c.onClaim
	c.registry.onRestore = c.sweepSuperseded
	return c, nil
}

// ID returns the ID of the hook.
func (c *Cluster) ID() string {
	return "cluster"
}

// Provides indicates which hook methods this hook provides.
func (c *Cluster) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqtt.OnSessionAdmit,
		mqtt.OnPacketRead,
		mqtt.OnSessionEstablished,
		mqtt.OnDisconnect,
		mqtt.OnClientExpired,
		mqtt.OnSubscribed,
		mqtt.OnUnsubscribed,
		mqtt.OnPublished,
		mqtt.OnWillSent,
		mqtt.OnRetainMessage,
	}, []byte{b})
}

// Init initializes the hook. The hook is configured by New, so config is ignored.
func (c *Cluster) Init(config any) error {
	return nil
}

// Name returns the name of this node.
func (c *Cluster) Name() string {
	return c.opts.NodeName
}

// Start listens for peer links, starts the session registry, and joins the cluster.
// Clients are refused until the node is in contact with a registry quorum.
func (c *Cluster) Start() error {
	if !c.started.CompareAndSwap(false, true) {
		return ErrAlreadyStarted
	}

	if err := c.start(); err != nil {
		_ = c.Stop()
		return err
	}

	c.log.Info("cluster node started", "gossip", c.ml.LocalNode().Address(),
		"forward", c.ln.Addr().String(), "raft", c.raftLn.Addr().String())
	return nil
}

func (c *Cluster) start() error {
	ln, err := net.Listen("tcp", c.opts.ForwardAddr)
	if err != nil {
		return fmt.Errorf("cluster listen: %w", err)
	}
	c.ln = ln
	c.forwardPort = ln.Addr().(*net.TCPAddr).Port

	c.wg.Add(1)
	go c.acceptLoop()

	raftLn, err := net.Listen("tcp", c.opts.RaftAddr)
	if err != nil {
		return fmt.Errorf("cluster raft listen: %w", err)
	}
	c.raftLn = raftLn
	c.raftPort = raftLn.Addr().(*net.TCPAddr).Port

	mc := c.opts.Memberlist
	if mc == nil {
		mc = memberlist.DefaultLANConfig()
	}

	mc.Name = c.opts.NodeName
	if c.opts.BindAddr != "" {
		mc.BindAddr = c.opts.BindAddr
	}
	if c.opts.BindPort != 0 {
		mc.BindPort = c.opts.BindPort
	}
	if c.opts.AdvertiseAddr != "" {
		mc.AdvertiseAddr = c.opts.AdvertiseAddr
	}
	if c.opts.AdvertisePort != 0 {
		mc.AdvertisePort = c.opts.AdvertisePort
	}
	mc.Delegate = &delegate{c: c}
	mc.Events = &events{c: c}
	mc.LogOutput = nil
	mc.Logger = log.New(&memberlistLog{log: c.log}, "", 0)

	ml, err := memberlist.Create(mc)
	if err != nil {
		return fmt.Errorf("cluster gossip: %w", err)
	}
	c.ml = ml

	if err := c.startRaft(); err != nil {
		return err
	}

	c.wg.Add(3)
	go c.reconcileLoop()
	go c.fenceLoop()
	go c.tombstoneLoop()

	if len(c.opts.Seeds) == 0 {
		return c.bootstrap()
	}

	if _, err := ml.Join(c.opts.Seeds); err != nil {
		if !c.opts.Bootstrap {
			return fmt.Errorf("%w: %w", ErrJoinFailed, err)
		}
		c.log.Warn("no cluster seeds reachable, bootstrapping a new cluster", "seeds", c.opts.Seeds, "error", err)
		return c.bootstrap()
	}

	return nil
}

// bootstrap makes this node the only voter of a new registry.
func (c *Cluster) bootstrap() error {
	err := c.raft.BootstrapCluster(raft.Configuration{Servers: []raft.Server{{
		Suffrage: raft.Voter,
		ID:       raft.ServerID(c.opts.NodeName),
		Address:  raft.ServerAddress(c.stream.Addr().String()),
	}}}).Error()
	if err != nil && !errors.Is(err, raft.ErrCantBootstrap) {
		return fmt.Errorf("cluster raft bootstrap: %w", err)
	}
	return nil
}

// startRaft starts the session registry. Raft advertises the same address as gossip.
func (c *Cluster) startRaft() error {
	rc := c.opts.Raft
	if rc == nil {
		rc = raft.DefaultConfig()
	}
	rc.LocalID = raft.ServerID(c.opts.NodeName)
	rc.Logger = newRaftLogger(c.log)
	c.leaderLease = rc.LeaderLeaseTimeout * 2 // raft checks its lease once per LeaderLeaseTimeout

	c.stream = &raftStream{
		Listener:  c.raftLn,
		advertise: &net.TCPAddr{IP: c.ml.LocalNode().Addr, Port: c.raftPort},
	}
	transport := raft.NewNetworkTransportWithConfig(&raft.NetworkTransportConfig{
		Stream:  c.stream,
		MaxPool: 3,
		Timeout: c.opts.WriteTimeout,
		Logger:  rc.Logger,
	})

	logs, stable, snaps := c.opts.RaftLogStore, c.opts.RaftStableStore, c.opts.RaftSnapshotStore
	if logs == nil || stable == nil {
		store := raft.NewInmemStore()
		logs, stable = store, store
	}
	if snaps == nil {
		snaps = raft.NewInmemSnapshotStore()
	}

	r, err := raft.NewRaft(rc, c.registry, logs, stable, snaps, transport)
	if err != nil {
		_ = transport.Close()
		return fmt.Errorf("cluster raft: %w", err)
	}
	c.raft = r
	return nil
}

// Stop leaves the cluster and closes all peer links. It is called automatically when
// the server is closed.
func (c *Cluster) Stop() error {
	if !c.started.Load() {
		return nil
	}

	c.stopOnce.Do(func() {
		c.stopping.Store(true)
		close(c.done)

		// Hand over leadership and leave gossip first, so the leader removes this node from
		// the registry before its raft stops answering.
		if c.raft != nil && c.raft.State() == raft.Leader {
			_ = c.raft.LeadershipTransfer().Error()
		}

		if c.ml != nil && c.gossipDown.CompareAndSwap(false, true) {
			_ = c.ml.Leave(leaveTimeout)
			c.awaitRaftRemoval(leaveTimeout)
			_ = c.ml.Shutdown()
		}

		if c.raft != nil {
			_ = c.raft.Shutdown().Error()
		}

		if c.ln != nil {
			_ = c.ln.Close()
		}
		if c.raftLn != nil {
			_ = c.raftLn.Close()
		}

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

		c.wg.Wait()
		c.log.Info("cluster node stopped")
	})

	return nil
}

// awaitRaftRemoval waits until another node, as leader, has removed this node from the
// raft configuration.
func (c *Cluster) awaitRaftRemoval(timeout time.Duration) {
	if c.raft == nil {
		return
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, leader := c.raft.LeaderWithID(); leader == "" || string(leader) == c.opts.NodeName {
			return // nobody else can remove this node
		}

		f := c.raft.GetConfiguration()
		if f.Error() != nil {
			return
		}

		member := false
		for _, s := range f.Configuration().Servers {
			if string(s.ID) == c.opts.NodeName {
				member = true
			}
		}
		if !member {
			return
		}

		time.Sleep(time.Millisecond * 50)
	}
}

// Leader returns the name of the current session registry leader, or "" if there is none.
func (c *Cluster) Leader() string {
	if c.raft == nil {
		return ""
	}
	_, id := c.raft.LeaderWithID()
	return string(id)
}

// Addr returns this node's gossip address, for other nodes to use as a seed.
func (c *Cluster) Addr() string {
	if c.ml == nil {
		return ""
	}
	return c.ml.LocalNode().Address()
}

// Members returns the names of the nodes currently in the cluster, including this one.
func (c *Cluster) Members() []string {
	if c.ml == nil {
		return nil
	}

	nodes := c.ml.Members()
	names := make([]string, 0, len(nodes))
	for _, n := range nodes {
		names = append(names, n.Name)
	}
	return names
}

// OnSubscribed announces any filters which have gained their first local subscriber.
func (c *Cluster) OnSubscribed(cl *mqtt.Client, pk packets.Packet, reasonCodes []byte) {
	for i, sub := range pk.Filters {
		if i < len(reasonCodes) && reasonCodes[i] >= packets.ErrUnspecifiedError.Code {
			continue // the subscription was refused
		}
		c.addLocalRoute(sub.Filter, subscriberKey(cl, sub))
	}
}

// OnUnsubscribed withdraws any filters which have lost their last local subscriber.
func (c *Cluster) OnUnsubscribed(cl *mqtt.Client, pk packets.Packet) {
	for _, sub := range pk.Filters {
		c.removeLocalRoute(sub.Filter, subscriberKey(cl, sub))
	}
}

// OnPublished forwards a message published to this node to other nodes with subscribers.
func (c *Cluster) OnPublished(cl *mqtt.Client, pk packets.Packet) {
	c.forward(pk)
}

// OnWillSent forwards a will message, which is published without OnPublished.
func (c *Cluster) OnWillSent(cl *mqtt.Client, pk packets.Packet) {
	c.forward(pk)
}

// subscriberKey identifies a local subscriber of a filter. Inline subscriptions all
// belong to the same client, so are told apart by their subscription identifiers.
func subscriberKey(cl *mqtt.Client, sub packets.Subscription) string {
	if cl.Net.Inline {
		return cl.ID + "#" + strconv.Itoa(sub.Identifier)
	}
	return cl.ID
}

// addLocalRoute adds a local subscriber to a filter, announcing the route if it is new.
func (c *Cluster) addLocalRoute(filter, key string) {
	if strings.HasPrefix(filter, mqtt.SysPrefix) {
		return // $SYS topics are never forwarded
	}

	c.localMu.Lock()
	defer c.localMu.Unlock()
	if c.local.add(filter, key) {
		c.broadcastRoute(encodeFrame(frameRouteAdd, []byte(filter)))
	}
}

// removeLocalRoute removes a local subscriber from a filter, withdrawing the route if
// it has no subscribers left.
func (c *Cluster) removeLocalRoute(filter, key string) {
	c.localMu.Lock()
	defer c.localMu.Unlock()
	if c.local.remove(filter, key) {
		c.broadcastRoute(encodeFrame(frameRouteDelete, []byte(filter)))
	}
}

// broadcastRoute queues a route delta for every peer. If a peer's queue is full, its
// link is resynced instead, which resends the complete route set. c.localMu must be held.
func (c *Cluster) broadcastRoute(frame []byte) {
	c.peersMu.RLock()
	defer c.peersMu.RUnlock()
	for _, p := range c.peers {
		if p.send(frame) {
			c.metrics.routeUpdatesSent.Add(1)
		} else {
			c.metrics.linkResyncs.Add(1)
			p.resync()
		}
	}
}

// routeSnapshot returns a frame containing all local routes.
func (c *Cluster) routeSnapshot() []byte {
	c.localMu.Lock()
	defer c.localMu.Unlock()
	return encodeFrame(frameRoutes, appendStrings(nil, c.local.filters()))
}

// forward sends a publish packet to each other node with matching subscribers.
func (c *Cluster) forward(pk packets.Packet) {
	if pk.Ignore || strings.HasPrefix(pk.TopicName, mqtt.SysPrefix) {
		return
	}

	subs := c.remote.match(pk.TopicName)
	if len(subs.Subscriptions) == 0 && len(subs.Shared) == 0 {
		return
	}

	targets := make(map[string][]string, len(subs.Subscriptions)) // node -> shared filters to serve
	for node := range subs.Subscriptions {
		targets[node] = nil
	}

	if len(subs.Shared) > 0 {
		local := c.server.Topics.Subscribers(pk.TopicName).Shared
		for filter, nodes := range subs.Shared {
			if _, ok := local[filter]; ok {
				continue // a local group member has already received it
			}
			node := pickNode(nodes)
			targets[node] = append(targets[node], filter)
		}
	}

	raw, err := encodePacket(pk)
	if err != nil {
		c.log.Warn("failed to encode forwarded packet", "error", err, "topic", pk.TopicName)
		return
	}

	type send struct {
		node  string
		peer  *peer
		frame []byte
	}

	var common []byte // the frame for nodes serving no shared subscriptions is the same for all
	sends := make([]send, 0, len(targets))
	c.peersMu.RLock()
	for node, shared := range targets {
		var frame []byte
		switch {
		case len(shared) > 0:
			frame = publishFrame(pk, raw, shared)
		case common == nil:
			common = publishFrame(pk, raw, nil)
			frame = common
		default:
			frame = common
		}
		sends = append(sends, send{node: node, peer: c.peers[node], frame: frame})
	}
	c.peersMu.RUnlock() // not held while waiting for a queue, which would hold up peers joining and leaving

	for _, s := range sends {
		if s.peer != nil && c.sendPublish(s.peer, pk.FixedHeader.Qos, s.frame) {
			c.metrics.publishesForwarded.Add(1)
			c.metrics.bytesForwarded.Add(int64(len(s.frame)))
			continue
		}

		c.metrics.publishesDropped.Add(1)
		c.log.Debug("dropped forwarded publish", "peer", s.node, "topic", pk.TopicName, "known", s.peer != nil)
	}
}

// sendPublish queues a forwarded publish for a peer. A qos 0 publish is dropped if the
// peer's queue is full. A qos 1 or 2 publish waits for room, for up to WriteTimeout, so
// that a peer which can't keep up slows its publishers down instead of losing messages
// they have had acknowledged: forward runs on the publishing client's read goroutine, so
// no more packets are read from that client while it waits.
func (c *Cluster) sendPublish(p *peer, qos byte, frame []byte) bool {
	if qos == 0 {
		return p.send(frame)
	}

	queued, waited := p.sendWait(frame, c.opts.WriteTimeout)
	if waited {
		c.metrics.forwardWaits.Add(1)
	}
	return queued
}

// pickNode returns a random node from a set of shared subscription group members.
func pickNode(nodes map[string]packets.Subscription) string {
	i := rand.Intn(len(nodes))
	for node := range nodes {
		if i == 0 {
			return node
		}
		i--
	}
	return ""
}

// acceptLoop accepts inbound peer links until the listener is closed.
func (c *Cluster) acceptLoop() {
	defer c.wg.Done()
	for {
		conn, err := c.ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			c.log.Warn("cluster accept failed", "error", err)
			continue
		}

		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			c.serveInbound(conn)
		}()
	}
}

// serveInbound reads frames from another node's link until it is closed.
func (c *Cluster) serveInbound(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReaderSize(conn, 64*1024)

	_ = conn.SetReadDeadline(time.Now().Add(c.opts.DialTimeout))
	t, body, err := readFrame(r)
	if err != nil {
		c.log.Debug("rejected cluster link", "remote", conn.RemoteAddr().String(), "error", err)
		return
	}

	if t != frameHello {
		_ = conn.SetReadDeadline(time.Time{})
		c.serveRequest(conn, t, body)
		return
	}
	_ = conn.SetReadDeadline(time.Time{})

	node := string(body)
	if node == c.opts.NodeName || node == "" {
		return
	}

	gen := c.registerInbound(node, conn)
	defer c.unregisterInbound(node, conn)

	for {
		t, body, err := readFrame(r)
		if err != nil {
			c.log.Debug("cluster link closed", "peer", node, "error", err)
			return
		}

		switch t {
		case frameRoutes:
			filters, _, err := readStrings(body)
			if err != nil {
				c.log.Warn("malformed cluster routes", "peer", node, "error", err)
				return
			}
			c.remote.set(node, gen, filters)
		case frameRouteAdd:
			c.remote.add(node, gen, string(body))
		case frameRouteDelete:
			c.remote.delete(node, gen, string(body))
		case framePublish:
			pk, shared, err := decodePublish(body)
			if err != nil {
				c.log.Warn("malformed cluster publish", "peer", node, "error", err)
				return
			}
			c.deliver(pk, shared)
		case frameRetained:
			v, deleted, pk, err := decodeRetained(body)
			if err != nil {
				c.log.Warn("malformed cluster retained message", "peer", node, "error", err)
				return
			}
			c.applyRetained(v, deleted, pk)
		default:
			c.log.Warn("unknown cluster frame", "peer", node, "type", t)
			return
		}
	}
}

// deliver publishes a forwarded packet to local subscribers.
func (c *Cluster) deliver(pk packets.Packet, shared []string) {
	sharedFilters := make(map[string]bool, len(shared))
	for _, f := range shared {
		sharedFilters[f] = true
	}

	c.metrics.publishesReceived.Add(1)
	c.server.DeliverLocal(pk, mqtt.DeliverOptions{SharedFilters: sharedFilters})
}

// registerInbound records a node's current inbound link, closing any previous one, and
// returns the link's route generation.
func (c *Cluster) registerInbound(node string, conn net.Conn) uint64 {
	c.inboundMu.Lock()
	defer c.inboundMu.Unlock()
	if c.stopping.Load() {
		_ = conn.Close() // the read loop will exit
	}

	if prev, ok := c.inbound[node]; ok {
		_ = prev.Close()
	}
	c.inbound[node] = conn
	return c.remote.attach(node)
}

// unregisterInbound forgets a node's inbound link, if it is still the current one.
func (c *Cluster) unregisterInbound(node string, conn net.Conn) {
	c.inboundMu.Lock()
	defer c.inboundMu.Unlock()
	if c.inbound[node] == conn {
		delete(c.inbound, node)
	}
}

// nodeJoined opens or updates the outbound link to a node.
func (c *Cluster) nodeJoined(n *memberlist.Node) {
	if n.Name == c.opts.NodeName {
		return
	}

	m, ok := decodeMeta(n.Meta)
	if !ok {
		c.log.Warn("cluster node has invalid metadata", "peer", n.Name, "meta", string(n.Meta))
		return
	}
	addr := net.JoinHostPort(n.Addr.String(), itoa(m.Forward))

	c.peersMu.Lock()
	defer c.peersMu.Unlock()
	if p, ok := c.peers[n.Name]; ok {
		p.draining.Store(m.Draining)
		p.setAddr(addr)
		return
	}

	p := newPeer(c, n.Name, addr)
	p.draining.Store(m.Draining)
	c.peers[n.Name] = p
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		p.run()
	}()
	c.log.Info("cluster node joined", "peer", n.Name, "addr", addr)
}

// nodeLeft closes the links to a node which has left or failed, and forgets its routes.
func (c *Cluster) nodeLeft(n *memberlist.Node) {
	if n.Name == c.opts.NodeName {
		return
	}

	c.peersMu.Lock()
	p, ok := c.peers[n.Name]
	delete(c.peers, n.Name)
	c.peersMu.Unlock()
	if ok {
		p.stop()
	}

	c.inboundMu.Lock()
	c.remote.purge(n.Name)
	if conn, ok := c.inbound[n.Name]; ok {
		_ = conn.Close()
		delete(c.inbound, n.Name)
	}
	c.inboundMu.Unlock()

	c.log.Info("cluster node left", "peer", n.Name)
}

// meta is the gossip metadata a node advertises to other nodes.
type meta struct {
	Forward  int  `json:"forward"`            // peer link port
	Raft     int  `json:"raft"`               // raft port
	Draining bool `json:"draining,omitempty"` // the node is draining
}

func decodeMeta(b []byte) (meta, bool) {
	var m meta
	if err := json.Unmarshal(b, &m); err != nil || m.Forward == 0 || m.Raft == 0 {
		return m, false
	}
	return m, true
}

func itoa(i int) string {
	return strconv.Itoa(i)
}

// delegate advertises this node's ports to other nodes.
type delegate struct {
	c *Cluster
}

func (d *delegate) NodeMeta(limit int) []byte {
	b, _ := json.Marshal(meta{Forward: d.c.forwardPort, Raft: d.c.raftPort, Draining: d.c.draining.Load()})
	return b
}

func (d *delegate) NotifyMsg([]byte)                           {}
func (d *delegate) GetBroadcasts(overhead, limit int) [][]byte { return nil }
func (d *delegate) LocalState(join bool) []byte                { return nil }
func (d *delegate) MergeRemoteState(buf []byte, join bool)     {}

// events handles gossip membership changes.
type events struct {
	c *Cluster
}

func (e *events) NotifyJoin(n *memberlist.Node)   { e.c.setMember(n); e.c.nodeJoined(n) }
func (e *events) NotifyLeave(n *memberlist.Node)  { e.c.removeMember(n.Name); e.c.nodeLeft(n) }
func (e *events) NotifyUpdate(n *memberlist.Node) { e.c.setMember(n); e.c.nodeJoined(n) }

// member is a copy of a gossip member's address and metadata. The nodes memberlist returns
// from Members are its own records, which it updates in place when a node's metadata
// changes, so they can't be read safely; events are delivered while memberlist holds its
// lock, so they can be copied then.
type member struct {
	addr string
	meta meta
	ok   bool // the metadata is valid
}

func (c *Cluster) setMember(n *memberlist.Node) {
	m, ok := decodeMeta(n.Meta)
	c.membersMu.Lock()
	defer c.membersMu.Unlock()
	c.members[n.Name] = member{addr: n.Addr.String(), meta: m, ok: ok}
}

func (c *Cluster) removeMember(name string) {
	c.membersMu.Lock()
	defer c.membersMu.Unlock()
	delete(c.members, name)
}

// memberSnapshot returns a copy of the gossip members.
func (c *Cluster) memberSnapshot() map[string]member {
	c.membersMu.Lock()
	defer c.membersMu.Unlock()
	out := make(map[string]member, len(c.members))
	for name, m := range c.members {
		out[name] = m
	}
	return out
}

// memberlistLog writes memberlist's log lines to a structured logger, keeping their level.
type memberlistLog struct {
	log *slog.Logger
}

func (m *memberlistLog) Write(b []byte) (int, error) {
	msg := strings.TrimSpace(string(b))
	level := slog.LevelDebug
	switch {
	case strings.HasPrefix(msg, "[ERR]"), strings.HasPrefix(msg, "[ERROR]"):
		level = slog.LevelError
	case strings.HasPrefix(msg, "[WARN]"):
		level = slog.LevelWarn
	case strings.HasPrefix(msg, "[INFO]"):
		level = slog.LevelInfo
	}
	m.log.Log(context.Background(), level, msg)
	return len(b), nil
}
