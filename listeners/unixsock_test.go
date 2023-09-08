// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: jason@zgwit.com

package listeners

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const testUnixAddr = "mochi.sock"

func TestNewUnixSock(t *testing.T) {
	l := NewUnixSock("t1", testUnixAddr)
	require.Equal(t, "t1", l.id)
	require.Equal(t, testUnixAddr, l.address)
}

func TestUnixSockID(t *testing.T) {
	l := NewUnixSock("t1", testUnixAddr)
	require.Equal(t, "t1", l.ID())
}

func TestUnixSockAddress(t *testing.T) {
	l := NewUnixSock("t1", testUnixAddr)
	require.Equal(t, testUnixAddr, l.Address())
}

func TestUnixSockProtocol(t *testing.T) {
	l := NewUnixSock("t1", testUnixAddr)
	require.Equal(t, "unix", l.Protocol())
}

func TestUnixSockInit(t *testing.T) {
	l := NewUnixSock("t1", testUnixAddr)
	err := l.Init(logger)
	l.Close(MockCloser)
	require.NoError(t, err)

	l2 := NewUnixSock("t2", testUnixAddr)
	err = l2.Init(logger)
	l2.Close(MockCloser)
	require.NoError(t, err)
}

func TestUnixSockServeAndClose(t *testing.T) {
	l := NewUnixSock("t1", testUnixAddr)
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

func TestUnixSockEstablishThenEnd(t *testing.T) {
	l := NewUnixSock("t1", testUnixAddr)
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
	_, _ = net.Dial("unix", l.listen.Addr().String())
	require.Equal(t, true, <-established)
	l.Close(MockCloser)
	<-o
}
