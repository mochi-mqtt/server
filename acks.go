// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 mochi-mqtt, mochi-co
// SPDX-FileContributor: MaestroHub

package mqtt

// With a hook that provides OnPublishedSettle, a QoS 1 PUBACK / QoS 2 PUBREC is
// written only once every hook that took the publish's PublishSettlement
// released it, in the order the publishes arrived on the client.
//
// Without it the broker acknowledges a publish before a hook that stores
// the message (writes it to a database, say) has done so: a crash between
// the two loses a message the client was told the broker has. With it, an
// acknowledgement means "handled", and a client that is not acknowledged
// still holds its message.
//
// Order is per client and required by MQTT ([MQTT-4.6.0-2]: PUBACKs in
// the order the PUBLISHes were received). One client's acks wait for its
// slowest pending publish; other clients are not involved — each client
// has its own sequencer.

import (
	"sync"
	"sync/atomic"

	"github.com/mochi-mqtt/server/v2/packets"
)

// PublishSettlement holds back the acknowledgement of one inbound publish
// until every hook that took a hold has released it. Hooks get it in
// OnPublishedSettle.
//
// For a QoS 0 publish, or one from an inline client, there is nothing to
// hold back: Hold returns a release that does nothing, so hooks treat every
// publish the same way.
type PublishSettlement struct {
	cl    *Client
	ack   *pendingAck // nil: nothing to acknowledge
	holds atomic.Int32
	// failed is the first refusal a release reported (nil: none).
	failed atomic.Pointer[packets.Code]
}

// Hold takes a hold on the publish's acknowledgement and returns the
// function that releases it. It must be called before OnPublishedSettle
// returns; release may be called later, from any goroutine, and must be
// called exactly once (later calls are ignored).
//
// release(code): a code below 0x80 means the hook is done with the message
// (handled, or refused for good and accounted for by the hook). A code of
// 0x80 or above means the hook could not handle it: an MQTT 5 client gets
// that reason code in its PUBACK/PUBREC; an MQTT 3 client, which has no
// reason codes, is disconnected without the acknowledgement, so it sends
// the message again.
func (s *PublishSettlement) Hold() (release func(code packets.Code)) {
	if !s.Withholds() {
		return func(packets.Code) {}
	}
	s.holds.Add(1)
	var once atomic.Bool
	return func(code packets.Code) {
		if once.Swap(true) {
			return
		}
		s.release(code)
	}
}

// Withholds reports whether an acknowledgement waits for this settlement:
// a QoS 1/2 publish from a network client. For any other publish Hold is a
// no-op and nobody can be told the message was not handled.
func (s *PublishSettlement) Withholds() bool {
	return s != nil && s.ack != nil
}

// Done is closed when the client the publish came from is gone: an
// acknowledgement could no longer reach it. Nil (never closed) when the
// settlement withholds nothing.
func (s *PublishSettlement) Done() <-chan struct{} {
	if !s.Withholds() || s.cl.State.open == nil {
		return nil
	}
	return s.cl.State.open.Done()
}

func (s *PublishSettlement) release(code packets.Code) {
	if code.Code >= packets.ErrUnspecifiedError.Code {
		s.failed.CompareAndSwap(nil, &code)
	}
	if s.holds.Add(-1) != 0 {
		return
	}
	reason := packets.Code{}
	if f := s.failed.Load(); f != nil {
		reason = *f
	}
	s.cl.State.acks.settle(s.cl, s.ack, reason)
}

// pendingAck is one acknowledgement waiting for its turn.
type pendingAck struct {
	ack     packets.Packet
	settled bool
	reason  packets.Code // refusal (Code >= 0x80) or zero
	// reserved: the ack of a publish that took receive quota and is
	// handled by hooks (reserve), as opposed to a refusal queued behind
	// the pending ones (writeNowOrQueue).
	reserved bool
}

// ackSequencer writes one client's publish acknowledgements in arrival
// order, each once it is settled. All of a client's QoS 1/2 publish acks go
// through it while any is pending, so a refusal written at once cannot
// overtake an earlier publish still being handled.
type ackSequencer struct {
	mu      sync.Mutex
	pending []*pendingAck
	// freed gets a token whenever an ack returned receive quota; the
	// reader waits on it (waitForQuota).
	freed chan struct{}
}

func (q *ackSequencer) init() {
	q.freed = make(chan struct{}, 1)
}

// reserve appends the ack of a publish being handled. Called by the
// client's reader, so the queue order is the arrival order.
func (q *ackSequencer) reserve(ack packets.Packet) *pendingAck {
	p := &pendingAck{ack: ack, reserved: true}
	q.mu.Lock()
	q.pending = append(q.pending, p)
	q.mu.Unlock()
	return p
}

// busy reports whether acknowledgements are waiting.
func (q *ackSequencer) busy() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.pending) > 0
}

// writeNowOrQueue writes an ack that needs no handling (a refusal decided
// on arrival) — at once if nothing is pending, else behind the pending
// ones.
func (q *ackSequencer) writeNowOrQueue(cl *Client, ack packets.Packet) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.pending) == 0 {
		return cl.WritePacket(ack)
	}
	q.pending = append(q.pending, &pendingAck{ack: ack, settled: true})
	return nil
}

// settle marks p settled and writes every settled ack at the head of the
// queue. Writing happens under the sequencer's lock, which is what keeps
// two settling goroutines from interleaving their writes out of order.
func (q *ackSequencer) settle(cl *Client, p *pendingAck, reason packets.Code) {
	q.mu.Lock()
	defer q.mu.Unlock()
	p.settled, p.reason = true, reason

	n := 0
	for n < len(q.pending) && q.pending[n].settled {
		n++
	}
	if n == 0 {
		return
	}
	ready := q.pending[:n]
	q.pending = append(q.pending[:0:0], q.pending[n:]...)

	acks := make([]packets.Packet, 0, len(ready))
	reserved := make([]bool, 0, len(ready))
	disconnect := false
	for _, r := range ready {
		if r.reason.Code >= packets.ErrUnspecifiedError.Code {
			if cl.Properties.ProtocolVersion < 5 {
				// No reason codes before MQTT 5: withhold this ack and
				// everything behind it, and end the connection so the
				// client sends them again.
				disconnect = true
				break
			}
			r.ack.ReasonCode = r.reason.Code
			r.ack.Properties.ReasonString = r.reason.Reason
		}
		acks = append(acks, r.ack)
		reserved = append(reserved, r.reserved)
	}
	if disconnect {
		q.pending = nil
	}

	if len(acks) > 0 {
		_ = cl.writeAcks(acks) // a closed client: the acks go nowhere, the client resends
		for i := range acks {
			if reserved[i] {
				cl.ackWritten(acks[i])
			}
		}
		select {
		case q.freed <- struct{}{}:
		default:
		}
	}
	if disconnect {
		cl.Stop(packets.ErrUnspecifiedError)
	}
}

// ackWritten does, for an ack the sequencer wrote, what processPublish does
// around an ack it writes at once: a PUBACK ends the QoS 1 flow and returns
// the receive quota; a PUBREC enters the inflight store to await PUBREL (or,
// carrying a refusal, ends the flow).
func (cl *Client) ackWritten(ack packets.Packet) {
	switch ack.FixedHeader.Type {
	case packets.Puback:
		cl.ops.hooks.OnQosPublish(cl, ack, ack.Created, 0)
		cl.State.Inflight.IncreaseReceiveQuota()
		cl.ops.hooks.OnQosComplete(cl, ack)
	case packets.Pubrec:
		if ack.ReasonCode >= packets.ErrUnspecifiedError.Code {
			cl.State.Inflight.IncreaseReceiveQuota() // a refused PUBREC ends the QoS 2 flow
			return
		}
		if ok := cl.State.Inflight.Set(ack); ok {
			atomic.AddInt64(&cl.ops.info.Inflight, 1)
			cl.ops.hooks.OnQosPublish(cl, ack, ack.Created, 0)
		}
	}
}

// waitForQuota blocks the client's reader until an acknowledgement returns
// receive quota, or the client closes (false). Back-pressure for a client
// that sends faster than its publishes are handled: its next packet is not
// read, so its TCP window closes, instead of the connection being dropped.
func (q *ackSequencer) waitForQuota(cl *Client) bool {
	for atomic.LoadInt32(&cl.State.Inflight.receiveQuota) == 0 {
		if !q.busy() {
			return true // nothing pending will return quota; let the caller decide
		}
		select {
		case <-q.freed:
		case <-cl.State.open.Done():
			return false
		}
	}
	return true
}
