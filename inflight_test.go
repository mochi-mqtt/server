// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package mqtt

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/mochi-mqtt/server/v2/packets"
	"github.com/stretchr/testify/require"
)

func TestInflightSet(t *testing.T) {
	cl, _, _ := newTestClient()

	r := cl.State.Inflight.Set(packets.Packet{PacketID: 1})
	require.True(t, r)
	require.NotNil(t, cl.State.Inflight.internal[1])
	require.NotEqual(t, 0, cl.State.Inflight.internal[1].PacketID)

	r = cl.State.Inflight.Set(packets.Packet{PacketID: 1})
	require.False(t, r)
}

func TestInflightGet(t *testing.T) {
	cl, _, _ := newTestClient()
	cl.State.Inflight.Set(packets.Packet{PacketID: 2})

	msg, ok := cl.State.Inflight.Get(2)
	require.True(t, ok)
	require.NotEqual(t, 0, msg.PacketID)
}

func TestInflightGetAllAndImmediate(t *testing.T) {
	cl, _, _ := newTestClient()
	cl.State.Inflight.Set(packets.Packet{PacketID: 1, Created: 1})
	cl.State.Inflight.Set(packets.Packet{PacketID: 2, Created: 2})
	cl.State.Inflight.Set(packets.Packet{PacketID: 3, Created: 3, Expiry: -1})
	cl.State.Inflight.Set(packets.Packet{PacketID: 4, Created: 4, Expiry: -1})
	cl.State.Inflight.Set(packets.Packet{PacketID: 5, Created: 5})

	require.Equal(t, []packets.Packet{
		{PacketID: 1, Created: 1},
		{PacketID: 2, Created: 2},
		{PacketID: 3, Created: 3, Expiry: -1},
		{PacketID: 4, Created: 4, Expiry: -1},
		{PacketID: 5, Created: 5},
	}, cl.State.Inflight.GetAll(false))

	require.Equal(t, []packets.Packet{
		{PacketID: 3, Created: 3, Expiry: -1},
		{PacketID: 4, Created: 4, Expiry: -1},
	}, cl.State.Inflight.GetAll(true))
}

func TestInflightLen(t *testing.T) {
	cl, _, _ := newTestClient()
	cl.State.Inflight.Set(packets.Packet{PacketID: 2})
	require.Equal(t, 1, cl.State.Inflight.Len())
}

func TestInflightClone(t *testing.T) {
	cl, _, _ := newTestClient()
	cl.State.Inflight.Set(packets.Packet{PacketID: 2})
	require.Equal(t, 1, cl.State.Inflight.Len())

	cloned := cl.State.Inflight.Clone()
	require.NotNil(t, cloned)
	require.NotSame(t, cloned, cl.State.Inflight)
}

func TestInflightDelete(t *testing.T) {
	cl, _, _ := newTestClient()

	cl.State.Inflight.Set(packets.Packet{PacketID: 3})
	require.NotNil(t, cl.State.Inflight.internal[3])

	r := cl.State.Inflight.Delete(3)
	require.True(t, r)
	require.Equal(t, uint16(0), cl.State.Inflight.internal[3].PacketID)

	_, ok := cl.State.Inflight.Get(3)
	require.False(t, ok)

	r = cl.State.Inflight.Delete(3)
	require.False(t, r)
}

func TestResetReceiveQuota(t *testing.T) {
	i := NewInflights()
	require.Equal(t, int32(0), atomic.LoadInt32(&i.maximumReceiveQuota))
	require.Equal(t, int32(0), atomic.LoadInt32(&i.receiveQuota))
	i.ResetReceiveQuota(6)
	require.Equal(t, int32(6), atomic.LoadInt32(&i.maximumReceiveQuota))
	require.Equal(t, int32(6), atomic.LoadInt32(&i.receiveQuota))
}

func TestReceiveQuota(t *testing.T) {
	i := NewInflights()
	i.receiveQuota = 4
	i.maximumReceiveQuota = 5
	require.Equal(t, int32(5), atomic.LoadInt32(&i.maximumReceiveQuota))
	require.Equal(t, int32(4), atomic.LoadInt32(&i.receiveQuota))

	// Return 1
	i.IncreaseReceiveQuota()
	require.Equal(t, int32(5), atomic.LoadInt32(&i.maximumReceiveQuota))
	require.Equal(t, int32(5), atomic.LoadInt32(&i.receiveQuota))

	// Try to go over max limit
	i.IncreaseReceiveQuota()
	require.Equal(t, int32(5), atomic.LoadInt32(&i.maximumReceiveQuota))
	require.Equal(t, int32(5), atomic.LoadInt32(&i.receiveQuota))

	// Reset to max 1
	i.ResetReceiveQuota(1)
	require.Equal(t, int32(1), atomic.LoadInt32(&i.maximumReceiveQuota))
	require.Equal(t, int32(1), atomic.LoadInt32(&i.receiveQuota))

	// Take 1
	i.DecreaseReceiveQuota()
	require.Equal(t, int32(1), atomic.LoadInt32(&i.maximumReceiveQuota))
	require.Equal(t, int32(0), atomic.LoadInt32(&i.receiveQuota))

	// Try to go below zero
	i.DecreaseReceiveQuota()
	require.Equal(t, int32(1), atomic.LoadInt32(&i.maximumReceiveQuota))
	require.Equal(t, int32(0), atomic.LoadInt32(&i.receiveQuota))
}

func TestResetSendQuota(t *testing.T) {
	i := NewInflights()
	require.Equal(t, int32(0), atomic.LoadInt32(&i.maximumSendQuota))
	require.Equal(t, int32(0), atomic.LoadInt32(&i.sendQuota))
	i.ResetSendQuota(6)
	require.Equal(t, int32(6), atomic.LoadInt32(&i.maximumSendQuota))
	require.Equal(t, int32(6), atomic.LoadInt32(&i.sendQuota))
}

func TestSendQuota(t *testing.T) {
	i := NewInflights()
	i.sendQuota = 4
	i.maximumSendQuota = 5
	require.Equal(t, int32(5), atomic.LoadInt32(&i.maximumSendQuota))
	require.Equal(t, int32(4), atomic.LoadInt32(&i.sendQuota))

	// Return 1
	i.IncreaseSendQuota()
	require.Equal(t, int32(5), atomic.LoadInt32(&i.maximumSendQuota))
	require.Equal(t, int32(5), atomic.LoadInt32(&i.sendQuota))

	// Try to go over max limit
	i.IncreaseSendQuota()
	require.Equal(t, int32(5), atomic.LoadInt32(&i.maximumSendQuota))
	require.Equal(t, int32(5), atomic.LoadInt32(&i.sendQuota))

	// Reset to max 1
	i.ResetSendQuota(1)
	require.Equal(t, int32(1), atomic.LoadInt32(&i.maximumSendQuota))
	require.Equal(t, int32(1), atomic.LoadInt32(&i.sendQuota))

	// Take 1
	i.DecreaseSendQuota()
	require.Equal(t, int32(1), atomic.LoadInt32(&i.maximumSendQuota))
	require.Equal(t, int32(0), atomic.LoadInt32(&i.sendQuota))

	// Try to go below zero
	i.DecreaseSendQuota()
	require.Equal(t, int32(1), atomic.LoadInt32(&i.maximumSendQuota))
	require.Equal(t, int32(0), atomic.LoadInt32(&i.sendQuota))
}

func TestNextImmediate(t *testing.T) {
	cl, _, _ := newTestClient()
	cl.State.Inflight.Set(packets.Packet{PacketID: 1, Created: 1})
	cl.State.Inflight.Set(packets.Packet{PacketID: 2, Created: 2})
	cl.State.Inflight.Set(packets.Packet{PacketID: 3, Created: 3, Expiry: -1})
	cl.State.Inflight.Set(packets.Packet{PacketID: 4, Created: 4, Expiry: -1})
	cl.State.Inflight.Set(packets.Packet{PacketID: 5, Created: 5})

	pk, ok := cl.State.Inflight.NextImmediate()
	require.True(t, ok)
	require.Equal(t, packets.Packet{PacketID: 3, Created: 3, Expiry: -1}, pk)

	r := cl.State.Inflight.Delete(3)
	require.True(t, r)

	pk, ok = cl.State.Inflight.NextImmediate()
	require.True(t, ok)
	require.Equal(t, packets.Packet{PacketID: 4, Created: 4, Expiry: -1}, pk)

	r = cl.State.Inflight.Delete(4)
	require.True(t, r)

	_, ok = cl.State.Inflight.NextImmediate()
	require.False(t, ok)
}

func TestInflightNextImmediateConcurrentWrites(t *testing.T) {
	i := NewInflights()
	stop := make(chan struct{})
	writer, reader := make(chan struct{}), make(chan struct{})

	go func() {
		defer close(writer)
		for n := 0; n < 20000; n++ {
			i.Set(packets.Packet{PacketID: uint16(n%100 + 1), Expiry: -1})
		}
	}()

	go func() {
		defer close(reader)
		for {
			select {
			case <-stop:
				return
			default:
				_, _ = i.NextImmediate()
			}
		}
	}()

	select {
	case <-writer:
	case <-time.After(time.Second * 20):
		t.Fatal("NextImmediate deadlocked with a concurrent writer")
	}

	close(stop)
	select {
	case <-reader:
	case <-time.After(time.Second * 5):
		t.Fatal("NextImmediate deadlocked with a concurrent writer")
	}
}

func TestInflightPendingOrder(t *testing.T) {
	i := NewInflights()
	i.Set(packets.Packet{PacketID: 1})
	for _, id := range []uint16{5, 3, 9} {
		require.True(t, i.SetPending(packets.Packet{PacketID: id, Expiry: 100}))
	}
	require.False(t, i.SetPending(packets.Packet{PacketID: 3, Expiry: 100})) // already pending
	require.Equal(t, 3, i.PendingLen())
	require.True(t, i.IsPending(5))
	require.False(t, i.IsPending(1))

	next, ok := i.NextImmediate() // peeks, without taking
	require.True(t, ok)
	require.Equal(t, uint16(5), next.PacketID)
	require.Equal(t, []packets.Packet{{PacketID: 5, Expiry: 100}, {PacketID: 3, Expiry: 100}, {PacketID: 9, Expiry: 100}}, i.GetAll(true))

	// Taking a packet marks it as sent, but keeps it inflight with its expiry.
	pk, ok := i.TakePending()
	require.True(t, ok)
	require.Equal(t, packets.Packet{PacketID: 5, Expiry: 100}, pk)
	require.False(t, i.IsPending(5))
	got, ok := i.Get(5)
	require.True(t, ok)
	require.Equal(t, int64(100), got.Expiry)

	// Deleted packets are no longer pending; requeued ones go back to the front.
	require.True(t, i.Delete(3))
	i.requeue(pk)
	pk, _ = i.TakePending()
	require.Equal(t, uint16(5), pk.PacketID)
	pk, _ = i.TakePending()
	require.Equal(t, uint16(9), pk.PacketID)
	_, ok = i.TakePending()
	require.False(t, ok)
	require.Equal(t, 3, i.Len()) // 1, 5 and 9 are still awaiting acknowledgement
}

func TestInflightPendingLegacyExpiry(t *testing.T) {
	i := NewInflights()
	i.Set(packets.Packet{PacketID: 1, Expiry: -1}) // the original pending marker
	require.True(t, i.IsPending(1))

	pk, ok := i.TakePending()
	require.True(t, ok)
	require.Zero(t, pk.Expiry)
	got, _ := i.Get(1)
	require.Zero(t, got.Expiry)
}

func TestInflightClearPendingAndClone(t *testing.T) {
	i := NewInflights()
	i.SetPending(packets.Packet{PacketID: 2})
	i.SetPending(packets.Packet{PacketID: 1, Expiry: -1})

	c := i.Clone()
	require.Equal(t, []packets.Packet{{PacketID: 2}, {PacketID: 1, Expiry: -1}}, c.GetAll(true))

	i.ClearPending()
	require.Zero(t, i.PendingLen())
	got, _ := i.Get(1)
	require.Zero(t, got.Expiry)
	require.Equal(t, 2, c.PendingLen()) // the clone is independent
}

func TestInflightPendingCompaction(t *testing.T) {
	i := NewInflights()
	for n := 0; n < 10000; n++ {
		id := uint16(n%500 + 1)
		i.SetPending(packets.Packet{PacketID: id})
		if n%3 == 0 {
			i.Delete(id)
		}
		if n%7 == 0 {
			i.TakePending()
		}
	}

	// The queue stays bounded, and only live pending packets come out, each once.
	require.LessOrEqual(t, len(i.pending), 64+i.PendingLen()*4)
	seen := map[uint16]bool{}
	for {
		pk, ok := i.TakePending()
		if !ok {
			break
		}
		require.False(t, seen[pk.PacketID])
		seen[pk.PacketID] = true
		_, ok = i.Get(pk.PacketID)
		require.True(t, ok)
	}
	require.Zero(t, i.PendingLen())
}

// BenchmarkInflightNextImmediate finds the next pending packet among many inflight, as the
// server does after each packet from a client which is behind on acknowledgements.
func BenchmarkInflightNextImmediate(b *testing.B) {
	i := NewInflights()
	for id := 1; id <= 8192; id++ {
		i.Set(packets.Packet{PacketID: uint16(id), Created: int64(id)})
	}
	i.Set(packets.Packet{PacketID: 9000, Expiry: -1})

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_, _ = i.NextImmediate()
	}
}
