// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package mqtt_test

// Behaviour of PUBACK coalescing through a real TCP listener (see
// Client.Read). Every assertion is about what the client receives and how many
// writes the broker made — never about elapsed time — so a coarse clock or a
// loaded machine cannot change the answer. Reads carry a generous deadline so
// a held-forever ack fails the test instead of hanging it.

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"
	"time"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
	"github.com/stretchr/testify/require"
)

const ackReadTimeout = 10 * time.Second

func (c *rawClient) expectPuback(t *testing.T, id uint16) {
	t.Helper()
	require.NoError(t, c.conn.SetReadDeadline(time.Now().Add(ackReadTimeout)))
	typ, body, err := c.next()
	require.NoError(t, err, "waiting for PUBACK %d", id)
	require.Equal(t, packets.Puback, typ, "waiting for PUBACK %d", id)
	require.GreaterOrEqual(t, len(body), 2)
	require.Equal(t, id, binary.BigEndian.Uint16(body), "PUBACKs must arrive in publish order")
	if len(body) > 2 {
		require.Less(t, body[2], byte(0x80), "PUBACK %d carries an error reason", id)
	}
}

func publishBurst(t *testing.T, from, to uint16) []byte {
	var burst []byte
	for id := from; id <= to; id++ {
		burst = append(burst, encodePublishQoS1(t, "coalesce/t", id, []byte("payload"))...)
	}
	return burst
}

// A burst written in one segment is acked completely, in order, in far fewer
// writes than packets.
func TestAckCoalesceBurstInOrderWithFewerWrites(t *testing.T) {
	addr, writes := startCountingBroker(t)
	c := dialV5(t, addr, "burst")

	const n = 200
	before := writes.Load()
	_, err := c.conn.Write(publishBurst(t, 1, n))
	require.NoError(t, err)

	for id := uint16(1); id <= n; id++ {
		c.expectPuback(t, id)
	}
	// One write per read-buffer's worth of publishes (~2 KiB), not one per ack.
	require.Less(t, writes.Load()-before, int64(n/4), "acks for a buffered burst must share writes")
}

// A lone publish — nothing buffered behind it — is acked by one write, without
// waiting for further input.
func TestAckCoalesceLonePublishFlushedImmediately(t *testing.T) {
	addr, writes := startCountingBroker(t)
	c := dialV5(t, addr, "lone")

	for id := uint16(1); id <= 5; id++ {
		before := writes.Load()
		_, err := c.conn.Write(encodePublishQoS1(t, "coalesce/t", id, []byte("x")))
		require.NoError(t, err)
		c.expectPuback(t, id)
		require.Equal(t, int64(1), writes.Load()-before)
	}
}

// When the next packet is only partly buffered the reader is about to block
// on the socket, so held acks must go out first: the client may be waiting on
// them (receive maximum) before it sends the rest.
func TestAckCoalescePartialNextPacketDoesNotHoldAcks(t *testing.T) {
	addr, _ := startCountingBroker(t)
	c := dialV5(t, addr, "partial")

	burst := publishBurst(t, 1, 10)
	next := encodePublishQoS1(t, "coalesce/t", 11, []byte("payload"))
	for _, split := range []int{1, 2, 4} { // inside the fixed header, the length, the body
		_, err := c.conn.Write(append(append([]byte{}, burst...), next[:split]...))
		require.NoError(t, err)
		for id := uint16(1); id <= 10; id++ {
			c.expectPuback(t, id)
		}

		_, err = c.conn.Write(next[split:])
		require.NoError(t, err)
		c.expectPuback(t, 11)
	}
}

// Publishes and a DISCONNECT in the same segment: the broker closes the
// connection on DISCONNECT, and every PUBACK must have been sent before it.
func TestAckCoalesceDisconnectInSameSegmentKeepsAcks(t *testing.T) {
	addr, _ := startCountingBroker(t)
	c := dialV5(t, addr, "disconnect")

	var disconnect bytes.Buffer
	require.NoError(t, (&packets.Packet{
		FixedHeader:     packets.FixedHeader{Type: packets.Disconnect},
		ProtocolVersion: 5,
	}).DisconnectEncode(&disconnect))

	_, err := c.conn.Write(append(publishBurst(t, 1, 20), disconnect.Bytes()...))
	require.NoError(t, err)

	for id := uint16(1); id <= 20; id++ {
		c.expectPuback(t, id)
	}
	_, _, err = c.next()
	require.True(t, errors.Is(err, io.EOF), "connection should close after DISCONNECT, got %v", err)
}

// gateHook makes publish 1 slow enough to cross maxAckCoalesceDelay and blocks
// publish 2 until the test has received PUBACK 1.
type gateHook struct {
	mqtt.HookBase
	release chan struct{}
}

func (h *gateHook) ID() string { return "gate" }

func (h *gateHook) Provides(b byte) bool { return b == mqtt.OnPublish }

func (h *gateHook) OnPublish(_ *mqtt.Client, pk packets.Packet) (packets.Packet, error) {
	switch pk.PacketID {
	case 1:
		time.Sleep(5 * time.Millisecond) // > maxAckCoalesceDelay (1ms), on any clock
	case 2:
		select {
		case <-h.release:
		case <-time.After(2 * ackReadTimeout):
		}
	}
	return pk, nil
}

// An ack held for a burst is flushed once the burst has run for the delay
// bound, even though more packets are still buffered: a slow hook on later
// packets must not hold earlier acks hostage.
func TestAckCoalesceHeldAckFlushedAfterDelayBound(t *testing.T) {
	gate := &gateHook{release: make(chan struct{})}
	addr, _ := startCountingBroker(t, gate)
	c := dialV5(t, addr, "bound")

	_, err := c.conn.Write(publishBurst(t, 1, 3))
	require.NoError(t, err)

	c.expectPuback(t, 1) // publish 2 is blocked until this arrives
	close(gate.release)
	c.expectPuback(t, 2)
	c.expectPuback(t, 3)
}
