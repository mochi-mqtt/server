// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package cluster

import (
	"fmt"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestClusterForwardBackpressure floods qos 1 publishes across nodes through a peer queue
// far too small to hold them. Every message must arrive, in order per publisher: the
// publishers wait for room in the queue rather than having their messages dropped.
func TestClusterForwardBackpressure(t *testing.T) {
	opts := testOptions("n1")
	opts.PeerQueueSize = 4
	n1 := newNodeWithOptions(t, opts)
	n2 := newNode(t, "n2", n1.cluster.Addr())
	require.Eventually(t, func() bool {
		return len(n1.cluster.Members()) == 2 && len(n2.cluster.Members()) == 2 &&
			n1.cluster.inContact() && n2.cluster.inContact()
	}, waitFor*4, tick, "cluster did not form")

	// The publishes are qos 1, which is what decides whether forwarding waits. The
	// subscriber takes them at qos 0, so it needn't acknowledge them: over a net.Pipe, a
	// client writing acks can deadlock with the server writing the next message. The
	// messages fit in the server's queue of writes for the client, so none are dropped there.
	sub := connect(t, n2, "sub", nil)
	sub.subscribe("bp/#", 0)
	waitRoutes(t, n1, "n2", "bp/#")

	const publishers, each = 4, 500
	var wg sync.WaitGroup
	errs := make(chan error, publishers)
	for p := 0; p < publishers; p++ {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			topic := fmt.Sprintf("bp/%d", p)
			for i := 0; i < each; i++ {
				if err := n1.server.Publish(topic, []byte(strconv.Itoa(i)), false, 1); err != nil {
					errs <- err
					return
				}
			}
		}(p)
	}

	next := map[string]int{} // the next message expected from each publisher
	for got := 0; got < publishers*each; got++ {
		pk, ok := sub.receive(waitFor)
		require.True(t, ok, "received %d of %d messages, dropped %d", got, publishers*each, n1.cluster.Stats().Dropped)
		require.Equal(t, strconv.Itoa(next[pk.TopicName]), string(pk.Payload), "order from %s", pk.TopicName)
		next[pk.TopicName]++
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	s := n1.cluster.Stats()
	require.Zero(t, s.Dropped)
	require.EqualValues(t, publishers*each, s.Forwarded)
	require.Positive(t, s.Waits, "the queue never filled, so the test didn't test anything")
}

// TestPeerSendWait checks when sendWait waits for room in a full queue, and for how long.
func TestPeerSendWait(t *testing.T) {
	c := &Cluster{opts: Options{PeerQueueSize: 1}}
	c.opts.ensureDefaults()
	frame := []byte("frame")

	newFullPeer := func(connected bool) *peer {
		p := newPeer(c, "p", "")
		require.True(t, p.send(frame))
		if connected {
			a, b := net.Pipe()
			t.Cleanup(func() { _ = a.Close(); _ = b.Close() })
			p.conn = a
		}
		return p
	}

	t.Run("room", func(t *testing.T) {
		p := newPeer(c, "p", "")
		queued, waited := p.sendWait(frame, time.Second)
		require.True(t, queued)
		require.False(t, waited)
	})

	t.Run("disconnected", func(t *testing.T) {
		start := time.Now()
		queued, waited := newFullPeer(false).sendWait(frame, time.Second)
		require.False(t, queued)
		require.True(t, waited)
		require.Less(t, time.Since(start), time.Millisecond*100, "a link which is down isn't draining")
	})

	t.Run("timeout", func(t *testing.T) {
		start := time.Now()
		queued, waited := newFullPeer(true).sendWait(frame, time.Millisecond*50)
		require.False(t, queued)
		require.True(t, waited)
		require.GreaterOrEqual(t, time.Since(start), time.Millisecond*50)
	})

	t.Run("drained", func(t *testing.T) {
		p := newFullPeer(true)
		go func() {
			time.Sleep(time.Millisecond * 20)
			<-p.queue
		}()
		queued, waited := p.sendWait(frame, time.Second)
		require.True(t, queued)
		require.True(t, waited)
	})

	t.Run("stopped", func(t *testing.T) {
		p := newFullPeer(true)
		go func() {
			time.Sleep(time.Millisecond * 20)
			p.stopOnce.Do(func() { close(p.done) })
		}()
		start := time.Now()
		queued, _ := p.sendWait(frame, time.Second)
		require.False(t, queued)
		require.Less(t, time.Since(start), time.Millisecond*500)
	})
}
