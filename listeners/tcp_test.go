// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package listeners

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewTCP(t *testing.T) {
	l := NewTCP("t1", testAddr, nil)
	require.Equal(t, "t1", l.id)
	require.Equal(t, testAddr, l.address)
}

func TestTCPID(t *testing.T) {
	l := NewTCP("t1", testAddr, nil)
	require.Equal(t, "t1", l.ID())
}

func TestTCPAddress(t *testing.T) {
	l := NewTCP("t1", testAddr, nil)
	require.Equal(t, testAddr, l.Address())
}

func TestTCPProtocol(t *testing.T) {
	l := NewTCP("t1", testAddr, nil)
	require.Equal(t, "tcp", l.Protocol())
}

func TestTCPProtocolTLS(t *testing.T) {
	l := NewTCP("t1", testAddr, &Config{
		TLSConfig: tlsConfigBasic,
	})

	_ = l.Init(logger)
	defer l.listen.Close()
	require.Equal(t, "tcp", l.Protocol())
}

func TestTCPInit(t *testing.T) {
	l := NewTCP("t1", testAddr, nil)
	err := l.Init(logger)
	l.Close(MockCloser)
	require.NoError(t, err)

	l2 := NewTCP("t2", testAddr, &Config{
		TLSConfig: tlsConfigBasic,
	})
	err = l2.Init(logger)
	l2.Close(MockCloser)
	require.NoError(t, err)
	require.NotNil(t, l2.config.TLSConfig)
}

func TestTCPServeAndClose(t *testing.T) {
	l := NewTCP("t1", testAddr, nil)
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
	l := NewTCP("t1", testAddr, &Config{
		TLSConfig: tlsConfigBasic,
	})
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

func TestTCPEstablishThenEnd(t *testing.T) {
	l := NewTCP("t1", testAddr, nil)
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
