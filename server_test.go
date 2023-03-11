// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-co
// SPDX-FileContributor: mochi-co

package mqtt

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"os"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mochi-co/mqtt/v2/hooks/storage"
	"github.com/mochi-co/mqtt/v2/listeners"
	"github.com/mochi-co/mqtt/v2/packets"
	"github.com/mochi-co/mqtt/v2/system"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

var logger = zerolog.New(os.Stderr).With().Timestamp().Logger().Level(zerolog.Disabled)

type ProtocolTest []struct {
	protocolVersion byte
	in              packets.TPacketCase
	out             packets.TPacketCase
	data            map[string]any
}

type AllowHook struct {
	HookBase
}

func (h *AllowHook) ID() string {
	return "allow-all-auth"
}

func (h *AllowHook) Provides(b byte) bool {
	return bytes.Contains([]byte{OnConnectAuthenticate, OnACLCheck}, []byte{b})
}

func (h *AllowHook) OnConnectAuthenticate(cl *Client, pk packets.Packet) bool { return true }
func (h *AllowHook) OnACLCheck(cl *Client, topic string, write bool) bool     { return true }

type DelayHook struct {
	HookBase
	DisconnectDelay time.Duration
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
	cc := *DefaultServerCapabilities
	cc.MaximumMessageExpiryInterval = 0
	cc.ReceiveMaximum = 0

	s := New(&Options{
		Logger:       &logger,
		Capabilities: &cc,
	})
	s.AddHook(new(AllowHook), nil)
	return s
}

func TestOptionsSetDefaults(t *testing.T) {
	opts := &Options{}
	opts.ensureDefaults()

	require.Equal(t, defaultSysTopicInterval, opts.SysTopicResendInterval)
	require.Equal(t, DefaultServerCapabilities, opts.Capabilities)

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
}

func TestNewNilOpts(t *testing.T) {
	s := New(nil)
	require.NotNil(t, s)
	require.NotNil(t, s.Options)
}

func TestServerNewClient(t *testing.T) {
	s := New(nil)
	s.Log = &logger
	r, _ := net.Pipe()

	cl := s.NewClient(r, "testing", "test", false)
	require.NotNil(t, cl)
	require.Equal(t, "test", cl.ID)
	require.Equal(t, "testing", cl.Net.Listener)
	require.False(t, cl.Net.Inline)
	require.NotNil(t, cl.State.Inflight.internal)
	require.NotNil(t, cl.State.Subscriptions)
	require.NotNil(t, cl.State.TopicAliases)
	require.Equal(t, defaultKeepalive, cl.State.keepalive)
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
	s.Log = &logger
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

func TestServerAddListenerInitFailure(t *testing.T) {
	s := newServer()
	defer s.Close()

	require.NotNil(t, s)

	m := listeners.NewMockListener("t1", ":1882")
	m.ErrListen = true
	err := s.AddListener(m)
	require.Error(t, err)
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
		r.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectMqtt311).RawBytes)
		r.Close()
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
		r.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectMalFixedHeader).RawBytes)
		r.Close()
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
		r.Write(packets.TPacketData[packets.Connack].Get(packets.TConnackAcceptedNoSession).RawBytes)
		r.Close()
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
		r.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectMalProtocolName).RawBytes)
		r.Close()
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
		w.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectClean).RawBytes)
		w.Write(packets.TPacketData[packets.Disconnect].Get(packets.TDisconnect).RawBytes)
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
	for _, v := range s.Clients.GetAll() {
		require.ErrorIs(t, v.StopCause(), packets.CodeDisconnect) // true error is disconnect
	}

	require.Equal(t, packets.TPacketData[packets.Connack].Get(packets.TConnackAcceptedNoSession).RawBytes, <-recv)

	w.Close()
	r.Close()

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
		w.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectClean).RawBytes)
		w.Close()
	}()

	err := <-o
	require.Error(t, err)
	require.ErrorIs(t, err, io.ErrClosedPipe)

	r.Close()
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
		w.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectMqtt5).RawBytes)
		w.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectClean).RawBytes) // second connect error
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
	for _, v := range s.Clients.GetAll() {
		require.ErrorIs(t, v.StopCause(), packets.ErrProtocolViolationSecondConnect) // true error is disconnect
	}

	ret := <-recv
	require.Equal(t, append(
		packets.TPacketData[packets.Connack].Get(packets.TConnackMinCleanMqtt5).RawBytes,
		packets.TPacketData[packets.Disconnect].Get(packets.TDisconnectSecondConnect).RawBytes...),
		ret,
	)

	w.Close()
	r.Close()
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
		w.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectMqtt311).RawBytes)
		time.Sleep(time.Millisecond) // we want to receive the queued inflight, so we need to wait a moment before sending the disconnect.
		w.Write(packets.TPacketData[packets.Disconnect].Get(packets.TDisconnect).RawBytes)
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
	for _, v := range s.Clients.GetAll() {
		require.ErrorIs(t, v.StopCause(), packets.CodeDisconnect) // true error is disconnect
	}

	connackPlusPacket := append(
		packets.TPacketData[packets.Connack].Get(packets.TConnackAcceptedSessionExists).RawBytes,
		packets.TPacketData[packets.Publish].Get(packets.TPublishQos1Dup).RawBytes...,
	)
	require.Equal(t, connackPlusPacket, <-recv)
	require.Equal(t, packets.TPacketData[packets.Disconnect].Get(packets.TDisconnectTakeover).RawBytes, <-takeover)

	time.Sleep(time.Microsecond * 100)
	w.Close()
	r.Close()

	clw, ok := s.Clients.Get(packets.TPacketData[packets.Connect].Get(packets.TConnectMqtt311).Packet.Connect.ClientIdentifier)
	require.True(t, ok)
	require.NotEmpty(t, clw.State.Subscriptions)

	// Prevent sequential takeover memory-bloom.
	require.Empty(t, cl.State.Subscriptions.GetAll())
}

// See https://github.com/mochi-co/mqtt/issues/173
func TestEstablishConnectionInheritExistingTrueTakeover(t *testing.T) {
	s := newServer()
	d := new(DelayHook)
	d.DisconnectDelay = time.Millisecond * 200
	s.AddHook(d, nil)
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
		w1.Write(cl1RawBytes)
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
		w2.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectUserPass).RawBytes)
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

	w2.Write(packets.TPacketData[packets.Disconnect].Get(packets.TDisconnect).RawBytes)
	require.NoError(t, <-o2)
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
		w.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectMqtt311).RawBytes)
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
	cl.Properties.Clean = true
	cl.State.Subscriptions.Add("a/b/c", packets.Subscription{Filter: "a/b/c", Qos: 1})
	s.Clients.Add(cl)

	r, w := net.Pipe()
	o := make(chan error)
	go func() {
		o <- s.EstablishConnection("tcp", r)
	}()

	go func() {
		w.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectMqtt311).RawBytes)
		w.Write(packets.TPacketData[packets.Disconnect].Get(packets.TDisconnect).RawBytes)
	}()

	// receive the disconnect
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
	for _, v := range s.Clients.GetAll() {
		require.ErrorIs(t, v.StopCause(), packets.CodeDisconnect) // true error is disconnect
	}

	require.Equal(t, packets.TPacketData[packets.Connack].Get(packets.TConnackAcceptedNoSession).RawBytes, <-recv)
	require.Equal(t, packets.TPacketData[packets.Disconnect].Get(packets.TDisconnect).RawBytes, <-takeover)

	w.Close()
	r.Close()

	clw, ok := s.Clients.Get(packets.TPacketData[packets.Connect].Get(packets.TConnectMqtt311).Packet.Connect.ClientIdentifier)
	require.True(t, ok)
	require.Equal(t, 0, clw.State.Subscriptions.Len())
}

func TestEstablishConnectionBadAuthentication(t *testing.T) {
	s := New(&Options{
		Logger: &logger,
	})
	defer s.Close()

	r, w := net.Pipe()
	o := make(chan error)
	go func() {
		o <- s.EstablishConnection("tcp", r)
	}()

	go func() {
		w.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectClean).RawBytes)
		w.Write(packets.TPacketData[packets.Disconnect].Get(packets.TDisconnect).RawBytes)
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

	w.Close()
	r.Close()
}

func TestEstablishConnectionBadAuthenticationAckFailure(t *testing.T) {
	s := New(&Options{
		Logger: &logger,
	})
	defer s.Close()

	r, w := net.Pipe()
	o := make(chan error)
	go func() {
		o <- s.EstablishConnection("tcp", r)
	}()

	go func() {
		w.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectClean).RawBytes)
		w.Close()
	}()

	err := <-o
	require.Error(t, err)
	require.ErrorIs(t, err, io.ErrClosedPipe)

	r.Close()
}

func TestServerEstablishConnectionInvalidConnect(t *testing.T) {
	s := newServer()

	r, w := net.Pipe()
	o := make(chan error)
	go func() {
		o <- s.EstablishConnection("tcp", r)
	}()

	go func() {
		w.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectMalReservedBit).RawBytes)
		w.Write(packets.TPacketData[packets.Disconnect].Get(packets.TDisconnect).RawBytes)
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

	r.Close()
}

// See https://github.com/mochi-co/mqtt/issues/178
func TestServerEstablishConnectionZeroByteUsernameIsValid(t *testing.T) {
	s := newServer()

	r, w := net.Pipe()
	o := make(chan error)
	go func() {
		o <- s.EstablishConnection("tcp", r)
	}()

	go func() {
		w.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectZeroByteUsername).RawBytes)
		w.Write(packets.TPacketData[packets.Disconnect].Get(packets.TDisconnect).RawBytes)
	}()

	// receive the connack error
	go func() {
		_, err := io.ReadAll(w)
		require.NoError(t, err)
	}()

	err := <-o
	require.NoError(t, err)

	r.Close()
}

func TestServerEstablishConnectionInvalidConnectAckFailure(t *testing.T) {
	s := newServer()

	r, w := net.Pipe()
	o := make(chan error)
	go func() {
		o <- s.EstablishConnection("tcp", r)
	}()

	go func() {
		w.Write(packets.TPacketData[packets.Connect].Get(packets.TConnectMalReservedBit).RawBytes)
		w.Close()
	}()

	err := <-o
	require.Error(t, err)
	require.ErrorIs(t, err, io.ErrClosedPipe)

	r.Close()
}

func TestServerEstablishConnectionBadPacket(t *testing.T) {
	s := newServer()

	r, w := net.Pipe()
	o := make(chan error)
	go func() {
		o <- s.EstablishConnection("tcp", r)
	}()

	go func() {
		w.Write(packets.TPacketData[packets.Connect].Get(packets.TConnackBadProtocolVersion).RawBytes)
		w.Write(packets.TPacketData[packets.Disconnect].Get(packets.TDisconnect).RawBytes)
	}()

	err := <-o
	require.Error(t, err)
	require.ErrorIs(t, err, packets.ErrProtocolViolationRequireFirstConnect)

	r.Close()
}

func TestServerSendConnack(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5
	s.Options.Capabilities.ServerKeepAlive = 20
	s.Options.Capabilities.MaximumQos = 1
	cl.Properties.Props = packets.Properties{
		AssignedClientID: "mochi",
	}
	go func() {
		err := s.sendConnack(cl, packets.CodeSuccess, true)
		require.NoError(t, err)
		w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Connack].Get(packets.TConnackMinMqtt5).RawBytes, buf)
}

func TestServerSendConnackFailureReason(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5
	s.Options.Capabilities.ServerKeepAlive = 20
	go func() {
		err := s.sendConnack(cl, packets.ErrUnspecifiedError, true)
		require.NoError(t, err)
		w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Connack].Get(packets.TConnackInvalidMinMqtt5).RawBytes, buf)
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
		err := s.sendConnack(cl, packets.CodeSuccess, false)
		require.NoError(t, err)
		w.Close()
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
		w.Close()
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
	s.Serve()
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
		w1.Close()
		time.Sleep(time.Millisecond * 10)
		w2.Close()
	}()

	require.Equal(t, packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).RawBytes, <-receiverBuf)
}

func TestServerDirectPublishAndReceive(t *testing.T) {
	s := newServer()
	s.Serve()
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
		w1.Close()
		time.Sleep(time.Millisecond * 10)
		w2.Close()
	}()

	require.Equal(t, packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).RawBytes, <-receiverBuf)
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
	s.Serve()
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
		w1.Close()
		w2.Close()
	}()

	require.Equal(t, packets.TPacketData[packets.Publish].Get(packets.TPublishRetain).RawBytes, <-receiverBuf)
	require.Equal(t, 1, len(s.Topics.Messages("a/b/c")))
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
		w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Publish].Get(packets.TPublishQos1).RawBytes, buf)
	require.Equal(t, int64(0), atomic.LoadInt64(&s.Info.Inflight))
	require.Equal(t, int32(4), cl.State.Inflight.sendQuota)
}

func TestServerProcessPacketPublishAckFailure(t *testing.T) {
	s := newServer()
	s.Serve()
	defer s.Close()

	cl, _, w := newTestClient()
	s.Clients.Add(cl)

	w.Close()
	err := s.processPublish(cl, *packets.TPacketData[packets.Publish].Get(packets.TPublishQos2).Packet)
	require.Error(t, err)
	require.ErrorIs(t, err, io.ErrClosedPipe)
}

func TestServerProcessPacketPublishMaximumReceive(t *testing.T) {
	s := newServer()
	s.Serve()
	defer s.Close()

	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5
	cl.State.Inflight.ResetReceiveQuota(0)
	s.Clients.Add(cl)

	go func() {
		err := s.processPacket(cl, *packets.TPacketData[packets.Publish].Get(packets.TPublishQos1).Packet)
		require.Error(t, err)
		require.ErrorIs(t, err, packets.ErrReceiveMaximum)
		w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Disconnect].Get(packets.TDisconnectReceiveMaximum).RawBytes, buf)
}

func TestServerProcessPublishInvalidTopic(t *testing.T) {
	s := newServer()
	s.Serve()
	defer s.Close()
	cl, _, _ := newTestClient()
	err := s.processPublish(cl, *packets.TPacketData[packets.Publish].Get(packets.TPublishSpecDenySysTopic).Packet)
	require.NoError(t, err) // $SYS topics should be ignored?
}

func TestServerProcessPublishACLCheckDeny(t *testing.T) {
	s := New(&Options{
		Logger: &logger,
	})
	s.Serve()
	defer s.Close()
	cl, _, _ := newTestClient()
	err := s.processPublish(cl, *packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).Packet)
	require.NoError(t, err) // ACL check fails silently
}

func TestServerProcessPublishOnMessageRecvRejected(t *testing.T) {
	s := newServer()
	require.NotNil(t, s)
	hook := new(modifiedHookBase)
	hook.fail = true
	hook.err = packets.ErrRejectPacket

	err := s.AddHook(hook, nil)
	require.NoError(t, err)

	s.Serve()
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
		w.Close()
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
		w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Puback].Get(packets.TPuback).RawBytes, buf)
	require.Equal(t, int64(0), atomic.LoadInt64(&s.Info.Inflight))
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
		w.Close()
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
		w.Close()
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
		w.Close()
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
		w.Close()
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
		w.Close()
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
		w1.Close()
		w2.Close()
		w3.Close()
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
		w1.Close()
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
		time.Sleep(time.Millisecond)
		w.Close()
	}()

	receiverBuf := make(chan []byte)
	go func() {
		buf, err := io.ReadAll(r)
		require.NoError(t, err)
		receiverBuf <- buf
	}()

	require.Equal(t, packets.TPacketData[packets.Publish].Get(packets.TPublishSubscriberIdentifier).RawBytes, <-receiverBuf)
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
		s.publishToClient(cl, packets.Subscription{Filter: "a/b/c", Qos: 2}, pkx)
		time.Sleep(time.Microsecond * 100)
		w.Close()
	}()

	receiverBuf := make(chan []byte)
	go func() {
		buf, err := io.ReadAll(r)
		require.NoError(t, err)
		receiverBuf <- buf
	}()

	require.Equal(t, packets.TPacketData[packets.Publish].Get(packets.TPublishQos1).RawBytes, <-receiverBuf)
}

func TestPublishToClientExceedClientWritesPending(t *testing.T) {
	s := newServer()

	_, w := net.Pipe()
	cl := newClient(w, &ops{
		info:  new(system.Info),
		hooks: new(Hooks),
		log:   &logger,
		options: &Options{
			Capabilities: &Capabilities{
				MaximumClientWritesPending: 3,
			},
		},
	})

	s.Clients.Add(cl)

	for i := int32(0); i < cl.ops.options.Capabilities.MaximumClientWritesPending; i++ {
		cl.State.outbound <- new(packets.Packet)
		atomic.AddInt32(&cl.State.outboundQty, 1)
	}

	_, err := s.publishToClient(cl, packets.Subscription{Filter: "a/b/c", Qos: 2}, packets.Packet{})
	require.Error(t, err)
	require.ErrorIs(t, packets.ErrPendingClientWritesExceeded, err)
}

func TestPublishToClientServerTopicAlias(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5
	cl.Properties.Props.TopicAliasMaximum = 5
	s.Clients.Add(cl)

	go func() {
		pkx := *packets.TPacketData[packets.Publish].Get(packets.TPublishBasicMqtt5).Packet
		s.publishToClient(cl, packets.Subscription{Filter: pkx.TopicName}, pkx)
		s.publishToClient(cl, packets.Subscription{Filter: pkx.TopicName}, pkx)
		time.Sleep(time.Millisecond)
		w.Close()
	}()

	receiverBuf := make(chan []byte)
	go func() {
		buf, err := io.ReadAll(r)
		require.NoError(t, err)
		receiverBuf <- buf
	}()

	ret := <-receiverBuf
	pk1 := make([]byte, len(packets.TPacketData[packets.Publish].Get(packets.TPublishBasicMqtt5).RawBytes))
	pk2 := make([]byte, len(packets.TPacketData[packets.Publish].Get(packets.TPublishBasicMqtt5).RawBytes)-5)
	copy(pk1, ret[:len(packets.TPacketData[packets.Publish].Get(packets.TPublishBasicMqtt5).RawBytes)])
	copy(pk2, ret[len(packets.TPacketData[packets.Publish].Get(packets.TPublishBasicMqtt5).RawBytes):])
	require.Equal(t, append(pk1, pk2...), ret)
}

func TestPublishToClientExhaustedPacketID(t *testing.T) {
	s := newServer()
	cl, _, _ := newTestClient()
	for i := uint32(0); i <= cl.ops.options.Capabilities.maximumPacketID; i++ {
		cl.State.Inflight.Set(packets.Packet{PacketID: uint16(i)})
	}

	_, err := s.publishToClient(cl, packets.Subscription{Filter: "a/b/c"}, *packets.TPacketData[packets.Publish].Get(packets.TPublishQos1).Packet)
	require.Error(t, err)
	require.ErrorIs(t, err, packets.ErrQuotaExceeded)
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
		s.processPacket(cl2, pkx)
		time.Sleep(time.Millisecond)
		w2.Close()
		w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).RawBytes, buf)
}

func TestPublishToSubscribersExhaustedSendQuota(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	s.Clients.Add(cl)
	cl.State.Inflight.sendQuota = 0

	subbed := s.Topics.Subscribe(cl.ID, packets.Subscription{Filter: "a/b/c", Qos: 2})
	require.True(t, subbed)

	// coverage: subscriber publish errors are non-returnable
	// can we hook into zerolog ?
	r.Close()
	pkx := *packets.TPacketData[packets.Publish].Get(packets.TPublishQos1).Packet
	pkx.PacketID = 0
	s.publishToSubscribers(pkx)
	time.Sleep(time.Millisecond)
	w.Close()
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
	// can we hook into zerolog ?
	r.Close()
	pkx := *packets.TPacketData[packets.Publish].Get(packets.TPublishQos1).Packet
	pkx.PacketID = 0
	s.publishToSubscribers(pkx)
	time.Sleep(time.Millisecond)
	w.Close()
}

func TestPublishToSubscribersNoConnection(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	s.Clients.Add(cl)
	subbed := s.Topics.Subscribe(cl.ID, packets.Subscription{Filter: "a/b/c", Qos: 2})
	require.True(t, subbed)

	// coverage: subscriber publish errors are non-returnable
	// can we hook into zerolog ?
	r.Close()
	s.publishToSubscribers(*packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).Packet)
	time.Sleep(time.Millisecond)
	w.Close()
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
		w.Close()
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
		w.Close()
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

	w.Close()
	s.publishRetainedToClient(cl, sub, false)
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
	w.Close()

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
	w.Close()

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
	w.Close()

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
	w.Close()

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
			w.Close()

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

			recv := make(chan []byte)
			go func() { // receive the ack
				buf, err := io.ReadAll(r)
				require.NoError(t, err)
				recv <- buf
			}()

			if i == 0 {
				s.publishToSubscribers(*tx.in.Packet)
			} else {
				err := s.processPacket(cl, *tx.in.Packet)
				require.NoError(t, err)
			}

			time.Sleep(time.Millisecond)
			w.Close()

			if i != 2 {
				require.Equal(t, tx.out.RawBytes, <-recv)
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
		w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Suback].Get(packets.TSubackMqtt5).RawBytes, buf)
}

func TestServerProcessPacketSubscribePacketIDInUse(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5
	cl.State.Inflight.Set(packets.Packet{PacketID: 15, FixedHeader: packets.FixedHeader{Type: packets.Publish}})

	pkx := *packets.TPacketData[packets.Subscribe].Get(packets.TSubscribeMqtt5).Packet
	pkx.PacketID = 15
	go func() {
		err := s.processPacket(cl, pkx)
		require.NoError(t, err)
		w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Suback].Get(packets.TSubackPacketIDInUse).RawBytes, buf)
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
		w.Close()
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
		w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Suback].Get(packets.TSubackInvalidSharedNoLocal).RawBytes, buf)
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
		w.Close()
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
		w.Close()
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
		w.Close()
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
		w.Close()
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
		w.Close()
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
	r.Close()
	err := s.processSubscribe(cl, *packets.TPacketData[packets.Subscribe].Get(packets.TSubscribe).Packet)
	require.Error(t, err)
	require.ErrorIs(t, err, io.ErrClosedPipe)
}

func TestServerProcessSubscribeACLCheckDeny(t *testing.T) {
	s := New(&Options{
		Logger: &logger,
	})
	s.Serve()
	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5

	go func() {
		err := s.processSubscribe(cl, *packets.TPacketData[packets.Subscribe].Get(packets.TSubscribe).Packet)
		require.NoError(t, err)
		w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Suback].Get(packets.TSubackDeny).RawBytes, buf)
}

func TestServerProcessSubscribeACLCheckDenyObscure(t *testing.T) {
	s := New(&Options{
		Logger: &logger,
	})
	s.Serve()
	s.Options.Capabilities.Compatibilities.ObscureNotAuthorized = true
	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5

	go func() {
		err := s.processSubscribe(cl, *packets.TPacketData[packets.Subscribe].Get(packets.TSubscribe).Packet)
		require.NoError(t, err)
		w.Close()
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
		w.Close()
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
		w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Unsuback].Get(packets.TUnsubackMqtt5).RawBytes, buf)
	require.Equal(t, int64(-1), atomic.LoadInt64(&s.Info.Subscriptions))
}

func TestServerProcessPacketUnsubscribePackedIDInUse(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5
	cl.State.Inflight.Set(packets.Packet{PacketID: 15, FixedHeader: packets.FixedHeader{Type: packets.Publish}})
	go func() {
		err := s.processPacket(cl, *packets.TPacketData[packets.Unsubscribe].Get(packets.TUnsubscribeMqtt5).Packet)
		require.NoError(t, err)
		w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Unsuback].Get(packets.TUnsubackPacketIDInUse).RawBytes, buf)
	require.Equal(t, int64(0), atomic.LoadInt64(&s.Info.Subscriptions))
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
		w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Disconnect].Get(packets.TDisconnectZeroNonZeroExpiry).RawBytes, buf)
}

func TestServerRecievePacketDisconnectClient(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()

	go func() {
		err := s.DisconnectClient(cl, packets.CodeDisconnect)
		require.NoError(t, err)
		w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Disconnect].Get(packets.TDisconnect).RawBytes, buf)
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
	require.Equal(t, uint32(1), atomic.LoadUint32(&cl.State.done))
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

func TestServerProcessPacketAuth(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()

	go func() {
		err := s.processPacket(cl, *packets.TPacketData[packets.Auth].Get(packets.TAuth).Packet)
		require.NoError(t, err)
		w.Close()
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
	s.Serve()
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
		w1.Close()
		w2.Close()
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
		w.Close()
	}()

	recv := make(chan []byte)
	go func() {
		buf, err := io.ReadAll(r)
		require.NoError(t, err)
		recv <- buf
	}()

	require.Equal(t, packets.TPacketData[packets.Publish].Get(packets.TPublishRetain).RawBytes, <-recv)
}

func TestServerReadStore(t *testing.T) {
	s := newServer()
	hook := new(modifiedHookBase)
	s.AddHook(hook, nil)

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
	}

	s := newServer()
	require.Equal(t, 0, s.Clients.Len())
	s.loadClients(v)
	require.Equal(t, 3, s.Clients.Len())
	cl, ok := s.Clients.Get("mochi")
	require.True(t, ok)
	require.Equal(t, "mochi", cl.ID)
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
		{Origin: "mochi", PacketID: 1, Payload: []byte("hello world"), TopicName: "a/b/c"},
		{Origin: "mochi", PacketID: 2, Payload: []byte("yes"), TopicName: "a/b/c"},
		{Origin: "zen", PacketID: 3, Payload: []byte("hello world"), TopicName: "a/b/c"},
		{Origin: "mochi-co", PacketID: 4, Payload: []byte("hello world"), TopicName: "a/b/c"},
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
	s.AddHook(hook, nil)

	cl, r, _ := newTestClient()
	cl.Net.Listener = "t1"
	cl.Properties.ProtocolVersion = 5
	s.Clients.Add(cl)

	err := s.AddListener(listeners.NewMockListener("t1", ":1882"))
	require.NoError(t, err)
	s.Serve()

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

	s.Close()
	time.Sleep(time.Millisecond)
	require.Equal(t, false, listener.(*listeners.MockListener).IsServing())
	require.Equal(t, packets.TPacketData[packets.Disconnect].Get(packets.TDisconnectShuttingDown).RawBytes, <-recv)
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
}

func TestServerClearExpiredRetained(t *testing.T) {
	s := New(nil)
	require.NotNil(t, s)
	s.Options.Capabilities.MaximumMessageExpiryInterval = 4

	n := time.Now().Unix()
	s.Topics.Retained.Add("a/b/c", packets.Packet{Created: n, Expiry: n - 1})
	s.Topics.Retained.Add("d/e/f", packets.Packet{Created: n, Expiry: n - 2})
	s.Topics.Retained.Add("g/h/i", packets.Packet{Created: n - 3}) // within bounds
	s.Topics.Retained.Add("j/k/l", packets.Packet{Created: n - 5}) // over max server expiry limit
	s.Topics.Retained.Add("m/n/o", packets.Packet{Created: n})

	require.Len(t, s.Topics.Retained.GetAll(), 5)
	s.clearExpiredRetainedMessages(n)
	require.Len(t, s.Topics.Retained.GetAll(), 2)
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
	cl0.State.done = 1
	cl0.Properties.ProtocolVersion = 5
	cl0.Properties.Props.SessionExpiryInterval = 12
	cl0.Properties.Props.SessionExpiryIntervalFlag = true
	s.Clients.Add(cl0)

	// Normal Expiry
	cl1, _, _ := newTestClient()
	cl1.ID = "c1"
	cl1.State.disconnected = n - 10
	cl1.State.done = 1
	cl1.Properties.ProtocolVersion = 5
	cl1.Properties.Props.SessionExpiryInterval = 8
	cl1.Properties.Props.SessionExpiryIntervalFlag = true
	s.Clients.Add(cl1)

	// No Expiry, indefinite session
	cl2, _, _ := newTestClient()
	cl2.ID = "c2"
	cl2.State.disconnected = n - 10
	cl2.State.done = 1
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

func TestAtomicItoa(t *testing.T) {
	i := int64(22)
	ip := &i
	require.Equal(t, "22", AtomicItoa(ip))
}
