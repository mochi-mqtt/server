// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package listeners

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMockEstablisher(t *testing.T) {
	_, w := net.Pipe()
	err := MockEstablisher("t1", w)
	require.NoError(t, err)
	_ = w.Close()
}

func TestNewMockListener(t *testing.T) {
	mocked := NewMockListener("t1", testAddr)
	require.Equal(t, "t1", mocked.id)
	require.Equal(t, testAddr, mocked.address)
}
func TestMockListenerID(t *testing.T) {
	mocked := NewMockListener("t1", testAddr)
	require.Equal(t, "t1", mocked.ID())
}

func TestMockListenerAddress(t *testing.T) {
	mocked := NewMockListener("t1", testAddr)
	require.Equal(t, testAddr, mocked.Address())
}
func TestMockListenerProtocol(t *testing.T) {
	mocked := NewMockListener("t1", testAddr)
	require.Equal(t, "mock", mocked.Protocol())
}

func TestNewMockListenerIsListening(t *testing.T) {
	mocked := NewMockListener("t1", testAddr)
	require.Equal(t, false, mocked.IsListening())
}

func TestNewMockListenerIsServing(t *testing.T) {
	mocked := NewMockListener("t1", testAddr)
	require.Equal(t, false, mocked.IsServing())
}

func TestNewMockListenerInit(t *testing.T) {
	mocked := NewMockListener("t1", testAddr)
	require.Equal(t, "t1", mocked.id)
	require.Equal(t, testAddr, mocked.address)

	require.Equal(t, false, mocked.IsListening())
	err := mocked.Init(nil)
	require.NoError(t, err)
	require.Equal(t, true, mocked.IsListening())
}

func TestNewMockListenerInitFailure(t *testing.T) {
	mocked := NewMockListener("t1", testAddr)
	mocked.ErrListen = true
	err := mocked.Init(nil)
	require.Error(t, err)
}

func TestMockListenerServe(t *testing.T) {
	mocked := NewMockListener("t1", testAddr)
	require.Equal(t, false, mocked.IsServing())

	o := make(chan bool)
	go func(o chan bool) {
		mocked.Serve(MockEstablisher)
		o <- true
	}(o)

	time.Sleep(time.Millisecond) // easy non-channel wait for start of serving
	require.Equal(t, true, mocked.IsServing())

	var closed bool
	mocked.Close(func(id string) {
		closed = true
	})
	require.Equal(t, true, closed)
	<-o

	_ = mocked.Init(nil)
}

func TestMockListenerClose(t *testing.T) {
	mocked := NewMockListener("t1", testAddr)
	var closed bool
	mocked.Close(func(id string) {
		closed = true
	})
	require.Equal(t, true, closed)
}
