// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-co
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
	l := NewWebsocket("t1", testAddr, nil)
	require.Equal(t, "t1", l.id)
	require.Equal(t, testAddr, l.address)
}

func TestWebsocketID(t *testing.T) {
	l := NewWebsocket("t1", testAddr, nil)
	require.Equal(t, "t1", l.ID())
}

func TestWebsocketAddress(t *testing.T) {
	l := NewWebsocket("t1", testAddr, nil)
	require.Equal(t, testAddr, l.Address())
}

func TestWebsocketProtocol(t *testing.T) {
	l := NewWebsocket("t1", testAddr, nil)
	require.Equal(t, "ws", l.Protocol())
}

func TestWebsocketProtocoTLS(t *testing.T) {
	l := NewWebsocket("t1", testAddr, &Config{
		TLSConfig: tlsConfigBasic,
	})
	require.Equal(t, "wss", l.Protocol())
}

func TestWebsockeInit(t *testing.T) {
	l := NewWebsocket("t1", testAddr, nil)
	require.Nil(t, l.listen)
	err := l.Init(&logger, slogger)
	require.NoError(t, err)
	require.NotNil(t, l.listen)
}

func TestWebsocketServeAndClose(t *testing.T) {
	l := NewWebsocket("t1", testAddr, nil)
	l.Init(&logger, slogger)

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
	l := NewWebsocket("t1", testAddr, &Config{
		TLSConfig: tlsConfigBasic,
	})
	err := l.Init(&logger, slogger)
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
	l := NewWebsocket("t1", testAddr, nil)
	l.Init(&logger, slogger)

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
	ws.Close()
}
