package listeners

import (
	//"errors"
	//"net"
	"testing"
	"time"

	"github.com/mochi-co/mqtt/server/listeners/auth"
	"github.com/stretchr/testify/require"
)

func TestNewWebsocket(t *testing.T) {
	l := NewWebsocket("t1", testPort)
	require.Equal(t, "t1", l.id)
	require.Equal(t, testPort, l.address)
}

func BenchmarkNewWebsocket(b *testing.B) {
	for n := 0; n < b.N; n++ {
		NewWebsocket("t1", testPort)
	}
}

func TestWebsocketSetConfig(t *testing.T) {
	l := NewWebsocket("t1", testPort)

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

func BenchmarkWebsocketSetConfig(b *testing.B) {
	l := NewWebsocket("t1", testPort)
	for n := 0; n < b.N; n++ {
		l.SetConfig(new(Config))
	}
}

func TestWebsocketID(t *testing.T) {
	l := NewWebsocket("t1", testPort)
	require.Equal(t, "t1", l.ID())
}

func BenchmarkWebsocketID(b *testing.B) {
	l := NewWebsocket("t1", testPort)
	for n := 0; n < b.N; n++ {
		l.ID()
	}
}

func TestWebsocketListen(t *testing.T) {
	l := NewWebsocket("t1", testPort)
	err := l.Listen(nil)
	require.NoError(t, err)

	l2 := NewWebsocket("t2", testPort)
	err = l2.Listen(nil)
	require.Error(t, err)
	l.listen.Close()
}

func TestWebsocketServeAndClose(t *testing.T) {
	l := NewWebsocket("t1", testPort)
	err := l.Listen(nil)
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
