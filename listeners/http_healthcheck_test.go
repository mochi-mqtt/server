// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 mochi-mqtt, mochi-co
// SPDX-FileContributor: Derek Duncan

package listeners

import (
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewHTTPHealthCheck(t *testing.T) {
	l := NewHTTPHealthCheck(basicConfig)
	require.Equal(t, basicConfig.ID, l.id)
	require.Equal(t, basicConfig.Address, l.address)
}

func TestHTTPHealthCheckID(t *testing.T) {
	l := NewHTTPHealthCheck(basicConfig)
	require.Equal(t, basicConfig.ID, l.ID())
}

func TestHTTPHealthCheckAddress(t *testing.T) {
	l := NewHTTPHealthCheck(basicConfig)
	require.Equal(t, basicConfig.Address, l.Address())
}

func TestHTTPHealthCheckProtocol(t *testing.T) {
	l := NewHTTPHealthCheck(basicConfig)
	require.Equal(t, "http", l.Protocol())
}

func TestHTTPHealthCheckTLSProtocol(t *testing.T) {
	l := NewHTTPHealthCheck(tlsConfig)
	_ = l.Init(logger)
	require.Equal(t, "https", l.Protocol())
}

func TestHTTPHealthCheckInit(t *testing.T) {
	l := NewHTTPHealthCheck(basicConfig)
	err := l.Init(logger)
	require.NoError(t, err)

	require.NotNil(t, l.listen)
	require.Equal(t, basicConfig.Address, l.listen.Addr)
}

func TestHTTPHealthCheckServeAndClose(t *testing.T) {
	// setup http stats listener
	l := NewHTTPHealthCheck(basicConfig)
	err := l.Init(logger)
	require.NoError(t, err)

	o := make(chan bool)
	go func(o chan bool) {
		l.Serve(MockEstablisher)
		o <- true
	}(o)

	time.Sleep(time.Millisecond)

	// call healthcheck
	resp, err := http.Get("http://localhost" + testAddr + "/healthcheck")
	require.NoError(t, err)
	require.NotNil(t, resp)

	defer resp.Body.Close()
	_, err = io.ReadAll(resp.Body)
	require.NoError(t, err)

	// ensure listening is closed
	var closed bool
	l.Close(func(id string) {
		closed = true
	})

	require.Equal(t, true, closed)

	_, err = http.Get("http://localhost/healthcheck" + testAddr + "/healthcheck")
	require.Error(t, err)
	<-o
}

func TestHTTPHealthCheckServeAndCloseMethodNotAllowed(t *testing.T) {
	// setup http stats listener
	l := NewHTTPHealthCheck(basicConfig)
	err := l.Init(logger)
	require.NoError(t, err)

	o := make(chan bool)
	go func(o chan bool) {
		l.Serve(MockEstablisher)
		o <- true
	}(o)

	time.Sleep(time.Millisecond)

	// make disallowed method type http request
	resp, err := http.Post("http://localhost"+testAddr+"/healthcheck", "application/json", http.NoBody)
	require.NoError(t, err)
	require.NotNil(t, resp)

	defer resp.Body.Close()
	_, err = io.ReadAll(resp.Body)
	require.NoError(t, err)

	// ensure listening is closed
	var closed bool
	l.Close(func(id string) {
		closed = true
	})

	require.Equal(t, true, closed)

	_, err = http.Post("http://localhost/healthcheck"+testAddr+"/healthcheck", "application/json", http.NoBody)
	require.Error(t, err)
	<-o
}

func TestHTTPHealthCheckServeTLSAndClose(t *testing.T) {
	l := NewHTTPHealthCheck(tlsConfig)
	err := l.Init(logger)
	require.NoError(t, err)

	o := make(chan bool)
	go func(o chan bool) {
		l.Serve(MockEstablisher)
		o <- true
	}(o)

	time.Sleep(time.Millisecond)
	l.Close(MockCloser)
}
