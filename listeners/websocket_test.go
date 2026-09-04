// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package listeners

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestNewWebsocket(t *testing.T) {
	l := NewWebsocket(basicConfig)
	require.Equal(t, "t1", l.id)
	require.Equal(t, testAddr, l.address)
}

func TestWebsocketID(t *testing.T) {
	l := NewWebsocket(basicConfig)
	require.Equal(t, "t1", l.ID())
}

func TestWebsocketAddress(t *testing.T) {
	l := NewWebsocket(basicConfig)
	require.Equal(t, testAddr, l.Address())
}

func TestWebsocketProtocol(t *testing.T) {
	l := NewWebsocket(basicConfig)
	require.Equal(t, "ws", l.Protocol())
}

func TestWebsocketProtocolTLS(t *testing.T) {
	l := NewWebsocket(tlsConfig)
	require.Equal(t, "wss", l.Protocol())
}

func TestWebsocketInit(t *testing.T) {
	l := NewWebsocket(basicConfig)
	require.Nil(t, l.listen)
	err := l.Init(logger)
	require.NoError(t, err)
	require.NotNil(t, l.listen)
	l.Close(MockCloser) // Init binds the socket; release it for the next test
}

func TestWebsocketServeAndClose(t *testing.T) {
	l := NewWebsocket(basicConfig)
	_ = l.Init(logger)

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
}

func TestWebsocketServeTLSAndClose(t *testing.T) {
	l := NewWebsocket(tlsConfig)
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

func TestWebsocketFailedToServe(t *testing.T) {
	config := tlsConfig
	config.Address = "wrong_addr"
	l := NewWebsocket(config)
	err := l.Init(logger)
	require.Error(t, err, "an unbindable address is refused at Init, where the caller can see it")

	o := make(chan bool)
	go func(o chan bool) {
		l.Serve(MockEstablisher)
		o <- true
	}(o)

	<-o
	var closed bool
	l.Close(func(id string) {
		closed = true
	})
	require.Equal(t, true, closed)
}

func TestWebsocketUpgrade(t *testing.T) {
	l := NewWebsocket(basicConfig)
	_ = l.Init(logger)

	e := make(chan bool)
	l.establish = func(id string, c net.Conn) error {
		e <- true
		return nil
	}

	s := httptest.NewServer(http.HandlerFunc(l.handler))
	ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(s.URL, "http"), nil)
	require.NoError(t, err)
	require.Equal(t, true, <-e)

	s.Close()
	_ = ws.Close()
}

func TestWebsocketConnectionReads(t *testing.T) {
	l := NewWebsocket(basicConfig)
	_ = l.Init(nil)

	recv := make(chan []byte)
	l.establish = func(id string, c net.Conn) error {
		var out []byte
		for {
			buf := make([]byte, 2048)
			n, err := c.Read(buf)
			require.NoError(t, err)
			out = append(out, buf[:n]...)
			if n < 2048 {
				break
			}
		}

		recv <- out
		return nil
	}

	s := httptest.NewServer(http.HandlerFunc(l.handler))
	ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(s.URL, "http"), nil)
	require.NoError(t, err)

	pkt := make([]byte, 3000) // make sure this is >2048
	for i := 0; i < len(pkt); i++ {
		pkt[i] = byte(i % 100)
	}

	err = ws.WriteMessage(websocket.BinaryMessage, pkt)
	require.NoError(t, err)

	got := <-recv
	require.Equal(t, 3000, len(got))
	require.Equal(t, pkt, got)

	s.Close()
	_ = ws.Close()
}

// A ":0" address is bound at Init, so Address reports the port the kernel
// assigned and the port is held before Serve runs — nothing else in the
// process can take it in between, and a caller told the address can dial it.
func TestWebsocketAddressIsTheBoundPortAfterInit(t *testing.T) {
	l := NewWebsocket(Config{ID: "t1", Address: "127.0.0.1:0"})
	require.NoError(t, l.Init(logger))
	defer l.Close(MockCloser)

	addr := l.Address()
	require.NotEqual(t, "127.0.0.1:0", addr)
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	require.NoError(t, err, "the door must accept from the moment Init returns")
	_ = conn.Close()
	_, err = net.Listen("tcp", addr)
	require.Error(t, err, "the port must be held, not merely reserved")
}
