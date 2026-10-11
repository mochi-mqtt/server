// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package mqtt

// Unit tests for the write side of PUBACK coalescing. They use
// newTestClient's net.Pipe, which is unbuffered: a Write blocks until the
// test reads it, so "this call returned without anyone reading" proves the
// bytes were held, and "the first read returned exactly N bytes" proves how
// much one write carried. No assertion depends on elapsed time.

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	"github.com/mochi-mqtt/server/v2/packets"
	"github.com/stretchr/testify/require"
)

const v5PubackLen = 4 // fixed header (2) + packet id (2); success carries no reason byte

func testPuback(id uint16) packets.Packet {
	return packets.Packet{FixedHeader: packets.FixedHeader{Type: packets.Puback}, PacketID: id}
}

// within runs fn in a goroutine and fails the test if it does not return
// promptly — the alternative to a held write is a hung test.
func within(t *testing.T, fn func() error) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- fn() }()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("call blocked: bytes were written to the pipe instead of held")
	}
}

func requirePubackIDs(t *testing.T, b []byte, first, last uint16) {
	t.Helper()
	require.Len(t, b, int(last-first+1)*v5PubackLen)
	for i, id := 0, first; id <= last; i, id = i+v5PubackLen, id+1 {
		require.Equal(t, []byte{packets.Puback << 4, 2}, b[i:i+2])
		require.Equal(t, id, binary.BigEndian.Uint16(b[i+2:]), "acks out of order")
	}
}

func newCoalescingTestClient(t *testing.T, bufSize int) (*Client, net.Conn) {
	cl, r, _ := newTestClient()
	t.Cleanup(func() { cl.Stop(errClientStop) })
	cl.ops.options.ClientNetWriteBufferSize = bufSize
	cl.Properties.ProtocolVersion = 5
	return cl, r
}

// Without a buffered packet behind it, writeAck is WritePacket: one write.
func TestWriteAckWithoutReadAheadWritesImmediately(t *testing.T) {
	cl, r := newCoalescingTestClient(t, 1024)

	go func() { _ = cl.writeAck(testPuback(1)) }()
	b := make([]byte, 64)
	n, err := r.Read(b)
	require.NoError(t, err)
	requirePubackIDs(t, b[:n], 1, 1)
}

// Held acks and a packet written through any other path leave in call order,
// in one write.
func TestWriteAckHeldThenOtherPacketFlushesInOrder(t *testing.T) {
	cl, r := newCoalescingTestClient(t, 1024)
	cl.State.readAhead.Store(true)

	for id := uint16(1); id <= 3; id++ {
		within(t, func() error { return cl.writeAck(testPuback(id)) })
	}

	go func() {
		_ = cl.WritePacket(packets.Packet{FixedHeader: packets.FixedHeader{Type: packets.Pingresp}})
	}()
	b := make([]byte, 256)
	n, err := r.Read(b)
	require.NoError(t, err)
	require.Equal(t, 3*v5PubackLen+2, n, "held acks and the next packet must share one write")
	requirePubackIDs(t, b[:3*v5PubackLen], 1, 3)
	require.Equal(t, []byte{packets.Pingresp << 4, 0}, b[3*v5PubackLen:n])
}

// A client that is not reading: held bytes never exceed the write buffer size
// before the writer blocks in a flush (back-pressure), and nothing is lost or
// reordered once it reads again.
func TestWriteAckSlowReaderBackPressureBounded(t *testing.T) {
	const bufSize = 16 * v5PubackLen
	const n = 40
	cl, r := newCoalescingTestClient(t, bufSize)
	cl.State.readAhead.Store(true)

	done := make(chan error, 1)
	go func() {
		for id := uint16(1); id <= n; id++ {
			if err := cl.writeAck(testPuback(id)); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()

	b := make([]byte, 4096)
	k, err := r.Read(b)
	require.NoError(t, err)
	require.Equal(t, bufSize, k, "a full buffer is flushed as one bounded write")
	requirePubackIDs(t, b[:k], 1, 16)

	k, err = r.Read(b)
	require.NoError(t, err)
	requirePubackIDs(t, b[:k], 17, 32)

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("writer still blocked after the reader drained two buffers")
	}

	// The tail (8 acks) is below the threshold: held until the reader flushes.
	go func() { _ = cl.flushPending() }()
	k, err = r.Read(b)
	require.NoError(t, err)
	requirePubackIDs(t, b[:k], 33, n)
}

// The connection closing while acks are held: no write is attempted on the
// closed conn, nothing blocks, and later writes report the closure.
func TestWriteAckCloseWhileHeld(t *testing.T) {
	cl, _ := newCoalescingTestClient(t, 1024)
	cl.State.readAhead.Store(true)

	within(t, func() error { return cl.writeAck(testPuback(1)) })
	cl.Stop(errClientStop)

	require.ErrorIs(t, cl.flushPending(), ErrConnectionClosed)
	require.ErrorIs(t, cl.writeAck(testPuback(2)), ErrConnectionClosed)
}

// Read flushes held acks when its burst ends because the handler failed.
func TestReadFlushesHeldAcksOnHandlerError(t *testing.T) {
	cl, r := newCoalescingTestClient(t, 1024)

	burst := make([]byte, 0, 32)
	for i := 0; i < 3; i++ {
		burst = append(burst, packets.Pingreq<<4, 0)
	}

	readErr := make(chan error, 1)
	acks := make(chan []byte, 1)
	go func() {
		_, _ = r.Write(burst)
		b, _ := io.ReadAll(r)
		acks <- b
	}()

	go func() {
		var seen uint16
		readErr <- cl.Read(func(cl *Client, pk packets.Packet) error {
			seen++
			if seen == 3 {
				return io.ErrUnexpectedEOF // the burst ends in an error
			}
			return cl.writeAck(testPuback(seen)) // held: the next packet is buffered
		})
		cl.Stop(errClientStop) // as attachClient does once Read returns
	}()

	require.ErrorIs(t, <-readErr, io.ErrUnexpectedEOF)
	requirePubackIDs(t, <-acks, 1, 2)
}

func TestNextPacketBuffered(t *testing.T) {
	long := append([]byte{packets.Publish << 4, 0xC8, 0x01}, make([]byte, 200)...) // remaining length 200

	tt := []struct {
		name string
		data []byte
		want bool
	}{
		{"empty", nil, false},
		{"type byte only", []byte{packets.Pingreq << 4}, false},
		{"zero-length packet", []byte{packets.Pingreq << 4, 0}, true},
		{"body short by one", []byte{packets.Publish << 4, 5, 1, 2, 3, 4}, false},
		{"body complete", []byte{packets.Publish << 4, 5, 1, 2, 3, 4, 5}, true},
		{"complete plus partial next", []byte{packets.Pingreq << 4, 0, packets.Publish << 4}, true},
		{"multi-byte length, complete", long, true},
		{"multi-byte length, short", long[:len(long)-1], false},
		{"length continues past buffer", []byte{packets.Publish << 4, 0xC8}, false},
		{"malformed length", []byte{packets.Publish << 4, 0xFF, 0xFF, 0xFF, 0xFF, 0x01}, false},
	}

	for _, tx := range tt {
		t.Run(tx.name, func(t *testing.T) {
			cl := &Client{}
			cl.Net.bconn = bufio.NewReaderSize(bytes.NewReader(tx.data), 512)
			_, _ = cl.Net.bconn.Peek(1) // fill the buffer, as a prior read would have
			require.Equal(t, tx.want, cl.nextPacketBuffered())
		})
	}

	require.False(t, (&Client{}).nextPacketBuffered(), "no reader")
}
