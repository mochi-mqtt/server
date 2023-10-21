// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package listeners

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/mochi-mqtt/server/v2/system"

	"github.com/stretchr/testify/require"
)

func TestNewHTTPStats(t *testing.T) {
	l := NewHTTPStats("t1", testAddr, nil, nil)
	require.Equal(t, "t1", l.id)
	require.Equal(t, testAddr, l.address)
}

func TestHTTPStatsID(t *testing.T) {
	l := NewHTTPStats("t1", testAddr, nil, nil)
	require.Equal(t, "t1", l.ID())
}

func TestHTTPStatsAddress(t *testing.T) {
	l := NewHTTPStats("t1", testAddr, nil, nil)
	require.Equal(t, testAddr, l.Address())
}

func TestHTTPStatsProtocol(t *testing.T) {
	l := NewHTTPStats("t1", testAddr, nil, nil)
	require.Equal(t, "http", l.Protocol())
}

func TestHTTPStatsTLSProtocol(t *testing.T) {
	l := NewHTTPStats("t1", testAddr, &Config{
		TLSConfig: tlsConfigBasic,
	}, nil)

	_ = l.Init(logger)
	require.Equal(t, "https", l.Protocol())
}

func TestHTTPStatsInit(t *testing.T) {
	sysInfo := new(system.Info)
	l := NewHTTPStats("t1", testAddr, nil, sysInfo)
	err := l.Init(logger)
	require.NoError(t, err)

	require.NotNil(t, l.sysInfo)
	require.Equal(t, sysInfo, l.sysInfo)
	require.NotNil(t, l.listen)
	require.Equal(t, testAddr, l.listen.Addr)
}

func TestHTTPStatsServeAndClose(t *testing.T) {
	sysInfo := &system.Info{
		Version: "test",
	}

	// setup http stats listener
	l := NewHTTPStats("t1", testAddr, nil, sysInfo)
	err := l.Init(logger)
	require.NoError(t, err)

	o := make(chan bool)
	go func(o chan bool) {
		l.Serve(MockEstablisher)
		o <- true
	}(o)

	time.Sleep(time.Millisecond)

	// get body from stats address
	resp, err := http.Get("http://localhost" + testAddr)
	require.NoError(t, err)
	require.NotNil(t, resp)

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	// decode body from json and check data
	v := new(system.Info)
	err = json.Unmarshal(body, v)
	require.NoError(t, err)
	require.Equal(t, "test", v.Version)

	// ensure listening is closed
	var closed bool
	l.Close(func(id string) {
		closed = true
	})

	require.Equal(t, true, closed)

	_, err = http.Get("http://localhost" + testAddr)
	require.Error(t, err)
	<-o
}

func TestHTTPStatsServeTLSAndClose(t *testing.T) {
	sysInfo := &system.Info{
		Version: "test",
	}

	l := NewHTTPStats("t1", testAddr, &Config{
		TLSConfig: tlsConfigBasic,
	}, sysInfo)

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

func TestHTTPStatsFailedToServe(t *testing.T) {
	sysInfo := &system.Info{
		Version: "test",
	}

	// setup http stats listener
	l := NewHTTPStats("t1", "wrong_addr", nil, sysInfo)
	err := l.Init(logger)
	require.NoError(t, err)

	o := make(chan bool)
	go func(o chan bool) {
		l.Serve(MockEstablisher)
		o <- true
	}(o)

	<-o
	// ensure listening is closed
	var closed bool
	l.Close(func(id string) {
		closed = true
	})
	require.Equal(t, true, closed)
}
