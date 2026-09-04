// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package listeners

import (
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewTCP(t *testing.T) {
	l := NewTCP(basicConfig)
	require.Equal(t, "t1", l.id)
	require.Equal(t, testAddr, l.address)
}

func TestTCPID(t *testing.T) {
	l := NewTCP(basicConfig)
	require.Equal(t, "t1", l.ID())
}

func TestTCPAddress(t *testing.T) {
	l := NewTCP(basicConfig)
	require.Equal(t, testAddr, l.Address())
}

func TestTCPProtocol(t *testing.T) {
	l := NewTCP(basicConfig)
	require.Equal(t, "tcp", l.Protocol())
}

func TestTCPProtocolTLS(t *testing.T) {
	l := NewTCP(tlsConfig)
	_ = l.Init(logger)
	defer l.listen.Close()
	require.Equal(t, "tcp", l.Protocol())
}

func TestTCPInit(t *testing.T) {
	l := NewTCP(basicConfig)
	err := l.Init(logger)
	l.Close(MockCloser)
	require.NoError(t, err)

	l2 := NewTCP(tlsConfig)
	err = l2.Init(logger)
	l2.Close(MockCloser)
	require.NoError(t, err)
	require.NotNil(t, l2.config.TLSConfig)
}

func TestTCPServeAndClose(t *testing.T) {
	l := NewTCP(basicConfig)
	err := l.Init(logger)
	require.NoError(t, err)

	o := make(chan bool)
	go func(o chan bool) {
		l.Serve(MockEstablisher)
		o <- true
	}(o)

	time.Sleep(time.Millisecond)

	var closed bool
	l.Close(func(id string) {
		closed = true
	})

	require.True(t, closed)
	<-o

	l.Close(MockCloser)      // coverage: close closed
	l.Serve(MockEstablisher) // coverage: serve closed
}

func TestTCPServeTLSAndClose(t *testing.T) {
	l := NewTCP(tlsConfig)
	err := l.Init(logger)
	require.NoError(t, err)

	o := make(chan bool)
	go func(o chan bool) {
		l.Serve(MockEstablisher)
		o <- true
	}(o)

	time.Sleep(time.Millisecond)

	var closed bool
	l.Close(func(id string) {
		closed = true
	})

	require.Equal(t, true, closed)
	<-o
}

// TestTCPCloseWaitsForInFlightEstablish is a deterministic unit test of the
// mechanism added for #424: Close must not return while an establish() call
// this listener spawned is still running, since that in-flight call is what
// (in the real Server) still owes a Listeners.ClientsWg.Add/Done pairing.
// The establish func here blocks until the test lets it finish, so if Close
// returned early the "closed too early" flag below would be set before
// establish had a chance to mark itself done -- this fails deterministically
// on the pre-fix Close, which returned as soon as the listener socket closed
// with no regard for in-flight establish() calls at all.
func TestTCPCloseWaitsForInFlightEstablish(t *testing.T) {
	l := NewTCP(basicConfig)
	require.NoError(t, l.Init(logger))
	addr := l.listen.Addr().String()

	release := make(chan struct{})
	var establishDone atomic.Bool
	go l.Serve(func(id string, c net.Conn) error {
		<-release
		establishDone.Store(true)
		return nil
	})

	conn, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	defer conn.Close()
	time.Sleep(10 * time.Millisecond) // let Serve accept and spawn establish()

	closed := make(chan struct{})
	go func() {
		l.Close(MockCloser)
		close(closed)
	}()

	// Close must still be blocked: establish() hasn't been released yet.
	select {
	case <-closed:
		t.Fatal("Close returned while an establish() call was still in flight")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not return after the in-flight establish() finished")
	}
	require.True(t, establishDone.Load())
}

func TestTCPEstablishThenEnd(t *testing.T) {
	l := NewTCP(basicConfig)
	err := l.Init(logger)
	require.NoError(t, err)

	o := make(chan bool)
	established := make(chan bool)
	go func() {
		l.Serve(func(id string, c net.Conn) error {
			established <- true
			return errors.New("ending") // return an error to exit immediately
		})
		o <- true
	}()

	time.Sleep(time.Millisecond)
	_, _ = net.Dial("tcp", l.listen.Addr().String())
	require.Equal(t, true, <-established)
	l.Close(MockCloser)
	<-o
}
