package listeners

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewNet(t *testing.T) {
	n, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	l := NewNet("t1", n)
	require.Equal(t, "t1", l.id)
}

func TestNetID(t *testing.T) {
	n, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	l := NewNet("t1", n)
	require.Equal(t, "t1", l.ID())
}

func TestNetAddress(t *testing.T) {
	n, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	l := NewNet("t1", n)
	require.Equal(t, n.Addr().String(), l.Address())
}

func TestNetProtocol(t *testing.T) {
	n, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	l := NewNet("t1", n)
	require.Equal(t, "tcp", l.Protocol())
}

func TestNetInit(t *testing.T) {
	n, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	l := NewNet("t1", n)
	err = l.Init(logger)
	l.Close(MockCloser)
	require.NoError(t, err)
}

func TestNetServeAndClose(t *testing.T) {
	n, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	l := NewNet("t1", n)
	err = l.Init(logger)
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

func TestNetEstablishThenEnd(t *testing.T) {
	n, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	l := NewNet("t1", n)
	err = l.Init(logger)
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
	_, _ = net.Dial("tcp", n.Addr().String())
	require.Equal(t, true, <-established)
	l.Close(MockCloser)
	<-o
}
