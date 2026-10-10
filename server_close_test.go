// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package mqtt

import (
	"io"
	"net"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"

	"github.com/stretchr/testify/require"
)

// closeBudget bounds every Close in these tests. Unpatched, the Close under
// test never returns, so the budget is the assertion; it is far below the
// default ConnectTimeout, so a pass cannot come from the pre-CONNECT deadline.
const closeBudget = 5 * time.Second

// signalListener wraps a TCP net.Listener and reports when Accept has handed
// out a connection and when the listener has been closed.
type signalListener struct {
	net.Listener
	accepted  chan struct{}
	closed    chan struct{}
	closeOnce sync.Once
}

func newSignalListener(t *testing.T) *signalListener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	return &signalListener{
		Listener: ln,
		accepted: make(chan struct{}, 64),
		closed:   make(chan struct{}),
	}
}

func (l *signalListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err == nil {
		l.accepted <- struct{}{}
	}
	return c, err
}

func (l *signalListener) Close() error {
	err := l.Listener.Close()
	l.closeOnce.Do(func() { close(l.closed) })
	return err
}

// gateConnectHook blocks OnConnect until released, holding a client between
// reading its CONNECT and registering it.
type gateConnectHook struct {
	HookBase
	entered chan struct{}
	release chan struct{}
}

func (h *gateConnectHook) ID() string { return "gate-connect" }

func (h *gateConnectHook) Provides(b byte) bool { return b == OnConnect }

func (h *gateConnectHook) OnConnect(cl *Client, pk packets.Packet) error {
	h.entered <- struct{}{}
	<-h.release
	return nil
}

func startCloseTestServer(t *testing.T, opts *Options, hooks ...Hook) (*Server, *signalListener) {
	t.Helper()
	if opts == nil {
		opts = new(Options)
	}
	opts.Logger = logger
	s := New(opts)
	require.NoError(t, s.AddHook(new(AllowHook), nil))
	for _, h := range hooks {
		require.NoError(t, s.AddHook(h, nil))
	}
	ln := newSignalListener(t)
	require.NoError(t, s.AddListener(listeners.NewNet("t1", ln)))
	require.NoError(t, s.Serve())
	return s, ln
}

func closeAsync(s *Server) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		_ = s.Close()
		close(done)
	}()
	return done
}

func waitFor(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(closeBudget):
		t.Fatalf("timed out waiting for %s", what)
	}
}

func requireClosed(t *testing.T, done <-chan struct{}, why string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(closeBudget):
		buf := make([]byte, 1<<20)
		t.Logf("goroutines:\n%s", buf[:runtime.Stack(buf, true)])
		t.Fatalf("Server.Close did not return within %s: %s", closeBudget, why)
	}
}

func dialRaw(t *testing.T, ln net.Listener) net.Conn {
	t.Helper()
	c, err := net.Dial("tcp", ln.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	return c
}

var connectBytes = packets.TPacketData[packets.Connect].Get(packets.TConnectMqtt311).RawBytes

// A connection accepted before Close that sends its CONNECT only after the
// listener has closed must not hold Close: it is past the disconnect sweep,
// so unless Close also ends it, it registers, reads its open socket and
// Close waits on it forever.
func TestServerCloseConnectAfterListenerClosed(t *testing.T) {
	s, ln := startCloseTestServer(t, nil)

	c := dialRaw(t, ln)
	waitFor(t, ln.accepted, "the connection to be accepted")

	done := closeAsync(s)
	waitFor(t, ln.closed, "the listener to close")

	_, _ = c.Write(connectBytes) // may fail: the server may already have closed it
	go func() { _, _ = io.Copy(io.Discard, c) }()

	requireClosed(t, done, "a connection that sent CONNECT after the listener closed is still being served")
}

// The same race, pinned where it was observed: the CONNECT has been read but
// the client registers (attachClient -> Clients.Add -> Client.Read) only
// after Close's disconnect sweep has run.
func TestServerCloseClientRegistersAfterSweep(t *testing.T) {
	gate := &gateConnectHook{entered: make(chan struct{}, 1), release: make(chan struct{})}
	s, ln := startCloseTestServer(t, nil, gate)

	c := dialRaw(t, ln)
	_, err := c.Write(connectBytes)
	require.NoError(t, err)
	go func() { _, _ = io.Copy(io.Discard, c) }()
	waitFor(t, gate.entered, "the CONNECT to reach OnConnect")

	done := closeAsync(s)
	waitFor(t, ln.closed, "the listener to close")
	close(gate.release) // registration now happens after the sweep

	requireClosed(t, done, "a client that registered after the disconnect sweep is still being served")
}

// A TCP connection that never sends CONNECT must not hold Close: Close ends
// connections that have not completed CONNECT yet.
func TestServerCloseSilentConnection(t *testing.T) {
	s, ln := startCloseTestServer(t, nil)

	c := dialRaw(t, ln)
	waitFor(t, ln.accepted, "the connection to be accepted")

	done := closeAsync(s)
	requireClosed(t, done, "a connection that never sent CONNECT is still open")

	_ = c.SetReadDeadline(time.Now().Add(closeBudget))
	_, err := c.Read(make([]byte, 1))
	require.ErrorIs(t, err, io.EOF, "the server must close the silent connection")
}

// A connection that reaches attachClient after Close has begun is refused
// and closed, never registered.
func TestServerAttachClientRefusedWhileClosing(t *testing.T) {
	s := newServer()
	require.NoError(t, s.Serve())
	require.NoError(t, s.Close())

	r, w := net.Pipe()
	defer w.Close()
	cl := s.NewClient(r, "t1", "", false)

	errs := make(chan error, 1)
	go func() { errs <- s.attachClient(cl, "t1") }()

	go func() { _, _ = w.Write(connectBytes) }()
	select {
	case err := <-errs:
		require.ErrorIs(t, err, packets.ErrServerShuttingDown)
	case <-time.After(closeBudget):
		t.Fatal("attachClient served a connection after Close")
	}
	require.True(t, cl.Closed())
	require.Equal(t, 0, s.Clients.Len())
}

// A connection that never sends CONNECT is closed after ConnectTimeout,
// without any Close.
func TestServerConnectTimeoutClosesSilentConnection(t *testing.T) {
	cc := NewDefaultServerCapabilities()
	cc.ConnectTimeout = 1
	s, ln := startCloseTestServer(t, &Options{Capabilities: cc})
	defer s.Close()

	c := dialRaw(t, ln)
	_ = c.SetReadDeadline(time.Now().Add(closeBudget))
	_, err := c.Read(make([]byte, 1))
	require.ErrorIs(t, err, io.EOF, "the server must close a connection that sends no CONNECT within ConnectTimeout")
}

// Once CONNECT has arrived the client's keepalive governs the connection:
// an idle client outlives ConnectTimeout.
func TestServerConnectTimeoutLiftedAfterConnect(t *testing.T) {
	cc := NewDefaultServerCapabilities()
	cc.ConnectTimeout = 1
	s, ln := startCloseTestServer(t, &Options{Capabilities: cc})
	defer s.Close()

	c := dialRaw(t, ln)
	_, err := c.Write(connectBytes) // keepalive 60 s
	require.NoError(t, err)
	_ = c.SetReadDeadline(time.Now().Add(closeBudget))
	connack := make([]byte, 4)
	_, err = io.ReadFull(c, connack)
	require.NoError(t, err)
	require.Equal(t, packets.Connack<<4, connack[0])

	time.Sleep(2 * time.Second) // idle past ConnectTimeout

	_, err = c.Write(packets.TPacketData[packets.Pingreq].Get(packets.TPingreq).RawBytes)
	require.NoError(t, err)
	pingresp := make([]byte, 2)
	_, err = io.ReadFull(c, pingresp)
	require.NoError(t, err, "the connection must still be open")
	require.Equal(t, packets.Pingresp<<4, pingresp[0])
}

func TestCapabilitiesConnectTimeout(t *testing.T) {
	require.Equal(t, 15*time.Second, NewDefaultServerCapabilities().connectTimeout())
	require.Equal(t, 15*time.Second, new(Capabilities).connectTimeout(), "zero means the default")
	require.Equal(t, 3*time.Second, (&Capabilities{ConnectTimeout: 3}).connectTimeout())
}

// Stress: many clients dial and CONNECT at random points around Close.
// Every Close must return. Run with -race -count=N.
func TestServerCloseRacesConnectingClients(t *testing.T) {
	const rounds = 20
	const clients = 16
	for i := 0; i < rounds; i++ {
		s, ln := startCloseTestServer(t, nil)

		var (
			wg     sync.WaitGroup
			mu     sync.Mutex
			conns  []net.Conn
			hungUp bool // the test has closed every client socket
		)
		for j := 0; j < clients; j++ {
			wg.Add(1)
			go func(j int) {
				defer wg.Done()
				c, err := net.Dial("tcp", ln.Addr().String())
				if err != nil {
					return // the listener closed first
				}
				mu.Lock()
				if hungUp {
					mu.Unlock()
					_ = c.Close()
					return
				}
				conns = append(conns, c)
				mu.Unlock()
				if j%4 != 0 { // a quarter stay silent
					time.Sleep(time.Duration(j*50) * time.Microsecond)
					_, _ = c.Write(connectBytes)
				}
				_, _ = io.Copy(io.Discard, c)
			}(j)
		}

		time.Sleep(time.Duration(i%5) * 200 * time.Microsecond)
		requireClosed(t, closeAsync(s), "a client connecting around Close kept it waiting")

		// Close's contract is the server side; release the clients without
		// waiting for the server to hang up on each of them.
		mu.Lock()
		hungUp = true
		for _, c := range conns {
			_ = c.Close()
		}
		mu.Unlock()
		wg.Wait()
	}
}
