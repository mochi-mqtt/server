// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co
// SPDX-FileContributor: Italo Nesi

package mqtt

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mochi-mqtt/server/v2/hooks/storage"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"
	"github.com/mochi-mqtt/server/v2/system"

	"github.com/stretchr/testify/require"
)

var logger = slog.New(slog.NewTextHandler(io.Discard, nil))

type ProtocolTest []struct {
	protocolVersion byte
	in              packets.TPacketCase
	out             packets.TPacketCase
	data            map[string]any
}

type AllowHook struct {
	HookBase
}

func (h *AllowHook) SetOpts(l *slog.Logger, opts *HookOptions) {
	h.Log = l
	h.Opts = opts
}

func (h *AllowHook) ID() string {
	return "allow-all-auth"
}

func (h *AllowHook) Provides(b byte) bool {
	return bytes.Contains([]byte{OnConnectAuthenticate, OnACLCheck}, []byte{b})
}

func (h *AllowHook) OnConnectAuthenticate(cl *Client, pk packets.Packet) bool { return true }
func (h *AllowHook) OnACLCheck(cl *Client, topic string, write bool) bool     { return true }

type DenyHook struct {
	HookBase
}

func (h *DenyHook) SetOpts(l *slog.Logger, opts *HookOptions) {
	h.Log = l
	h.Opts = opts
}

func (h *DenyHook) ID() string {
	return "deny-all-auth"
}

func (h *DenyHook) Provides(b byte) bool {
	return bytes.Contains([]byte{OnConnectAuthenticate, OnACLCheck}, []byte{b})
}

func (h *DenyHook) OnConnectAuthenticate(cl *Client, pk packets.Packet) bool { return false }
func (h *DenyHook) OnACLCheck(cl *Client, topic string, write bool) bool     { return false }

type DelayHook struct {
	HookBase
	DisconnectDelay time.Duration
}

func (h *DelayHook) SetOpts(l *slog.Logger, opts *HookOptions) {
	h.Log = l
	h.Opts = opts
}

func (h *DelayHook) ID() string {
	return "delay-hook"
}

func (h *DelayHook) Provides(b byte) bool {
	return bytes.Contains([]byte{OnDisconnect}, []byte{b})
}

func (h *DelayHook) OnDisconnect(cl *Client, err error, expire bool) {
	time.Sleep(h.DisconnectDelay)
}

func newServer() *Server {
	cc := NewDefaultServerCapabilities()
	cc.MaximumMessageExpiryInterval = 0
	cc.ReceiveMaximum = 0
	s := New(&Options{
		Logger:       logger,
		Capabilities: cc,
	})
	_ = s.AddHook(new(AllowHook), nil)
	return s
}

func newServerWithInlineClient() *Server {
	cc := NewDefaultServerCapabilities()
	cc.MaximumMessageExpiryInterval = 0
	cc.ReceiveMaximum = 0
	s := New(&Options{
		Logger:       logger,
		Capabilities: cc,
		InlineClient: true,
	})
	_ = s.AddHook(new(AllowHook), nil)
	return s
}

func TestOptionsSetDefaults(t *testing.T) {
	opts := &Options{}
	opts.ensureDefaults()

	require.Equal(t, defaultSysTopicInterval, opts.SysTopicResendInterval)
	require.Equal(t, NewDefaultServerCapabilities(), opts.Capabilities)

	opts = new(Options)
	opts.ensureDefaults()
	require.Equal(t, defaultSysTopicInterval, opts.SysTopicResendInterval)
}

func TestNew(t *testing.T) {
	s := New(nil)
	require.NotNil(t, s)
	require.NotNil(t, s.Clients)
	require.NotNil(t, s.Listeners)
	require.NotNil(t, s.Topics)
	require.NotNil(t, s.Info)
	require.NotNil(t, s.Log)
	require.NotNil(t, s.Options)
	require.NotNil(t, s.loop)
	require.NotNil(t, s.loop.sysTopics)
	require.NotNil(t, s.loop.inflightExpiry)
	require.NotNil(t, s.loop.clientExpiry)
	require.NotNil(t, s.hooks)
	require.NotNil(t, s.hooks.Log)
	require.NotNil(t, s.done)
	require.Nil(t, s.inlineClient)
	require.Equal(t, 0, s.Clients.Len())
}

func TestNewWithInlineClient(t *testing.T) {
	s := New(&Options{
		InlineClient: true,
	})
	require.NotNil(t, s.inlineClient)
	require.Equal(t, 1, s.Clients.Len())
}

func TestNewNilOpts(t *testing.T) {
	s := New(nil)
	require.NotNil(t, s)
	require.NotNil(t, s.Options)
}

func TestServerNewClient(t *testing.T) {
	s := New(nil)
	s.Log = logger
	r, _ := net.Pipe()

	cl := s.NewClient(r, "testing", "test", false)
	require.NotNil(t, cl)
	require.Equal(t, "test", cl.ID)
	require.Equal(t, "testing", cl.Net.Listener)
	require.False(t, cl.Net.Inline)
	require.NotNil(t, cl.State.Inflight.internal)
	require.NotNil(t, cl.State.Subscriptions)
	require.NotNil(t, cl.State.TopicAliases)
	require.Equal(t, defaultKeepalive, cl.State.Keepalive)
	require.Equal(t, defaultClientProtocolVersion, cl.Properties.ProtocolVersion)
	require.NotNil(t, cl.Net.Conn)
	require.NotNil(t, cl.Net.bconn)
	require.NotNil(t, cl.ops)
	require.Equal(t, s.Log, cl.ops.log)
}

func TestServerNewClientInline(t *testing.T) {
	s := New(nil)
	cl := s.NewClient(nil, "testing", "test", true)
	require.True(t, cl.Net.Inline)
}

func TestServerAddHook(t *testing.T) {
	s := New(nil)

	s.Log = logger
	require.NotNil(t, s)

	require.Equal(t, int64(0), s.hooks.Len())
	err := s.AddHook(new(HookBase), nil)
	require.NoError(t, err)
	require.Equal(t, int64(1), s.hooks.Len())
}

func TestServerAddListener(t *testing.T) {
	s := newServer()
	defer s.Close()

	require.NotNil(t, s)

	err := s.AddListener(listeners.NewMockListener("t1", ":1882"))
	require.NoError(t, err)

	// add existing listener
	err = s.AddListener(listeners.NewMockListener("t1", ":1882"))
	require.Error(t, err)
	require.Equal(t, ErrListenerIDExists, err)
}

func TestServerAddHooksFromConfig(t *testing.T) {
	s := newServer()
	defer s.Close()
	require.NotNil(t, s)
	s.Log = logger

	hooks := []HookLoadConfig{
		{Hook: new(modifiedHookBase)},
	}

	err := s.AddHooksFromConfig(hooks)
	require.NoError(t, err)
}

func TestServerAddHooksFromConfigError(t *testing.T) {
	s := newServer()
	defer s.Close()
	require.NotNil(t, s)
	s.Log = logger

	hooks := []HookLoadConfig{
		{Hook: new(modifiedHookBase), Config: map[string]interface{}{}},
	}

	err := s.AddHooksFromConfig(hooks)
	require.Error(t, err)
}

func TestServerAddListenerInitFailure(t *testing.T) {
	s := newServer()
	defer s.Close()

	require.NotNil(t, s)

	m := listeners.NewMockListener("t1", ":1882")
	m.ErrListen = true
	err := s.AddListener(m)
	require.Error(t, err)
}

func TestServerAddListenersFromConfig(t *testing.T) {
	s := newServer()
	defer s.Close()
	require.NotNil(t, s)
	s.Log = logger

	lc := []listeners.Config{
		{Type: listeners.TypeTCP, ID: "tcp", Address: ":1883"},
		{Type: listeners.TypeWS, ID: "ws", Address: ":1882"},
		{Type: listeners.TypeHealthCheck, ID: "health", Address: ":1881"},
		{Type: listeners.TypeSysInfo, ID: "info", Address: ":1880"},
		{Type: listeners.TypeUnix, ID: "unix", Address: "mochi.sock"},
		{Type: listeners.TypeMock, ID: "mock", Address: "0"},
		{Type: "unknown", ID: "unknown"},
	}

	err := s.AddListenersFromConfig(lc)
	require.NoError(t, err)
	require.Equal(t, 6, s.Listeners.Len())

	// The port, not the whole address: a wildcard bind reports "[::]:1883"
	// where IPv6 is available and "0.0.0.0:1883" where it is not, and which
	// of those the kernel chose is not what this test is about.
	port := func(addr string) string {
		_, p, err := net.SplitHostPort(addr)
		require.NoError(t, err)
		return p
	}

	tcp, _ := s.Listeners.Get("tcp")
	require.Equal(t, "1883", port(tcp.Address()))

	ws, _ := s.Listeners.Get("ws")
	require.Equal(t, "1882", port(ws.Address())) // bound at Init, reported like the TCP listener

	health, _ := s.Listeners.Get("health")
	require.Equal(t, ":1881", health.Address())

	info, _ := s.Listeners.Get("info")
	require.Equal(t, ":1880", info.Address())

	unix, _ := s.Listeners.Get("unix")
	require.Equal(t, "mochi.sock", unix.Address())

	mock, _ := s.Listeners.Get("mock")
	require.Equal(t, "0", mock.Address())
}

func TestServerAddListenersFromConfigError(t *testing.T) {
	s := newServer()
	defer s.Close()
	require.NotNil(t, s)
	s.Log = logger

	lc := []listeners.Config{
		{Type: listeners.TypeTCP, ID: "tcp", Address: "x"},
	}

	err := s.AddListenersFromConfig(lc)
	require.Error(t, err)
	require.Equal(t, 0, s.Listeners.Len())
}

func TestServerServe(t *testing.T) {
	s := newServer()
	defer s.Close()

	require.NotNil(t, s)

	err := s.AddListener(listeners.NewMockListener("t1", ":1882"))
	require.NoError(t, err)

	err = s.Serve()
	require.NoError(t, err)

	time.Sleep(time.Millisecond)

	require.Equal(t, 1, s.Listeners.Len())
	listener, ok := s.Listeners.Get("t1")

	require.Equal(t, true, ok)
	require.Equal(t, true, listener.(*listeners.MockListener).IsServing())
}

func TestServerServeFromConfig(t *testing.T) {
	s := newServer()
	defer s.Close()
	require.NotNil(t, s)

	s.Options.Listeners = []listeners.Config{
		{Type: listeners.TypeMock, ID: "mock", Address: "0"},
	}

	s.Options.Hooks = []HookLoadConfig{
		{Hook: new(modifiedHookBase)},
	}

	err := s.Serve()
	require.NoError(t, err)

	time.Sleep(time.Millisecond)

	require.Equal(t, 1, s.Listeners.Len())
	listener, ok := s.Listeners.Get("mock")

	require.Equal(t, true, ok)
	require.Equal(t, true, listener.(*listeners.MockListener).IsServing())
}

func TestServerServeFromConfigListenerError(t *testing.T) {
	s := newServer()
	defer s.Close()
	require.NotNil(t, s)

	s.Options.Listeners = []listeners.Config{
		{Type: listeners.TypeTCP, ID: "tcp", Address: "x"},
	}

	err := s.Serve()
	require.Error(t, err)
}

func TestServerServeFromConfigHookError(t *testing.T) {
	s := newServer()
	defer s.Close()
	require.NotNil(t, s)

	s.Options.Hooks = []HookLoadConfig{
		{Hook: new(modifiedHookBase), Config: map[string]interface{}{}},
	}

	err := s.Serve()
	require.Error(t, err)
}

func TestServerServeReadStoreFailure(t *testing.T) {
	s := newServer()
	defer s.Close()

	require.NotNil(t, s)

	err := s.AddListener(listeners.NewMockListener("t1", ":1882"))
	require.NoError(t, err)

	hook := new(modifiedHookBase)
	hook.failAt = 1
	err = s.AddHook(hook, nil)
	require.NoError(t, err)

	err = s.Serve()
	require.Error(t, err)
}

func TestServerEventLoop(t *testing.T) {
	s := newServer()
	defer s.Close()

	s.loop.sysTopics = time.NewTicker(time.Millisecond)
	s.loop.inflightExpiry = time.NewTicker(time.Millisecond)
	s.loop.clientExpiry = time.NewTicker(time.Millisecond)
	s.loop.retainedExpiry = time.NewTicker(time.Millisecond)
	s.loop.willDelaySend = time.NewTicker(time.Millisecond)
	go s.eventLoop()

	time.Sleep(time.Millisecond * 3)
}

func TestServerReadConnectionPacket(t *testing.T) {
	s := newServer()
	defer s.Close()

	cl, r, _ := newTestClient()
	s.Clients.Add(cl)

	o := make(chan packets.Packet)
	go func() {
		pk, err := s.readConnectionPacket(cl)
		require.NoError(t, err)
		o <- pk
	}()

	go func() {
		_, _ = r.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectMqtt311).RawBytes)
		_ = r.Close()
	}()

	require.Equal(t, *packets.TPacketData[packets.Connect].Get(packets.TConnectMqtt311).Packet, <-o)
}

func TestServerReadConnectionPacketBadFixedHeader(t *testing.T) {
	s := newServer()
	defer s.Close()

	cl, r, _ := newTestClient()
	s.Clients.Add(cl)

	o := make(chan error)
	go func() {
		_, err := s.readConnectionPacket(cl)
		o <- err
	}()

	go func() {
		_, _ = r.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectMalFixedHeader).RawBytes)
		_ = r.Close()
	}()

	err := <-o
	require.Error(t, err)
	require.Equal(t, packets.ErrMalformedVariableByteInteger, err)
}

func TestServerReadConnectionPacketBadPacketType(t *testing.T) {
	s := newServer()
	defer s.Close()

	cl, r, _ := newTestClient()
	s.Clients.Add(cl)

	go func() {
		_, _ = r.Write(packets.TPacketData[packets.Connack].Get(packets.TConnackAcceptedNoSession).RawBytes)
		_ = r.Close()
	}()

	_, err := s.readConnectionPacket(cl)
	require.Error(t, err)
	require.Equal(t, packets.ErrProtocolViolationRequireFirstConnect, err)
}

func TestServerReadConnectionPacketBadPacket(t *testing.T) {
	s := newServer()
	defer s.Close()

	cl, r, _ := newTestClient()
	s.Clients.Add(cl)

	go func() {
		_, _ = r.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectMalProtocolName).RawBytes)
		_ = r.Close()
	}()

	_, err := s.readConnectionPacket(cl)
	require.Error(t, err)
	require.ErrorIs(t, err, packets.ErrMalformedProtocolName)
}

func TestEstablishConnection(t *testing.T) {
	s := newServer()
	defer s.Close()

	r, w := net.Pipe()
	o := make(chan error)
	go func() {
		o <- s.EstablishConnection("tcp", r)
	}()

	go func() {
		_, _ = w.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectClean).RawBytes)
		_, _ = w.Write(packets.TPacketData[packets.Disconnect].Get(packets.TDisconnect).RawBytes)
	}()

	// receive the connack
	recv := make(chan []byte)
	go func() {
		buf, err := io.ReadAll(w)
		require.NoError(t, err)
		recv <- buf
	}()

	err := <-o
	require.NoError(t, err)

	// Todo:
	// 		s.Clients is already empty here. Is it necessary to check v.StopCause()?

	// for _, v := range s.Clients.GetAll() {
	// 	require.ErrorIs(t, v.StopCause(), packets.CodeDisconnect) // true error is disconnect
	// }

	require.Equal(t, packets.TPacketData[packets.Connack].Get(packets.TConnackAcceptedNoSession).RawBytes, <-recv)

	_ = w.Close()
	_ = r.Close()

	// client must be deleted on session close if Clean = true
	_, ok := s.Clients.Get(packets.TPacketData[packets.Connect].Get(packets.TConnectClean).Packet.Connect.ClientIdentifier)
	require.False(t, ok)
}

func TestEstablishConnectionAckFailure(t *testing.T) {
	s := newServer()
	defer s.Close()

	r, w := net.Pipe()
	o := make(chan error)
	go func() {
		o <- s.EstablishConnection("tcp", r)
	}()

	go func() {
		_, _ = w.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectClean).RawBytes)
		_ = w.Close()
	}()

	err := <-o
	require.Error(t, err)
	require.ErrorIs(t, err, io.ErrClosedPipe)

	_ = r.Close()
}

func TestEstablishConnectionReadError(t *testing.T) {
	s := newServer()
	defer s.Close()

	r, w := net.Pipe()
	o := make(chan error)
	go func() {
		o <- s.EstablishConnection("tcp", r)
	}()

	go func() {
		_, _ = w.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectMqtt5).RawBytes)
		_, _ = w.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectClean).RawBytes) // second connect error
	}()

	// receive the connack
	recv := make(chan []byte)
	go func() {
		buf, err := io.ReadAll(w)
		require.NoError(t, err)
		recv <- buf
	}()

	err := <-o
	require.Error(t, err)

	// Retrieve the client corresponding to the Client Identifier.
	retrievedCl, ok := s.Clients.Get(packets.TPacketData[packets.Connect].Get(packets.TConnectMqtt5).Packet.Connect.ClientIdentifier)
	require.True(t, ok)
	require.ErrorIs(t, retrievedCl.StopCause(), packets.ErrProtocolViolationSecondConnect) // true error is disconnect

	ret := <-recv
	require.Equal(t, append(
		packets.TPacketData[packets.Connack].Get(packets.TConnackMinCleanMqtt5).RawBytes,
		packets.TPacketData[packets.Disconnect].Get(packets.TDisconnectSecondConnect).RawBytes...),
		ret,
	)

	_ = w.Close()
	_ = r.Close()
}

func TestEstablishConnectionInheritExisting(t *testing.T) {
	s := newServer()
	defer s.Close()

	cl, r0, _ := newTestClient()
	cl.Properties.ProtocolVersion = 5
	cl.Properties.Username = []byte("mochi")
	cl.ID = packets.TPacketData[packets.Connect].Get(packets.TConnectMqtt311).Packet.Connect.ClientIdentifier
	cl.State.Subscriptions.Add("a/b/c", packets.Subscription{Filter: "a/b/c", Qos: 1})
	cl.State.Inflight.Set(*packets.TPacketData[packets.Publish].Get(packets.TPublishQos1).Packet)
	s.Clients.Add(cl)

	r, w := net.Pipe()
	o := make(chan error)
	go func() {
		err := s.EstablishConnection("tcp", r)
		o <- err
	}()

	go func() {
		_, _ = w.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectMqtt311).RawBytes)
		time.Sleep(time.Millisecond) // we want to receive the queued inflight, so we need to wait a moment before sending the disconnect.
		_, _ = w.Write(packets.TPacketData[packets.Disconnect].Get(packets.TDisconnect).RawBytes)
	}()

	// receive the disconnect session takeover
	takeover := make(chan []byte)
	go func() {
		buf, err := io.ReadAll(r0)
		require.NoError(t, err)
		takeover <- buf
	}()

	// receive the connack
	recv := make(chan []byte)
	go func() {
		buf, err := io.ReadAll(w)
		require.NoError(t, err)
		recv <- buf
	}()

	err := <-o
	require.NoError(t, err)

	// Retrieve the client corresponding to the Client Identifier.
	retrievedCl, ok := s.Clients.Get(packets.TPacketData[packets.Connect].Get(packets.TConnectMqtt311).Packet.Connect.ClientIdentifier)
	require.True(t, ok)
	require.ErrorIs(t, retrievedCl.StopCause(), packets.CodeDisconnect) // true error is disconnect

	connackPlusPacket := append(
		packets.TPacketData[packets.Connack].Get(packets.TConnackAcceptedSessionExists).RawBytes,
		packets.TPacketData[packets.Publish].Get(packets.TPublishQos1Dup).RawBytes...,
	)
	require.Equal(t, connackPlusPacket, <-recv)
	require.Equal(t, packets.TPacketData[packets.Disconnect].Get(packets.TDisconnectTakeover).RawBytes, <-takeover)

	time.Sleep(time.Microsecond * 100)
	_ = w.Close()
	_ = r.Close()

	clw, ok := s.Clients.Get(packets.TPacketData[packets.Connect].Get(packets.TConnectMqtt311).Packet.Connect.ClientIdentifier)
	require.True(t, ok)
	require.NotEmpty(t, clw.State.Subscriptions)
	require.True(t, cl.IsTakenOver())

	// Prevent sequential takeover memory-bloom.
	require.Empty(t, cl.State.Subscriptions.GetAll())
}

// See https://github.com/mochi-mqtt/server/issues/173
func TestEstablishConnectionInheritExistingTrueTakeover(t *testing.T) {
	s := newServer()
	d := new(DelayHook)
	d.DisconnectDelay = time.Millisecond * 200
	_ = s.AddHook(d, nil)
	defer s.Close()

	// Clean session, 0 session expiry interval
	cl1RawBytes := []byte{
		packets.Connect << 4, 21, // Fixed header
		0, 4, // Protocol Name - MSB+LSB
		'M', 'Q', 'T', 'T', // Protocol Name
		5,      // Protocol Version
		1 << 1, // Packet Flags
		0, 30,  // Keepalive
		5,              // Properties length
		17, 0, 0, 0, 0, // Session Expiry Interval (17)
		0, 3, // Client ID - MSB+LSB
		'z', 'e', 'n', // Client ID "zen"
	}

	// Make first connection
	r1, w1 := net.Pipe()
	o1 := make(chan error)
	go func() {
		err := s.EstablishConnection("tcp", r1)
		o1 <- err
	}()
	go func() {
		_, _ = w1.Write(cl1RawBytes)
	}()

	// receive the first connack
	recv := make(chan []byte)
	go func() {
		buf, err := io.ReadAll(w1)
		require.NoError(t, err)
		recv <- buf
	}()

	// Get the first client pointer
	time.Sleep(time.Millisecond * 50)
	cl1, ok := s.Clients.Get(packets.TPacketData[packets.Connect].Get(packets.TConnectUserPass).Packet.Connect.ClientIdentifier)
	require.True(t, ok)
	cl1.State.Subscriptions.Add("a/b/c", packets.Subscription{Filter: "a/b/c", Qos: 1})
	cl1.State.Subscriptions.Add("d/e/f", packets.Subscription{Filter: "d/e/f", Qos: 0})
	time.Sleep(time.Millisecond * 50)

	// Make the second connection
	r2, w2 := net.Pipe()
	o2 := make(chan error)
	go func() {
		err := s.EstablishConnection("tcp", r2)
		o2 <- err
	}()
	go func() {
		x := packets.TPacketData[packets.Connect].Get(packets.TConnectUserPass).RawBytes[:]
		x[19] = '.' // differentiate username bytes in debugging
		_, _ = w2.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectUserPass).RawBytes)
	}()

	// receive the second connack
	recv2 := make(chan []byte)
	go func() {
		buf, err := io.ReadAll(w2)
		require.NoError(t, err)
		recv2 <- buf
	}()

	// Capture first Client pointer
	clp1, ok := s.Clients.Get("zen")
	require.True(t, ok)
	require.Empty(t, clp1.Properties.Username)
	require.NotEmpty(t, clp1.State.Subscriptions.GetAll())

	err1 := <-o1
	require.Error(t, err1)
	require.ErrorIs(t, err1, io.ErrClosedPipe)

	// Capture second Client pointer
	clp2, ok := s.Clients.Get("zen")
	require.True(t, ok)
	require.Equal(t, []byte(".ochi"), clp2.Properties.Username)
	require.NotEmpty(t, clp2.State.Subscriptions.GetAll())
	require.Empty(t, clp1.State.Subscriptions.GetAll())

	_, _ = w2.Write(packets.TPacketData[packets.Disconnect].Get(packets.TDisconnect).RawBytes)
	require.NoError(t, <-o2)

	require.True(t, clp1.IsTakenOver())
	require.False(t, clp2.IsTakenOver())
}

func TestEstablishConnectionResentPendingInflightsError(t *testing.T) {
	s := newServer()
	defer s.Close()

	n := time.Now().Unix()
	cl, r0, _ := newTestClient()
	cl.Properties.ProtocolVersion = 5
	cl.ID = packets.TPacketData[packets.Connect].Get(packets.TConnectMqtt311).Packet.Connect.ClientIdentifier
	cl.State.Inflight = NewInflights()
	cl.State.Inflight.Set(packets.Packet{PacketID: 2, Created: n - 2}) // no packet type
	s.Clients.Add(cl)

	r, w := net.Pipe()
	o := make(chan error)
	go func() {
		o <- s.EstablishConnection("tcp", r)
	}()

	go func() {
		_, _ = w.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectMqtt311).RawBytes)
	}()

	go func() {
		_, err := io.ReadAll(r0)
		require.NoError(t, err)
	}()

	go func() {
		_, err := io.ReadAll(w)
		require.NoError(t, err)
	}()

	err := <-o
	require.Error(t, err)
	require.ErrorIs(t, err, packets.ErrNoValidPacketAvailable)
}

func TestEstablishConnectionInheritExistingClean(t *testing.T) {
	s := newServer()
	defer s.Close()

	cl, r0, _ := newTestClient()
	cl.ID = packets.TPacketData[packets.Connect].Get(packets.TConnectMqtt311).Packet.Connect.ClientIdentifier
	cl.Properties.ProtocolVersion = 4 // the clean-session rule under test is the v3 one
	cl.Properties.Clean = true
	cl.State.Subscriptions.Add("a/b/c", packets.Subscription{Filter: "a/b/c", Qos: 1})
	s.Clients.Add(cl)

	r, w := net.Pipe()
	o := make(chan error)
	go func() {
		o <- s.EstablishConnection("tcp", r)
	}()

	go func() {
		_, _ = w.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectMqtt311).RawBytes)
		_, _ = w.Write(packets.TPacketData[packets.Disconnect].Get(packets.TDisconnect).RawBytes)
	}()

	// a v3 client is closed without a Disconnect packet, so nothing arrives here
	takeover := make(chan []byte)
	go func() {
		buf, err := io.ReadAll(r0)
		require.NoError(t, err)
		takeover <- buf
	}()

	// receive the connack
	recv := make(chan []byte)
	go func() {
		buf, err := io.ReadAll(w)
		require.NoError(t, err)
		recv <- buf
	}()

	err := <-o
	require.NoError(t, err)

	// Retrieve the client corresponding to the Client Identifier.
	retrievedCl, ok := s.Clients.Get(packets.TPacketData[packets.Connect].Get(packets.TConnectMqtt311).Packet.Connect.ClientIdentifier)
	require.True(t, ok)
	require.ErrorIs(t, retrievedCl.StopCause(), packets.CodeDisconnect) // true error is disconnect

	require.Equal(t, packets.TPacketData[packets.Connack].Get(packets.TConnackAcceptedNoSession).RawBytes, <-recv)
	require.Empty(t, <-takeover)

	require.True(t, cl.IsTakenOver())

	_ = w.Close()
	_ = r.Close()

	clw, ok := s.Clients.Get(packets.TPacketData[packets.Connect].Get(packets.TConnectMqtt311).Packet.Connect.ClientIdentifier)
	require.True(t, ok)
	require.Equal(t, 0, clw.State.Subscriptions.Len())

}

func TestEstablishConnectionBadAuthentication(t *testing.T) {
	s := New(&Options{
		Logger: logger,
	})
	defer s.Close()

	r, w := net.Pipe()
	o := make(chan error)
	go func() {
		o <- s.EstablishConnection("tcp", r)
	}()

	go func() {
		_, _ = w.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectClean).RawBytes)
		_, _ = w.Write(packets.TPacketData[packets.Disconnect].Get(packets.TDisconnect).RawBytes)
	}()

	// receive the connack
	recv := make(chan []byte)
	go func() {
		buf, err := io.ReadAll(w)
		require.NoError(t, err)
		recv <- buf
	}()

	err := <-o
	require.Error(t, err)
	require.ErrorIs(t, err, packets.ErrBadUsernameOrPassword)
	require.Equal(t, packets.TPacketData[packets.Connack].Get(packets.TConnackBadUsernamePasswordNoSession).RawBytes, <-recv)

	_ = w.Close()
	_ = r.Close()
}

func TestEstablishConnectionBadAuthenticationAckFailure(t *testing.T) {
	s := New(&Options{
		Logger: logger,
	})
	defer s.Close()

	r, w := net.Pipe()
	o := make(chan error)
	go func() {
		o <- s.EstablishConnection("tcp", r)
	}()

	go func() {
		_, _ = w.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectClean).RawBytes)
		_ = w.Close()
	}()

	err := <-o
	require.Error(t, err)
	require.ErrorIs(t, err, io.ErrClosedPipe)

	_ = r.Close()
}

func TestServerEstablishConnectionInvalidConnect(t *testing.T) {
	s := newServer()

	r, w := net.Pipe()
	o := make(chan error)
	go func() {
		o <- s.EstablishConnection("tcp", r)
	}()

	go func() {
		_, _ = w.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectMalReservedBit).RawBytes)
		_, _ = w.Write(packets.TPacketData[packets.Disconnect].Get(packets.TDisconnect).RawBytes)
	}()

	// receive the connack
	recv := make(chan []byte)
	go func() {
		buf, err := io.ReadAll(w)
		require.NoError(t, err)
		recv <- buf
	}()

	err := <-o
	require.Error(t, err)
	require.ErrorIs(t, packets.ErrProtocolViolationReservedBit, err)
	require.Equal(t, packets.TPacketData[packets.Connack].Get(packets.TConnackProtocolViolationNoSession).RawBytes, <-recv)

	_ = r.Close()
}

func TestServerEstablishConnectionZeroValuedProperties(t *testing.T) {
	for _, tt := range []struct {
		name  string
		tCase byte
		code  packets.Code
	}{
		{"receive maximum", packets.TConnectInvalidZeroReceiveMaximum, packets.ErrProtocolViolationZeroReceiveMaximum},
		{"maximum packet size", packets.TConnectInvalidZeroMaximumPacketSize, packets.ErrProtocolViolationZeroMaximumPacketSize},
	} {
		tt := tt // go.mod says go 1.21, so the range variable is shared
		t.Run(tt.name, func(t *testing.T) {
			s := newServer()

			r, w := net.Pipe()
			o := make(chan error)
			go func() {
				o <- s.EstablishConnection("tcp", r)
			}()

			go func() {
				_, _ = w.Write(packets.TPacketData[packets.Connect].Get(tt.tCase).RawBytes)
				// A disconnect behind it so that a server which wrongly
				// accepts the connect ends the test by returning nil,
				// rather than by holding the connection open until the
				// suite times out.
				_, _ = w.Write(packets.TPacketData[packets.Disconnect].Get(packets.TDisconnect).RawBytes)
			}()

			// receive the connack
			recv := make(chan []byte)
			go func() {
				buf, err := io.ReadAll(w)
				require.NoError(t, err)
				recv <- buf
			}()

			err := <-o
			require.Error(t, err)
			require.ErrorIs(t, tt.code, err)

			// The refusal has to reach the client, not merely be returned
			// here: a code the server never sends is one no client can act
			// on. Byte 0 is the fixed header, 2 the session present flag,
			// 3 the reason code.
			buf := <-recv
			require.Equal(t, byte(packets.Connack<<4), buf[0])
			require.Equal(t, tt.code.Code, buf[3])

			_ = r.Close()
		})
	}
}

func TestServerAnswersAnOversizedPacketInsteadOfResetting(t *testing.T) {
	cc := NewDefaultServerCapabilities()
	cc.MaximumPacketSize = 100 // larger than the connect below, smaller than the publish
	s := New(&Options{Logger: logger, Capabilities: cc})
	_ = s.AddHook(new(AllowHook), nil)
	defer s.Close()

	// A QoS 0 publish past the bound. Built here rather than taken from a
	// fixture because its whole point is its size.
	body := []byte{0, 5, 'a', '/', 'b', '/', 'c', 0}
	body = append(body, bytes.Repeat([]byte{'x'}, 110)...)
	// One byte of remaining length, which is right up to 127 and silently
	// wrong above it: 128 encodes as 0x80, whose continuation bit swallows
	// the next byte, and the server then answers "malformed packet" to a
	// test that believes it asked about size.
	require.Less(t, len(body), 128, "the remaining length no longer fits in one byte")
	oversized := append([]byte{packets.Publish << 4, byte(len(body))}, body...)

	r, w := net.Pipe()
	o := make(chan error)
	go func() {
		o <- s.EstablishConnection("tcp", r)
	}()

	go func() {
		_, _ = w.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectMqtt5).RawBytes)
		_, _ = w.Write(oversized)
	}()

	recv := make(chan []byte)
	go func() {
		buf, err := io.ReadAll(w)
		require.NoError(t, err)
		recv <- buf
	}()

	err := <-o
	require.Error(t, err)
	require.ErrorIs(t, packets.ErrPacketTooLarge, err)

	// The connack first, then the packet this test is about. Walking the
	// remaining length rather than searching for the type byte, which would
	// find one in a payload.
	buf := <-recv
	n, mult := 0, 1
	for i := 1; ; i++ {
		require.Less(t, i, len(buf), "no complete connack in %02x", buf)
		n += int(buf[i]&127) * mult
		mult *= 128
		if buf[i]&128 == 0 {
			buf = buf[i+1+n:]
			break
		}
	}

	require.NotEmpty(t, buf, "the client was reset with nothing after the connack, so a "+
		"bound this server advertised itself went unexplained")
	require.Equal(t, byte(packets.Disconnect<<4), buf[0], "want a disconnect, got %02x", buf)
	require.Equal(t, packets.ErrPacketTooLarge.Code, buf[2], "disconnected with 0x%02X, want 0x95", buf[2])
}

func TestEstablishConnectionMaximumClientsReached(t *testing.T) {
	cc := NewDefaultServerCapabilities()
	cc.MaximumClients = 0
	s := New(&Options{
		Logger:       logger,
		Capabilities: cc,
	})
	_ = s.AddHook(new(AllowHook), nil)
	defer s.Close()

	r, w := net.Pipe()
	o := make(chan error)
	go func() {
		o <- s.EstablishConnection("tcp", r)
	}()

	go func() {
		_, _ = w.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectClean).RawBytes)
	}()

	// receive the connack
	recv := make(chan []byte)
	go func() {
		buf, err := io.ReadAll(w)
		require.NoError(t, err)
		recv <- buf
	}()

	err := <-o
	require.Error(t, err)
	require.ErrorIs(t, err, packets.ErrServerBusy)

	_ = r.Close()
}

// See https://github.com/mochi-mqtt/server/issues/178
func TestServerEstablishConnectionZeroByteUsernameIsValid(t *testing.T) {
	s := newServer()

	r, w := net.Pipe()
	o := make(chan error)
	go func() {
		o <- s.EstablishConnection("tcp", r)
	}()

	go func() {
		_, _ = w.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectZeroByteUsername).RawBytes)
		_, _ = w.Write(packets.TPacketData[packets.Disconnect].Get(packets.TDisconnect).RawBytes)
	}()

	// receive the connack error
	go func() {
		_, err := io.ReadAll(w)
		require.NoError(t, err)
	}()

	err := <-o
	require.NoError(t, err)

	_ = r.Close()
}

func TestServerEstablishConnectionInvalidConnectAckFailure(t *testing.T) {
	s := newServer()

	r, w := net.Pipe()
	o := make(chan error)
	go func() {
		o <- s.EstablishConnection("tcp", r)
	}()

	go func() {
		_, _ = w.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectMalReservedBit).RawBytes)
		_ = w.Close()
	}()

	err := <-o
	require.Error(t, err)
	require.ErrorIs(t, err, io.ErrClosedPipe)

	_ = r.Close()
}

func TestServerEstablishConnectionBadPacket(t *testing.T) {
	s := newServer()

	r, w := net.Pipe()
	o := make(chan error)
	go func() {
		o <- s.EstablishConnection("tcp", r)
	}()

	go func() {
		_, _ = w.Write(packets.TPacketData[packets.Connect].Get(packets.TConnackBadProtocolVersion).RawBytes)
		_, _ = w.Write(packets.TPacketData[packets.Disconnect].Get(packets.TDisconnect).RawBytes)
	}()

	err := <-o
	require.Error(t, err)
	require.ErrorIs(t, err, packets.ErrProtocolViolationRequireFirstConnect)

	_ = r.Close()
}

func TestServerEstablishConnectionOnConnectError(t *testing.T) {
	s := newServer()
	hook := new(modifiedHookBase)
	hook.fail = true
	err := s.AddHook(hook, nil)
	require.NoError(t, err)

	r, w := net.Pipe()
	o := make(chan error)
	go func() {
		o <- s.EstablishConnection("tcp", r)
	}()

	go func() {
		_, _ = w.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectClean).RawBytes)
	}()

	err = <-o
	require.Error(t, err)
	require.ErrorIs(t, err, errTestHook)

	_ = r.Close()
}

func TestServerSendConnack(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5
	s.Options.Capabilities.MaximumQos = 1
	cl.Properties.Props = packets.Properties{
		AssignedClientID: "mochi",
	}
	go func() {
		err := s.SendConnack(cl, packets.CodeSuccess, true, nil)
		require.NoError(t, err)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Connack].Get(packets.TConnackMinMqtt5).RawBytes, buf)
}

func TestServerSendConnackFailureReason(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5
	go func() {
		err := s.SendConnack(cl, packets.ErrUnspecifiedError, true, nil)
		require.NoError(t, err)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Connack].Get(packets.TConnackInvalidMinMqtt5).RawBytes, buf)
}

func TestServerSendConnackWithServerKeepalive(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5
	cl.State.Keepalive = 10
	cl.State.ServerKeepalive = true
	go func() {
		err := s.SendConnack(cl, packets.CodeSuccess, true, nil)
		require.NoError(t, err)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Connack].Get(packets.TConnackServerKeepalive).RawBytes, buf)
}

func TestServerValidateConnect(t *testing.T) {
	packet := *packets.TPacketData[packets.Connect].Get(packets.TConnectMqtt5).Packet
	invalidBitPacket := packet
	invalidBitPacket.ReservedBit = 1
	packetCleanIdPacket := packet
	packetCleanIdPacket.Connect.Clean = false
	packetCleanIdPacket.Connect.ClientIdentifier = ""
	tt := []struct {
		desc         string
		client       *Client
		capabilities Capabilities
		packet       packets.Packet
		expect       packets.Code
	}{
		{
			desc:         "unsupported protocol version",
			client:       &Client{Properties: ClientProperties{ProtocolVersion: 3}},
			capabilities: Capabilities{MinimumProtocolVersion: 4},
			packet:       packet,
			expect:       packets.ErrUnsupportedProtocolVersion,
		},
		{
			desc:         "will qos not supported",
			client:       &Client{Properties: ClientProperties{Will: Will{Qos: 2}}},
			capabilities: Capabilities{MaximumQos: 1},
			packet:       packet,
			expect:       packets.ErrQosNotSupported,
		},
		{
			desc:         "retain not supported",
			client:       &Client{Properties: ClientProperties{Will: Will{Retain: true}}},
			capabilities: Capabilities{RetainAvailable: 0},
			packet:       packet,
			expect:       packets.ErrRetainNotSupported,
		},
		{
			desc:         "invalid packet validate",
			client:       &Client{Properties: ClientProperties{Will: Will{Retain: true}}},
			capabilities: Capabilities{RetainAvailable: 0},
			packet:       invalidBitPacket,
			expect:       packets.ErrProtocolViolationReservedBit,
		},
		{
			desc:         "mqtt3 clean no client id ",
			client:       &Client{Properties: ClientProperties{ProtocolVersion: 3}},
			capabilities: Capabilities{},
			packet:       packetCleanIdPacket,
			expect:       packets.ErrUnspecifiedError,
		},
	}

	s := newServer()
	for _, tx := range tt {
		t.Run(tx.desc, func(t *testing.T) {
			s.Options.Capabilities = &tx.capabilities
			err := s.validateConnect(tx.client, tx.packet)
			require.Error(t, err)
			require.ErrorIs(t, err, tx.expect)
		})
	}
}

func TestServerSendConnackAdjustedExpiryInterval(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5
	cl.Properties.Props.SessionExpiryInterval = uint32(300)
	s.Options.Capabilities.MaximumSessionExpiryInterval = 120
	go func() {
		err := s.SendConnack(cl, packets.CodeSuccess, false, nil)
		require.NoError(t, err)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Connack].Get(packets.TConnackAcceptedAdjustedExpiryInterval).RawBytes, buf)
}

func TestInheritClientSession(t *testing.T) {
	s := newServer()

	n := time.Now().Unix()

	existing, _, _ := newTestClient()
	existing.Net.Conn = nil
	existing.ID = "mochi"
	existing.State.Subscriptions.Add("a/b/c", packets.Subscription{Filter: "a/b/c", Qos: 1})
	existing.State.Inflight = NewInflights()
	existing.State.Inflight.Set(packets.Packet{PacketID: 1, Created: n - 1})
	existing.State.Inflight.Set(packets.Packet{PacketID: 2, Created: n - 2})

	s.Clients.Add(existing)

	cl, _, _ := newTestClient()
	cl.Properties.ProtocolVersion = 5

	require.Equal(t, 0, cl.State.Inflight.Len())
	require.Equal(t, 0, cl.State.Subscriptions.Len())

	// Inherit existing client properties
	b := s.inheritClientSession(packets.Packet{Connect: packets.ConnectParams{ClientIdentifier: "mochi"}}, cl)
	require.True(t, b)
	require.Equal(t, 2, cl.State.Inflight.Len())
	require.Equal(t, 1, cl.State.Subscriptions.Len())

	// On clean, clear existing properties
	cl, _, _ = newTestClient()
	cl.Properties.ProtocolVersion = 5
	b = s.inheritClientSession(packets.Packet{Connect: packets.ConnectParams{ClientIdentifier: "mochi", Clean: true}}, cl)
	require.False(t, b)
	require.Equal(t, 0, cl.State.Inflight.Len())
	require.Equal(t, 0, cl.State.Subscriptions.Len())
}

func TestServerUnsubscribeClient(t *testing.T) {
	s := newServer()
	cl, _, _ := newTestClient()
	pk := packets.Subscription{Filter: "a/b/c", Qos: 1}
	cl.State.Subscriptions.Add("a/b/c", pk)
	s.Topics.Subscribe(cl.ID, pk)
	subs := s.Topics.Subscribers("a/b/c")
	require.Equal(t, 1, len(subs.Subscriptions))
	s.UnsubscribeClient(cl)
	subs = s.Topics.Subscribers("a/b/c")
	require.Equal(t, 0, len(subs.Subscriptions))
}

func TestServerProcessPacketFailure(t *testing.T) {
	s := newServer()
	cl, _, _ := newTestClient()
	err := s.processPacket(cl, packets.Packet{})
	require.Error(t, err)
}

func TestServerProcessPacketConnect(t *testing.T) {
	s := newServer()
	cl, _, _ := newTestClient()

	err := s.processPacket(cl, *packets.TPacketData[packets.Connect].Get(packets.TConnectClean).Packet)
	require.Error(t, err)
}

func TestServerProcessPacketPingreq(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()

	go func() {
		err := s.processPacket(cl, *packets.TPacketData[packets.Pingreq].Get(packets.TPingreq).Packet)
		require.NoError(t, err)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Pingresp].Get(packets.TPingresp).RawBytes, buf)
}

func TestServerProcessPacketPingreqError(t *testing.T) {
	s := newServer()
	cl, _, _ := newTestClient()
	cl.Stop(packets.CodeDisconnect)

	err := s.processPacket(cl, *packets.TPacketData[packets.Pingreq].Get(packets.TPingreq).Packet)
	require.Error(t, err)
	require.ErrorIs(t, cl.StopCause(), packets.CodeDisconnect)
}

func TestServerProcessPacketPublishInvalid(t *testing.T) {
	s := newServer()
	cl, _, _ := newTestClient()

	err := s.processPacket(cl, *packets.TPacketData[packets.Publish].Get(packets.TPublishInvalidQosMustPacketID).Packet)
	require.Error(t, err)
	require.ErrorIs(t, err, packets.ErrProtocolViolationNoPacketID)
}

func TestInjectPacketPublishAndReceive(t *testing.T) {
	s := newServer()
	_ = s.Serve()
	defer s.Close()

	sender, _, w1 := newTestClient()
	sender.Net.Inline = true
	sender.ID = "sender"
	s.Clients.Add(sender)

	receiver, r2, w2 := newTestClient()
	receiver.ID = "receiver"
	s.Clients.Add(receiver)
	s.Topics.Subscribe(receiver.ID, packets.Subscription{Filter: "a/b/c"})

	require.Equal(t, int64(0), atomic.LoadInt64(&s.Info.PacketsReceived))

	receiverBuf := make(chan []byte)
	go func() {
		buf, err := io.ReadAll(r2)
		require.NoError(t, err)
		receiverBuf <- buf
	}()

	go func() {
		err := s.InjectPacket(sender, *packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).Packet)
		require.NoError(t, err)
		_ = w1.Close()
		time.Sleep(time.Millisecond * 10)
		_ = w2.Close()
	}()

	require.Equal(t, packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).RawBytes, <-receiverBuf)
}

func TestServerPublishAndReceive(t *testing.T) {
	s := newServerWithInlineClient()

	_ = s.Serve()
	defer s.Close()

	sender, _, w1 := newTestClient()
	sender.Net.Inline = true
	sender.ID = "sender"
	s.Clients.Add(sender)

	receiver, r2, w2 := newTestClient()
	receiver.ID = "receiver"
	s.Clients.Add(receiver)
	s.Topics.Subscribe(receiver.ID, packets.Subscription{Filter: "a/b/c"})

	require.Equal(t, int64(0), atomic.LoadInt64(&s.Info.PacketsReceived))

	receiverBuf := make(chan []byte)
	go func() {
		buf, err := io.ReadAll(r2)
		require.NoError(t, err)
		receiverBuf <- buf
	}()

	go func() {
		pkx := *packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).Packet
		err := s.Publish(pkx.TopicName, pkx.Payload, pkx.FixedHeader.Retain, pkx.FixedHeader.Qos)
		require.NoError(t, err)
		_ = w1.Close()
		time.Sleep(time.Millisecond * 10)
		_ = w2.Close()
	}()

	require.Equal(t, packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).RawBytes, <-receiverBuf)
}

func TestServerPublishNoInlineClient(t *testing.T) {
	s := newServer()
	pkx := *packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).Packet
	err := s.Publish(pkx.TopicName, pkx.Payload, pkx.FixedHeader.Retain, pkx.FixedHeader.Qos)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInlineClientNotEnabled)
}

func TestInjectPacketError(t *testing.T) {
	s := newServer()
	defer s.Close()
	cl, _, _ := newTestClient()
	cl.Net.Inline = true
	pkx := *packets.TPacketData[packets.Subscribe].Get(packets.TSubscribe).Packet
	pkx.Filters = packets.Subscriptions{}
	err := s.InjectPacket(cl, pkx)
	require.Error(t, err)
}

func TestInjectPacketPublishInvalidTopic(t *testing.T) {
	s := newServer()
	defer s.Close()
	cl, _, _ := newTestClient()
	cl.Net.Inline = true
	pkx := *packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).Packet
	pkx.TopicName = "$SYS/test"
	err := s.InjectPacket(cl, pkx)
	require.NoError(t, err) // bypass topic validity and acl checks
}

func TestServerProcessPacketPublishAndReceive(t *testing.T) {
	s := newServer()
	_ = s.Serve()
	defer s.Close()

	sender, _, w1 := newTestClient()
	sender.ID = "sender"
	s.Clients.Add(sender)

	receiver, r2, w2 := newTestClient()
	receiver.ID = "receiver"
	s.Clients.Add(receiver)
	s.Topics.Subscribe(receiver.ID, packets.Subscription{Filter: "a/b/c"})

	require.Equal(t, int64(0), atomic.LoadInt64(&s.Info.PacketsReceived))
	require.Equal(t, 0, len(s.Topics.Messages("a/b/c")))

	receiverBuf := make(chan []byte)
	go func() {
		buf, err := io.ReadAll(r2)
		require.NoError(t, err)
		receiverBuf <- buf
	}()

	go func() {
		err := s.processPacket(sender, *packets.TPacketData[packets.Publish].Get(packets.TPublishRetain).Packet)
		require.NoError(t, err)
		time.Sleep(time.Millisecond * 10)
		_ = w1.Close()
		_ = w2.Close()
	}()

	require.Equal(t, packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).RawBytes, <-receiverBuf)
	require.Equal(t, 1, len(s.Topics.Messages("a/b/c")))
}

func TestServerBuildAck(t *testing.T) {
	s := newServer()
	properties := packets.Properties{
		User: []packets.UserProperty{
			{Key: "hello", Val: "世界"},
		},
	}
	ack := s.buildAck(7, packets.Puback, 1, properties, packets.CodeGrantedQos1)
	require.Equal(t, packets.Puback, ack.FixedHeader.Type)
	require.Equal(t, uint8(1), ack.FixedHeader.Qos)
	require.Equal(t, packets.CodeGrantedQos1.Code, ack.ReasonCode)
	require.Equal(t, properties, ack.Properties)
}

func TestServerBuildAckError(t *testing.T) {
	s := newServer()
	properties := packets.Properties{
		User: []packets.UserProperty{
			{Key: "hello", Val: "世界"},
		},
	}
	ack := s.buildAck(7, packets.Puback, 1, properties, packets.ErrMalformedPacket)
	require.Equal(t, packets.Puback, ack.FixedHeader.Type)
	require.Equal(t, uint8(1), ack.FixedHeader.Qos)
	require.Equal(t, packets.ErrMalformedPacket.Code, ack.ReasonCode)
	properties.ReasonString = packets.ErrMalformedPacket.Reason
	require.Equal(t, properties, ack.Properties)
}

func TestServerBuildAckPahoCompatibility(t *testing.T) {
	s := newServer()
	s.Options.Capabilities.Compatibilities.NoInheritedPropertiesOnAck = true
	properties := packets.Properties{
		User: []packets.UserProperty{
			{Key: "hello", Val: "世界"},
		},
	}
	ack := s.buildAck(7, packets.Puback, 1, properties, packets.CodeGrantedQos1)
	require.Equal(t, packets.Puback, ack.FixedHeader.Type)
	require.Equal(t, uint8(1), ack.FixedHeader.Qos)
	require.Equal(t, packets.CodeGrantedQos1.Code, ack.ReasonCode)
	require.Equal(t, packets.Properties{}, ack.Properties)
}

func TestServerQoS1PubackReasonCode(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()

	bufCh := make(chan []byte, 1)
	errCh := make(chan error, 1)
	go func() {
		buf, err := io.ReadAll(r)
		if err != nil {
			errCh <- err
			return
		}
		bufCh <- buf
	}()

	pk := *packets.TPacketData[packets.Publish].Get(packets.TPublishQos1).Packet
	err := s.processPublish(cl, pk)
	require.NoError(t, err)
	_ = w.Close()

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case buf := <-bufCh:
		require.Equal(t, packets.TPacketData[packets.Puback].Get(packets.TPuback).RawBytes, buf)
	}
}

func TestServerProcessPacketAndNextImmediate(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()

	next := *packets.TPacketData[packets.Publish].Get(packets.TPublishQos1).Packet
	next.Expiry = -1
	cl.State.Inflight.Set(next)
	atomic.StoreInt64(&s.Info.Inflight, 1)
	require.Equal(t, int64(1), atomic.LoadInt64(&s.Info.Inflight))
	require.Equal(t, int32(5), cl.State.Inflight.sendQuota)

	go func() {
		err := s.processPacket(cl, *packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).Packet)
		require.NoError(t, err)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Publish].Get(packets.TPublishQos1).RawBytes, buf)
	require.Equal(t, int64(0), atomic.LoadInt64(&s.Info.Inflight))
	require.Equal(t, int32(4), cl.State.Inflight.sendQuota)
}

func TestServerProcessPublishAckFailure(t *testing.T) {
	s := newServer()
	_ = s.Serve()
	defer s.Close()

	cl, _, w := newTestClient()
	s.Clients.Add(cl)

	_ = w.Close()
	err := s.processPublish(cl, *packets.TPacketData[packets.Publish].Get(packets.TPublishQos2).Packet)
	require.Error(t, err)
	require.ErrorIs(t, err, io.ErrClosedPipe)
}

func TestServerProcessPublishOnPublishAckErrorRWError(t *testing.T) {
	s := newServer()
	hook := new(modifiedHookBase)
	hook.fail = true
	hook.err = packets.ErrUnspecifiedError
	err := s.AddHook(hook, nil)
	require.NoError(t, err)

	cl, _, w := newTestClient()
	cl.Properties.ProtocolVersion = 5
	s.Clients.Add(cl)
	_ = w.Close()

	err = s.processPublish(cl, *packets.TPacketData[packets.Publish].Get(packets.TPublishQos1).Packet)
	require.Error(t, err)
	require.ErrorIs(t, err, io.ErrClosedPipe)
}

func TestServerProcessPublishOnPublishAckErrorContinue(t *testing.T) {
	s := newServer()
	hook := new(modifiedHookBase)
	hook.fail = true
	hook.err = packets.ErrPayloadFormatInvalid
	err := s.AddHook(hook, nil)
	require.NoError(t, err)
	_ = s.Serve()
	defer s.Close()

	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5
	s.Clients.Add(cl)

	go func() {
		err := s.processPacket(cl, *packets.TPacketData[packets.Publish].Get(packets.TPublishQos1).Packet)
		require.NoError(t, err)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Puback].Get(packets.TPubackUnexpectedError).RawBytes, buf)
}

// TestServerProcessPublishOnPublishAckErrorQoS2GetsPubrec ensures a QoS 2
// publish rejected by an OnPublish hook is answered with a PUBREC, not a
// PUBACK — PUBREC is the spec-defined response to a QoS 2 PUBLISH, and a
// PUBACK here previously left a compliant sender's QoS 2 state machine
// with an ack type it never expects for that packet ID.
func TestServerProcessPublishOnPublishAckErrorQoS2GetsPubrec(t *testing.T) {
	s := newServer()
	hook := new(modifiedHookBase)
	hook.fail = true
	hook.err = packets.ErrPayloadFormatInvalid
	err := s.AddHook(hook, nil)
	require.NoError(t, err)
	_ = s.Serve()
	defer s.Close()

	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5
	s.Clients.Add(cl)

	go func() {
		err := s.processPacket(cl, *packets.TPacketData[packets.Publish].Get(packets.TPublishQos2).Packet)
		require.NoError(t, err)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)

	expected := &packets.Packet{
		FixedHeader:     packets.FixedHeader{Type: packets.Pubrec},
		ProtocolVersion: 5,
		PacketID:        packets.TPacketData[packets.Publish].Get(packets.TPublishQos2).Packet.PacketID,
		ReasonCode:      packets.ErrPayloadFormatInvalid.Code,
		Properties: packets.Properties{
			ReasonString: packets.ErrPayloadFormatInvalid.Reason,
		},
	}
	var expectedBuf bytes.Buffer
	require.NoError(t, expected.PubrecEncode(&expectedBuf))
	require.Equal(t, expectedBuf.Bytes(), buf)
}

func TestServerProcessPublishOnPublishPkIgnore(t *testing.T) {
	s := newServer()
	hook := new(modifiedHookBase)
	hook.fail = true
	hook.err = packets.CodeSuccessIgnore
	err := s.AddHook(hook, nil)
	require.NoError(t, err)
	_ = s.Serve()
	defer s.Close()

	cl, r, w := newTestClient()
	s.Clients.Add(cl)

	receiver, r2, w2 := newTestClient()
	receiver.ID = "receiver"
	s.Clients.Add(receiver)
	s.Topics.Subscribe(receiver.ID, packets.Subscription{Filter: "a/b/c"})

	require.Equal(t, int64(0), atomic.LoadInt64(&s.Info.PacketsReceived))
	require.Equal(t, 0, len(s.Topics.Messages("a/b/c")))

	receiverBuf := make(chan []byte)
	go func() {
		buf, err := io.ReadAll(r2)
		require.NoError(t, err)
		receiverBuf <- buf
	}()

	go func() {
		err := s.processPacket(cl, *packets.TPacketData[packets.Publish].Get(packets.TPublishQos1).Packet)
		require.NoError(t, err)
		_ = w.Close()
		_ = w2.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Puback].Get(packets.TPuback).RawBytes, buf)
	require.Equal(t, []byte{}, <-receiverBuf)
	require.Equal(t, 0, len(s.Topics.Messages("a/b/c")))
}

func TestServerProcessPacketPublishMaximumReceive(t *testing.T) {
	s := newServer()
	_ = s.Serve()
	defer s.Close()

	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5
	cl.State.Inflight.ResetReceiveQuota(0)
	s.Clients.Add(cl)

	go func() {
		err := s.processPacket(cl, *packets.TPacketData[packets.Publish].Get(packets.TPublishQos1).Packet)
		require.Error(t, err)
		require.ErrorIs(t, err, packets.ErrReceiveMaximum)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Disconnect].Get(packets.TDisconnectReceiveMaximum).RawBytes, buf)
}

// A publish to a $SYS topic is refused, and a client that asked for an
// acknowledgement gets one. Dropping the packet silently leaves a QoS 1 or
// 2 client waiting for a reply that never comes and holding a slot of its
// send quota for the life of the session.
//
// The answers are the same ones a publish an ACL denies receives, because
// it is the same refusal: the client may not write there.
func TestServerProcessPublishInvalidTopic(t *testing.T) {
	sysTopic := func(c byte) packets.Packet {
		pk := *packets.TPacketData[packets.Publish].Get(c).Packet
		pk.TopicName = "$SYS/any"
		return pk
	}

	tt := []struct {
		name             string
		protocolVersion  byte
		pk               packets.Packet
		expectErr        error
		expectResponse   []byte
		expectDisconnect bool
	}{
		{
			name:            "v4_QOS0",
			protocolVersion: 4,
			pk:              sysTopic(packets.TPublishBasic),
		},
		{
			// MQTT 3.1.1 has no reason code to put in a PUBACK, so the
			// refusal is a disconnect, exactly as it is for an ACL denial.
			// It is a silent one: v3 has no server-to-client Disconnect
			// packet, so the connection closes and nothing is sent.
			name:             "v4_QOS1",
			protocolVersion:  4,
			pk:               sysTopic(packets.TPublishQos1),
			expectErr:        packets.ErrNotAuthorized,
			expectDisconnect: true,
		},
		{
			name:             "v4_QOS2",
			protocolVersion:  4,
			pk:               sysTopic(packets.TPublishQos2),
			expectErr:        packets.ErrNotAuthorized,
			expectDisconnect: true,
		},
		{
			name:            "v5_QOS0",
			protocolVersion: 5,
			pk:              sysTopic(packets.TPublishBasicMqtt5),
		},
		{
			name:            "v5_QOS1",
			protocolVersion: 5,
			pk:              sysTopic(packets.TPublishQos1Mqtt5),
			expectResponse:  packets.TPacketData[packets.Puback].Get(packets.TPubackMqtt5NotAuthorized).RawBytes,
		},
		{
			name:            "v5_QOS2",
			protocolVersion: 5,
			pk:              sysTopic(packets.TPublishQos2Mqtt5),
			expectResponse:  packets.TPacketData[packets.Pubrec].Get(packets.TPubrecMqtt5NotAuthorized).RawBytes,
		},
	}

	for _, tx := range tt {
		t.Run(tx.name, func(t *testing.T) {
			s := newServer()
			_ = s.Serve()
			defer s.Close()

			cl, r, w := newTestClient()
			cl.Properties.ProtocolVersion = tx.protocolVersion
			s.Clients.Add(cl)

			// The published error is captured rather than asserted inside
			// the goroutine: require calls Goexit on failure, which would
			// skip the Close below and leave the read blocking for ever.
			var publishErr error
			wg := sync.WaitGroup{}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() { _ = w.Close() }()
				publishErr = s.processPublish(cl, tx.pk)
			}()

			buf, err := io.ReadAll(r)
			require.NoError(t, err)
			wg.Wait()

			require.ErrorIs(t, publishErr, tx.expectErr)
			if tx.expectResponse != nil {
				require.Equal(t, tx.expectResponse, buf)
			} else {
				require.Empty(t, buf)
			}
			require.Equal(t, tx.expectDisconnect, cl.Closed())
		})
	}
}

func TestServerProcessPublishACLCheckDeny(t *testing.T) {
	tt := []struct {
		name             string
		protocolVersion  byte
		pk               packets.Packet
		expectErr        error
		expectReponse    []byte
		expectDisconnect bool
	}{
		{
			name:             "v4_QOS0",
			protocolVersion:  4,
			pk:               *packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).Packet,
			expectErr:        nil,
			expectReponse:    nil,
			expectDisconnect: false,
		},
		{
			name:             "v4_QOS1",
			protocolVersion:  4,
			pk:               *packets.TPacketData[packets.Publish].Get(packets.TPublishQos1).Packet,
			expectErr:        packets.ErrNotAuthorized,
			expectReponse:    nil,
			expectDisconnect: true,
		},
		{
			name:             "v4_QOS2",
			protocolVersion:  4,
			pk:               *packets.TPacketData[packets.Publish].Get(packets.TPublishQos2).Packet,
			expectErr:        packets.ErrNotAuthorized,
			expectReponse:    nil,
			expectDisconnect: true,
		},
		{
			name:             "v5_QOS0",
			protocolVersion:  5,
			pk:               *packets.TPacketData[packets.Publish].Get(packets.TPublishBasicMqtt5).Packet,
			expectErr:        nil,
			expectReponse:    nil,
			expectDisconnect: false,
		},
		{
			name:             "v5_QOS1",
			protocolVersion:  5,
			pk:               *packets.TPacketData[packets.Publish].Get(packets.TPublishQos1Mqtt5).Packet,
			expectErr:        nil,
			expectReponse:    packets.TPacketData[packets.Puback].Get(packets.TPubrecMqtt5NotAuthorized).RawBytes,
			expectDisconnect: false,
		},
		{
			name:             "v5_QOS2",
			protocolVersion:  5,
			pk:               *packets.TPacketData[packets.Publish].Get(packets.TPublishQos2Mqtt5).Packet,
			expectErr:        nil,
			expectReponse:    packets.TPacketData[packets.Pubrec].Get(packets.TPubrecMqtt5NotAuthorized).RawBytes,
			expectDisconnect: false,
		},
	}

	for _, tx := range tt {
		tx := tx // one variable per iteration: the goroutine below reads it
		t.Run(tx.name, func(t *testing.T) {
			cc := NewDefaultServerCapabilities()
			s := New(&Options{
				Logger:       logger,
				Capabilities: cc,
			})
			_ = s.AddHook(new(DenyHook), nil)
			_ = s.Serve()
			defer s.Close()

			cl, r, w := newTestClient()
			cl.Properties.ProtocolVersion = tx.protocolVersion
			s.Clients.Add(cl)

			wg := sync.WaitGroup{}
			errCh := make(chan error, 1)
			wg.Add(1)
			go func() {
				defer wg.Done()
				errCh <- s.processPublish(cl, tx.pk)
				_ = w.Close()
			}()

			buf, err := io.ReadAll(r)
			wg.Wait()
			require.NoError(t, err)

			if tx.expectReponse != nil {
				require.Equal(t, tx.expectReponse, buf)
			}

			require.ErrorIs(t, <-errCh, tx.expectErr)
			require.Equal(t, tx.expectDisconnect, cl.Closed())
		})
	}
}

func TestServerProcessPublishOnMessageRecvRejected(t *testing.T) {
	s := newServer()
	require.NotNil(t, s)
	hook := new(modifiedHookBase)
	hook.fail = true
	hook.err = packets.ErrRejectPacket

	err := s.AddHook(hook, nil)
	require.NoError(t, err)

	_ = s.Serve()
	defer s.Close()
	cl, _, _ := newTestClient()
	err = s.processPublish(cl, *packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).Packet)
	require.NoError(t, err) // packets rejected silently
}

func TestServerProcessPacketPublishQos0(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()

	go func() {
		err := s.processPacket(cl, *packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).Packet)
		require.NoError(t, err)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, []byte{}, buf)
}

func TestServerProcessPacketPublishQos1PacketIDInUse(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	cl.State.Inflight.Set(packets.Packet{PacketID: 7, FixedHeader: packets.FixedHeader{Type: packets.Publish}})
	atomic.StoreInt64(&s.Info.Inflight, 1)

	go func() {
		err := s.processPacket(cl, *packets.TPacketData[packets.Publish].Get(packets.TPublishQos1).Packet)
		require.NoError(t, err)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Puback].Get(packets.TPuback).RawBytes, buf)

	// A Publish entry is a message the SERVER sent, under an identifier the
	// server assigned, so it is not in use by this exchange and is left
	// alone. The Pubrec case below is the one that is.
	require.Equal(t, int64(1), atomic.LoadInt64(&s.Info.Inflight))
}

// Packet Identifiers assigned by the Client and by the Server are
// independent (MQTT-2.2.1), and the specification's own comment there
// describes this case: a client publishing under an identifier the server is
// concurrently using for one of its own deliveries. The delivery has to
// survive it — discarding it ends the QoS 1 flow, so it is never
// acknowledged, never resent, and its send quota is never returned.
func TestServerProcessPacketPublishDoesNotDiscardServerDelivery(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()

	delivery := packets.Packet{
		FixedHeader: packets.FixedHeader{Type: packets.Publish, Qos: 1},
		PacketID:    7,
		TopicName:   "server/delivery",
		Payload:     []byte("out"),
	}
	cl.State.Inflight.Set(delivery)
	atomic.StoreInt64(&s.Info.Inflight, 1)

	go func() {
		err := s.processPacket(cl, *packets.TPacketData[packets.Publish].Get(packets.TPublishQos1).Packet)
		require.NoError(t, err)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Puback].Get(packets.TPuback).RawBytes, buf)

	kept, ok := cl.State.Inflight.Get(7)
	require.True(t, ok, "the server's own QoS 1 delivery was discarded by an inbound PUBLISH carrying the same packet identifier")
	require.Equal(t, packets.Publish, kept.FixedHeader.Type)
	require.Equal(t, "server/delivery", kept.TopicName)
	require.Equal(t, int64(1), atomic.LoadInt64(&s.Info.Inflight))
}

func TestServerProcessPacketPublishQos2PacketIDInUse(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5
	cl.State.Inflight.Set(packets.Packet{PacketID: 7, FixedHeader: packets.FixedHeader{Type: packets.Pubrec}})
	atomic.StoreInt64(&s.Info.Inflight, 1)

	go func() {
		err := s.processPacket(cl, *packets.TPacketData[packets.Publish].Get(packets.TPublishQos2Mqtt5).Packet)
		require.NoError(t, err)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Pubrec].Get(packets.TPubrecMqtt5IDInUse).RawBytes, buf)
	require.Equal(t, int64(1), atomic.LoadInt64(&s.Info.Inflight))
}

func TestServerProcessPacketPublishQos1(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()

	go func() {
		err := s.processPacket(cl, *packets.TPacketData[packets.Publish].Get(packets.TPublishQos1).Packet)
		require.NoError(t, err)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Puback].Get(packets.TPuback).RawBytes, buf)
}

func TestServerProcessPacketPublishQos2(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()

	go func() {
		err := s.processPacket(cl, *packets.TPacketData[packets.Publish].Get(packets.TPublishQos2).Packet)
		require.NoError(t, err)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Pubrec].Get(packets.TPubrec).RawBytes, buf)
}

func TestServerProcessPacketPublishDowngradeQos(t *testing.T) {
	s := newServer()
	s.Options.Capabilities.MaximumQos = 1
	cl, r, w := newTestClient()

	go func() {
		err := s.processPacket(cl, *packets.TPacketData[packets.Publish].Get(packets.TPublishQos2).Packet)
		require.NoError(t, err)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Puback].Get(packets.TPuback).RawBytes, buf)
}

func TestPublishToSubscribersSelfNoLocal(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	s.Clients.Add(cl)
	subbed := s.Topics.Subscribe(cl.ID, packets.Subscription{Filter: "a/b/c", NoLocal: true})
	require.True(t, subbed)

	go func() {
		pkx := *packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).Packet
		pkx.Origin = cl.ID
		s.publishToSubscribers(pkx)
		time.Sleep(time.Millisecond)
		_ = w.Close()
	}()

	receiverBuf := make(chan []byte)
	go func() {
		buf, err := io.ReadAll(r)
		require.NoError(t, err)
		receiverBuf <- buf
	}()

	require.Equal(t, []byte{}, <-receiverBuf)
}

func TestPublishToSubscribers(t *testing.T) {
	s := newServer()
	cl, r1, w1 := newTestClient()
	cl.ID = "cl1"
	cl2, r2, w2 := newTestClient()
	cl2.ID = "cl2"
	cl3, r3, w3 := newTestClient()
	cl3.ID = "cl3"
	s.Clients.Add(cl)
	s.Clients.Add(cl2)
	s.Clients.Add(cl3)
	require.True(t, s.Topics.Subscribe(cl.ID, packets.Subscription{Filter: "a/b/c"}))
	require.True(t, s.Topics.Subscribe(cl2.ID, packets.Subscription{Filter: SharePrefix + "/tmp/a/b/c"}))
	require.True(t, s.Topics.Subscribe(cl3.ID, packets.Subscription{Filter: SharePrefix + "/tmp/a/b/c"}))

	cl1Recv := make(chan []byte)
	go func() {
		buf, err := io.ReadAll(r1)
		require.NoError(t, err)
		cl1Recv <- buf
	}()

	cl2Recv := make(chan []byte)
	go func() {
		buf, err := io.ReadAll(r2)
		require.NoError(t, err)
		cl2Recv <- buf
	}()

	cl3Recv := make(chan []byte)
	go func() {
		buf, err := io.ReadAll(r3)
		require.NoError(t, err)
		cl3Recv <- buf
	}()

	go func() {
		s.publishToSubscribers(*packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).Packet)
		time.Sleep(time.Millisecond)
		_ = w1.Close()
		_ = w2.Close()
		_ = w3.Close()
	}()

	require.Equal(t, packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).RawBytes, <-cl1Recv)
	rcv2 := <-cl2Recv
	rcv3 := <-cl3Recv

	ok := false
	if len(rcv2) > 0 {
		require.Equal(t, packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).RawBytes, rcv2)
		require.Equal(t, []byte{}, rcv3)
		ok = true
	} else if len(rcv3) > 0 {
		require.Equal(t, packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).RawBytes, rcv3)
		require.Equal(t, []byte{}, rcv2)
		ok = true
	}
	require.True(t, ok)
}

func TestPublishToSubscribersMessageExpiryDelta(t *testing.T) {
	s := newServer()
	s.Options.Capabilities.MaximumMessageExpiryInterval = 86400
	cl, r1, w1 := newTestClient()
	cl.ID = "cl1"
	cl.Properties.ProtocolVersion = 5
	s.Clients.Add(cl)
	require.True(t, s.Topics.Subscribe(cl.ID, packets.Subscription{Filter: "a/b/c"}))

	cl1Recv := make(chan []byte)
	go func() {
		buf, err := io.ReadAll(r1)
		require.NoError(t, err)
		cl1Recv <- buf
	}()

	go func() {
		pkx := *packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).Packet
		pkx.Created = time.Now().Unix() - 30
		s.publishToSubscribers(pkx)
		time.Sleep(time.Millisecond)
		_ = w1.Close()
	}()

	b := <-cl1Recv
	pk := new(packets.Packet)
	pk.ProtocolVersion = 5
	require.Equal(t, uint32(s.Options.Capabilities.MaximumMessageExpiryInterval-30), binary.BigEndian.Uint32(b[11:15]))
}

func TestPublishToSubscribersIdentifiers(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5
	s.Clients.Add(cl)
	subbed := s.Topics.Subscribe(cl.ID, packets.Subscription{Filter: "a/b/+", Identifier: 2})
	require.True(t, subbed)
	subbed = s.Topics.Subscribe(cl.ID, packets.Subscription{Filter: "a/#", Identifier: 3})
	require.True(t, subbed)
	subbed = s.Topics.Subscribe(cl.ID, packets.Subscription{Filter: "d/e/f", Identifier: 4})
	require.True(t, subbed)

	go func() {
		s.publishToSubscribers(*packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).Packet)
	}()

	want := packets.TPacketData[packets.Publish].Get(packets.TPublishSubscriberIdentifier).RawBytes
	receiverBuf := make(chan []byte)
	go func() {
		buf := make([]byte, len(want))
		_, err := io.ReadFull(r, buf)
		require.NoError(t, err)
		receiverBuf <- buf
	}()

	require.Equal(t, want, <-receiverBuf)
	_ = w.Close()
	rest, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Empty(t, rest, "the server wrote more than the packet asserted on")
}

func TestPublishToSubscribersPkIgnore(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	s.Clients.Add(cl)
	subbed := s.Topics.Subscribe(cl.ID, packets.Subscription{Filter: "#", Identifier: 1})
	require.True(t, subbed)

	go func() {
		pk := *packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).Packet
		pk.Ignore = true
		s.publishToSubscribers(pk)
		time.Sleep(time.Millisecond)
		_ = w.Close()
	}()

	receiverBuf := make(chan []byte)
	go func() {
		buf, err := io.ReadAll(r)
		require.NoError(t, err)
		receiverBuf <- buf
	}()

	require.Equal(t, []byte{}, <-receiverBuf)
}

// selectSubscribersHook records every call and removes one client from the
// delivery, which is the first thing OnSelectSubscribers is documented to
// be for: "programmatically remove or add clients to a publish to
// subscribers process".
type selectSubscribersHook struct {
	HookBase
	calls  int
	remove string
}

func (h *selectSubscribersHook) ID() string { return "select-subscribers" }

func (h *selectSubscribersHook) Provides(b byte) bool {
	return b == OnSelectSubscribers
}

func (h *selectSubscribersHook) OnSelectSubscribers(subs *Subscribers, pk packets.Packet) *Subscribers {
	h.calls++
	delete(subs.Subscriptions, h.remove)
	return subs
}

// OnSelectSubscribers is documented as being called "when subscribers have
// been collected for a topic", and as being usable to "programmatically
// remove or add clients to a publish to subscribers process". It was called
// only when the collected set held a shared subscription, so on a topic
// with none — which is most topics on most brokers — the hook never ran and
// that first purpose could not be reached at all.
//
// A hook that withholds a message from a client therefore did nothing, with
// no error and no log line, which reads as a hook that was never installed.
func TestPublishToSubscribersCallsOnSelectSubscribersWithoutSharedSubscriptions(t *testing.T) {
	s := newServer()
	hook := &selectSubscribersHook{remove: "removed"}
	require.NoError(t, s.AddHook(hook, nil))

	kept, r, w := newTestClient()
	kept.ID = "kept"
	s.Clients.Add(kept)
	require.True(t, s.Topics.Subscribe(kept.ID, packets.Subscription{Filter: "#"}))

	removed, r2, w2 := newTestClient()
	removed.ID = "removed"
	s.Clients.Add(removed)
	require.True(t, s.Topics.Subscribe(removed.ID, packets.Subscription{Filter: "#"}))

	go func() {
		s.publishToSubscribers(*packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).Packet)
		time.Sleep(time.Millisecond)
		_ = w.Close()
		_ = w2.Close()
	}()

	keptBuf := make(chan []byte)
	go func() {
		buf, err := io.ReadAll(r)
		require.NoError(t, err)
		keptBuf <- buf
	}()
	removedBuf := make(chan []byte)
	go func() {
		buf, err := io.ReadAll(r2)
		require.NoError(t, err)
		removedBuf <- buf
	}()

	require.Equal(t,
		packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).RawBytes, <-keptBuf)
	// The one the hook removed receives nothing, which is the whole point
	// of the documented behaviour.
	require.Equal(t, []byte{}, <-removedBuf)
	require.Equal(t, 1, hook.calls)
}

// sharedSelectingHook chooses one member of a shared group and empties the
// group while doing it, which the documentation invites: a hook selecting
// "the subscriber for a shared group in a custom manner" says which by
// filling SharedSelected, and what it leaves in Shared is its own business.
type sharedSelectingHook struct {
	HookBase
	choose string
}

func (h *sharedSelectingHook) ID() string { return "shared-selecting" }

func (h *sharedSelectingHook) Provides(b byte) bool {
	return b == OnSelectSubscribers
}

func (h *sharedSelectingHook) OnSelectSubscribers(subs *Subscribers, pk packets.Packet) *Subscribers {
	var chosen packets.Subscription
	for _, group := range subs.Shared {
		for id, sub := range group {
			if id == h.choose {
				chosen = sub
			}
		}
	}
	subs.Shared = map[string]map[string]packets.Subscription{}
	subs.SharedSelected = map[string]packets.Subscription{h.choose: chosen}
	return subs
}

// A hook that selects for a shared group by emptying Shared must still have
// its choice delivered.
//
// Whether there were shared subscriptions has to be decided before the hook
// runs. Gating the merge on what the hook left behind reads as no shared
// subscriptions at all, so SharedSelected is never merged into
// Subscriptions and the message reaches nobody — a queue whose workers
// simply stop being given work, with nothing anywhere reporting a fault.
func TestPublishToSubscribersMergesASharedChoiceThatEmptiedTheGroup(t *testing.T) {
	s := newServer()
	require.NoError(t, s.AddHook(&sharedSelectingHook{choose: "worker-b"}, nil))

	a, ra, wa := newTestClient()
	a.ID = "worker-a"
	s.Clients.Add(a)
	require.True(t, s.Topics.Subscribe(a.ID, packets.Subscription{
		Filter: "$share/group/a/b/c", Identifier: 1}))

	b, rb, wb := newTestClient()
	b.ID = "worker-b"
	s.Clients.Add(b)
	require.True(t, s.Topics.Subscribe(b.ID, packets.Subscription{
		Filter: "$share/group/a/b/c", Identifier: 1}))

	go func() {
		s.publishToSubscribers(*packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).Packet)
		time.Sleep(time.Millisecond)
		_ = wa.Close()
		_ = wb.Close()
	}()

	bufA := make(chan []byte)
	go func() {
		buf, err := io.ReadAll(ra)
		require.NoError(t, err)
		bufA <- buf
	}()
	bufB := make(chan []byte)
	go func() {
		buf, err := io.ReadAll(rb)
		require.NoError(t, err)
		bufB <- buf
	}()

	// The chosen worker receives it, and the other does not.
	require.Equal(t, []byte{}, <-bufA)
	require.NotEmpty(t, <-bufB)
}

func TestPublishToClientServerDowngradeQos(t *testing.T) {
	s := newServer()
	s.Options.Capabilities.MaximumQos = 1

	cl, r, w := newTestClient()
	s.Clients.Add(cl)

	_, ok := cl.State.Inflight.Get(1)
	require.False(t, ok)
	cl.State.packetID = 6 // just to match the same packet id (7) in the fixtures

	go func() {
		pkx := *packets.TPacketData[packets.Publish].Get(packets.TPublishQos1).Packet
		pkx.FixedHeader.Qos = 2
		_, _ = s.publishToClient(cl, packets.Subscription{Filter: "a/b/c", Qos: 2}, pkx)
	}()

	// publishToClient queues the packet; WriteLoop writes it. Reading the
	// bytes the assertion is about is what waits for that write — closing
	// the pipe on a timer instead let ReadAll return early and short, and
	// the failure it produced named the payload rather than the race.
	want := packets.TPacketData[packets.Publish].Get(packets.TPublishQos1).RawBytes
	receiverBuf := make(chan []byte)
	go func() {
		buf := make([]byte, len(want))
		_, err := io.ReadFull(r, buf)
		require.NoError(t, err)
		receiverBuf <- buf
	}()

	require.Equal(t, want, <-receiverBuf)

	// And nothing beyond it: reading a fixed length would otherwise pass on
	// a prefix of a longer write, which the ReadAll this replaced caught.
	_ = w.Close()
	rest, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Empty(t, rest, "the server wrote more than the packet asserted on")
}

func TestPublishToClientSubscriptionDowngradeQos(t *testing.T) {
	s := newServer()
	s.Options.Capabilities.MaximumQos = 2

	cl, r, w := newTestClient()
	s.Clients.Add(cl)

	_, ok := cl.State.Inflight.Get(1)
	require.False(t, ok)
	cl.State.packetID = 6 // just to match the same packet id (7) in the fixtures

	go func() {
		pkx := *packets.TPacketData[packets.Publish].Get(packets.TPublishQos1).Packet
		pkx.FixedHeader.Qos = 2
		_, _ = s.publishToClient(cl, packets.Subscription{Filter: "a/b/c", Qos: 1}, pkx)
	}()

	// publishToClient queues the packet; WriteLoop writes it. Reading the
	// bytes the assertion is about is what waits for that write — closing
	// the pipe on a timer instead let ReadAll return early and short, and
	// the failure it produced named the payload rather than the race.
	want := packets.TPacketData[packets.Publish].Get(packets.TPublishQos1).RawBytes
	receiverBuf := make(chan []byte)
	go func() {
		buf := make([]byte, len(want))
		_, err := io.ReadFull(r, buf)
		require.NoError(t, err)
		receiverBuf <- buf
	}()

	require.Equal(t, want, <-receiverBuf)

	// And nothing beyond it: reading a fixed length would otherwise pass on
	// a prefix of a longer write, which the ReadAll this replaced caught.
	_ = w.Close()
	rest, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Empty(t, rest, "the server wrote more than the packet asserted on")
}

func TestPublishToClientExceedClientWritesPending(t *testing.T) {
	var sendQuota uint16 = 5
	s := newServer()

	_, w := net.Pipe()
	cl := newClient(w, &ops{
		info:  new(system.Info),
		hooks: new(Hooks),
		log:   logger,
		options: &Options{
			Capabilities: &Capabilities{
				MaximumClientWritesPending: 3,
				maximumPacketID:            10,
			},
		},
	})
	cl.Properties.Props.ReceiveMaximum = sendQuota
	cl.State.Inflight.ResetSendQuota(int32(cl.Properties.Props.ReceiveMaximum))

	s.Clients.Add(cl)

	for i := int32(0); i < cl.ops.options.Capabilities.MaximumClientWritesPending; i++ {
		cl.State.outbound <- new(packets.Packet)
		atomic.AddInt32(&cl.State.outboundQty, 1)
	}

	id, _ := cl.NextPacketID()
	cl.State.Inflight.Set(packets.Packet{PacketID: uint16(id)})
	cl.State.Inflight.DecreaseSendQuota()
	sendQuota--

	_, err := s.publishToClient(cl, packets.Subscription{Filter: "a/b/c", Qos: 2}, packets.Packet{})
	require.Error(t, err)
	require.ErrorIs(t, packets.ErrPendingClientWritesExceeded, err)
	require.Equal(t, int32(sendQuota), atomic.LoadInt32(&cl.State.Inflight.sendQuota))

	_, err = s.publishToClient(cl, packets.Subscription{Filter: "a/b/c", Qos: 2}, packets.Packet{FixedHeader: packets.FixedHeader{Qos: 1}})
	require.Error(t, err)
	require.ErrorIs(t, packets.ErrPendingClientWritesExceeded, err)
	require.Equal(t, int32(sendQuota), atomic.LoadInt32(&cl.State.Inflight.sendQuota))
}

func TestPublishToClientServerTopicAlias(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5
	cl.Properties.Props.TopicAliasMaximum = 5
	s.Clients.Add(cl)

	go func() {
		pkx := *packets.TPacketData[packets.Publish].Get(packets.TPublishBasicMqtt5).Packet
		_, _ = s.publishToClient(cl, packets.Subscription{Filter: pkx.TopicName}, pkx)
		_, _ = s.publishToClient(cl, packets.Subscription{Filter: pkx.TopicName}, pkx)
	}()

	// The second publish carries the alias instead of the topic, so it is
	// five bytes shorter. Reading exactly that many is what waits for both
	// writes — the sleep this replaced could return with nothing read, and
	// then the slicing below panicked rather than failed.
	first := len(packets.TPacketData[packets.Publish].Get(packets.TPublishBasicMqtt5).RawBytes)
	receiverBuf := make(chan []byte)
	go func() {
		buf := make([]byte, first+first-5)
		_, err := io.ReadFull(r, buf)
		require.NoError(t, err)
		receiverBuf <- buf
	}()

	ret := <-receiverBuf
	pk1 := make([]byte, first)
	pk2 := make([]byte, first-5)
	copy(pk1, ret[:first])
	copy(pk2, ret[first:])
	require.Equal(t, append(pk1, pk2...), ret)

	_ = w.Close()
	rest, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Empty(t, rest, "the server wrote more than the two packets asserted on")
}

func TestPublishToClientMqtt3RetainFalseLeverageNoConn(t *testing.T) {
	s := newServer()
	cl, _, _ := newTestClient()
	cl.Net.Conn = nil

	out, err := s.publishToClient(cl, packets.Subscription{Filter: "a/b/c", RetainAsPublished: true}, *packets.TPacketData[packets.Publish].Get(packets.TPublishRetain).Packet)
	require.False(t, out.FixedHeader.Retain)
	require.Error(t, err)
	require.ErrorIs(t, err, packets.CodeDisconnect)
}

func TestPublishToClientMqtt5RetainAsPublishedTrueLeverageNoConn(t *testing.T) {
	s := newServer()
	cl, _, _ := newTestClient()
	cl.Properties.ProtocolVersion = 5
	cl.Net.Conn = nil

	out, err := s.publishToClient(cl, packets.Subscription{Filter: "a/b/c", RetainAsPublished: true}, *packets.TPacketData[packets.Publish].Get(packets.TPublishRetain).Packet)
	require.True(t, out.FixedHeader.Retain)
	require.Error(t, err)
	require.ErrorIs(t, err, packets.CodeDisconnect)
}

func TestPublishToClientExceedMaximumInflight(t *testing.T) {
	const MaxInflight uint16 = 5
	s := newServer()
	cl, _, _ := newTestClient()
	s.Options.Capabilities.MaximumInflight = MaxInflight
	cl.ops.options.Capabilities.MaximumInflight = MaxInflight
	for i := uint16(0); i < MaxInflight; i++ {
		cl.State.Inflight.Set(packets.Packet{PacketID: i})
	}

	_, err := s.publishToClient(cl, packets.Subscription{Filter: "a/b/c", Qos: 1}, *packets.TPacketData[packets.Publish].Get(packets.TPublishQos1).Packet)
	require.Error(t, err)
	require.ErrorIs(t, err, packets.ErrQuotaExceeded)
	require.Equal(t, int64(1), atomic.LoadInt64(&s.Info.InflightDropped))
}

func TestPublishToClientExhaustedPacketID(t *testing.T) {
	s := newServer()
	cl, _, _ := newTestClient()
	for i := uint32(0); i <= cl.ops.options.Capabilities.maximumPacketID; i++ {
		cl.State.Inflight.Set(packets.Packet{PacketID: uint16(i)})
	}

	_, err := s.publishToClient(cl, packets.Subscription{Filter: "a/b/c", Qos: 1}, *packets.TPacketData[packets.Publish].Get(packets.TPublishQos1).Packet)
	require.Error(t, err)
	require.ErrorIs(t, err, packets.ErrQuotaExceeded)
	require.Equal(t, int64(1), atomic.LoadInt64(&s.Info.InflightDropped))
}

func TestPublishToClientACLNotAuthorized(t *testing.T) {
	s := New(&Options{
		Logger: logger,
	})
	err := s.AddHook(new(DenyHook), nil)
	require.NoError(t, err)
	cl, _, _ := newTestClient()

	_, err = s.publishToClient(cl, packets.Subscription{Filter: "a/b/c"}, *packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).Packet)
	require.Error(t, err)
	require.ErrorIs(t, err, packets.ErrNotAuthorized)
}

func TestPublishToClientNoConn(t *testing.T) {
	s := newServer()
	cl, _, _ := newTestClient()
	cl.Net.Conn = nil

	_, err := s.publishToClient(cl, packets.Subscription{Filter: "a/b/c"}, *packets.TPacketData[packets.Publish].Get(packets.TPublishQos1).Packet)
	require.Error(t, err)
	require.ErrorIs(t, err, packets.CodeDisconnect)
}

func TestProcessPublishWithTopicAlias(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	s.Clients.Add(cl)
	subbed := s.Topics.Subscribe(cl.ID, packets.Subscription{Filter: "a/b/c", Qos: 0})
	require.True(t, subbed)

	cl2, _, w2 := newTestClient()
	cl2.Properties.ProtocolVersion = 5
	cl2.State.TopicAliases.Inbound.Set(1, "a/b/c")

	go func() {
		pkx := *packets.TPacketData[packets.Publish].Get(packets.TPublishMqtt5).Packet
		pkx.Properties.SubscriptionIdentifier = []int{} // must not contain from client to server
		pkx.TopicName = ""
		pkx.Properties.TopicAlias = 1
		_ = s.processPacket(cl2, pkx)
		time.Sleep(time.Millisecond)
		_ = w2.Close()
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).RawBytes, buf)
}

func TestProcessPublishWithUnregisteredTopicAlias(t *testing.T) {
	s := newServer()
	_ = s.Serve()
	defer s.Close()

	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5
	s.Clients.Add(cl)

	go func() {
		// Closed however this goroutine ends. A failed require calls
		// Goexit, so a plain call at the bottom is skipped on failure and
		// the ReadAll below then blocks until the whole run times out --
		// a test that cannot report its own failure.
		defer func() { _ = w.Close() }()

		// A publish carrying an alias but no topic name, on a connection
		// that never registered that alias -- which is every reconnect by a
		// client that aliases, since [MQTT-3.3.2-7] drops the mappings with
		// the connection. Section 3.3.2.3.4 case 3a: a Protocol Error.
		pkx := *packets.TPacketData[packets.Publish].Get(packets.TPublishMqtt5).Packet
		pkx.Properties.SubscriptionIdentifier = []int{} // must not contain from client to server
		pkx.TopicName = ""
		pkx.Properties.TopicAliasFlag = true
		pkx.Properties.TopicAlias = 1

		err := s.processPacket(cl, pkx)
		require.Error(t, err)
		require.ErrorIs(t, err, packets.ErrProtocolViolationNoTopic)
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.NotEmpty(t, buf)
	require.Equal(t, byte(packets.Disconnect<<4), buf[0])
	require.Equal(t, packets.ErrProtocolViolationNoTopic.Code, buf[2])

	// The alias must not have been registered against the empty topic on
	// the way through, and nothing may have been published to it.
	require.NotContains(t, cl.State.TopicAliases.Inbound.internal, uint16(1))
	require.Equal(t, 0, len(s.Topics.Messages("")))
}

func TestReceivePacketLogsARefusalAtDebugWithoutThePacket(t *testing.T) {
	s := newServer()

	// This server's own logger, replacing the discarding one after New has
	// taken it, so the buffer below is provably the handler in use rather
	// than one something else has since swapped out. Debug level, because
	// the whole claim is that the refusal is reported there.
	var buf bytes.Buffer
	s.Log = slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	_ = s.Serve()
	defer s.Close()

	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5
	s.Clients.Add(cl)

	payload := []byte("SENTINEL-PAYLOAD-DO-NOT-LOG")

	// Waited on before the buffer is read. DisconnectClient stops the
	// client, so the ReadAll below can return while receivePacket has yet
	// to write its log line -- and a buffer read at that moment reports an
	// empty log about a server that logged.
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		defer func() { _ = w.Close() }()

		// An ordinary refusal carrying a payload: a wildcard in a topic
		// name, which [MQTT-3.3.2-2] forbids. The client is answered with a
		// DISCONNECT, which is the server working rather than failing.
		pkx := *packets.TPacketData[packets.Publish].Get(packets.TPublishMqtt5).Packet
		pkx.Properties.SubscriptionIdentifier = []int{}
		pkx.TopicName = "a/b/+"
		pkx.Payload = payload

		err := s.receivePacket(cl, pkx)
		require.Error(t, err)
	}()

	_, err := io.ReadAll(r)
	require.NoError(t, err)
	wg.Wait()

	out := buf.String()

	// The instrument first: if nothing was logged at all, everything below
	// passes for the wrong reason.
	require.Contains(t, out, "packet refused",
		"no debug line for the refusal reached this test's logger; if nothing was "+
			"logged at all, the assertions below pass for the wrong reason")

	require.NotContains(t, out, "level=WARN",
		"an ordinary refusal was reported to the operator as a problem")
	require.NotContains(t, out, string(payload),
		"the client's payload was written into the log")
	require.NotContains(t, out, "Payload:",
		"the whole packet was formatted into the log")
}

func TestPublishToSubscribersExhaustedSendQuota(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	s.Clients.Add(cl)
	cl.State.Inflight.sendQuota = 0

	subbed := s.Topics.Subscribe(cl.ID, packets.Subscription{Filter: "a/b/c", Qos: 2})
	require.True(t, subbed)

	// coverage: subscriber publish errors are non-returnable
	// can we hook into log/slog ?
	_ = r.Close()
	pkx := *packets.TPacketData[packets.Publish].Get(packets.TPublishQos1).Packet
	pkx.PacketID = 0
	s.publishToSubscribers(pkx)
	time.Sleep(time.Millisecond)
	_ = w.Close()
}

func TestPublishToSubscribersExhaustedPacketIDs(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	s.Clients.Add(cl)
	for i := uint32(0); i <= cl.ops.options.Capabilities.maximumPacketID; i++ {
		cl.State.Inflight.Set(packets.Packet{PacketID: 1})
	}

	subbed := s.Topics.Subscribe(cl.ID, packets.Subscription{Filter: "a/b/c", Qos: 2})
	require.True(t, subbed)

	// coverage: subscriber publish errors are non-returnable
	// can we hook into log/slog ?
	_ = r.Close()
	pkx := *packets.TPacketData[packets.Publish].Get(packets.TPublishQos1).Packet
	pkx.PacketID = 0
	s.publishToSubscribers(pkx)
	time.Sleep(time.Millisecond)
	_ = w.Close()
}

func TestPublishToSubscribersNoConnection(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	s.Clients.Add(cl)
	subbed := s.Topics.Subscribe(cl.ID, packets.Subscription{Filter: "a/b/c", Qos: 2})
	require.True(t, subbed)

	// coverage: subscriber publish errors are non-returnable
	// can we hook into log/slog ?
	_ = r.Close()
	s.publishToSubscribers(*packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).Packet)
	time.Sleep(time.Millisecond)
	_ = w.Close()
}

func TestPublishRetainedToClient(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	s.Clients.Add(cl)

	subbed := s.Topics.Subscribe(cl.ID, packets.Subscription{Filter: "a/b/c", Qos: 2})
	require.True(t, subbed)

	retained := s.Topics.RetainMessage(*packets.TPacketData[packets.Publish].Get(packets.TPublishRetainMqtt5).Packet)
	require.Equal(t, int64(1), retained)

	go func() {
		s.publishRetainedToClient(cl, packets.Subscription{Filter: "a/b/c"}, false)
		time.Sleep(time.Millisecond)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Publish].Get(packets.TPublishRetain).RawBytes, buf)
}

func TestPublishRetainedToClientIsShared(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	s.Clients.Add(cl)

	sub := packets.Subscription{Filter: SharePrefix + "/test/a/b/c"}
	subbed := s.Topics.Subscribe(cl.ID, sub)
	require.True(t, subbed)

	go func() {
		s.publishRetainedToClient(cl, sub, false)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, []byte{}, buf)
}

func TestPublishRetainedToClientError(t *testing.T) {
	s := newServer()
	cl, _, w := newTestClient()
	s.Clients.Add(cl)

	sub := packets.Subscription{Filter: "a/b/c"}
	subbed := s.Topics.Subscribe(cl.ID, sub)
	require.True(t, subbed)

	retained := s.Topics.RetainMessage(*packets.TPacketData[packets.Publish].Get(packets.TPublishRetain).Packet)
	require.Equal(t, int64(1), retained)

	_ = w.Close()
	s.publishRetainedToClient(cl, sub, false)
}

func TestNoRetainMessageIfUnavailable(t *testing.T) {
	s := newServer()
	s.Options.Capabilities.RetainAvailable = 0
	cl, _, _ := newTestClient()
	s.Clients.Add(cl)

	s.retainMessage(new(Client), *packets.TPacketData[packets.Publish].Get(packets.TPublishRetain).Packet)
	require.Equal(t, int64(0), atomic.LoadInt64(&s.Info.Retained))
}

func TestNoRetainMessageIfPkIgnore(t *testing.T) {
	s := newServer()
	cl, _, _ := newTestClient()
	s.Clients.Add(cl)

	pk := *packets.TPacketData[packets.Publish].Get(packets.TPublishRetain).Packet
	pk.Ignore = true
	s.retainMessage(new(Client), pk)
	require.Equal(t, int64(0), atomic.LoadInt64(&s.Info.Retained))
}

func TestNoRetainMessage(t *testing.T) {
	s := newServer()
	cl, _, _ := newTestClient()
	s.Clients.Add(cl)

	s.retainMessage(new(Client), *packets.TPacketData[packets.Publish].Get(packets.TPublishRetain).Packet)
	require.Equal(t, int64(1), atomic.LoadInt64(&s.Info.Retained))
}

func TestServerProcessPacketPuback(t *testing.T) {
	tt := ProtocolTest{
		{
			protocolVersion: 4,
			in:              packets.TPacketData[packets.Puback].Get(packets.TPuback),
		},
		{
			protocolVersion: 5,
			in:              packets.TPacketData[packets.Puback].Get(packets.TPubackMqtt5),
		},
	}

	for _, tx := range tt {
		t.Run(strconv.Itoa(int(tx.protocolVersion)), func(t *testing.T) {
			pID := uint16(7)
			s := newServer()
			cl, _, _ := newTestClient()
			cl.State.Inflight.sendQuota = 3
			cl.State.Inflight.receiveQuota = 3

			cl.State.Inflight.Set(packets.Packet{PacketID: pID})
			atomic.AddInt64(&s.Info.Inflight, 1)

			err := s.processPacket(cl, *tx.in.Packet)
			require.NoError(t, err)

			require.Equal(t, int32(4), atomic.LoadInt32(&cl.State.Inflight.sendQuota))
			require.Equal(t, int32(3), atomic.LoadInt32(&cl.State.Inflight.receiveQuota))

			require.Equal(t, int64(0), atomic.LoadInt64(&s.Info.Inflight))
			_, ok := cl.State.Inflight.Get(pID)
			require.False(t, ok)
		})
	}
}

func TestServerProcessPacketPubackNoPacketID(t *testing.T) {
	s := newServer()
	cl, _, _ := newTestClient()
	cl.State.Inflight.sendQuota = 3
	cl.State.Inflight.receiveQuota = 3

	pk := *packets.TPacketData[packets.Puback].Get(packets.TPuback).Packet
	err := s.processPacket(cl, pk)
	require.NoError(t, err)

	require.Equal(t, int32(3), atomic.LoadInt32(&cl.State.Inflight.sendQuota))
	require.Equal(t, int32(3), atomic.LoadInt32(&cl.State.Inflight.receiveQuota))
}

func TestServerProcessPacketPubrec(t *testing.T) {
	pID := uint16(7)
	s := newServer()
	cl, r, w := newTestClient()
	cl.State.Inflight.sendQuota = 3
	cl.State.Inflight.receiveQuota = 3

	cl.State.Inflight.Set(packets.Packet{PacketID: pID})
	atomic.AddInt64(&s.Info.Inflight, 1)

	recv := make(chan []byte)
	go func() { // receive the ack
		buf, err := io.ReadAll(r)
		require.NoError(t, err)
		recv <- buf
	}()

	err := s.processPacket(cl, *packets.TPacketData[packets.Pubrec].Get(packets.TPubrec).Packet)
	require.NoError(t, err)
	_ = w.Close()

	require.Equal(t, packets.TPacketData[packets.Pubrel].Get(packets.TPubrel).RawBytes, <-recv)

	require.Equal(t, int32(2), atomic.LoadInt32(&cl.State.Inflight.receiveQuota))
	require.Equal(t, int32(3), atomic.LoadInt32(&cl.State.Inflight.sendQuota))
	require.Equal(t, int64(1), atomic.LoadInt64(&s.Info.Inflight))
	_, ok := cl.State.Inflight.Get(pID)
	require.True(t, ok)
}

func TestServerProcessPacketPubrecNoPacketID(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5
	cl.State.Inflight.sendQuota = 3
	cl.State.Inflight.receiveQuota = 3

	recv := make(chan []byte)
	go func() { // receive the ack
		buf, err := io.ReadAll(r)
		require.NoError(t, err)
		recv <- buf
	}()

	pk := *packets.TPacketData[packets.Pubrec].Get(packets.TPubrec).Packet // not sending properties
	err := s.processPacket(cl, pk)
	require.NoError(t, err)
	_ = w.Close()

	require.Equal(t, packets.TPacketData[packets.Pubrel].Get(packets.TPubrelMqtt5AckNoPacket).RawBytes, <-recv)

	require.Equal(t, int32(3), atomic.LoadInt32(&cl.State.Inflight.sendQuota))
	require.Equal(t, int32(3), atomic.LoadInt32(&cl.State.Inflight.receiveQuota))
}

func TestServerProcessPacketPubrecInvalidReason(t *testing.T) {
	pID := uint16(7)
	s := newServer()
	cl, _, _ := newTestClient()
	cl.State.Inflight.Set(packets.Packet{PacketID: pID})
	err := s.processPacket(cl, *packets.TPacketData[packets.Pubrec].Get(packets.TPubrecInvalidReason).Packet)
	require.NoError(t, err)
	require.Equal(t, int64(-1), atomic.LoadInt64(&s.Info.Inflight))
	_, ok := cl.State.Inflight.Get(pID)
	require.False(t, ok)
}

func TestServerProcessPacketPubrecFailure(t *testing.T) {
	pID := uint16(7)
	s := newServer()
	cl, _, _ := newTestClient()
	cl.State.Inflight.Set(packets.Packet{PacketID: pID})
	cl.Stop(packets.CodeDisconnect)
	err := s.processPacket(cl, *packets.TPacketData[packets.Pubrec].Get(packets.TPubrec).Packet)
	require.Error(t, err)
	require.ErrorIs(t, cl.StopCause(), packets.CodeDisconnect)
}

func TestServerProcessPacketPubrel(t *testing.T) {
	pID := uint16(7)
	s := newServer()
	cl, r, w := newTestClient()
	cl.State.Inflight.sendQuota = 3
	cl.State.Inflight.receiveQuota = 3

	cl.State.Inflight.Set(packets.Packet{PacketID: pID})
	atomic.AddInt64(&s.Info.Inflight, 1)

	recv := make(chan []byte)
	go func() { // receive the ack
		buf, err := io.ReadAll(r)
		require.NoError(t, err)
		recv <- buf
	}()

	err := s.processPacket(cl, *packets.TPacketData[packets.Pubrel].Get(packets.TPubrel).Packet)
	require.NoError(t, err)
	_ = w.Close()

	require.Equal(t, int32(4), atomic.LoadInt32(&cl.State.Inflight.receiveQuota))
	require.Equal(t, int32(4), atomic.LoadInt32(&cl.State.Inflight.sendQuota))

	require.Equal(t, packets.TPacketData[packets.Pubcomp].Get(packets.TPubcomp).RawBytes, <-recv)

	require.Equal(t, int64(0), atomic.LoadInt64(&s.Info.Inflight))
	_, ok := cl.State.Inflight.Get(pID)
	require.False(t, ok)
}

func TestServerProcessPacketPubrelNoPacketID(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5
	cl.State.Inflight.sendQuota = 3
	cl.State.Inflight.receiveQuota = 3

	recv := make(chan []byte)
	go func() { // receive the ack
		buf, err := io.ReadAll(r)
		require.NoError(t, err)
		recv <- buf
	}()

	pk := *packets.TPacketData[packets.Pubrel].Get(packets.TPubrel).Packet // not sending properties
	err := s.processPacket(cl, pk)
	require.NoError(t, err)
	_ = w.Close()

	require.Equal(t, packets.TPacketData[packets.Pubcomp].Get(packets.TPubcompMqtt5AckNoPacket).RawBytes, <-recv)

	require.Equal(t, int32(3), atomic.LoadInt32(&cl.State.Inflight.sendQuota))
	require.Equal(t, int32(3), atomic.LoadInt32(&cl.State.Inflight.receiveQuota))
}

func TestServerProcessPacketPubrelFailure(t *testing.T) {
	pID := uint16(7)
	s := newServer()
	cl, _, _ := newTestClient()
	cl.State.Inflight.Set(packets.Packet{PacketID: pID})
	cl.Stop(packets.CodeDisconnect)
	err := s.processPacket(cl, *packets.TPacketData[packets.Pubrel].Get(packets.TPubrel).Packet)
	require.Error(t, err)
	require.ErrorIs(t, cl.StopCause(), packets.CodeDisconnect)
}

func TestServerProcessPacketPubrelBadReason(t *testing.T) {
	pID := uint16(7)
	s := newServer()
	cl, _, _ := newTestClient()
	cl.State.Inflight.Set(packets.Packet{PacketID: pID})
	err := s.processPacket(cl, *packets.TPacketData[packets.Pubrel].Get(packets.TPubrelInvalidReason).Packet)
	require.NoError(t, err)
	require.Equal(t, int64(-1), atomic.LoadInt64(&s.Info.Inflight))
	_, ok := cl.State.Inflight.Get(pID)
	require.False(t, ok)
}

func TestServerProcessPacketPubcomp(t *testing.T) {
	tt := ProtocolTest{
		{
			protocolVersion: 4,
			in:              packets.TPacketData[packets.Pubcomp].Get(packets.TPubcomp),
		},
		{
			protocolVersion: 5,
			in:              packets.TPacketData[packets.Pubcomp].Get(packets.TPubcompMqtt5),
		},
	}

	for _, tx := range tt {
		t.Run(strconv.Itoa(int(tx.protocolVersion)), func(t *testing.T) {
			pID := uint16(7)
			s := newServer()
			cl, _, _ := newTestClient()
			cl.Properties.ProtocolVersion = tx.protocolVersion
			cl.State.Inflight.sendQuota = 3
			cl.State.Inflight.receiveQuota = 3

			cl.State.Inflight.Set(packets.Packet{PacketID: pID})
			atomic.AddInt64(&s.Info.Inflight, 1)

			err := s.processPacket(cl, *tx.in.Packet)
			require.NoError(t, err)
			require.Equal(t, int64(0), atomic.LoadInt64(&s.Info.Inflight))

			require.Equal(t, int32(4), atomic.LoadInt32(&cl.State.Inflight.receiveQuota))
			require.Equal(t, int32(4), atomic.LoadInt32(&cl.State.Inflight.sendQuota))

			_, ok := cl.State.Inflight.Get(pID)
			require.False(t, ok)
		})
	}
}

func TestServerProcessInboundQos2Flow(t *testing.T) {
	tt := ProtocolTest{
		{
			protocolVersion: 5,
			in:              packets.TPacketData[packets.Publish].Get(packets.TPublishQos2),
			out:             packets.TPacketData[packets.Pubrec].Get(packets.TPubrec),
			data: map[string]any{
				"sendquota": int32(3),
				"recvquota": int32(2),
				"inflight":  int64(1),
			},
		},
		{
			protocolVersion: 5,
			in:              packets.TPacketData[packets.Pubrel].Get(packets.TPubrel),
			out:             packets.TPacketData[packets.Pubcomp].Get(packets.TPubcomp),
			data: map[string]any{
				"sendquota": int32(4),
				"recvquota": int32(3),
				"inflight":  int64(0),
			},
		},
	}

	pID := uint16(7)
	s := newServer()
	cl, r, w := newTestClient()
	cl.State.Inflight.sendQuota = 3
	cl.State.Inflight.receiveQuota = 3

	for i, tx := range tt {
		t.Run("qos step"+strconv.Itoa(i), func(t *testing.T) {
			r, w = net.Pipe()
			cl.Net.Conn = w

			recv := make(chan []byte)
			go func() { // receive the ack
				buf, err := io.ReadAll(r)
				require.NoError(t, err)
				recv <- buf
			}()

			err := s.processPacket(cl, *tx.in.Packet)
			require.NoError(t, err)
			_ = w.Close()

			require.Equal(t, tx.out.RawBytes, <-recv)
			if i == 0 {
				_, ok := cl.State.Inflight.Get(pID)
				require.True(t, ok)
			}

			require.Equal(t, tx.data["inflight"].(int64), atomic.LoadInt64(&s.Info.Inflight))
			require.Equal(t, tx.data["recvquota"].(int32), atomic.LoadInt32(&cl.State.Inflight.receiveQuota))
			require.Equal(t, tx.data["sendquota"].(int32), atomic.LoadInt32(&cl.State.Inflight.sendQuota))
		})
	}

	_, ok := cl.State.Inflight.Get(pID)
	require.False(t, ok)
}

func TestServerProcessOutboundQos2Flow(t *testing.T) {
	tt := ProtocolTest{
		{
			protocolVersion: 5,
			in:              packets.TPacketData[packets.Publish].Get(packets.TPublishQos2),
			out:             packets.TPacketData[packets.Publish].Get(packets.TPublishQos2),
			data: map[string]any{
				"sendquota": int32(2),
				"recvquota": int32(3),
				"inflight":  int64(1),
			},
		},
		{
			protocolVersion: 5,
			in:              packets.TPacketData[packets.Pubrec].Get(packets.TPubrec),
			out:             packets.TPacketData[packets.Pubrel].Get(packets.TPubrel),
			data: map[string]any{
				"sendquota": int32(2),
				"recvquota": int32(2),
				"inflight":  int64(1),
			},
		},
		{
			protocolVersion: 5,
			in:              packets.TPacketData[packets.Pubcomp].Get(packets.TPubcomp),
			data: map[string]any{
				"sendquota": int32(3),
				"recvquota": int32(3),
				"inflight":  int64(0),
			},
		},
	}

	pID := uint16(6)
	s := newServer()
	cl, _, _ := newTestClient()
	cl.State.packetID = uint32(6)
	cl.State.Inflight.sendQuota = 3
	cl.State.Inflight.receiveQuota = 3
	s.Clients.Add(cl)
	s.Topics.Subscribe(cl.ID, packets.Subscription{Filter: "a/b/c", Qos: 2})

	for i, tx := range tt {
		t.Run("qos step"+strconv.Itoa(i), func(t *testing.T) {
			r, w := net.Pipe()
			time.Sleep(time.Millisecond)
			cl.Net.Conn = w

			// Reading exactly what this step expects is what waits for
			// the write loop; the sleep it replaces was a guess about a
			// goroutine, and a short read failed naming the payload rather
			// than the race. Step 2 expects nothing, so there is nothing
			// to wait for and the close is what ends the read.
			recv := make(chan []byte)
			go func() { // receive the ack
				if i == 2 {
					buf, err := io.ReadAll(r)
					require.NoError(t, err)
					recv <- buf
					return
				}
				buf := make([]byte, len(tx.out.RawBytes))
				_, err := io.ReadFull(r, buf)
				require.NoError(t, err)
				recv <- buf
			}()

			if i == 0 {
				s.publishToSubscribers(*tx.in.Packet)
			} else {
				err := s.processPacket(cl, *tx.in.Packet)
				require.NoError(t, err)
			}

			if i != 2 {
				require.Equal(t, tx.out.RawBytes, <-recv)
			}
			_ = w.Close()
			if i == 2 {
				require.Empty(t, <-recv, "step 2 expects no packet")
			}

			require.Equal(t, tx.data["inflight"].(int64), atomic.LoadInt64(&s.Info.Inflight))
			require.Equal(t, tx.data["recvquota"].(int32), atomic.LoadInt32(&cl.State.Inflight.receiveQuota))
			require.Equal(t, tx.data["sendquota"].(int32), atomic.LoadInt32(&cl.State.Inflight.sendQuota))
		})
	}

	_, ok := cl.State.Inflight.Get(pID)
	require.False(t, ok)
}

func TestServerProcessPacketSubscribe(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5
	go func() {
		err := s.processPacket(cl, *packets.TPacketData[packets.Subscribe].Get(packets.TSubscribeMqtt5).Packet)
		require.NoError(t, err)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Suback].Get(packets.TSubackMqtt5).RawBytes, buf)
}

func TestServerProcessPacketSubscribeInvalid(t *testing.T) {
	s := newServer()
	cl, _, _ := newTestClient()
	cl.Properties.ProtocolVersion = 5

	err := s.processPacket(cl, *packets.TPacketData[packets.Subscribe].Get(packets.TSubscribeSpecQosMustPacketID).Packet)
	require.Error(t, err)
	require.ErrorIs(t, err, packets.ErrProtocolViolationNoPacketID)
}

func TestServerProcessPacketSubscribeInvalidFilter(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5

	go func() {
		err := s.processPacket(cl, *packets.TPacketData[packets.Subscribe].Get(packets.TSubscribeInvalidFilter).Packet)
		require.NoError(t, err)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Suback].Get(packets.TSubackInvalidFilter).RawBytes, buf)
}

func TestServerProcessPacketSubscribeInvalidSharedNoLocal(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5

	go func() {
		err := s.processPacket(cl, *packets.TPacketData[packets.Subscribe].Get(packets.TSubscribeInvalidSharedNoLocal).Packet)
		require.NoError(t, err)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Suback].Get(packets.TSubackInvalidSharedNoLocal).RawBytes, buf)
}

// A Pubrec entry is the client's OWN QoS 2 publish still in its
// PUBREL exchange, so that identifier really is in use by the client
// and 0x91 is correct. A Publish entry is the server's outbound
// message, whose identifiers are a separate space — see
// TestServerProcessSubscribeWithServerInflightPacketID.
func TestServerProcessPacketSubscribePacketIDInUse(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5
	cl.State.Inflight.Set(packets.Packet{PacketID: 15, FixedHeader: packets.FixedHeader{Type: packets.Pubrec}})

	pkx := *packets.TPacketData[packets.Subscribe].Get(packets.TSubscribeMqtt5).Packet
	pkx.PacketID = 15
	go func() {
		err := s.processPacket(cl, pkx)
		require.NoError(t, err)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Suback].Get(packets.TSubackPacketIDInUse).RawBytes, buf)
}

// A Pubrec entry is the client's OWN QoS 2 publish still in its
// PUBREL exchange, so that identifier really is in use by the client
// and 0x91 is correct. A Publish entry is the server's outbound
// message, whose identifiers are a separate space — see
// TestServerProcessSubscribeWithServerInflightPacketID.
func TestServerProcessPacketUnsubscribePackedIDInUse(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5
	cl.State.Inflight.Set(packets.Packet{PacketID: 15, FixedHeader: packets.FixedHeader{Type: packets.Pubrec}})
	go func() {
		err := s.processPacket(cl, *packets.TPacketData[packets.Unsubscribe].Get(packets.TUnsubscribeMqtt5).Packet)
		require.NoError(t, err)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Unsuback].Get(packets.TUnsubackPacketIDInUse).RawBytes, buf)
	require.Equal(t, int64(0), atomic.LoadInt64(&s.Info.Subscriptions))
}

// Packet Identifiers assigned by the Client and by the Server are
// independent: MQTT 3.1.1 2.3.1 and MQTT 5.0 2.2.1 both say so, and both
// give the same example — a Client can send a packet with identifier
// 0x1234 while receiving a different one carrying 0x1234 from its Server.
//
// The server's own outbound QoS 1 and 2 messages live in cl.State.Inflight,
// so checking a client-assigned SUBSCRIBE identifier against that store
// refuses a packet the specification permits. It happens whenever a
// persistent session has a queued message and the client subscribes on
// connect, which is what every client library's on-connect callback does.
func TestServerProcessSubscribeWithServerInflightPacketID(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()

	// A message the server sent and is waiting to have acknowledged, using
	// the same identifier the client is about to choose for its SUBSCRIBE.
	cl.State.Inflight.Set(packets.Packet{
		FixedHeader: packets.FixedHeader{Type: packets.Publish, Qos: 1},
		PacketID:    15,
		TopicName:   "x/y/z",
	})

	go func() {
		err := s.processPacket(cl, *packets.TPacketData[packets.Subscribe].Get(packets.TSubscribe).Packet)
		require.NoError(t, err)

		time.Sleep(time.Millisecond)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Suback].Get(packets.TSuback).RawBytes, buf)
}

// The same for UNSUBSCRIBE, which carries the identical check.
func TestServerProcessUnsubscribeWithServerInflightPacketID(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	s.Topics.Subscribe(cl.ID, packets.Subscription{Filter: "a/b/c"})
	cl.State.Subscriptions.Add("a/b/c", packets.Subscription{Qos: 0})

	cl.State.Inflight.Set(packets.Packet{
		FixedHeader: packets.FixedHeader{Type: packets.Publish, Qos: 1},
		PacketID:    15,
		TopicName:   "x/y/z",
	})

	go func() {
		err := s.processPacket(cl, *packets.TPacketData[packets.Unsubscribe].Get(packets.TUnsubscribe).Packet)
		require.NoError(t, err)

		time.Sleep(time.Millisecond)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Unsuback].Get(packets.TUnsuback).RawBytes, buf)
}

func TestServerProcessSubscribeWithRetain(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()

	retained := s.Topics.RetainMessage(*packets.TPacketData[packets.Publish].Get(packets.TPublishRetain).Packet)
	require.Equal(t, int64(1), retained)

	go func() {
		err := s.processPacket(cl, *packets.TPacketData[packets.Subscribe].Get(packets.TSubscribe).Packet)
		require.NoError(t, err)

		time.Sleep(time.Millisecond)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, append(
		packets.TPacketData[packets.Suback].Get(packets.TSuback).RawBytes,
		packets.TPacketData[packets.Publish].Get(packets.TPublishRetain).RawBytes...,
	), buf)
}

func TestServerProcessSubscribeDowngradeQos(t *testing.T) {
	s := newServer()
	s.Options.Capabilities.MaximumQos = 1
	cl, r, w := newTestClient()

	go func() {
		err := s.processPacket(cl, *packets.TPacketData[packets.Subscribe].Get(packets.TSubscribeMany).Packet)
		require.NoError(t, err)

		time.Sleep(time.Millisecond)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, []byte{0, 1, 1}, buf[4:])
}

func TestServerProcessSubscribeWithRetainHandling1(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	s.Topics.Subscribe(cl.ID, packets.Subscription{Filter: "a/b/c"})
	s.Clients.Add(cl)

	retained := s.Topics.RetainMessage(*packets.TPacketData[packets.Publish].Get(packets.TPublishRetain).Packet)
	require.Equal(t, int64(1), retained)

	go func() {
		err := s.processPacket(cl, *packets.TPacketData[packets.Subscribe].Get(packets.TSubscribeRetainHandling1).Packet)
		require.NoError(t, err)

		time.Sleep(time.Millisecond)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Suback].Get(packets.TSuback).RawBytes, buf)
}

func TestServerProcessSubscribeWithRetainHandling2(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	s.Clients.Add(cl)

	retained := s.Topics.RetainMessage(*packets.TPacketData[packets.Publish].Get(packets.TPublishRetain).Packet)
	require.Equal(t, int64(1), retained)

	go func() {
		err := s.processPacket(cl, *packets.TPacketData[packets.Subscribe].Get(packets.TSubscribeRetainHandling2).Packet)
		require.NoError(t, err)

		time.Sleep(time.Millisecond)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Suback].Get(packets.TSuback).RawBytes, buf)
}

func TestServerProcessSubscribeWithNotRetainAsPublished(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	s.Clients.Add(cl)

	retained := s.Topics.RetainMessage(*packets.TPacketData[packets.Publish].Get(packets.TPublishRetain).Packet)
	require.Equal(t, int64(1), retained)

	go func() {
		err := s.processPacket(cl, *packets.TPacketData[packets.Subscribe].Get(packets.TSubscribeRetainAsPublished).Packet)
		require.NoError(t, err)

		time.Sleep(time.Millisecond)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, append(
		packets.TPacketData[packets.Suback].Get(packets.TSuback).RawBytes,
		packets.TPacketData[packets.Publish].Get(packets.TPublishRetain).RawBytes...,
	), buf)
}

func TestServerProcessSubscribeNoConnection(t *testing.T) {
	s := newServer()
	cl, r, _ := newTestClient()
	_ = r.Close()
	err := s.processSubscribe(cl, *packets.TPacketData[packets.Subscribe].Get(packets.TSubscribe).Packet)
	require.Error(t, err)
	require.ErrorIs(t, err, io.ErrClosedPipe)
}

func TestServerProcessSubscribeACLCheckDeny(t *testing.T) {
	s := New(&Options{
		Logger: logger,
	})
	_ = s.Serve()
	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5

	go func() {
		err := s.processSubscribe(cl, *packets.TPacketData[packets.Subscribe].Get(packets.TSubscribe).Packet)
		require.NoError(t, err)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Suback].Get(packets.TSubackDeny).RawBytes, buf)
}

func TestServerProcessSubscribeACLCheckDenyObscure(t *testing.T) {
	s := New(&Options{
		Logger: logger,
	})
	_ = s.Serve()
	s.Options.Capabilities.Compatibilities.ObscureNotAuthorized = true
	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5

	go func() {
		err := s.processSubscribe(cl, *packets.TPacketData[packets.Subscribe].Get(packets.TSubscribe).Packet)
		require.NoError(t, err)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Suback].Get(packets.TSubackUnspecifiedErrorMqtt5).RawBytes, buf)
}

func TestServerProcessSubscribeErrorDowngrade(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 3
	cl.State.packetID = 1 // just to match the same packet id (7) in the fixtures

	go func() {
		err := s.processPacket(cl, *packets.TPacketData[packets.Subscribe].Get(packets.TSubscribeInvalidSharedNoLocal).Packet)
		require.NoError(t, err)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Suback].Get(packets.TSubackUnspecifiedError).RawBytes, buf)
}

func TestServerProcessPacketUnsubscribe(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5
	s.Topics.Subscribe(cl.ID, packets.Subscription{Filter: "a/b", Qos: 0})
	go func() {
		err := s.processPacket(cl, *packets.TPacketData[packets.Unsubscribe].Get(packets.TUnsubscribeMqtt5).Packet)
		require.NoError(t, err)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Unsuback].Get(packets.TUnsubackMqtt5).RawBytes, buf)
	require.Equal(t, int64(-1), atomic.LoadInt64(&s.Info.Subscriptions))
}

func TestServerProcessPacketUnsubscribeInvalid(t *testing.T) {
	s := newServer()
	cl, _, _ := newTestClient()
	err := s.processPacket(cl, *packets.TPacketData[packets.Unsubscribe].Get(packets.TUnsubscribeSpecQosMustPacketID).Packet)
	require.Error(t, err)
	require.ErrorIs(t, err, packets.ErrProtocolViolationNoPacketID)
}

func TestServerReceivePacketError(t *testing.T) {
	s := newServer()
	cl, _, _ := newTestClient()
	err := s.receivePacket(cl, *packets.TPacketData[packets.Unsubscribe].Get(packets.TUnsubscribeSpecQosMustPacketID).Packet)
	require.Error(t, err)
	require.ErrorIs(t, err, packets.ErrProtocolViolationNoPacketID)
}

func TestServerRecievePacketDisconnectClientZeroNonZero(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	cl.Properties.Props.SessionExpiryInterval = 0
	cl.Properties.ProtocolVersion = 5
	cl.Properties.Props.RequestProblemInfo = 0
	cl.Properties.Props.RequestProblemInfoFlag = true
	go func() {
		err := s.receivePacket(cl, *packets.TPacketData[packets.Disconnect].Get(packets.TDisconnectMqtt5).Packet)
		require.Error(t, err)
		require.ErrorIs(t, err, packets.ErrProtocolViolationZeroNonZeroExpiry)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Disconnect].Get(packets.TDisconnectZeroNonZeroExpiry).RawBytes, buf)
}

func TestServerRecievePacketDisconnectClient(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5

	go func() {
		err := s.DisconnectClient(cl, packets.CodeDisconnect)
		require.NoError(t, err)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, []byte{
		packets.Disconnect << 4, 2, // fixed header
		packets.CodeDisconnect.Code, // Reason Code
		0,                           // Properties Length
	}, buf)
}

func TestServerRecievePacketDisconnectClientV3(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 4 // MQTT v3.1.1

	go func() {
		err := s.DisconnectClient(cl, packets.CodeDisconnect)
		require.NoError(t, err)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)

	// The server-to-client Disconnect packet was introduced in MQTT v5. A v3
	// client is closed without one rather than being sent a packet type its
	// own specification only defines in the other direction.
	require.Empty(t, buf)
}

func TestServerProcessPacketDisconnect(t *testing.T) {
	s := newServer()
	cl, _, _ := newTestClient()
	cl.Properties.Props.SessionExpiryInterval = 30
	cl.Properties.ProtocolVersion = 5

	s.loop.willDelayed.Add(cl.ID, packets.Packet{TopicName: "a/b/c", Payload: []byte("hello")})
	require.Equal(t, 1, s.loop.willDelayed.Len())

	err := s.processPacket(cl, *packets.TPacketData[packets.Disconnect].Get(packets.TDisconnectMqtt5).Packet)
	require.NoError(t, err)

	require.Equal(t, 0, s.loop.willDelayed.Len())
	require.True(t, cl.Closed())
	require.Equal(t, time.Now().Unix(), atomic.LoadInt64(&cl.State.disconnected))
}

func TestServerProcessPacketDisconnectNonZeroExpiryViolation(t *testing.T) {
	s := newServer()
	cl, _, _ := newTestClient()
	cl.Properties.Props.SessionExpiryInterval = 0
	cl.Properties.ProtocolVersion = 5
	cl.Properties.Props.RequestProblemInfo = 0
	cl.Properties.Props.RequestProblemInfoFlag = true

	err := s.processPacket(cl, *packets.TPacketData[packets.Disconnect].Get(packets.TDisconnectMqtt5).Packet)
	require.Error(t, err)
	require.ErrorIs(t, err, packets.ErrProtocolViolationZeroNonZeroExpiry)
}

func TestServerProcessPacketDisconnectDisconnectWithWillMessage(t *testing.T) {
	s := newServer()
	cl, _, _ := newTestClient()
	cl.Properties.Props.SessionExpiryInterval = 30
	cl.Properties.ProtocolVersion = 5

	s.loop.willDelayed.Add(cl.ID, packets.Packet{TopicName: "a/b/c", Payload: []byte("hello")})
	require.Equal(t, 1, s.loop.willDelayed.Len())

	err := s.processPacket(cl, *packets.TPacketData[packets.Disconnect].Get(packets.TDisconnectMqtt5DisconnectWithWillMessage).Packet)
	require.Error(t, err)

	require.Equal(t, 1, s.loop.willDelayed.Len())
	require.False(t, cl.Closed())
}

func TestServerProcessPacketAuth(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()

	go func() {
		err := s.processPacket(cl, *packets.TPacketData[packets.Auth].Get(packets.TAuth).Packet)
		require.NoError(t, err)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, []byte{}, buf)
}

func TestServerProcessPacketAuthInvalidReason(t *testing.T) {
	s := newServer()
	cl, _, _ := newTestClient()
	pkx := *packets.TPacketData[packets.Auth].Get(packets.TAuth).Packet
	pkx.ReasonCode = 99
	err := s.processPacket(cl, pkx)
	require.Error(t, err)
	require.ErrorIs(t, packets.ErrProtocolViolationInvalidReason, err)
}

func TestServerProcessPacketAuthFailure(t *testing.T) {
	s := newServer()
	cl, _, _ := newTestClient()

	hook := new(modifiedHookBase)
	hook.fail = true
	err := s.AddHook(hook, nil)
	require.NoError(t, err)

	err = s.processAuth(cl, *packets.TPacketData[packets.Auth].Get(packets.TAuth).Packet)
	require.Error(t, err)
	require.ErrorIs(t, errTestHook, err)
}

func TestServerSendLWT(t *testing.T) {
	s := newServer()
	_ = s.Serve()
	defer s.Close()

	sender, _, w1 := newTestClient()
	sender.ID = "sender"
	sender.Properties.Will = Will{
		Flag:      1,
		TopicName: "a/b/c",
		Payload:   []byte("hello mochi"),
	}
	s.Clients.Add(sender)

	receiver, r2, w2 := newTestClient()
	receiver.ID = "receiver"
	s.Clients.Add(receiver)
	s.Topics.Subscribe(receiver.ID, packets.Subscription{Filter: "a/b/c", Qos: 0})

	require.Equal(t, int64(0), atomic.LoadInt64(&s.Info.PacketsReceived))
	require.Equal(t, 0, len(s.Topics.Messages("a/b/c")))

	receiverBuf := make(chan []byte)
	go func() {
		buf, err := io.ReadAll(r2)
		require.NoError(t, err)
		receiverBuf <- buf
	}()

	go func() {
		s.sendLWT(sender)
		time.Sleep(time.Millisecond * 10)
		_ = w1.Close()
		_ = w2.Close()
	}()

	require.Equal(t, packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).RawBytes, <-receiverBuf)
}

func TestServerSendLWTRetain(t *testing.T) {
	s := newServer()
	_ = s.Serve()
	defer s.Close()

	sender, _, w1 := newTestClient()
	sender.ID = "sender"
	sender.Properties.Will = Will{
		Flag:      1,
		TopicName: "a/b/c",
		Payload:   []byte("hello mochi"),
		Retain:    true,
	}
	s.Clients.Add(sender)

	receiver, r2, w2 := newTestClient()
	receiver.ID = "receiver"
	s.Clients.Add(receiver)
	s.Topics.Subscribe(receiver.ID, packets.Subscription{Filter: "a/b/c", Qos: 0})

	require.Equal(t, int64(0), atomic.LoadInt64(&s.Info.PacketsReceived))
	require.Equal(t, 0, len(s.Topics.Messages("a/b/c")))

	receiverBuf := make(chan []byte)
	go func() {
		buf, err := io.ReadAll(r2)
		require.NoError(t, err)
		receiverBuf <- buf
	}()

	go func() {
		s.sendLWT(sender)
		time.Sleep(time.Millisecond * 10)
		_ = w1.Close()
		_ = w2.Close()
	}()

	require.Equal(t, packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).RawBytes, <-receiverBuf)
}

func TestServerSendLWTDelayed(t *testing.T) {
	s := newServer()
	cl1, _, _ := newTestClient()
	cl1.ID = "cl1"
	cl1.Properties.Will = Will{
		Flag:              1,
		TopicName:         "a/b/c",
		Payload:           []byte("hello mochi"),
		Retain:            true,
		WillDelayInterval: 2,
	}
	s.Clients.Add(cl1)

	cl2, r, w := newTestClient()
	cl2.ID = "cl2"
	s.Clients.Add(cl2)
	require.True(t, s.Topics.Subscribe(cl2.ID, packets.Subscription{Filter: "a/b/c"}))

	go func() {
		s.sendLWT(cl1)
		pk, ok := s.loop.willDelayed.Get(cl1.ID)
		require.True(t, ok)
		pk.Expiry = time.Now().Unix() - 1 // set back expiry time
		s.loop.willDelayed.Add(cl1.ID, pk)
		require.Equal(t, 1, s.loop.willDelayed.Len())
		s.sendDelayedLWT(time.Now().Unix())
		require.Equal(t, 0, s.loop.willDelayed.Len())
		time.Sleep(time.Millisecond)
		_ = w.Close()
	}()

	recv := make(chan []byte)
	go func() {
		buf, err := io.ReadAll(r)
		require.NoError(t, err)
		recv <- buf
	}()

	require.Equal(t, packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).RawBytes, <-recv)
}

func TestServerReadStore(t *testing.T) {
	s := newServer()
	hook := new(modifiedHookBase)
	_ = s.AddHook(hook, nil)

	hook.failAt = 1 // clients
	err := s.readStore()
	require.Error(t, err)

	hook.failAt = 2 // subscriptions
	err = s.readStore()
	require.Error(t, err)

	hook.failAt = 3 // inflight
	err = s.readStore()
	require.Error(t, err)

	hook.failAt = 4 // retained
	err = s.readStore()
	require.Error(t, err)

	hook.failAt = 5 // sys info
	err = s.readStore()
	require.Error(t, err)
}

func TestServerLoadClients(t *testing.T) {
	v := []storage.Client{
		{ID: "mochi"},
		{ID: "zen"},
		{ID: "mochi-co"},
		{ID: "v3-clean", ProtocolVersion: 4, Clean: true},
		{ID: "v3-not-clean", ProtocolVersion: 4, Clean: false},
		{
			ID:              "v5-clean",
			ProtocolVersion: 5,
			Clean:           true,
			Properties: storage.ClientProperties{
				SessionExpiryInterval: 10,
			},
		},
		{
			ID:              "v5-expire-interval-0",
			ProtocolVersion: 5,
			Properties: storage.ClientProperties{
				SessionExpiryInterval: 0,
			},
		},
		{
			ID:              "v5-expire-interval-not-0",
			ProtocolVersion: 5,
			Properties: storage.ClientProperties{
				SessionExpiryInterval: 10,
			},
		},
	}

	s := newServer()
	require.Equal(t, 0, s.Clients.Len())
	s.loadClients(v)
	require.Equal(t, 6, s.Clients.Len())
	cl, ok := s.Clients.Get("mochi")
	require.True(t, ok)
	require.Equal(t, "mochi", cl.ID)

	_, ok = s.Clients.Get("v3-clean")
	require.False(t, ok)
	_, ok = s.Clients.Get("v3-not-clean")
	require.True(t, ok)
	_, ok = s.Clients.Get("v5-clean")
	require.True(t, ok)
	_, ok = s.Clients.Get("v5-expire-interval-0")
	require.False(t, ok)
	_, ok = s.Clients.Get("v5-expire-interval-not-0")
	require.True(t, ok)
}

func TestServerLoadSubscriptions(t *testing.T) {
	v := []storage.Subscription{
		{ID: "sub1", Client: "mochi", Filter: "a/b/c"},
		{ID: "sub2", Client: "mochi", Filter: "d/e/f", Qos: 1},
		{ID: "sub3", Client: "mochi", Filter: "h/i/j", Qos: 2},
	}

	s := newServer()
	cl, _, _ := newTestClient()
	s.Clients.Add(cl)
	require.Equal(t, 0, cl.State.Subscriptions.Len())
	s.loadSubscriptions(v)
	require.Equal(t, 3, cl.State.Subscriptions.Len())
}

func TestServerLoadInflightMessages(t *testing.T) {
	s := newServer()
	s.loadClients([]storage.Client{
		{ID: "mochi"},
		{ID: "zen"},
		{ID: "mochi-co"},
	})

	require.Equal(t, 3, s.Clients.Len())

	v := []storage.Message{
		{Client: "mochi", Origin: "mochi", PacketID: 1, Payload: []byte("hello world"), TopicName: "a/b/c"},
		{Client: "mochi", Origin: "mochi", PacketID: 2, Payload: []byte("yes"), TopicName: "a/b/c"},
		{Client: "zen", Origin: "zen", PacketID: 3, Payload: []byte("hello world"), TopicName: "a/b/c"},
		{Client: "mochi-co", Origin: "mochi-co", PacketID: 4, Payload: []byte("hello world"), TopicName: "a/b/c"},
	}
	s.loadInflight(v)

	cl, ok := s.Clients.Get("mochi")
	require.True(t, ok)
	require.Equal(t, "mochi", cl.ID)

	msg, ok := cl.State.Inflight.Get(2)
	require.True(t, ok)
	require.Equal(t, []byte{'y', 'e', 's'}, msg.Payload)
	require.Equal(t, "a/b/c", msg.TopicName)

	cl, ok = s.Clients.Get("mochi-co")
	require.True(t, ok)
	msg, ok = cl.State.Inflight.Get(4)
	require.True(t, ok)
}

func TestServerLoadRetainedMessages(t *testing.T) {
	s := newServer()

	v := []storage.Message{
		{Origin: "mochi", FixedHeader: packets.FixedHeader{Retain: true}, Payload: []byte("hello world"), TopicName: "a/b/c"},
		{Origin: "mochi-co", FixedHeader: packets.FixedHeader{Retain: true}, Payload: []byte("yes"), TopicName: "d/e/f"},
		{Origin: "zen", FixedHeader: packets.FixedHeader{Retain: true}, Payload: []byte("hello world"), TopicName: "h/i/j"},
	}
	s.loadRetained(v)
	require.Equal(t, 1, len(s.Topics.Messages("a/b/c")))
	require.Equal(t, 1, len(s.Topics.Messages("d/e/f")))
	require.Equal(t, 1, len(s.Topics.Messages("h/i/j")))
	require.Equal(t, 0, len(s.Topics.Messages("w/x/y")))
}

func TestServerClose(t *testing.T) {
	s := newServer()

	hook := new(modifiedHookBase)
	_ = s.AddHook(hook, nil)

	cl, r, _ := newTestClient()
	cl.Net.Listener = "t1"
	cl.Properties.ProtocolVersion = 5
	s.Clients.Add(cl)

	err := s.AddListener(listeners.NewMockListener("t1", ":1882"))
	require.NoError(t, err)
	_ = s.Serve()

	// receive the disconnect
	recv := make(chan []byte)
	go func() {
		buf, err := io.ReadAll(r)
		require.NoError(t, err)
		recv <- buf
	}()

	time.Sleep(time.Millisecond)
	require.Equal(t, 1, s.Listeners.Len())

	listener, ok := s.Listeners.Get("t1")
	require.Equal(t, true, ok)
	require.Equal(t, true, listener.(*listeners.MockListener).IsServing())

	_ = s.Close()
	time.Sleep(time.Millisecond)
	require.Equal(t, false, listener.(*listeners.MockListener).IsServing())
	require.Equal(t, packets.TPacketData[packets.Disconnect].Get(packets.TDisconnectShuttingDown).RawBytes, <-recv)
}

// blockingConnectHook holds attachClient inside OnConnect until it is
// released, so that Close is called while a client is demonstrably still
// being attached rather than at a moment the test had to guess.
type blockingConnectHook struct {
	HookBase
	entered chan struct{}
	release chan struct{}
}

func (h *blockingConnectHook) ID() string { return "blocking-connect" }

func (h *blockingConnectHook) Provides(b byte) bool { return b == OnConnect }

func (h *blockingConnectHook) OnConnect(cl *Client, pk packets.Packet) error {
	close(h.entered)
	<-h.release
	return nil
}

// Close does not return while a client is still being attached, including a
// client handed to EstablishConnection directly rather than accepted by a
// listener. Registering only inside the listener's establisher covered the
// second and not the first, so Close returned early for an embedder feeding
// the server its own connections and for every test in this file.
func TestServerCloseWaitsForDirectEstablishConnection(t *testing.T) {
	h := &blockingConnectHook{entered: make(chan struct{}), release: make(chan struct{})}

	s := newServer()
	require.NoError(t, s.AddHook(h, nil))

	r, w := net.Pipe()
	attached := make(chan error, 1)
	go func() {
		attached <- s.EstablishConnection("tcp", r)
	}()
	go func() {
		_, _ = w.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectClean).RawBytes)
	}()
	go func() { _, _ = io.ReadAll(w) }()

	<-h.entered // the client is inside attachClient and cannot leave

	closed := make(chan struct{})
	go func() {
		_ = s.Close()
		close(closed)
	}()

	select {
	case <-closed:
		t.Fatal("Close returned while a client was still being attached")
	case <-time.After(250 * time.Millisecond):
	}

	// Hang the connection up from this side before waiting, because Close
	// now genuinely waits for it: the client is attached under a listener
	// id no listener owns, so closeListenerClients never reaches it and it
	// would otherwise sit until its keepalive expired.
	close(h.release)
	_ = w.Close()
	<-closed
	<-attached
}

// A connection arriving after shutdown has latched is refused rather than
// attached, whichever way it arrives.
func TestServerEstablishConnectionRefusedAfterClose(t *testing.T) {
	s := newServer()
	require.NoError(t, s.Close())

	r, w := net.Pipe()
	defer w.Close()

	// Nothing is written to the pipe: a refused connection is closed
	// without its CONNECT being read, so this returns without waiting for
	// one. Unlatched it blocks in readConnectionPacket instead, which is
	// what the deadline below catches.
	done := make(chan error, 1)
	go func() { done <- s.EstablishConnection("tcp", r) }()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("a connection arriving after shutdown was attached rather than refused")
	}

	require.Equal(t, 0, s.Clients.Len())
}

func TestServerClearExpiredInflights(t *testing.T) {
	s := New(nil)
	require.NotNil(t, s)
	s.Options.Capabilities.MaximumMessageExpiryInterval = 4

	n := time.Now().Unix()
	cl, _, _ := newTestClient()
	cl.ops.info = s.Info

	cl.State.Inflight.Set(packets.Packet{PacketID: 1, Expiry: n - 1})
	cl.State.Inflight.Set(packets.Packet{PacketID: 2, Expiry: n - 2})
	cl.State.Inflight.Set(packets.Packet{PacketID: 3, Created: n - 3}) // within bounds
	cl.State.Inflight.Set(packets.Packet{PacketID: 5, Created: n - 5}) // over max server expiry limit
	cl.State.Inflight.Set(packets.Packet{PacketID: 7, Created: n})

	s.Clients.Add(cl)

	require.Len(t, cl.State.Inflight.GetAll(false), 5)
	s.clearExpiredInflights(n)
	require.Len(t, cl.State.Inflight.GetAll(false), 2)
	require.Equal(t, int64(-3), s.Info.Inflight)

	s.Options.Capabilities.MaximumMessageExpiryInterval = 0
	cl.State.Inflight.Set(packets.Packet{PacketID: 8, Expiry: n - 8})
	s.clearExpiredInflights(n)
	require.Len(t, cl.State.Inflight.GetAll(false), 3)
}

func TestServerClearExpiredRetained(t *testing.T) {
	s := New(nil)
	require.NotNil(t, s)
	s.Options.Capabilities.MaximumMessageExpiryInterval = 4

	n := time.Now().Unix()
	s.Topics.Retained.Add("a/b/c", packets.Packet{ProtocolVersion: 5, Created: n, Expiry: n - 1})
	s.Topics.Retained.Add("d/e/f", packets.Packet{ProtocolVersion: 5, Created: n, Expiry: n - 2})
	s.Topics.Retained.Add("g/h/i", packets.Packet{ProtocolVersion: 5, Created: n - 3}) // within bounds
	s.Topics.Retained.Add("j/k/l", packets.Packet{ProtocolVersion: 5, Created: n - 5}) // over max server expiry limit
	s.Topics.Retained.Add("m/n/o", packets.Packet{ProtocolVersion: 5, Created: n})

	require.Len(t, s.Topics.Retained.GetAll(), 5)
	s.clearExpiredRetainedMessages(n)
	require.Len(t, s.Topics.Retained.GetAll(), 2)

	s.Topics.Retained.Add("p/q/r", packets.Packet{Created: n, Expiry: n - 1})
	s.Topics.Retained.Add("s/t/u", packets.Packet{Created: n, Expiry: n - 2}) // expiry is ineffective for v3.
	s.Topics.Retained.Add("v/w/x", packets.Packet{Created: n - 3})            // within bounds for v3
	s.Topics.Retained.Add("y/z/1", packets.Packet{Created: n - 5})            // over max server expiry limit
	require.Len(t, s.Topics.Retained.GetAll(), 6)
	s.clearExpiredRetainedMessages(n)
	require.Len(t, s.Topics.Retained.GetAll(), 5)

	s.Options.Capabilities.MaximumMessageExpiryInterval = 0
	s.Topics.Retained.Add("2/3/4", packets.Packet{Created: n - 8})
	s.clearExpiredRetainedMessages(n)
	require.Len(t, s.Topics.Retained.GetAll(), 6)
}

func TestServerClearExpiredClients(t *testing.T) {
	s := New(nil)
	require.NotNil(t, s)

	n := time.Now().Unix()

	cl, _, _ := newTestClient()
	cl.ID = "cl"
	s.Clients.Add(cl)

	// No Expiry
	cl0, _, _ := newTestClient()
	cl0.ID = "c0"
	cl0.State.disconnected = n - 10
	cl0.State.cancelOpen()
	cl0.Properties.ProtocolVersion = 5
	cl0.Properties.Props.SessionExpiryInterval = 12
	cl0.Properties.Props.SessionExpiryIntervalFlag = true
	s.Clients.Add(cl0)

	// Normal Expiry
	cl1, _, _ := newTestClient()
	cl1.ID = "c1"
	cl1.State.disconnected = n - 10
	cl1.State.cancelOpen()
	cl1.Properties.ProtocolVersion = 5
	cl1.Properties.Props.SessionExpiryInterval = 8
	cl1.Properties.Props.SessionExpiryIntervalFlag = true
	s.Clients.Add(cl1)

	// No Expiry, indefinite session
	cl2, _, _ := newTestClient()
	cl2.ID = "c2"
	cl2.State.disconnected = n - 10
	cl2.State.cancelOpen()
	cl2.Properties.ProtocolVersion = 5
	cl2.Properties.Props.SessionExpiryInterval = 0
	cl2.Properties.Props.SessionExpiryIntervalFlag = true
	s.Clients.Add(cl2)

	require.Equal(t, 4, s.Clients.Len())

	s.clearExpiredClients(n)
	require.Equal(t, 2, s.Clients.Len())
}

func TestLoadServerInfoRestoreOnRestart(t *testing.T) {
	s := New(nil)
	s.Options.Capabilities.Compatibilities.RestoreSysInfoOnRestart = true
	info := system.Info{
		BytesReceived: 60,
	}

	s.loadServerInfo(info)
	require.Equal(t, int64(60), s.Info.BytesReceived)
}

func TestItoa(t *testing.T) {
	i := int64(22)
	require.Equal(t, "22", Int64toa(i))
}

func TestServerSubscribe(t *testing.T) {
	handler := func(cl *Client, sub packets.Subscription, pk packets.Packet) {}

	s := newServerWithInlineClient()
	require.NotNil(t, s)

	tt := []struct {
		desc       string
		filter     string
		identifier int
		handler    InlineSubFn
		expect     error
	}{
		{
			desc:       "subscribe",
			filter:     "a/b/c",
			identifier: 1,
			handler:    handler,
			expect:     nil,
		},
		{
			desc:       "re-subscribe",
			filter:     "a/b/c",
			identifier: 1,
			handler:    handler,
			expect:     nil,
		},
		{
			desc:       "subscribe d/e/f",
			filter:     "d/e/f",
			identifier: 1,
			handler:    handler,
			expect:     nil,
		},
		{
			desc:       "re-subscribe d/e/f by different identifier",
			filter:     "d/e/f",
			identifier: 2,
			handler:    handler,
			expect:     nil,
		},
		{
			desc:       "subscribe different handler",
			filter:     "a/b/c",
			identifier: 1,
			handler:    func(cl *Client, sub packets.Subscription, pk packets.Packet) {},
			expect:     nil,
		},
		{
			desc:       "subscribe $SYS/info",
			filter:     "$SYS/info",
			identifier: 1,
			handler:    handler,
			expect:     nil,
		},
		{
			desc:       "subscribe invalid ###",
			filter:     "###",
			identifier: 1,
			handler:    handler,
			expect:     packets.ErrTopicFilterInvalid,
		},
		{
			desc:       "subscribe invalid handler",
			filter:     "a/b/c",
			identifier: 1,
			handler:    nil,
			expect:     packets.ErrInlineSubscriptionHandlerInvalid,
		},
	}

	for _, tx := range tt {
		t.Run(tx.desc, func(t *testing.T) {
			require.Equal(t, tx.expect, s.Subscribe(tx.filter, tx.identifier, tx.handler))
		})
	}
}

func TestServerSubscribeNoInlineClient(t *testing.T) {
	s := newServer()
	err := s.Subscribe("a/b/c", 1, func(cl *Client, sub packets.Subscription, pk packets.Packet) {})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInlineClientNotEnabled)
}

func TestServerUnsubscribe(t *testing.T) {
	handler := func(cl *Client, sub packets.Subscription, pk packets.Packet) {
		// handler logic
	}

	s := newServerWithInlineClient()
	err := s.Subscribe("a/b/c", 1, handler)
	require.Nil(t, err)

	err = s.Subscribe("d/e/f", 1, handler)
	require.Nil(t, err)

	err = s.Subscribe("d/e/f", 2, handler)
	require.Nil(t, err)

	err = s.Unsubscribe("a/b/c", 1)
	require.Nil(t, err)

	err = s.Unsubscribe("d/e/f", 1)
	require.Nil(t, err)

	err = s.Unsubscribe("d/e/f", 2)
	require.Nil(t, err)

	err = s.Unsubscribe("not/exist", 1)
	require.Nil(t, err)

	err = s.Unsubscribe("#/#/invalid", 1)
	require.Equal(t, packets.ErrTopicFilterInvalid, err)
}

func TestServerUnsubscribeNoInlineClient(t *testing.T) {
	s := newServer()
	err := s.Unsubscribe("a/b/c", 1)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInlineClientNotEnabled)
}

func TestPublishToInlineSubscriber(t *testing.T) {
	s := newServerWithInlineClient()
	finishCh := make(chan bool)
	err := s.Subscribe("a/b/c", 1, func(cl *Client, sub packets.Subscription, pk packets.Packet) {
		require.Equal(t, []byte("hello mochi"), pk.Payload)
		require.Equal(t, InlineClientId, cl.ID)
		require.Equal(t, LocalListener, cl.Net.Listener)
		require.Equal(t, "a/b/c", sub.Filter)
		require.Equal(t, 1, sub.Identifier)
		finishCh <- true
	})
	require.Nil(t, err)

	go func() {
		pkx := *packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).Packet
		s.publishToSubscribers(pkx)
	}()

	require.Equal(t, true, <-finishCh)
}

func TestPublishToInlineSubscribersDifferentFilter(t *testing.T) {
	s := newServerWithInlineClient()
	subNumber := 2
	finishCh := make(chan bool, subNumber)

	err := s.Subscribe("a/b/c", 1, func(cl *Client, sub packets.Subscription, pk packets.Packet) {
		require.Equal(t, []byte("hello mochi"), pk.Payload)
		require.Equal(t, InlineClientId, cl.ID)
		require.Equal(t, LocalListener, cl.Net.Listener)
		require.Equal(t, "a/b/c", sub.Filter)
		require.Equal(t, 1, sub.Identifier)
		finishCh <- true
	})
	require.Nil(t, err)

	err = s.Subscribe("z/e/n", 1, func(cl *Client, sub packets.Subscription, pk packets.Packet) {
		require.Equal(t, []byte("mochi mochi"), pk.Payload)
		require.Equal(t, InlineClientId, cl.ID)
		require.Equal(t, LocalListener, cl.Net.Listener)
		require.Equal(t, "z/e/n", sub.Filter)
		require.Equal(t, 1, sub.Identifier)
		finishCh <- true
	})
	require.Nil(t, err)

	go func() {
		pkx := *packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).Packet
		s.publishToSubscribers(pkx)

		pkx = *packets.TPacketData[packets.Publish].Get(packets.TPublishCopyBasic).Packet
		s.publishToSubscribers(pkx)
	}()

	for i := 0; i < subNumber; i++ {
		require.Equal(t, true, <-finishCh)
	}
}

func TestPublishToInlineSubscribersDifferentIdentifier(t *testing.T) {
	s := newServerWithInlineClient()
	subNumber := 2
	finishCh := make(chan bool, subNumber)

	err := s.Subscribe("a/b/c", 1, func(cl *Client, sub packets.Subscription, pk packets.Packet) {
		require.Equal(t, []byte("hello mochi"), pk.Payload)
		require.Equal(t, InlineClientId, cl.ID)
		require.Equal(t, LocalListener, cl.Net.Listener)
		require.Equal(t, "a/b/c", sub.Filter)
		require.Equal(t, 1, sub.Identifier)
		finishCh <- true
	})
	require.Nil(t, err)

	err = s.Subscribe("a/b/c", 2, func(cl *Client, sub packets.Subscription, pk packets.Packet) {
		require.Equal(t, []byte("hello mochi"), pk.Payload)
		require.Equal(t, InlineClientId, cl.ID)
		require.Equal(t, LocalListener, cl.Net.Listener)
		require.Equal(t, "a/b/c", sub.Filter)
		require.Equal(t, 2, sub.Identifier)
		finishCh <- true
	})
	require.Nil(t, err)

	go func() {
		pkx := *packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).Packet
		s.publishToSubscribers(pkx)
	}()

	for i := 0; i < subNumber; i++ {
		require.Equal(t, true, <-finishCh)
	}
}

func TestServerSubscribeWithRetain(t *testing.T) {
	s := newServerWithInlineClient()
	subNumber := 1
	finishCh := make(chan bool, subNumber)

	retained := s.Topics.RetainMessage(*packets.TPacketData[packets.Publish].Get(packets.TPublishRetain).Packet)
	require.Equal(t, int64(1), retained)

	err := s.Subscribe("a/b/c", 1, func(cl *Client, sub packets.Subscription, pk packets.Packet) {
		require.Equal(t, []byte("hello mochi"), pk.Payload)
		require.Equal(t, InlineClientId, cl.ID)
		require.Equal(t, LocalListener, cl.Net.Listener)
		require.Equal(t, "a/b/c", sub.Filter)
		require.Equal(t, 1, sub.Identifier)
		finishCh <- true
	})
	require.Nil(t, err)
	require.Equal(t, true, <-finishCh)
}

func TestServerSubscribeWithRetainDifferentFilter(t *testing.T) {
	s := newServerWithInlineClient()
	subNumber := 2
	finishCh := make(chan bool, subNumber)

	retained := s.Topics.RetainMessage(*packets.TPacketData[packets.Publish].Get(packets.TPublishRetain).Packet)
	require.Equal(t, int64(1), retained)
	retained = s.Topics.RetainMessage(*packets.TPacketData[packets.Publish].Get(packets.TPublishCopyBasic).Packet)
	require.Equal(t, int64(1), retained)

	err := s.Subscribe("a/b/c", 1, func(cl *Client, sub packets.Subscription, pk packets.Packet) {
		require.Equal(t, []byte("hello mochi"), pk.Payload)
		require.Equal(t, InlineClientId, cl.ID)
		require.Equal(t, LocalListener, cl.Net.Listener)
		require.Equal(t, "a/b/c", sub.Filter)
		require.Equal(t, 1, sub.Identifier)
		finishCh <- true
	})
	require.Nil(t, err)

	err = s.Subscribe("z/e/n", 1, func(cl *Client, sub packets.Subscription, pk packets.Packet) {
		require.Equal(t, []byte("mochi mochi"), pk.Payload)
		require.Equal(t, InlineClientId, cl.ID)
		require.Equal(t, LocalListener, cl.Net.Listener)
		require.Equal(t, "z/e/n", sub.Filter)
		require.Equal(t, 1, sub.Identifier)
		finishCh <- true
	})
	require.Nil(t, err)

	for i := 0; i < subNumber; i++ {
		require.Equal(t, true, <-finishCh)
	}
}

func TestServerSubscribeWithRetainDifferentIdentifier(t *testing.T) {
	s := newServerWithInlineClient()
	subNumber := 2
	finishCh := make(chan bool, subNumber)

	retained := s.Topics.RetainMessage(*packets.TPacketData[packets.Publish].Get(packets.TPublishRetain).Packet)
	require.Equal(t, int64(1), retained)

	err := s.Subscribe("a/b/c", 1, func(cl *Client, sub packets.Subscription, pk packets.Packet) {
		require.Equal(t, []byte("hello mochi"), pk.Payload)
		require.Equal(t, InlineClientId, cl.ID)
		require.Equal(t, LocalListener, cl.Net.Listener)
		require.Equal(t, "a/b/c", sub.Filter)
		require.Equal(t, 1, sub.Identifier)
		finishCh <- true
	})
	require.Nil(t, err)

	err = s.Subscribe("a/b/c", 2, func(cl *Client, sub packets.Subscription, pk packets.Packet) {
		require.Equal(t, []byte("hello mochi"), pk.Payload)
		require.Equal(t, InlineClientId, cl.ID)
		require.Equal(t, LocalListener, cl.Net.Listener)
		require.Equal(t, "a/b/c", sub.Filter)
		require.Equal(t, 2, sub.Identifier)
		finishCh <- true
	})
	require.Nil(t, err)

	for i := 0; i < subNumber; i++ {
		require.Equal(t, true, <-finishCh)
	}
}

func TestMinimum(t *testing.T) {
	require.EqualValues(t, 0, minimum(0, 0))
	require.EqualValues(t, 1, minimum(0, 1))
	require.EqualValues(t, 1, minimum(1, 0))
	require.EqualValues(t, 10, minimum(10, 20))
	require.EqualValues(t, 20, minimum(30, 20))
	require.EqualValues(t, -1, minimum(-1, 0)) // negative values are not used, but included here for completeness
	require.EqualValues(t, -1, minimum(-1, 20))
	require.EqualValues(t, -2, minimum(-1, -2))
}

// refuseFilterHook refuses one filter of a SUBSCRIBE by setting its reason
// code, and leaves the rest of the packet alone.
type refuseFilterHook struct {
	HookBase
	filter string
	code   byte
}

func (h *refuseFilterHook) ID() string { return "refuse-filter" }

func (h *refuseFilterHook) Provides(b byte) bool {
	return bytes.Contains([]byte{OnSubscribe}, []byte{b})
}

func (h *refuseFilterHook) OnSubscribe(cl *Client, pk packets.Packet) packets.Packet {
	pk.ReasonCodes = make([]byte, len(pk.Filters))
	for i, sub := range pk.Filters {
		if sub.Filter == h.filter {
			pk.ReasonCodes[i] = h.code
		}
	}
	return pk
}

// An OnSubscribe hook may refuse a single filter and say why. Without it
// the only hook-driven refusal is OnACLCheck, which can answer 0x87 and
// nothing else, so a hook rejecting a filter for any other reason has to
// mislabel it as an authorization failure or disconnect the client.
func TestServerProcessSubscribeHookReasonCodes(t *testing.T) {
	s := newServer()
	require.NoError(t, s.AddHook(&refuseFilterHook{
		filter: "d/e/f/g/h/i",
		code:   packets.ErrTopicFilterInvalid.Code,
	}, nil))
	_ = s.Serve()

	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5
	s.Clients.Add(cl)

	go func() {
		err := s.processSubscribe(cl, *packets.TPacketData[packets.Subscribe].Get(packets.TSubscribeMany).Packet)
		require.NoError(t, err)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)

	// The reason codes are the SUBACK payload, so they are the last byte
	// per filter. The refused one carries the hook's code; the others are
	// the granted QoS, untouched.
	require.Equal(t,
		[]byte{0, packets.ErrTopicFilterInvalid.Code, 2},
		buf[len(buf)-3:])

	// A refused filter is not subscribed, or the client would receive
	// messages for a subscription the SUBACK told it it did not have.
	require.Empty(t, s.Topics.Subscribers("d/e/f/g/h/i").Subscriptions)
	require.Contains(t, s.Topics.Subscribers("a/b").Subscriptions, cl.ID)
	require.Contains(t, s.Topics.Subscribers("x/y/z").Subscriptions, cl.ID)

	_, ok := cl.State.Subscriptions.Get("d/e/f/g/h/i")
	require.False(t, ok)
}

// A successful PUBACK must carry a PUBACK reason code. 0x01 is Granted QoS
// 1, a SUBACK code, and [MQTT-3.4.2-1] requires one of the codes §3.4.2.1
// lists — 0x00, 0x10, and the 0x80-and-above refusals.
//
// It is invisible until the publish carries a property the acknowledgement
// keeps, because with none the encoder may omit the reason byte and its
// absence reads as 0x00. A User Property is enough: the byte is then written
// and the wrong value appears.
func TestServerProcessPacketPublishQos1AckReasonCode(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5
	s.Clients.Add(cl)

	pk := *packets.TPacketData[packets.Publish].Get(packets.TPublishQos1Mqtt5).Packet
	pk.Properties.User = []packets.UserProperty{{Key: "mine", Val: "ok"}}

	go func() {
		require.NoError(t, s.processPacket(cl, pk))
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.NotEmpty(t, buf)
	require.Equal(t, packets.Puback, buf[0]>>4)

	// Fixed header, then the two packet-identifier bytes, then the reason.
	require.Greater(t, len(buf), 4, "the reason byte is written once there are properties")
	require.Equal(t, packets.CodeSuccess.Code, buf[4],
		"a successful PUBACK must carry a PUBACK reason code, not a granted-QoS one")
}

func TestServerUnsubscribeCountsAndAnswersOnlyWhatItRemoved(t *testing.T) {
	s := newServer()

	// Two clients on one filter, so the filter node outlives the first
	// client's subscription on it.
	a, ar, _ := newTestClient()
	a.ID = "cl-a"
	a.Properties.ProtocolVersion = 5
	b, br, _ := newTestClient()
	b.ID = "cl-b"
	b.Properties.ProtocolVersion = 5

	// **Each packet arrives on a channel rather than in a buffer.**
	// net.Pipe hands the write back as soon as the bytes are copied, so a
	// reader appending them to a slice can still be behind when the next
	// assertion runs — the first version of this test read the previous
	// acknowledgement and reported the fix broken.
	acks := make(chan []byte, 8)
	drain := func(c net.Conn) {
		buf := make([]byte, 256)
		for {
			n, err := c.Read(buf)
			if n > 0 {
				acks <- append([]byte(nil), buf[:n]...)
			}
			if err != nil {
				return
			}
		}
	}
	go drain(ar)
	go drain(br)
	reasonCode := func() byte {
		select {
		case pk := <-acks:
			require.NotEmpty(t, pk)
			return pk[len(pk)-1]
		case <-time.After(2 * time.Second):
			t.Fatal("no acknowledgement arrived")
			return 0
		}
	}

	subscribe := func(cl *Client) {
		require.NoError(t, s.processSubscribe(cl, packets.Packet{
			FixedHeader: packets.FixedHeader{Type: packets.Subscribe},
			PacketID:    1,
			Filters:     packets.Subscriptions{{Filter: "a/b", Qos: 0}},
		}))
		<-acks // the SUBACK, which this test is not about
	}
	unsubscribe := func(cl *Client) byte {
		require.NoError(t, s.processUnsubscribe(cl, packets.Packet{
			FixedHeader: packets.FixedHeader{Type: packets.Unsubscribe},
			PacketID:    2,
			Filters:     packets.Subscriptions{{Filter: "a/b"}},
		}))
		return reasonCode()
	}

	subscribe(a)
	subscribe(b)
	require.Equal(t, int64(2), atomic.LoadInt64(&s.Info.Subscriptions))

	require.Equal(t, packets.CodeSuccess.Code, unsubscribe(a))
	require.Equal(t, int64(1), atomic.LoadInt64(&s.Info.Subscriptions))

	// cl-a has already gone and cl-b is holding the node open, so there is
	// nothing of cl-a's to remove: no decrement, and 0x11 rather than
	// success.
	require.Equal(t, packets.CodeNoSubscriptionExisted.Code, unsubscribe(a))
	require.Equal(t, int64(1), atomic.LoadInt64(&s.Info.Subscriptions))

	require.Equal(t, packets.CodeSuccess.Code, unsubscribe(b))
	require.Equal(t, int64(0), atomic.LoadInt64(&s.Info.Subscriptions))
}

// refusedHook records the CONNECT refusals a hook is told about.
type refusedHook struct {
	HookBase
	mu    sync.Mutex
	seen  []string
	codes []packets.Code
}

func (h *refusedHook) ID() string { return "refused" }

func (h *refusedHook) Provides(b byte) bool { return b == OnConnectRefused }

func (h *refusedHook) OnConnectRefused(cl *Client, pk packets.Packet, code packets.Code) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.seen = append(h.seen, cl.ID)
	h.codes = append(h.codes, code)
}

func (h *refusedHook) refusals() ([]string, []packets.Code) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.seen...), append([]packets.Code(nil), h.codes...)
}

// A CONNECT refused before any other hook runs tells a hook which client it
// was. Without it the only record is the returned code, which says nothing
// about who was refused.
func TestOnConnectRefusedNamesTheClient(t *testing.T) {
	cc := NewDefaultServerCapabilities()
	cc.MaximumClients = 0
	s := New(&Options{Logger: logger, Capabilities: cc})
	hook := new(refusedHook)
	require.NoError(t, s.AddHook(hook, nil))
	require.NoError(t, s.AddHook(new(AllowHook), nil))
	defer s.Close()

	r, w := net.Pipe()
	o := make(chan error)
	go func() { o <- s.EstablishConnection("tcp", r) }()
	go func() {
		_, _ = w.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectClean).RawBytes)
	}()
	go func() { _, _ = io.ReadAll(w) }()

	require.ErrorIs(t, <-o, packets.ErrServerBusy)
	_ = w.Close()

	ids, codes := hook.refusals()
	require.Len(t, ids, 1)
	require.Equal(t, "zen", ids[0]) // the fixture CONNECT's client id

	// The fixture is a 3.1.1 CONNECT, and 3.1.1 has no Server Busy: the
	// client is sent Server Unavailable, so that is what the hook is told.
	require.Equal(t, packets.ErrServerUnavailable, codes[0])
}

// The same refusal to an MQTT 5 client, which is sent Server Busy. The two
// versions are the whole of the difference, so both are checked -- a hook
// told a code the client never received cannot name what it was turned away
// with.
func TestOnConnectRefusedNamesTheCodeTheClientWasSentV5(t *testing.T) {
	cc := NewDefaultServerCapabilities()
	cc.MaximumClients = 0
	s := New(&Options{Logger: logger, Capabilities: cc})
	hook := new(refusedHook)
	require.NoError(t, s.AddHook(hook, nil))
	require.NoError(t, s.AddHook(new(AllowHook), nil))
	defer s.Close()

	r, w := net.Pipe()
	o := make(chan error)
	go func() { o <- s.EstablishConnection("tcp", r) }()
	go func() {
		_, _ = w.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectMqtt5).RawBytes)
	}()
	go func() { _, _ = io.ReadAll(w) }()

	require.ErrorIs(t, <-o, packets.ErrServerBusy)
	_ = w.Close()

	_, codes := hook.refusals()
	require.Len(t, codes, 1)
	require.Equal(t, packets.ErrServerBusy, codes[0])
}

// The same for a refusal validateConnect makes — here an unacceptable
// protocol version, which is the case an operator meets when they close the
// door on an older fleet.
func TestOnConnectRefusedCoversValidateConnect(t *testing.T) {
	cc := NewDefaultServerCapabilities()
	cc.MinimumProtocolVersion = 5
	s := New(&Options{Logger: logger, Capabilities: cc})
	hook := new(refusedHook)
	require.NoError(t, s.AddHook(hook, nil))
	require.NoError(t, s.AddHook(new(AllowHook), nil))
	defer s.Close()

	r, w := net.Pipe()
	o := make(chan error)
	go func() { o <- s.EstablishConnection("tcp", r) }()
	go func() {
		_, _ = w.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectClean).RawBytes)
	}()
	go func() { _, _ = io.ReadAll(w) }()

	require.ErrorIs(t, <-o, packets.ErrUnsupportedProtocolVersion)
	_ = w.Close()

	ids, codes := hook.refusals()
	require.Len(t, ids, 1)
	require.Equal(t, "zen", ids[0]) // the fixture CONNECT's client id
	require.Equal(t, packets.ErrUnsupportedProtocolVersion, codes[0])
}

// A server with no such hook behaves exactly as before, which is what makes
// this additive: HookBase's no-op means every existing hook opts out.
func TestOnConnectRefusedIsOptional(t *testing.T) {
	cc := NewDefaultServerCapabilities()
	cc.MaximumClients = 0
	s := New(&Options{Logger: logger, Capabilities: cc})
	require.NoError(t, s.AddHook(new(AllowHook), nil))
	defer s.Close()

	r, w := net.Pipe()
	o := make(chan error)
	go func() { o <- s.EstablishConnection("tcp", r) }()
	go func() {
		_, _ = w.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectClean).RawBytes)
	}()
	go func() { _, _ = io.ReadAll(w) }()

	require.ErrorIs(t, <-o, packets.ErrServerBusy)
	_ = w.Close()
}

// A hook that refuses a publish by WRAPPING a packets.Code, which is what
// fmt.Errorf("%w") produces and what a hook returning context with its
// refusal will do.
type wrappingRefusalHook struct {
	HookBase
}

func (h *wrappingRefusalHook) ID() string { return "wrapping-refusal" }

func (h *wrappingRefusalHook) Provides(b byte) bool { return b == OnPublish }

func (h *wrappingRefusalHook) OnPublish(cl *Client, pk packets.Packet) (packets.Packet, error) {
	return pk, fmt.Errorf("refused by policy: %w", packets.ErrNotAuthorized)
}

// A refusal carrying a wrapped code is answered, not panicked on.
//
// errors.As succeeds through a wrapper, so a type assertion on the error
// itself takes the broker down on an ordinary publish refusal - a hook is
// entitled to add context to the code it returns.
func TestPublishRefusedWithAWrappedCodeDoesNotPanic(t *testing.T) {
	s := newServer()
	require.NoError(t, s.AddHook(new(wrappingRefusalHook), nil))
	defer s.Close()

	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5
	// Nothing here drains the pipe, and the refusal is written to it. Left
	// alone the write now waits out the keepalive bound, so this test would
	// take fifteen seconds to assert something that happens immediately.
	cl.ops.options.ClientNetWriteTimeout = 50 * time.Millisecond
	s.Clients.Add(cl)

	go func() {
		_, _ = io.ReadAll(w)
	}()

	pk := *packets.TPacketData[packets.Publish].Get(packets.TPublishQos1).Packet
	require.NotPanics(t, func() {
		_ = s.processPublish(cl, pk)
	})

	_ = r.Close()
	_ = w.Close()
}

// DisconnectClient always disconnects, even for a v3 client under
// PassiveClientDisconnect.
//
// That option means "the client was sent a DISCONNECT and will close the
// connection itself". A v3 client is sent nothing, so honouring it there
// would leave the connection open after the server decided to end it.
func TestDisconnectClientClosesAV3ClientUnderPassiveDisconnect(t *testing.T) {
	s := newServer()
	s.Options.Capabilities.Compatibilities.PassiveClientDisconnect = true
	defer s.Close()

	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 4 // 3.1.1: no server-to-client DISCONNECT
	s.Clients.Add(cl)

	go func() { _, _ = io.ReadAll(w) }()

	_ = s.DisconnectClient(cl, packets.ErrNotAuthorized)

	require.True(t, cl.Closed(), "a v3 client the server chose to disconnect was left connected")

	_ = r.Close()
	_ = w.Close()
}
