// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package mqtt_test

// End-to-end benchmarks for QoS 1 ingress through a real TCP
// listener. They count the broker's conn.Write calls (one per write(2) on a
// plain TCP conn) so the cost the ack path pays per message is visible:
//
//	go test -run '^$' -bench 'QoS1' -benchtime 200000x .
//
// writes/msg is the headline number for PUBACK coalescing; p50/p99 µs in the
// idle-latency benchmark must not regress (a lone publish is acked at once).

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"
)

// countingConn counts Write calls the broker makes on an accepted connection.
type countingConn struct {
	net.Conn
	writes *atomic.Int64
}

func (c *countingConn) Write(b []byte) (int, error) {
	c.writes.Add(1)
	return c.Conn.Write(b)
}

type countingListener struct {
	net.Listener
	writes *atomic.Int64
}

func (l *countingListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &countingConn{Conn: c, writes: l.writes}, nil
}

// busyHook spends a fixed amount of CPU in OnPublish, standing in for the
// per-message work an embedding application does on the reader goroutine.
type busyHook struct {
	mqtt.HookBase
	cost time.Duration
}

func (h *busyHook) ID() string { return "busy" }

func (h *busyHook) Provides(b byte) bool { return b == mqtt.OnPublish }

func (h *busyHook) OnPublish(_ *mqtt.Client, pk packets.Packet) (packets.Packet, error) {
	for start := time.Now(); time.Since(start) < h.cost; {
	}
	return pk, nil
}

// startCountingBroker starts a broker on an ephemeral loopback port that
// allows everyone, plus the given hooks, and counts its conn.Write calls.
func startCountingBroker(tb testing.TB, hooks ...mqtt.Hook) (string, *atomic.Int64) {
	tb.Helper()
	s := mqtt.New(&mqtt.Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	for _, h := range append([]mqtt.Hook{new(auth.AllowHook)}, hooks...) {
		if err := s.AddHook(h, nil); err != nil {
			tb.Fatal(err)
		}
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		tb.Fatal(err)
	}
	writes := new(atomic.Int64)
	if err := s.AddListener(listeners.NewNet("bench", &countingListener{Listener: ln, writes: writes})); err != nil {
		tb.Fatal(err)
	}
	if err := s.Serve(); err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { _ = s.Close() })
	return ln.Addr().String(), writes
}

type rawClient struct {
	conn net.Conn
	r    *bufio.Reader
}

func dialV5(tb testing.TB, addr, id string) *rawClient {
	tb.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { _ = conn.Close() })

	pk := packets.Packet{
		FixedHeader:     packets.FixedHeader{Type: packets.Connect},
		ProtocolVersion: 5,
		Connect: packets.ConnectParams{
			ProtocolName:     []byte("MQTT"),
			ClientIdentifier: id,
			Keepalive:        60,
			Clean:            true,
		},
	}
	var buf bytes.Buffer
	if err := pk.ConnectEncode(&buf); err != nil {
		tb.Fatal(err)
	}
	if _, err := conn.Write(buf.Bytes()); err != nil {
		tb.Fatal(err)
	}

	c := &rawClient{conn: conn, r: bufio.NewReader(conn)}
	typ, body, err := c.next()
	if err != nil || typ != packets.Connack || len(body) < 2 || body[1] != 0 {
		tb.Fatalf("connack: type=%d body=%v err=%v", typ, body, err)
	}
	return c
}

// next reads one packet, returning its type and body.
func (c *rawClient) next() (byte, []byte, error) {
	h, err := c.r.ReadByte()
	if err != nil {
		return 0, nil, err
	}
	n, _, err := packets.DecodeLength(c.r)
	if err != nil {
		return 0, nil, err
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(c.r, body); err != nil {
		return 0, nil, err
	}
	return h >> 4, body, nil
}

func encodePublishQoS1(tb testing.TB, topic string, id uint16, payload []byte) []byte {
	tb.Helper()
	pk := packets.Packet{
		FixedHeader:     packets.FixedHeader{Type: packets.Publish, Qos: 1},
		ProtocolVersion: 5,
		TopicName:       topic,
		PacketID:        id,
		Payload:         payload,
	}
	var buf bytes.Buffer
	if err := pk.PublishEncode(&buf); err != nil {
		tb.Fatal(err)
	}
	return buf.Bytes()
}

// BenchmarkQoS1PublishTCP: N clients each keep a window of QoS 1 publishes in
// flight (one conn.Write per publish, like a real client library) and wait
// for every PUBACK. One op is one acknowledged publish.
func BenchmarkQoS1PublishTCP(b *testing.B) {
	for _, hookCost := range []time.Duration{0, 20 * time.Microsecond} {
		for _, clients := range []int{1, 50, 500} {
			b.Run(fmt.Sprintf("hook=%v/clients=%d", hookCost, clients), func(b *testing.B) {
				benchQoS1(b, clients, 32, hookCost)
			})
		}
	}
}

func benchQoS1(b *testing.B, clients, window int, hookCost time.Duration) {
	var hooks []mqtt.Hook
	if hookCost > 0 {
		hooks = append(hooks, &busyHook{cost: hookCost})
	}
	addr, writes := startCountingBroker(b, hooks...)
	payload := bytes.Repeat([]byte("x"), 64)

	cls := make([]*rawClient, clients)
	frames := make([][][]byte, clients)
	for i := range cls {
		cls[i] = dialV5(b, addr, fmt.Sprintf("bench-%d", i))
		// Packet ids cycle through the window; a slot is reused only after its ack.
		frames[i] = make([][]byte, window)
		for w := 0; w < window; w++ {
			frames[i][w] = encodePublishQoS1(b, fmt.Sprintf("bench/%d", i), uint16(w+1), payload)
		}
	}

	per := make([]int, clients)
	for i := 0; i < b.N; i++ {
		per[i%clients]++
	}

	b.ReportAllocs()
	before := writes.Load()
	b.ResetTimer()
	start := time.Now()

	var wg sync.WaitGroup
	errs := make(chan error, 2*clients)
	lats := make([][]time.Duration, clients)
	for i, c := range cls {
		if per[i] == 0 {
			continue
		}
		slots := make(chan struct{}, window)
		// Acks arrive in publish order, so ack k answers publish k; its send
		// time sits in slot k%window until the ack frees the slot.
		sent := make([]atomic.Int64, window)
		lats[i] = make([]time.Duration, 0, per[i])
		wg.Add(2)
		go func(c *rawClient, n int, fr [][]byte) { // sender
			defer wg.Done()
			for k := 0; k < n; k++ {
				slots <- struct{}{}
				sent[k%window].Store(time.Now().UnixNano())
				if _, err := c.conn.Write(fr[k%len(fr)]); err != nil {
					errs <- err
					return
				}
			}
		}(c, per[i], frames[i])
		go func(c *rawClient, n int, lat *[]time.Duration) { // receiver
			defer wg.Done()
			for k := 0; k < n; k++ {
				typ, _, err := c.next()
				if err != nil {
					errs <- err
					return
				}
				if typ != packets.Puback {
					errs <- fmt.Errorf("expected PUBACK, got type %d", typ)
					return
				}
				*lat = append(*lat, time.Duration(time.Now().UnixNano()-sent[k%window].Load()))
				<-slots
			}
		}(c, per[i], &lats[i])
	}
	wg.Wait()
	elapsed := time.Since(start)
	b.StopTimer()

	select {
	case err := <-errs:
		b.Fatal(err)
	default:
	}

	b.ReportMetric(float64(writes.Load()-before)/float64(b.N), "writes/msg")
	b.ReportMetric(float64(b.N)/elapsed.Seconds(), "msg/s")
	var all []time.Duration
	for _, l := range lats {
		all = append(all, l...)
	}
	b.ReportMetric(quantileMicros(all, 0.99), "ack-p99-µs")
}

func quantileMicros(lat []time.Duration, p float64) float64 {
	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
	return float64(lat[min(len(lat)-1, int(p*float64(len(lat))))].Nanoseconds()) / 1e3
}

// BenchmarkQoS1PubackLatencyIdle: one client, one publish at a time — nothing
// is ever buffered behind it, so its PUBACK must be written immediately. One
// op is one publish → PUBACK round trip.
func BenchmarkQoS1PubackLatencyIdle(b *testing.B) {
	addr, writes := startCountingBroker(b)
	c := dialV5(b, addr, "lat")
	frame := encodePublishQoS1(b, "bench/lat", 1, bytes.Repeat([]byte("x"), 64))

	lat := make([]time.Duration, 0, b.N)
	b.ReportAllocs()
	before := writes.Load()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		t0 := time.Now()
		if _, err := c.conn.Write(frame); err != nil {
			b.Fatal(err)
		}
		typ, _, err := c.next()
		if err != nil || typ != packets.Puback {
			b.Fatalf("type=%d err=%v", typ, err)
		}
		lat = append(lat, time.Since(t0))
	}
	b.StopTimer()

	b.ReportMetric(quantileMicros(lat, 0.50), "p50-µs")
	b.ReportMetric(quantileMicros(lat, 0.99), "p99-µs")
	b.ReportMetric(float64(writes.Load()-before)/float64(b.N), "writes/msg")
}
