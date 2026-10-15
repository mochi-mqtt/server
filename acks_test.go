// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 mochi-mqtt, mochi-co
// SPDX-FileContributor: MaestroHub

package mqtt

import (
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mochi-mqtt/server/v2/packets"
	"github.com/stretchr/testify/require"
)

// settleHook takes a hold on every publish and hands the release to the
// test, so the test decides when (and how) each publish is settled.
type settleHook struct {
	HookBase
	mu       sync.Mutex
	releases map[string]func(packets.Code)
	seen     atomic.Int32
	qos0Hold atomic.Bool
}

func newSettleHook() *settleHook { return &settleHook{releases: map[string]func(packets.Code){}} }

func (h *settleHook) ID() string { return "settle-test" }

func (h *settleHook) Provides(b byte) bool { return b == OnPublishedSettle }

func (h *settleHook) OnPublishedSettle(cl *Client, pk packets.Packet, s *PublishSettlement) {
	h.seen.Add(1)
	release := s.Hold()
	if pk.FixedHeader.Qos == 0 {
		release(packets.CodeSuccess) // a no-op: nothing to acknowledge
		h.qos0Hold.Store(true)
		return
	}
	h.mu.Lock()
	h.releases[key(cl.ID, pk.PacketID)] = release
	h.mu.Unlock()
}

func key(clientID string, id uint16) string { return fmt.Sprintf("%s/%d", clientID, id) }

func (h *settleHook) release(t *testing.T, id uint16, code packets.Code) {
	t.Helper()
	h.releaseFor(t, "mochi", id, code)
}

func (h *settleHook) releaseFor(t *testing.T, clientID string, id uint16, code packets.Code) {
	t.Helper()
	require.Eventually(t, func() bool {
		h.mu.Lock()
		defer h.mu.Unlock()
		return h.releases[key(clientID, id)] != nil
	}, time.Second, time.Millisecond)
	h.mu.Lock()
	r := h.releases[key(clientID, id)]
	h.mu.Unlock()
	r(code)
}

// wirePacket is one packet the client received: type, packet id, reason.
type wirePacket struct {
	typ    byte
	id     uint16
	reason byte
}

// readWire decodes what the server writes to the client side of the pipe,
// one packet at a time.
func readWire(r net.Conn) <-chan wirePacket {
	out := make(chan wirePacket, 64)
	go func() {
		defer close(out)
		hdr := make([]byte, 1)
		for {
			if _, err := io.ReadFull(r, hdr); err != nil {
				return
			}
			n, mult := 0, 1
			for {
				b := make([]byte, 1)
				if _, err := io.ReadFull(r, b); err != nil {
					return
				}
				n += int(b[0]&0x7f) * mult
				if b[0]&0x80 == 0 {
					break
				}
				mult *= 128
			}
			body := make([]byte, n)
			if _, err := io.ReadFull(r, body); err != nil {
				return
			}
			wp := wirePacket{typ: hdr[0] >> 4}
			if n >= 2 {
				wp.id = uint16(body[0])<<8 | uint16(body[1])
			}
			if n >= 3 {
				wp.reason = body[2]
			}
			out <- wp
		}
	}()
	return out
}

func publishQos(id uint16, qos byte) packets.Packet {
	pk := *packets.TPacketData[packets.Publish].Get(packets.TPublishQos1).Packet
	pk.PacketID = id
	pk.FixedHeader.Qos = qos
	return pk
}

func expectNone(t *testing.T, wire <-chan wirePacket) {
	t.Helper()
	select {
	case p, ok := <-wire:
		if ok {
			t.Fatalf("unexpected packet written: %+v", p)
		}
	case <-time.After(50 * time.Millisecond):
	}
}

func expectAck(t *testing.T, wire <-chan wirePacket, typ byte, id uint16) wirePacket {
	t.Helper()
	select {
	case p := <-wire:
		require.Equal(t, typ, p.typ, "packet type")
		require.Equal(t, id, p.id, "packet id")
		return p
	case <-time.After(2 * time.Second):
		t.Fatalf("no ack for packet %d", id)
	}
	return wirePacket{}
}

func newSettleServer(t *testing.T) (*Server, *settleHook) {
	t.Helper()
	s := newServer()
	h := newSettleHook()
	require.NoError(t, s.AddHook(h, nil))
	return s, h
}

func TestAckWaitsForSettlement(t *testing.T) {
	s, h := newSettleServer(t)
	cl, r, _ := newTestClient()
	wire := readWire(r)

	require.NoError(t, s.processPacket(cl, publishQos(1, 1)))
	expectNone(t, wire) // handled? not yet: no PUBACK

	h.release(t, 1, packets.CodeSuccess)
	expectAck(t, wire, packets.Puback, 1)
	require.Equal(t, int32(10), cl.State.Inflight.receiveQuota, "receive quota returned with the ack")
	require.Equal(t, 0, cl.State.Inflight.Len(), "a QoS 1 ack never enters the inflight store")
}

func TestAcksLeaveInArrivalOrder(t *testing.T) {
	s, h := newSettleServer(t)
	cl, r, _ := newTestClient()
	wire := readWire(r)

	for id := uint16(1); id <= 3; id++ {
		require.NoError(t, s.processPacket(cl, publishQos(id, 1)))
	}
	h.release(t, 3, packets.CodeSuccess)
	h.release(t, 2, packets.CodeSuccess)
	expectNone(t, wire) // 1 still pending holds 2 and 3 back

	h.release(t, 1, packets.CodeSuccess)
	expectAck(t, wire, packets.Puback, 1)
	expectAck(t, wire, packets.Puback, 2)
	expectAck(t, wire, packets.Puback, 3)
}

func TestSettledPrefixIsOneWrite(t *testing.T) {
	s, h := newSettleServer(t)
	cl, r, w := newTestClient()
	counter := &writeCounter{Conn: w}
	cl.Net.Conn = counter
	cl.ops.options.ClientNetWriteBufferSize = 2048 // the server's default (newTestClient leaves it 0)
	wire := readWire(r)

	for id := uint16(1); id <= 3; id++ {
		require.NoError(t, s.processPacket(cl, publishQos(id, 1)))
	}
	h.release(t, 3, packets.CodeSuccess)
	h.release(t, 2, packets.CodeSuccess)
	h.release(t, 1, packets.CodeSuccess)
	for id := uint16(1); id <= 3; id++ {
		expectAck(t, wire, packets.Puback, id)
	}
	require.Equal(t, int32(1), counter.writes.Load(), "three settled acks, one write")
}

type writeCounter struct {
	net.Conn
	writes atomic.Int32
}

func (c *writeCounter) Write(b []byte) (int, error) {
	c.writes.Add(1)
	return c.Conn.Write(b)
}

func TestRefusalWaitsBehindPendingAcks(t *testing.T) {
	s, h := newSettleServer(t)
	cl, r, _ := newTestClient()
	cl.Properties.ProtocolVersion = 5
	wire := readWire(r)

	require.NoError(t, s.processPacket(cl, publishQos(1, 1)))
	// A refusal decided on arrival (MQTT 5, a reason code from OnPublish)
	// must not overtake publish 1, which is still being handled.
	require.NoError(t, s.AddHook(&refuseHook{}, nil))
	require.NoError(t, s.processPacket(cl, publishQos(2, 1)))
	expectNone(t, wire)

	h.release(t, 1, packets.CodeSuccess)
	expectAck(t, wire, packets.Puback, 1)
	p := expectAck(t, wire, packets.Puback, 2)
	require.Equal(t, packets.ErrQuotaExceeded.Code, p.reason)
}

type refuseHook struct{ HookBase }

func (h *refuseHook) ID() string           { return "refuse" }
func (h *refuseHook) Provides(b byte) bool { return b == OnPublish }
func (h *refuseHook) OnPublish(cl *Client, pk packets.Packet) (packets.Packet, error) {
	return pk, packets.ErrQuotaExceeded
}

func TestFailedSettlementMqtt5ReasonCode(t *testing.T) {
	s, h := newSettleServer(t)
	cl, r, _ := newTestClient()
	cl.Properties.ProtocolVersion = 5
	wire := readWire(r)

	require.NoError(t, s.processPacket(cl, publishQos(1, 1)))
	h.release(t, 1, packets.ErrUnspecifiedError)
	p := expectAck(t, wire, packets.Puback, 1)
	require.Equal(t, packets.ErrUnspecifiedError.Code, p.reason)
	require.False(t, cl.Closed())
}

func TestFailedSettlementMqtt3Disconnects(t *testing.T) {
	s, h := newSettleServer(t)
	cl, r, _ := newTestClient()
	wire := readWire(r)

	require.NoError(t, s.processPacket(cl, publishQos(1, 1)))
	require.NoError(t, s.processPacket(cl, publishQos(2, 1)))
	h.release(t, 2, packets.CodeSuccess)
	h.release(t, 1, packets.ErrUnspecifiedError)
	// No reason codes before MQTT 5: no ack for 1, none for 2 behind it,
	// and the connection ends so the client sends both again.
	require.Eventually(t, cl.Closed, time.Second, time.Millisecond)
	expectNone(t, wire)
}

func TestQos0AndInlineHoldIsNoop(t *testing.T) {
	s, h := newSettleServer(t)
	cl, r, _ := newTestClient()
	wire := readWire(r)

	require.NoError(t, s.processPacket(cl, publishQos(0, 0)))
	require.True(t, h.qos0Hold.Load(), "OnPublishedSettle sees QoS 0 publishes too")
	expectNone(t, wire)
	require.False(t, cl.State.acks.busy())
}

func TestQos2PubrecWaitsAndEntersInflight(t *testing.T) {
	s, h := newSettleServer(t)
	cl, r, _ := newTestClient()
	wire := readWire(r)

	require.NoError(t, s.processPacket(cl, publishQos(7, 2)))
	expectNone(t, wire)
	require.Equal(t, 0, cl.State.Inflight.Len(), "nothing inflight before the PUBREC is written")

	h.release(t, 7, packets.CodeSuccess)
	expectAck(t, wire, packets.Pubrec, 7)
	require.Eventually(t, func() bool { return cl.State.Inflight.Len() == 1 }, time.Second, time.Millisecond)

	pubrel := *packets.TPacketData[packets.Pubrel].Get(packets.TPubrel).Packet
	pubrel.PacketID = 7
	require.NoError(t, s.processPacket(cl, pubrel))
	expectAck(t, wire, packets.Pubcomp, 7)
}

func TestOtherClientsAreNotHeldBack(t *testing.T) {
	s, h := newSettleServer(t)
	a, ra, _ := newTestClient()
	b, rb, _ := newTestClient()
	b.ID = "other"
	wireA, wireB := readWire(ra), readWire(rb)

	require.NoError(t, s.processPacket(a, publishQos(1, 1)))
	require.NoError(t, s.processPacket(b, publishQos(1, 1)))
	// Settle b's only: a stays pending, b is acknowledged.
	h.releaseFor(t, "other", 1, packets.CodeSuccess)
	expectAck(t, wireB, packets.Puback, 1)
	expectNone(t, wireA)
	require.True(t, a.State.acks.busy())
}

func TestMqtt3BackPressureInsteadOfDisconnect(t *testing.T) {
	s, h := newSettleServer(t)
	cl, r, _ := newTestClient()
	cl.State.Inflight.ResetReceiveQuota(2)
	wire := readWire(r)

	require.NoError(t, s.processPacket(cl, publishQos(1, 1)))
	require.NoError(t, s.processPacket(cl, publishQos(2, 1)))

	// Quota spent: the third publish waits for an ack instead of the
	// client being disconnected.
	done := make(chan error, 1)
	go func() { done <- s.processPacket(cl, publishQos(3, 1)) }()
	select {
	case err := <-done:
		t.Fatalf("third publish was not held back: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	h.release(t, 1, packets.CodeSuccess)
	expectAck(t, wire, packets.Puback, 1)
	require.NoError(t, <-done)
	require.False(t, cl.Closed())

	h.release(t, 2, packets.CodeSuccess)
	h.release(t, 3, packets.CodeSuccess)
	expectAck(t, wire, packets.Puback, 2)
	expectAck(t, wire, packets.Puback, 3)
}

func TestMqtt5ExceedingReceiveMaximumStillDisconnects(t *testing.T) {
	s, _ := newSettleServer(t)
	cl, r, _ := newTestClient()
	cl.Properties.ProtocolVersion = 5
	cl.State.Inflight.ResetReceiveQuota(1)
	go func() { _, _ = io.Copy(io.Discard, r) }()

	require.NoError(t, s.processPacket(cl, publishQos(1, 1)))
	err := s.processPacket(cl, publishQos(2, 1))
	require.ErrorIs(t, err, packets.ErrReceiveMaximum)
}

func TestWithoutSettleHookAckIsImmediate(t *testing.T) {
	s := newServer()
	cl, r, _ := newTestClient()
	wire := readWire(r)

	require.NoError(t, s.processPacket(cl, publishQos(1, 1)))
	expectAck(t, wire, packets.Puback, 1)
}

func TestReleaseTwiceIsIgnored(t *testing.T) {
	s, h := newSettleServer(t)
	cl, r, _ := newTestClient()
	wire := readWire(r)

	require.NoError(t, s.processPacket(cl, publishQos(1, 1)))
	require.NoError(t, s.processPacket(cl, publishQos(2, 1)))
	h.mu.Lock()
	rel1 := h.releases[key("mochi", 1)]
	h.mu.Unlock()
	rel1(packets.CodeSuccess)
	rel1(packets.CodeSuccess) // must not release publish 2's hold or ack twice
	expectAck(t, wire, packets.Puback, 1)
	expectNone(t, wire)
	h.release(t, 2, packets.CodeSuccess)
	expectAck(t, wire, packets.Puback, 2)
}

// doneHook records the settlement it was handed.
type doneHook struct {
	HookBase
	got chan *PublishSettlement
}

func (h *doneHook) ID() string           { return "done-test" }
func (h *doneHook) Provides(b byte) bool { return b == OnPublishedSettle }
func (h *doneHook) OnPublishedSettle(cl *Client, pk packets.Packet, s *PublishSettlement) {
	_ = s.Hold() // never released: the client goes away first
	h.got <- s
}

func TestSettlementWithholdsAndDone(t *testing.T) {
	s := newServer()
	h := &doneHook{got: make(chan *PublishSettlement, 2)}
	require.NoError(t, s.AddHook(h, nil))
	cl, r, _ := newTestClient()
	go func() { _, _ = io.Copy(io.Discard, r) }()

	require.NoError(t, s.processPacket(cl, publishQos(0, 0)))
	q0 := <-h.got
	require.False(t, q0.Withholds(), "QoS 0: nothing to withhold")
	require.Nil(t, q0.Done(), "QoS 0: Done never closes")

	require.NoError(t, s.processPacket(cl, publishQos(1, 1)))
	q1 := <-h.got
	require.True(t, q1.Withholds())
	select {
	case <-q1.Done():
		t.Fatal("Done closed while the client is connected")
	default:
	}
	cl.Stop(errClientStop)
	select {
	case <-q1.Done():
	case <-time.After(time.Second):
		t.Fatal("Done not closed after the client went away")
	}
}

func TestRefusalCodeReachesTheClient(t *testing.T) {
	s, h := newSettleServer(t)
	cl, r, _ := newTestClient()
	cl.Properties.ProtocolVersion = 5
	wire := readWire(r)

	require.NoError(t, s.processPacket(cl, publishQos(1, 1)))
	h.release(t, 1, packets.ErrQuotaExceeded)
	p := expectAck(t, wire, packets.Puback, 1)
	require.Equal(t, packets.ErrQuotaExceeded.Code, p.reason)
}
