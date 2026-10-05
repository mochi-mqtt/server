// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package mqtt

import (
	"sort"
	"sync"
	"sync/atomic"

	"github.com/mochi-mqtt/server/v2/packets"
)

// Inflight holds the in-flight QoS messages of a client, keyed on packet id.
//
// The packet identifiers the server assigns to its own deliveries and the ones
// the client assigns to its publishes are independent of each other (MQTT 3.1.1
// section 2.3.1, MQTT 5 section 2.2.1), so the same number can be in flight in
// both directions at once. The two directions are therefore held apart: the
// server's flows (an outbound Publish awaiting its PUBACK or PUBREC, or a Pubrel
// awaiting its PUBCOMP) in one map, and the client's QoS 2 flows (a Pubrec
// awaiting the client's PUBREL, or a Pubcomp not yet written) in another.
type Inflight struct {
	sync.RWMutex
	internal            map[uint16]packets.Packet // the server's flows, keyed on the server's packet ids
	inbound             map[uint16]packets.Packet // the client's qos 2 flows, keyed on the client's packet ids
	receiveQuota        int32                     // remaining inbound qos quota for flow control
	sendQuota           int32                     // remaining outbound qos quota for flow control
	maximumReceiveQuota int32                     // maximum allowed receive quota
	maximumSendQuota    int32                     // maximum allowed send quota
}

// NewInflights returns a new instance of an Inflight packets map.
func NewInflights() *Inflight {
	return &Inflight{
		internal: map[uint16]packets.Packet{},
		inbound:  map[uint16]packets.Packet{},
	}
}

// isInbound returns true if m, as an in-flight message, belongs to a QoS 2 flow
// the client started: a Pubrec or Pubcomp the server holds while the client's
// PUBLISH is being released. Its packet id is one the client chose.
func isInbound(m packets.Packet) bool {
	return m.InboundFlow || m.FixedHeader.Type == packets.Pubrec || m.FixedHeader.Type == packets.Pubcomp
}

// side returns the map an in-flight message belongs to. The lock must be held.
func (i *Inflight) side(m packets.Packet) map[uint16]packets.Packet {
	if isInbound(m) {
		return i.inbound
	}
	return i.internal
}

// Set adds or updates an inflight packet by packet id, on the side of the
// direction it belongs to: a Pubrec or Pubcomp, or any packet marked
// InboundFlow, is held for the client's flow, and anything else for the
// server's. The returned value is true if no packet with that id was in flight
// in that direction. A packet held for the client's flow is marked InboundFlow.
func (i *Inflight) Set(m packets.Packet) bool {
	i.Lock()
	defer i.Unlock()

	if isInbound(m) {
		m.InboundFlow = true
	}

	side := i.side(m)
	_, ok := side[m.PacketID]
	side[m.PacketID] = m
	return !ok
}

// Get returns a packet the server has in flight to the client, by the packet id
// the server assigned.
func (i *Inflight) Get(id uint16) (packets.Packet, bool) {
	i.RLock()
	defer i.RUnlock()

	if m, ok := i.internal[id]; ok {
		return m, true
	}

	return packets.Packet{}, false
}

// GetInbound returns a packet held for a QoS 2 flow the client started, by the
// packet id the client assigned.
func (i *Inflight) GetInbound(id uint16) (packets.Packet, bool) {
	i.RLock()
	defer i.RUnlock()

	if m, ok := i.inbound[id]; ok {
		return m, true
	}

	return packets.Packet{}, false
}

// Len returns the number of inflight messages in both directions.
func (i *Inflight) Len() int {
	i.RLock()
	defer i.RUnlock()
	return len(i.internal) + len(i.inbound)
}

// Clone returns a new instance of Inflight with the same message data.
// This is used when transferring inflights from a taken-over session.
func (i *Inflight) Clone() *Inflight {
	c := NewInflights()
	i.RLock()
	defer i.RUnlock()
	for k, v := range i.internal {
		c.internal[k] = v
	}
	for k, v := range i.inbound {
		c.inbound[k] = v
	}
	return c
}

// GetAll returns all the inflight messages, in both directions. If immediate
// is true, only the server's messages waiting for send quota are returned.
func (i *Inflight) GetAll(immediate bool) []packets.Packet {
	i.RLock()
	defer i.RUnlock()

	m := []packets.Packet{}
	for _, v := range i.internal {
		if !immediate || (immediate && v.Expiry < 0) {
			m = append(m, v)
		}
	}

	if !immediate {
		for _, v := range i.inbound {
			m = append(m, v)
		}
	}

	// MQTT-4.6.0-1 requires that re-sent PUBLISH packets go out in the order
	// the originals were sent, so this comparison has to be a total order.
	// Created alone is not one: it is a unix timestamp in seconds, so every
	// packet made in the same second compares equal, and sort.Slice is not
	// stable — a tie is broken by map iteration order, which Go randomises.
	// Under any real publish rate most of a session's window is one second
	// wide.
	//
	// The packet identifier breaks the tie. It is allocated in increasing
	// order, so within a second it is the order the packets were sent.
	// The sort is stable so that, where a server delivery and a client flow
	// share an identifier, the server's comes first as it was appended first.
	sort.SliceStable(m, func(i, j int) bool {
		if m[i].Created != m[j].Created {
			return m[i].Created < m[j].Created
		}
		return m[i].PacketID < m[j].PacketID
	})

	return m
}

// NextImmediate returns the next inflight packet which is indicated to be sent immediately.
// This typically occurs when the quota has been exhausted, and we need to wait until new quota
// is free to continue sending.
func (i *Inflight) NextImmediate() (packets.Packet, bool) {
	// No lock is taken here: GetAll takes the read lock itself, and
	// sync.RWMutex is not reentrant for readers. Holding it across that call
	// deadlocks whenever a writer arrives between the two acquisitions — the
	// second RLock waits for the queued writer, and the writer waits for the
	// first RLock.
	m := i.GetAll(true)
	if len(m) > 0 {
		return m[0], true
	}

	return packets.Packet{}, false
}

// Delete removes a packet the server has in flight to the client, by the packet
// id the server assigned. Returns true if the message existed.
func (i *Inflight) Delete(id uint16) bool {
	i.Lock()
	defer i.Unlock()

	_, ok := i.internal[id]
	delete(i.internal, id)

	return ok
}

// DeleteInbound removes a packet held for a QoS 2 flow the client started, by
// the packet id the client assigned. Returns true if the message existed.
func (i *Inflight) DeleteInbound(id uint16) bool {
	i.Lock()
	defer i.Unlock()

	_, ok := i.inbound[id]
	delete(i.inbound, id)

	return ok
}

// deleteEntry removes m from the side of the direction it belongs to, as Set
// chooses it. It is for messages returned by GetAll. Returns true if the
// message existed.
func (i *Inflight) deleteEntry(m packets.Packet) bool {
	i.Lock()
	defer i.Unlock()

	side := i.side(m)
	_, ok := side[m.PacketID]
	delete(side, m.PacketID)

	return ok
}

// TakeRecieveQuota reduces the receive quota by 1.
func (i *Inflight) DecreaseReceiveQuota() {
	if atomic.LoadInt32(&i.receiveQuota) > 0 {
		atomic.AddInt32(&i.receiveQuota, -1)
	}
}

// TakeRecieveQuota increases the receive quota by 1.
func (i *Inflight) IncreaseReceiveQuota() {
	if atomic.LoadInt32(&i.receiveQuota) < atomic.LoadInt32(&i.maximumReceiveQuota) {
		atomic.AddInt32(&i.receiveQuota, 1)
	}
}

// ResetReceiveQuota resets the receive quota to the maximum allowed value.
func (i *Inflight) ResetReceiveQuota(n int32) {
	atomic.StoreInt32(&i.receiveQuota, n)
	atomic.StoreInt32(&i.maximumReceiveQuota, n)
}

// DecreaseSendQuota reduces the send quota by 1.
func (i *Inflight) DecreaseSendQuota() {
	if atomic.LoadInt32(&i.sendQuota) > 0 {
		atomic.AddInt32(&i.sendQuota, -1)
	}
}

// IncreaseSendQuota increases the send quota by 1.
func (i *Inflight) IncreaseSendQuota() {
	if atomic.LoadInt32(&i.sendQuota) < atomic.LoadInt32(&i.maximumSendQuota) {
		atomic.AddInt32(&i.sendQuota, 1)
	}
}

// ResetSendQuota resets the send quota to the maximum allowed value.
func (i *Inflight) ResetSendQuota(n int32) {
	atomic.StoreInt32(&i.sendQuota, n)
	atomic.StoreInt32(&i.maximumSendQuota, n)
}
