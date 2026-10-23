// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package cluster

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/require"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/packets"
)

var testLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

const (
	waitFor = time.Second * 5
	tick    = time.Millisecond * 10
)

type node struct {
	server  *mqtt.Server
	cluster *Cluster
	mu      sync.Mutex
	clients []*client
	closed  bool
}

func (n *node) addClient(c *client) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.clients = append(n.clients, c)
}

// testOptions returns options for a node on loopback, with fast gossip and raft timings.
func testOptions(name string, seeds ...string) Options {
	mc := memberlist.DefaultLocalConfig()
	mc.BindAddr = "127.0.0.1"
	mc.BindPort = 0

	rc := raft.DefaultConfig()
	rc.HeartbeatTimeout = time.Millisecond * 100
	rc.ElectionTimeout = time.Millisecond * 100
	rc.LeaderLeaseTimeout = time.Millisecond * 50
	rc.CommitTimeout = time.Millisecond * 5

	return Options{
		NodeName:     name,
		ForwardAddr:  "127.0.0.1:0",
		RaftAddr:     "127.0.0.1:0",
		Seeds:        seeds,
		ClaimTimeout: time.Millisecond * 500,
		LeaseTimeout: time.Second,
		Memberlist:   mc,
		Raft:         rc,
	}
}

// newNode starts a server and cluster node on loopback, joining any seeds.
func newNode(t *testing.T, name string, seeds ...string) *node {
	t.Helper()
	return newNodeWithOptions(t, testOptions(name, seeds...))
}

func newNodeWithOptions(t *testing.T, opts Options) *node {
	t.Helper()
	s := mqtt.New(&mqtt.Options{Logger: testLogger, InlineClient: true})
	require.NoError(t, s.AddHook(new(auth.AllowHook), nil))

	c, err := New(s, opts)
	require.NoError(t, err)
	require.NoError(t, s.AddHook(c, nil))
	require.NoError(t, s.Serve())
	require.NoError(t, c.Start())

	n := &node{server: s, cluster: c}
	t.Cleanup(n.close)
	return n
}

// close stops the node's server, if it hasn't already been stopped.
func (n *node) close() {
	n.mu.Lock()
	if n.closed {
		n.mu.Unlock()
		return
	}
	n.closed = true
	clients := n.clients
	n.mu.Unlock()

	for _, c := range clients { // the server waits for its clients to disconnect
		_ = c.conn.Close()
	}
	_ = n.server.Close()
}

// newCluster starts n nodes, and waits until they all know each other.
func newCluster(t *testing.T, names ...string) []*node {
	t.Helper()
	nodes := []*node{newNode(t, names[0])}
	for _, name := range names[1:] {
		nodes = append(nodes, newNode(t, name, nodes[0].cluster.Addr()))
	}

	ready := func() bool {
		for _, n := range nodes {
			if len(n.cluster.Members()) != len(names) || !n.cluster.inContact() || raftVoters(n) != len(names) {
				return false
			}
		}
		return true
	}

	deadline := time.Now().Add(waitFor * 4) // gossip can take a while to converge on a busy machine
	for !ready() {
		if time.Now().After(deadline) {
			var state []string
			for _, n := range nodes {
				state = append(state, fmt.Sprintf("%s: members=%v in-contact=%v voters=%d leader=%q raft=%s",
					n.cluster.Name(), n.cluster.Members(), n.cluster.inContact(), raftVoters(n),
					n.cluster.Leader(), n.cluster.raft.State()))
			}
			t.Fatalf("cluster did not form:\n%s", strings.Join(state, "\n"))
		}
		time.Sleep(tick)
	}

	return nodes
}

// raftVoters returns the number of voters in a node's raft configuration.
func raftVoters(n *node) int {
	f := n.cluster.raft.GetConfiguration()
	if f.Error() != nil {
		return 0
	}
	return len(f.Configuration().Servers)
}

// waitRoutes waits until a node has learned another node's route filters.
func waitRoutes(t *testing.T, at *node, from string, filters ...string) {
	t.Helper()
	if filters == nil {
		filters = []string{}
	}
	require.Eventually(t, func() bool {
		return assertEqual(filters, at.cluster.remote.filters(from))
	}, waitFor, tick, "node %s routes from %s", at.cluster.Name(), from)
}

func assertEqual(want, got []string) bool {
	if len(want) != len(got) {
		return false
	}
	for i := range want {
		if want[i] != got[i] {
			return false
		}
	}
	return true
}

// client is a minimal mqtt v5 client connected to a server over a pipe.
type client struct {
	t        *testing.T
	conn     net.Conn
	r        *bufio.Reader
	packetID uint16
}

// connectOptions configures a test client's connection.
type connectOptions struct {
	persistent bool   // resume any existing session, rather than starting clean
	expiry     uint32 // session expiry interval in seconds
	will       *packets.ConnectParams
}

// connect connects a client with a clean session, failing the test if it is refused.
func connect(t *testing.T, n *node, id string, will *packets.ConnectParams) *client {
	t.Helper()
	c, _ := connectWith(t, n, id, connectOptions{will: will})
	return c
}

// connectWith connects a client, failing the test if it is refused, and returns its CONNACK.
func connectWith(t *testing.T, n *node, id string, o connectOptions) (*client, packets.Packet) {
	t.Helper()
	c, ack, err := dial(t, n, id, o)
	require.NoError(t, err)
	require.Equal(t, packets.Connack, ack.FixedHeader.Type)
	require.Equal(t, packets.CodeSuccess.Code, ack.ReasonCode, "connack reason")
	return c, ack
}

// dial connects a client and returns its CONNACK. It doesn't fail the test, so it can be
// called from other goroutines.
func dial(t *testing.T, n *node, id string, o connectOptions) (*client, packets.Packet, error) {
	conn, srv := net.Pipe()
	go func() { _ = n.server.EstablishConnection("test", srv) }()

	pk := packets.Packet{
		FixedHeader:     packets.FixedHeader{Type: packets.Connect},
		ProtocolVersion: 5,
		Connect: packets.ConnectParams{
			ProtocolName:     []byte("MQTT"),
			Clean:            !o.persistent,
			Keepalive:        30,
			ClientIdentifier: id,
		},
	}
	if o.expiry > 0 {
		pk.Properties.SessionExpiryInterval = o.expiry
		pk.Properties.SessionExpiryIntervalFlag = true
	}
	if o.will != nil {
		pk.Connect.WillFlag = true
		pk.Connect.WillTopic = o.will.WillTopic
		pk.Connect.WillPayload = o.will.WillPayload
		pk.Connect.WillQos = o.will.WillQos
		pk.Connect.WillRetain = o.will.WillRetain
		pk.Connect.WillProperties = o.will.WillProperties
	}

	c := &client{t: t, conn: conn, r: bufio.NewReader(conn)}
	n.addClient(c)

	var buf bytes.Buffer
	if err := pk.ConnectEncode(&buf); err != nil {
		return nil, pk, err
	}

	_ = conn.SetDeadline(time.Now().Add(waitFor * 2))
	if _, err := conn.Write(buf.Bytes()); err != nil {
		return nil, pk, err
	}

	ack := packets.Packet{ProtocolVersion: 5}
	hb, err := c.r.ReadByte()
	if err != nil {
		return nil, ack, err
	}
	if err := ack.FixedHeader.Decode(hb); err != nil {
		return nil, ack, err
	}
	n2, _, err := packets.DecodeLength(c.r)
	if err != nil {
		return nil, ack, err
	}
	body := make([]byte, n2)
	if _, err := io.ReadFull(c.r, body); err != nil {
		return nil, ack, err
	}
	if err := ack.ConnackDecode(body); err != nil {
		return nil, ack, err
	}
	_ = conn.SetDeadline(time.Time{})

	return c, ack, nil
}

func (c *client) write(b []byte) {
	_ = c.conn.SetWriteDeadline(time.Now().Add(waitFor))
	_, err := c.conn.Write(b)
	require.NoError(c.t, err)
}

// read returns the next packet from the server, or false if none arrives within d.
func (c *client) read(d time.Duration) (packets.Packet, bool) {
	_ = c.conn.SetReadDeadline(time.Now().Add(d))
	pk := packets.Packet{ProtocolVersion: 5}

	hb, err := c.r.ReadByte()
	if err != nil {
		return pk, false
	}
	require.NoError(c.t, pk.FixedHeader.Decode(hb))

	n, _, err := packets.DecodeLength(c.r)
	require.NoError(c.t, err)
	body := make([]byte, n)
	_, err = io.ReadFull(c.r, body)
	require.NoError(c.t, err)
	pk.FixedHeader.Remaining = n

	switch pk.FixedHeader.Type {
	case packets.Connack:
		err = pk.ConnackDecode(body)
	case packets.Publish:
		err = pk.PublishDecode(body)
	case packets.Suback:
		err = pk.SubackDecode(body)
	case packets.Unsuback:
		err = pk.UnsubackDecode(body)
	case packets.Puback:
		err = pk.PubackDecode(body)
	case packets.Disconnect:
		err = pk.DisconnectDecode(body)
	}
	require.NoError(c.t, err)
	return pk, true
}

// expect reads packets until one of type t arrives.
func (c *client) expect(t byte) packets.Packet {
	for {
		pk, ok := c.read(waitFor)
		require.True(c.t, ok, "expected packet type %d", t)
		if pk.FixedHeader.Type == t {
			return pk
		}
	}
}

func (c *client) nextID() uint16 {
	c.packetID++
	return c.packetID
}

func (c *client) subscribe(filter string, qos byte) {
	pk := packets.Packet{
		FixedHeader:     packets.FixedHeader{Type: packets.Subscribe, Qos: 1},
		ProtocolVersion: 5,
		PacketID:        c.nextID(),
		Filters:         packets.Subscriptions{{Filter: filter, Qos: qos}},
	}
	var buf bytes.Buffer
	require.NoError(c.t, pk.SubscribeEncode(&buf))
	c.write(buf.Bytes())
	ack := c.expect(packets.Suback)
	require.Equal(c.t, []byte{qos}, ack.ReasonCodes)
}

func (c *client) unsubscribe(filter string) {
	pk := packets.Packet{
		FixedHeader:     packets.FixedHeader{Type: packets.Unsubscribe, Qos: 1},
		ProtocolVersion: 5,
		PacketID:        c.nextID(),
		Filters:         packets.Subscriptions{{Filter: filter}},
	}
	var buf bytes.Buffer
	require.NoError(c.t, pk.UnsubscribeEncode(&buf))
	c.write(buf.Bytes())
	c.expect(packets.Unsuback)
}

func (c *client) publish(topic string, payload []byte, qos byte) {
	pk := packets.Packet{
		FixedHeader:     packets.FixedHeader{Type: packets.Publish, Qos: qos},
		ProtocolVersion: 5,
		TopicName:       topic,
		Payload:         payload,
	}
	if qos > 0 {
		pk.PacketID = c.nextID()
	}
	var buf bytes.Buffer
	require.NoError(c.t, pk.PublishEncode(&buf))
	c.write(buf.Bytes())
	if qos == 1 {
		c.expect(packets.Puback)
	}
}

// receive waits for a publish, returning false if none arrives within d.
func (c *client) receive(d time.Duration) (packets.Packet, bool) {
	for {
		pk, ok := c.read(d)
		if !ok || pk.FixedHeader.Type == packets.Publish {
			return pk, ok
		}
	}
}

func TestClusterForward(t *testing.T) {
	nodes := newCluster(t, "n1", "n2", "n3")

	sub2 := connect(t, nodes[1], "sub2", nil)
	sub2.subscribe("a/b", 1)
	sub3 := connect(t, nodes[2], "sub3", nil)
	sub3.subscribe("a/#", 0)
	waitRoutes(t, nodes[0], "n2", "a/b")
	waitRoutes(t, nodes[0], "n3", "a/#")

	pub := connect(t, nodes[0], "pub", nil)
	pub.publish("a/b", []byte("hello"), 1)

	pk, ok := sub2.receive(waitFor)
	require.True(t, ok)
	require.Equal(t, "a/b", pk.TopicName)
	require.Equal(t, []byte("hello"), pk.Payload)
	require.Equal(t, byte(1), pk.FixedHeader.Qos)

	pk, ok = sub3.receive(waitFor)
	require.True(t, ok)
	require.Equal(t, []byte("hello"), pk.Payload)
	require.Equal(t, byte(0), pk.FixedHeader.Qos)

	require.EqualValues(t, 2, nodes[0].cluster.Stats().Forwarded)
	require.EqualValues(t, 1, nodes[1].cluster.Stats().Received)
	require.EqualValues(t, 1, nodes[2].cluster.Stats().Received)

	// Received messages are never forwarded again.
	require.Zero(t, nodes[1].cluster.Stats().Forwarded)
	require.Zero(t, nodes[2].cluster.Stats().Forwarded)

	// Each subscriber gets the message once.
	_, ok = sub2.receive(time.Millisecond * 100)
	require.False(t, ok)
}

func TestClusterNoRouteNoForward(t *testing.T) {
	nodes := newCluster(t, "n1", "n2")

	pub := connect(t, nodes[0], "pub", nil)
	pub.publish("nobody/listens", []byte("x"), 1)
	require.NoError(t, nodes[0].server.Publish("$SYS/never/forwarded", []byte("x"), false, 0))

	require.Zero(t, nodes[0].cluster.Stats().Forwarded)
	require.Zero(t, nodes[0].cluster.Stats().Dropped)
}

func TestClusterInlinePublishForwarded(t *testing.T) {
	nodes := newCluster(t, "n1", "n2")

	sub := connect(t, nodes[1], "sub", nil)
	sub.subscribe("inline/#", 1)
	waitRoutes(t, nodes[0], "n2", "inline/#")

	require.NoError(t, nodes[0].server.Publish("inline/x", []byte("from inline"), false, 1))
	pk, ok := sub.receive(waitFor)
	require.True(t, ok)
	require.Equal(t, []byte("from inline"), pk.Payload)
}

func TestClusterInlineSubscriptionRoutes(t *testing.T) {
	nodes := newCluster(t, "n1", "n2")
	noop := func(cl *mqtt.Client, sub packets.Subscription, pk packets.Packet) {}

	require.NoError(t, nodes[1].server.Subscribe("i/a", 1, noop))
	require.NoError(t, nodes[1].server.Subscribe("i/a", 2, noop))
	waitRoutes(t, nodes[0], "n2", "i/a")

	// The route stays while any inline subscription remains.
	require.NoError(t, nodes[1].server.Unsubscribe("i/a", 1))
	require.Never(t, func() bool { return len(nodes[0].cluster.remote.filters("n2")) == 0 }, time.Millisecond*200, tick)

	require.NoError(t, nodes[1].server.Unsubscribe("i/a", 2))
	waitRoutes(t, nodes[0], "n2")
}

func TestClusterUnsubscribeWithdrawsRoute(t *testing.T) {
	nodes := newCluster(t, "n1", "n2")

	a := connect(t, nodes[1], "a", nil)
	b := connect(t, nodes[1], "b", nil)
	a.subscribe("x/y", 0)
	b.subscribe("x/y", 0)
	waitRoutes(t, nodes[0], "n2", "x/y")

	a.unsubscribe("x/y")
	require.Never(t, func() bool { return len(nodes[0].cluster.remote.filters("n2")) == 0 }, time.Millisecond*200, tick)

	b.unsubscribe("x/y")
	waitRoutes(t, nodes[0], "n2")
}

func TestClusterDisconnectWithdrawsRoute(t *testing.T) {
	nodes := newCluster(t, "n1", "n2")

	sub := connect(t, nodes[1], "sub", nil)
	sub.subscribe("gone/soon", 0)
	waitRoutes(t, nodes[0], "n2", "gone/soon")

	_ = sub.conn.Close() // a clean session expires on disconnect
	waitRoutes(t, nodes[0], "n2")
}

func TestClusterSharedSubscription(t *testing.T) {
	nodes := newCluster(t, "n1", "n2", "n3")
	const filter = "$share/g/jobs/+"

	sub2 := connect(t, nodes[1], "sub2", nil)
	sub2.subscribe(filter, 0)
	sub3 := connect(t, nodes[2], "sub3", nil)
	sub3.subscribe(filter, 0)
	waitRoutes(t, nodes[0], "n2", filter)
	waitRoutes(t, nodes[0], "n3", filter)

	var got2, got3 atomic.Int32
	count := func(c *client, n *atomic.Int32) {
		for {
			if _, ok := c.receive(time.Millisecond * 300); !ok {
				return
			}
			n.Add(1)
		}
	}

	const total = 40
	pub := connect(t, nodes[0], "pub", nil)
	for i := 0; i < total; i++ {
		pub.publish("jobs/run", []byte("job"), 0)
	}

	done := make(chan struct{}, 2)
	go func() { count(sub2, &got2); done <- struct{}{} }()
	go func() { count(sub3, &got3); done <- struct{}{} }()
	<-done
	<-done

	// Each message reaches exactly one member of the group across the cluster.
	require.EqualValues(t, total, got2.Load()+got3.Load())
	require.Positive(t, got2.Load())
	require.Positive(t, got3.Load())
}

func TestClusterSharedSubscriptionPrefersLocal(t *testing.T) {
	nodes := newCluster(t, "n1", "n2")
	const filter = "$share/g/work"

	local := connect(t, nodes[0], "local", nil)
	local.subscribe(filter, 0)
	remote := connect(t, nodes[1], "remote", nil)
	remote.subscribe(filter, 0)
	waitRoutes(t, nodes[0], "n2", filter)

	pub := connect(t, nodes[0], "pub", nil)
	pub.publish("work", []byte("x"), 0)

	_, ok := local.receive(waitFor)
	require.True(t, ok)
	_, ok = remote.receive(time.Millisecond * 200)
	require.False(t, ok)
	require.Zero(t, nodes[0].cluster.Stats().Forwarded)
}

func TestClusterWillForwarded(t *testing.T) {
	nodes := newCluster(t, "n1", "n2")

	sub := connect(t, nodes[1], "sub", nil)
	sub.subscribe("will/#", 0)
	waitRoutes(t, nodes[0], "n2", "will/#")

	dying := connect(t, nodes[0], "dying", &packets.ConnectParams{
		WillTopic:   "will/dying",
		WillPayload: []byte("goodbye"),
	})
	_ = dying.conn.Close() // disconnect without DISCONNECT sends the will

	pk, ok := sub.receive(waitFor)
	require.True(t, ok)
	require.Equal(t, "will/dying", pk.TopicName)
	require.Equal(t, []byte("goodbye"), pk.Payload)
}

func TestClusterNodeLeavePurgesRoutes(t *testing.T) {
	nodes := newCluster(t, "n1", "n2", "n3")

	sub := connect(t, nodes[2], "sub", nil)
	sub.subscribe("t/#", 0)
	waitRoutes(t, nodes[0], "n3", "t/#")

	nodes[2].close()
	waitRoutes(t, nodes[0], "n3")
	waitRoutes(t, nodes[1], "n3")
	require.Eventually(t, func() bool {
		nodes[0].cluster.peersMu.RLock()
		defer nodes[0].cluster.peersMu.RUnlock()
		_, ok := nodes[0].cluster.peers["n3"]
		return !ok
	}, waitFor, tick)
}

func TestClusterLinkResync(t *testing.T) {
	nodes := newCluster(t, "n1", "n2")

	sub := connect(t, nodes[1], "sub", nil)
	sub.subscribe("before", 0)
	waitRoutes(t, nodes[0], "n2", "before")

	// Drop n2's link to n1, and change n2's routes while it reconnects.
	nodes[1].cluster.peersMu.RLock()
	p := nodes[1].cluster.peers["n1"]
	nodes[1].cluster.peersMu.RUnlock()
	p.resync()
	sub.subscribe("after", 0)
	sub.unsubscribe("before")

	waitRoutes(t, nodes[0], "n2", "after")

	pub := connect(t, nodes[0], "pub", nil)
	pub.publish("after", []byte("still connected"), 0)
	pk, ok := sub.receive(waitFor)
	require.True(t, ok)
	require.Equal(t, []byte("still connected"), pk.Payload)
}

func TestNewRequiresNodeName(t *testing.T) {
	_, err := New(mqtt.New(&mqtt.Options{Logger: testLogger}), Options{})
	require.ErrorIs(t, err, ErrNodeNameRequired)
}

func TestStartTwice(t *testing.T) {
	n := newNode(t, "n1")
	require.ErrorIs(t, n.cluster.Start(), ErrAlreadyStarted)
}

func TestJoinFailed(t *testing.T) {
	s := mqtt.New(&mqtt.Options{Logger: testLogger})
	opts := testOptions("lonely", "127.0.0.1:1")
	opts.Memberlist.TCPTimeout = time.Millisecond * 200

	c, err := New(s, opts)
	require.NoError(t, err)
	require.ErrorIs(t, c.Start(), ErrJoinFailed)
}
