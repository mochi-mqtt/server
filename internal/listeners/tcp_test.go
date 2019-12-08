package listeners

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/mochi-co/mqtt/internal/auth"
	"github.com/stretchr/testify/require"
)

const (
	testPort = ":22222"
)

func TestNewTCP(t *testing.T) {
	l := NewTCP("t1", testPort)
	require.Equal(t, "t1", l.id)
	require.Equal(t, testPort, l.address)
}

func BenchmarkNewTCP(b *testing.B) {
	for n := 0; n < b.N; n++ {
		NewTCP("t1", testPort)
	}
}

func TestTCPSetConfig(t *testing.T) {
	l := NewTCP("t1", testPort)

	l.SetConfig(&Config{
		Auth: new(auth.Allow),
	})
	require.NotNil(t, l.config)
	require.NotNil(t, l.config.Auth)
	require.Equal(t, new(auth.Allow), l.config.Auth)

	// Switch to disallow on bad config set.
	l.SetConfig(new(Config))
	require.NotNil(t, l.config)
	require.NotNil(t, l.config.Auth)
	require.Equal(t, new(auth.Disallow), l.config.Auth)
}

func BenchmarkTCPSetConfig(b *testing.B) {
	l := NewTCP("t1", testPort)
	for n := 0; n < b.N; n++ {
		l.SetConfig(new(Config))
	}
}

func TestTCPID(t *testing.T) {
	l := NewTCP("t1", testPort)
	require.Equal(t, "t1", l.ID())
}

func BenchmarkTCPID(b *testing.B) {
	l := NewTCP("t1", testPort)
	for n := 0; n < b.N; n++ {
		l.ID()
	}
}

func TestTCPListen(t *testing.T) {
	l := NewTCP("t1", testPort)
	err := l.Listen()
	require.NoError(t, err)

	l2 := NewTCP("t2", testPort)
	err = l2.Listen()
	require.Error(t, err)
	l.listen.Close()
}

func TestTCPServeAndClose(t *testing.T) {
	l := NewTCP("t1", testPort)
	err := l.Listen()
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

func TestTCPCloseError(t *testing.T) {
	l := NewTCP("t1", testPort)
	err := l.Listen()
	require.NoError(t, err)
	o := make(chan bool)
	go func(o chan bool) {
		l.Serve(MockEstablisher)
		o <- true
	}(o)

	time.Sleep(time.Millisecond)
	l.listen.Close()
	l.Close(MockCloser)
	<-o
}

func TestTCPServeEnd(t *testing.T) {
	l := NewTCP("t1", testPort)
	err := l.Listen()
	require.NoError(t, err)

	l.Close(MockCloser)
	l.Serve(func(id string, c net.Conn, ac auth.Controller) error {
		return nil
	})
}

func TestTCPEstablishThenError(t *testing.T) {
	l := NewTCP("t1", testPort)
	err := l.Listen()
	require.NoError(t, err)

	o := make(chan bool)
	established := make(chan bool)
	go func() {
		l.Serve(func(id string, c net.Conn, ac auth.Controller) error {
			established <- true
			return errors.New("testing") // return an error to exit immediately
		})
		o <- true
	}()

	time.Sleep(time.Millisecond)
	net.Dial(l.protocol, l.listen.Addr().String())
	require.Equal(t, true, <-established)
	l.Close(MockCloser)
	<-o
}

func TestTCPEstablishButEnding(t *testing.T) {
	l := NewTCP("t1", testPort)
	err := l.Listen()
	require.NoError(t, err)
	l.end = 1

	o := make(chan bool)
	go func() {
		l.Serve(func(id string, c net.Conn, ac auth.Controller) error {
			return nil
		})
		o <- true
	}()

	net.Dial(l.protocol, l.listen.Addr().String())

	time.Sleep(time.Millisecond)
	<-o
}
