package listeners

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/mochi-co/mqtt/server/listeners/auth"
	"github.com/stretchr/testify/require"
)

func TestWsConnClose(t *testing.T) {
	r, _ := net.Pipe()
	ws := &wsConn{r, new(websocket.Conn)}
	err := ws.Close()
	require.NoError(t, err)
}

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
	require.Nil(t, l.listen)
	err := l.Listen(nil)
	require.NoError(t, err)
	require.NotNil(t, l.listen)
}

func TestWebsocketListenTLS(t *testing.T) {
	l := NewWebsocket("t1", testPort)
	l.SetConfig(&Config{
		Auth: new(auth.Allow),
		TLS: &TLS{
			Certificate: testCertificate,
			PrivateKey:  testPrivateKey,
		},
	})
	err := l.Listen(nil)
	require.NoError(t, err)
	require.NotNil(t, l.listen.TLSConfig)
	l.listen.Close()
}

func TestWebsocketListenTLSInvalid(t *testing.T) {
	l := NewWebsocket("t1", testPort)
	l.SetConfig(&Config{
		Auth: new(auth.Allow),
		TLS: &TLS{
			Certificate: []byte("abcde"),
			PrivateKey:  testPrivateKey,
		},
	})
	err := l.Listen(nil)
	require.Error(t, err)
}

func TestWebsocketServeAndClose(t *testing.T) {
	l := NewWebsocket("t1", testPort)
	l.Listen(nil)
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

func TestWebsocketServeTLSAndClose(t *testing.T) {
	l := NewWebsocket("t1", testPort)
	l.SetConfig(&Config{
		Auth: new(auth.Allow),
		TLS: &TLS{
			Certificate: testCertificate,
			PrivateKey:  testPrivateKey,
		},
	})
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
}

func TestWebsocketUpgrade(t *testing.T) {
	l := NewWebsocket("t1", testPort)
	l.Listen(nil)
	e := make(chan bool)
	l.establish = func(id string, c net.Conn, ac auth.Controller) error {
		e <- true
		return nil
	}
	s := httptest.NewServer(http.HandlerFunc(l.handler))
	u := "ws" + strings.TrimPrefix(s.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)
	require.Equal(t, true, <-e)

	s.Close()
	ws.Close()

}
