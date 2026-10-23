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

// Inflight is a map of InflightMessage keyed on packet id.
type Inflight struct {
	sync.RWMutex
	drainMu             sync.Mutex                // serialises sending pending packets, to keep their order
	internal            map[uint16]packets.Packet // internal contains the inflight packets
	pending             []uint16                  // ids of packets waiting to be sent, in order; may include ids no longer pending
	queued              map[uint16]struct{}       // ids of packets waiting to be sent
	receiveQuota        int32                     // remaining inbound qos quota for flow control
	sendQuota           int32                     // remaining outbound qos quota for flow control
	maximumReceiveQuota int32                     // maximum allowed receive quota
	maximumSendQuota    int32                     // maximum allowed send quota
}

// NewInflights returns a new instance of an Inflight packets map.
func NewInflights() *Inflight {
	return &Inflight{
		internal: map[uint16]packets.Packet{},
		queued:   map[uint16]struct{}{},
	}
}

// Set adds or updates an inflight packet by packet id. A packet with a negative expiry is
// marked as waiting to be sent, as by SetPending.
func (i *Inflight) Set(m packets.Packet) bool {
	i.Lock()
	defer i.Unlock()

	_, ok := i.internal[m.PacketID]
	i.internal[m.PacketID] = m
	if m.Expiry < 0 {
		i.queue(m.PacketID)
	}
	return !ok
}

// SetPending adds or updates an inflight packet which is waiting to be sent, for example
// because the client's send quota is exhausted. Pending packets are returned by
// NextImmediate in the order they were added.
func (i *Inflight) SetPending(m packets.Packet) bool {
	i.Lock()
	defer i.Unlock()

	_, ok := i.internal[m.PacketID]
	i.internal[m.PacketID] = m
	i.queue(m.PacketID)
	return !ok
}

// queue marks a packet as waiting to be sent. i must be locked.
func (i *Inflight) queue(id uint16) {
	if _, ok := i.queued[id]; ok {
		return
	}
	i.queued[id] = struct{}{}
	i.pending = append(i.pending, id)
}

// IsPending returns true if a packet is waiting to be sent.
func (i *Inflight) IsPending(id uint16) bool {
	i.RLock()
	defer i.RUnlock()
	_, ok := i.queued[id]
	return ok
}

// PendingLen returns the number of packets waiting to be sent.
func (i *Inflight) PendingLen() int {
	i.RLock()
	defer i.RUnlock()
	return len(i.queued)
}

// TakePending returns the next packet waiting to be sent, and marks it as sent. The packet
// stays inflight until it is acknowledged. A packet which was marked as pending by a
// negative expiry has its expiry cleared.
func (i *Inflight) TakePending() (packets.Packet, bool) {
	i.Lock()
	defer i.Unlock()

	for len(i.pending) > 0 {
		id := i.pending[0]
		i.pending = i.pending[1:]
		if _, ok := i.queued[id]; !ok {
			continue // no longer pending
		}
		delete(i.queued, id)

		m, ok := i.internal[id]
		if !ok {
			continue
		}

		if m.Expiry < 0 {
			m.Expiry = 0
			i.internal[id] = m
		}
		i.compact()
		return m, true
	}

	i.pending = nil
	return packets.Packet{}, false
}

// requeue returns a packet taken by TakePending to the front of the queue, as when it
// couldn't be sent after all.
func (i *Inflight) requeue(m packets.Packet) {
	i.Lock()
	defer i.Unlock()
	if _, ok := i.internal[m.PacketID]; !ok {
		return // acknowledged or expired meanwhile
	}
	if _, ok := i.queued[m.PacketID]; ok {
		return
	}
	i.queued[m.PacketID] = struct{}{}
	i.pending = append([]uint16{m.PacketID}, i.pending...)
}

// hasSendQuota returns true if another qos > 0 packet may be sent. A zero maximum send
// quota means sending isn't limited.
func (i *Inflight) hasSendQuota() bool {
	return atomic.LoadInt32(&i.maximumSendQuota) == 0 || atomic.LoadInt32(&i.sendQuota) > 0
}

// ClearPending marks every pending packet as sent, as when all inflight packets are resent.
func (i *Inflight) ClearPending() {
	i.Lock()
	defer i.Unlock()
	for id := range i.queued {
		if m, ok := i.internal[id]; ok && m.Expiry < 0 {
			m.Expiry = 0
			i.internal[id] = m
		}
	}
	i.pending = nil
	i.queued = map[uint16]struct{}{}
}

// compact releases the queue's backing array once it is mostly stale entries. i must be locked.
func (i *Inflight) compact() {
	if len(i.pending) > 64 && len(i.pending) > len(i.queued)*4 {
		live := make([]uint16, 0, len(i.queued))
		for _, id := range i.pending {
			if _, ok := i.queued[id]; ok {
				live = append(live, id)
			}
		}
		i.pending = live
	}
}

// Get returns an inflight packet by packet id.
func (i *Inflight) Get(id uint16) (packets.Packet, bool) {
	i.RLock()
	defer i.RUnlock()

	if m, ok := i.internal[id]; ok {
		return m, true
	}

	return packets.Packet{}, false
}

// Len returns the size of the inflight messages map.
func (i *Inflight) Len() int {
	i.RLock()
	defer i.RUnlock()
	return len(i.internal)
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
	for _, id := range i.pending {
		if _, ok := i.queued[id]; ok {
			c.queue(id)
		}
	}
	return c
}

// GetAll returns all the inflight messages, or if immediate is true, those waiting to be
// sent, in the order they are to be sent.
func (i *Inflight) GetAll(immediate bool) []packets.Packet {
	i.RLock()
	defer i.RUnlock()

	m := []packets.Packet{}
	if immediate {
		for _, id := range i.pending {
			if _, ok := i.queued[id]; ok {
				if v, ok := i.internal[id]; ok {
					m = append(m, v)
				}
			}
		}
		return m
	}

	for _, v := range i.internal {
		m = append(m, v)
	}

	sort.Slice(m, func(i, j int) bool {
		return uint16(m[i].Created) < uint16(m[j].Created)
	})

	return m
}

// NextImmediate returns the next inflight packet which is waiting to be sent, without
// marking it as sent. This typically occurs when the quota has been exhausted, and we need
// to wait until new quota is free to continue sending.
func (i *Inflight) NextImmediate() (packets.Packet, bool) {
	i.RLock()
	defer i.RUnlock()

	for _, id := range i.pending {
		if _, ok := i.queued[id]; ok {
			if v, ok := i.internal[id]; ok {
				return v, true
			}
		}
	}

	return packets.Packet{}, false
}

// Delete removes an in-flight message from the map. Returns true if the message existed.
func (i *Inflight) Delete(id uint16) bool {
	i.Lock()
	defer i.Unlock()

	_, ok := i.internal[id]
	delete(i.internal, id)
	delete(i.queued, id)
	i.compact()

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
