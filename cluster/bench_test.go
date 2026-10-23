// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package cluster

import (
	"fmt"
	"testing"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

func benchPacket() packets.Packet {
	return packets.Packet{
		FixedHeader: packets.FixedHeader{Type: packets.Publish, Qos: 1},
		TopicName:   "sensors/2/500/temp",
		Payload:     make([]byte, 256),
		PacketID:    7,
		Origin:      "publisher",
		Created:     1700000000,
		Properties: packets.Properties{
			ContentType: "application/json",
			User:        []packets.UserProperty{{Key: "site", Val: "a"}},
		},
	}
}

func BenchmarkEncodePublish(b *testing.B) {
	pk := benchPacket()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		raw, _ := encodePacket(pk)
		_ = publishFrame(pk, raw, nil)
	}
}

func BenchmarkDecodePublish(b *testing.B) {
	pk := benchPacket()
	raw, _ := encodePacket(pk)
	body := encodePublish(pk, raw, nil)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _, _ = decodePublish(body)
	}
}

// benchRoutes returns remote routes for nodes n1 to n3, each with 1000 filters, one in
// ten wild.
func benchRoutes() *remoteRoutes {
	r := newRemoteRoutes()
	for n := 1; n < 4; n++ {
		node := fmt.Sprintf("n%d", n)
		gen := r.attach(node)
		for i := 0; i < 1000; i++ {
			f := fmt.Sprintf("sensors/%d/%d/temp", n, i)
			if i%10 == 0 {
				f = fmt.Sprintf("sensors/%d/%d/+", n, i)
			}
			r.add(node, gen, f)
		}
		r.add(node, gen, "sensors/#")
	}
	return r
}

func BenchmarkRouteMatch(b *testing.B) {
	r := benchRoutes()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.match("sensors/2/500/temp")
	}
}

// BenchmarkForward measures routing and queueing a publish for three peers.
func BenchmarkForward(b *testing.B) {
	s := mqtt.New(&mqtt.Options{Logger: testLogger})
	c, _ := New(s, Options{NodeName: "n0", PeerQueueSize: 1 << 16})
	c.remote = benchRoutes()

	done := make(chan struct{})
	defer close(done)
	for n := 1; n < 4; n++ {
		p := newPeer(c, fmt.Sprintf("n%d", n), "")
		c.peers[p.name] = p
		go func() { // drain the queue, as the link would
			for {
				select {
				case <-p.queue:
				case <-done:
					return
				}
			}
		}()
	}

	pk := benchPacket()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.forward(pk)
	}
	b.StopTimer()
	if c.metrics.publishesDropped.Load() > 0 {
		b.Fatalf("dropped %d publishes", c.metrics.publishesDropped.Load())
	}
}

func BenchmarkLocalRoutes(b *testing.B) {
	l := localRoutes{}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprint(i % 100)
		l.add("a/b/c", key)
		l.remove("a/b/c", key)
	}
}

func BenchmarkRetainedApply(b *testing.B) {
	s := mqtt.New(&mqtt.Options{Logger: testLogger})
	c, _ := New(s, Options{NodeName: "n0"})
	pk := benchPacket()
	pk.FixedHeader.Retain = true
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pk.TopicName = fmt.Sprintf("retained/%d", i%1000)
		c.applyRetained(retainVersion{Time: int64(i + 1), Node: "n1"}, false, pk)
	}
}
