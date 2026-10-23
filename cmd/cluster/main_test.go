// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/mochi-mqtt/server/v2/packets"
)

func freeAddr(t *testing.T) string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	return ln.Addr().String()
}

// TestRun starts a single node cluster, checks it becomes healthy and accepts mqtt
// clients, then stops it.
func TestRun(t *testing.T) {
	gossip, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	gossipPort := gossip.Addr().(*net.TCPAddr).Port
	require.NoError(t, gossip.Close())

	cfg := &config{
		NodeName:     "test-0",
		MQTTAddr:     freeAddr(t),
		AdminAddr:    freeAddr(t),
		BindAddr:     "127.0.0.1",
		GossipPort:   gossipPort,
		ForwardAddr:  "127.0.0.1:0",
		RaftAddr:     "127.0.0.1:0",
		Bootstrap:    "true",
		AllowAll:     true,
		DrainOnStop:  true,
		DrainTimeout: 5 * time.Second,
		LogFormat:    "text",
		LogLevel:     slog.LevelError,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- run(ctx, cfg) }()

	require.Eventually(t, func() bool {
		resp, err := http.Get("http://" + cfg.AdminAddr + "/healthz")
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 15*time.Second, 100*time.Millisecond, "node never became healthy")

	conn, err := net.DialTimeout("tcp", cfg.MQTTAddr, time.Second)
	require.NoError(t, err)
	defer conn.Close()

	connect := packets.Packet{
		FixedHeader:     packets.FixedHeader{Type: packets.Connect},
		ProtocolVersion: 4,
		Connect:         packets.ConnectParams{ProtocolName: []byte("MQTT"), Clean: true, Keepalive: 30, ClientIdentifier: "run-test"},
	}
	buf := new(bytes.Buffer)
	require.NoError(t, connect.ConnectEncode(buf))
	_, err = conn.Write(buf.Bytes())
	require.NoError(t, err)

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	ack := make([]byte, 4)
	_, err = io.ReadFull(conn, ack)
	require.NoError(t, err)
	require.Equal(t, byte(packets.Connack<<4), ack[0])
	require.Equal(t, byte(0), ack[3], "connection refused")

	cancel()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(15 * time.Second):
		t.Fatal("node did not stop")
	}
}
