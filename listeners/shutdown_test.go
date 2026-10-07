// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package listeners

// A connection accepted after Close has begun must be closed, not dropped
// open. Close sets end, disconnects the clients, then
// closes the listening socket; a client that reconnects the moment it is
// disconnected is accepted in that window. Serve skips establish for it,
// and before the patch left it open: nobody read it and nobody closed it,
// so the client waited for a CONNACK until its own connect timeout.

import (
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// serveUntilAccepting starts Serve on l and returns once it has accepted
// one connection, so it is known to be back in Accept for the next one.
func serveUntilAccepting(t *testing.T, l interface {
	Serve(EstablishFn)
}, addr string) (served chan struct{}) {
	t.Helper()
	accepted := make(chan struct{}, 1)
	served = make(chan struct{})
	go func() {
		l.Serve(func(id string, c net.Conn) error {
			_ = c.Close()
			accepted <- struct{}{}
			return nil
		})
		close(served)
	}()
	probe, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	_ = probe.Close()
	select {
	case <-accepted:
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not accept the probe connection")
	}
	// Serve loops straight back into Accept after establish returns; give
	// the goroutine the moment it needs to get there.
	time.Sleep(20 * time.Millisecond)
	return served
}

// readEndsWithEOF dials addr and requires the read to end with EOF: the
// listener closed the connection it accepted. Unpatched, the read waits
// for the whole deadline.
func readEndsWithEOF(t *testing.T, addr string) {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	_, err = conn.Read(make([]byte, 1))
	require.ErrorIs(t, err, io.EOF, "a connection accepted while the listener closes must be closed by the listener, not left open")
}

func TestTCPServeClosesConnectionAcceptedAfterEnd(t *testing.T) {
	l := NewTCP(Config{ID: "shutdown-tcp", Address: "127.0.0.1:0"})
	require.NoError(t, l.Init(logger))
	addr := l.listen.Addr().String()
	served := serveUntilAccepting(t, l, addr)

	// What Close does first, with the listening socket still open.
	atomic.StoreUint32(&l.end, 1)
	readEndsWithEOF(t, addr)

	<-served
	l.Close(MockCloser)
}

func TestNetServeClosesConnectionAcceptedAfterEnd(t *testing.T) {
	inner, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	l := NewNet("shutdown-net", inner)
	require.NoError(t, l.Init(logger))
	addr := inner.Addr().String()
	served := serveUntilAccepting(t, l, addr)

	atomic.StoreUint32(&l.end, 1)
	readEndsWithEOF(t, addr)

	<-served
	l.Close(MockCloser)
}
