// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

// loadtest measures throughput and latency through one broker or across a cluster.
// Publishers connect to the -pub brokers and subscribers to the -sub brokers (round
// robin), so publishing to one cluster node and subscribing on another measures the cost
// of forwarding between nodes. Every subscriber receives every message.
//
//	go run ./examples/benchmark/loadtest -pub 127.0.0.1:1883 -sub 127.0.0.1:1884
package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mochi-mqtt/server/v2/packets"
)

var username, password string // credentials every client connects with

func main() {
	pubAddrs := flag.String("pub", "127.0.0.1:1883", "comma-separated broker addresses for publishers")
	subAddrs := flag.String("sub", "", "comma-separated broker addresses for subscribers (defaults to -pub)")
	publishers := flag.Int("publishers", 4, "number of publishers")
	subscribers := flag.Int("subscribers", 2, "number of subscribers, each receiving every message")
	messages := flag.Int("messages", 25000, "messages per publisher")
	size := flag.Int("size", 64, "payload size in bytes (at least 8)")
	qos := flag.Int("qos", 0, "publish and subscribe qos (0 or 1)")
	window := flag.Int("window", 256, "unacknowledged qos 1 publishes per publisher")
	rate := flag.Int("rate", 0, "messages per second per publisher (0 for as fast as possible)")
	timeout := flag.Duration("timeout", time.Second*30, "time to wait for all messages to arrive")
	label := flag.String("label", "", "label for the result line")
	connections := flag.Int("connections", 0, "instead of messages, measure connecting this many clients (round robin over -pub)")
	concurrency := flag.Int("concurrency", 16, "clients connecting at once, with -connections")
	flag.StringVar(&username, "username", "", "username to connect with, if the broker requires one")
	flag.StringVar(&password, "password", "", "password to connect with")
	flag.Parse()

	if *connections > 0 {
		connectBench(*label, *pubAddrs, *connections, *concurrency)
		return
	}

	if *subAddrs == "" {
		*subAddrs = *pubAddrs
	}
	if *size < 8 {
		*size = 8
	}

	run := fmt.Sprint(time.Now().UnixNano())
	topic := "loadtest/" + run

	expected := int64(*publishers) * int64(*messages) * int64(*subscribers)
	var received, probes, lastReceived atomic.Int64
	allReceived := make(chan struct{})
	latencies := make([][]time.Duration, *subscribers)

	var subsReady sync.WaitGroup
	var subsDone sync.WaitGroup
	subs := make([]*client, *subscribers)
	for i := 0; i < *subscribers; i++ {
		addr := pick(*subAddrs, i)
		c, err := connect(addr, fmt.Sprintf("loadtest-sub-%s-%d", run, i))
		if err != nil {
			log.Fatalf("subscriber %d: %v", i, err)
		}
		if err := c.subscribe(topic+"/#", byte(*qos)); err != nil {
			log.Fatalf("subscriber %d: %v", i, err)
		}
		subs[i] = c

		subsReady.Add(1)
		subsDone.Add(1)
		go func(i int, c *client) {
			defer subsDone.Done()
			subsReady.Done()
			lat := make([]time.Duration, 0, *publishers**messages)
			want := *publishers * *messages
			for len(lat) < want {
				pk, err := c.next()
				if err != nil {
					break
				}
				if pk.FixedHeader.Type != packets.Publish || len(pk.Payload) < 8 {
					continue
				}
				if strings.HasSuffix(pk.TopicName, "/probe") {
					probes.Add(1)
					continue
				}
				sent := int64(binary.BigEndian.Uint64(pk.Payload))
				now := time.Now().UnixNano()
				lastReceived.Store(now)
				lat = append(lat, time.Duration(now-sent))
				if received.Add(1) == expected {
					close(allReceived)
				}
			}
			latencies[i] = lat
			c.close()
		}(i, c)
	}
	subsReady.Wait()

	// Subscribing on one cluster node and publishing on another needs the route to reach
	// the publishers' nodes first, so wait until a probe arrives.
	probe, err := connect(pick(*pubAddrs, 0), "loadtest-probe-"+run)
	if err != nil {
		log.Fatal(err)
	}
	waitForRoutes(probe, topic, *subscribers, &probes)
	probe.close()

	pubs := make([]*client, *publishers)
	for i := range pubs {
		c, err := connectWindow(pick(*pubAddrs, i), fmt.Sprintf("loadtest-pub-%s-%d", run, i), *window)
		if err != nil {
			log.Fatalf("publisher %d: %v", i, err)
		}
		pubs[i] = c
	}

	start := time.Now()
	var pubWG sync.WaitGroup
	for i, c := range pubs {
		pubWG.Add(1)
		go func(i int, c *client) {
			defer pubWG.Done()
			payload := make([]byte, *size)
			t := fmt.Sprintf("%s/%d", topic, i)
			var interval time.Duration
			if *rate > 0 {
				interval = time.Second / time.Duration(*rate)
			}
			next := time.Now()
			for n := 0; n < *messages; n++ {
				if interval > 0 {
					next = next.Add(interval)
					if d := time.Until(next); d > 0 {
						time.Sleep(d)
					}
				}
				binary.BigEndian.PutUint64(payload, uint64(time.Now().UnixNano()))
				if err := c.publish(t, payload, byte(*qos)); err != nil {
					log.Fatalf("publisher %d: %v", i, err)
				}
			}
			c.waitAcks()
		}(i, c)
	}
	pubWG.Wait()
	published := time.Since(start)

	select {
	case <-allReceived:
	case <-time.After(*timeout):
	}
	elapsed := time.Since(start)
	for _, c := range pubs {
		c.close()
	}
	for _, c := range subs {
		c.close() // ends any subscriber still waiting for messages
	}
	subsDone.Wait()

	var all []time.Duration
	for _, lat := range latencies {
		all = append(all, lat...)
	}
	sort.Slice(all, func(i, j int) bool { return all[i] < all[j] })

	total := int64(*publishers) * int64(*messages)
	got := received.Load()
	if last := lastReceived.Load(); last > 0 {
		elapsed = time.Duration(last - start.UnixNano()) // up to the last message received
	}
	fmt.Printf("%-28s qos=%d pubs=%d subs=%d size=%dB  publish %9.0f msg/s  receive %9.0f msg/s  latency p50=%-9s p95=%-9s p99=%-9s  delivered %d/%d (%.2f%%)\n",
		*label, *qos, *publishers, *subscribers, *size,
		float64(total)/published.Seconds(), float64(got)/elapsed.Seconds(),
		percentile(all, 0.50), percentile(all, 0.95), percentile(all, 0.99),
		got, expected, float64(got)*100/float64(expected))

	if got < expected && *qos > 0 { // qos 0 messages may be dropped when subscribers fall behind
		os.Exit(1)
	}
}

// connectBench measures how quickly clients can connect: each connection is claimed in a
// cluster's session registry.
func connectBench(label, addrs string, n, concurrency int) {
	run := fmt.Sprint(time.Now().UnixNano())
	lat := make([]time.Duration, n)
	jobs := make(chan int)
	var failed atomic.Int64
	var wg sync.WaitGroup

	start := time.Now()
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				t := time.Now()
				c, err := connect(pick(addrs, i), fmt.Sprintf("connect-%s-%d", run, i))
				if err != nil {
					failed.Add(1)
					continue
				}
				lat[i] = time.Since(t)
				c.close()
			}
		}()
	}
	for i := 0; i < n; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	elapsed := time.Since(start)

	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
	ok := lat[failed.Load():]
	fmt.Printf("%-28s connections=%d concurrency=%d  %8.0f connects/s  connack p50=%-9s p95=%-9s p99=%-9s  failed %d\n",
		label, n, concurrency, float64(n)/elapsed.Seconds(),
		percentile(ok, 0.50), percentile(ok, 0.95), percentile(ok, 0.99), failed.Load())
	if failed.Load() > 0 {
		os.Exit(1)
	}
}

func pick(addrs string, i int) string {
	list := strings.Split(addrs, ",")
	return list[i%len(list)]
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	return sorted[int(float64(len(sorted)-1)*p)].Round(time.Microsecond)
}

// waitForRoutes publishes probes until every subscriber receives one.
func waitForRoutes(c *client, topic string, subscribers int, probes *atomic.Int64) {
	deadline := time.Now().Add(time.Second * 30)
	for time.Now().Before(deadline) {
		probes.Store(0)
		if err := c.publish(topic+"/probe", make([]byte, 8), 0); err != nil {
			log.Fatal(err)
		}
		_ = c.flush()
		time.Sleep(time.Millisecond * 200)
		if probes.Load() >= int64(subscribers) {
			return
		}
	}
	log.Fatal("subscriptions did not reach the publishers' broker")
}

// client is a minimal mqtt v5 client.
type client struct {
	conn net.Conn
	r    *bufio.Reader
	w    *bufio.Writer
	wmu  sync.Mutex

	nextID   uint16
	inflight chan struct{} // qos 1 publishes awaiting acks
	packets  chan packets.Packet
	readErr  atomic.Value
}

func connect(addr, id string) (*client, error) {
	return connectWindow(addr, id, 256)
}

// connectWindow connects a client which may have window qos 1 publishes unacknowledged.
func connectWindow(addr, id string, window int) (*client, error) {
	conn, err := net.DialTimeout("tcp", addr, time.Second*5)
	if err != nil {
		return nil, err
	}

	c := &client{
		conn:     conn,
		r:        bufio.NewReaderSize(conn, 64*1024),
		w:        bufio.NewWriterSize(conn, 64*1024),
		inflight: make(chan struct{}, max(window, 1)),
		packets:  make(chan packets.Packet, 64*1024),
	}

	pk := packets.Packet{
		FixedHeader:     packets.FixedHeader{Type: packets.Connect},
		ProtocolVersion: 5,
		Connect:         packets.ConnectParams{ProtocolName: []byte("MQTT"), Clean: true, Keepalive: 60, ClientIdentifier: id},
		Properties:      packets.Properties{ReceiveMaximum: 65535},
	}
	if username != "" {
		pk.Connect.UsernameFlag, pk.Connect.Username = true, []byte(username)
	}
	if password != "" {
		pk.Connect.PasswordFlag, pk.Connect.Password = true, []byte(password)
	}
	if err := c.send(pk, true); err != nil {
		return nil, err
	}

	ack, err := c.read()
	if err != nil {
		return nil, err
	}
	if ack.FixedHeader.Type != packets.Connack || ack.ReasonCode != 0 {
		return nil, fmt.Errorf("connection refused: %#x", ack.ReasonCode)
	}

	go c.readLoop()
	go c.flushLoop()
	return c, nil
}

// flushLoop flushes buffered writes every millisecond, so batching writes doesn't add
// more than that to latency.
func (c *client) flushLoop() {
	t := time.NewTicker(time.Millisecond)
	defer t.Stop()
	for range t.C {
		c.wmu.Lock()
		var err error
		if c.w.Buffered() > 0 {
			err = c.w.Flush()
		}
		c.wmu.Unlock()
		if err != nil || c.readErr.Load() != nil {
			return
		}
	}
}

func (c *client) send(pk packets.Packet, flush bool) error {
	pk.ProtocolVersion = 5
	var buf bytes.Buffer
	var err error
	switch pk.FixedHeader.Type {
	case packets.Connect:
		err = pk.ConnectEncode(&buf)
	case packets.Publish:
		err = pk.PublishEncode(&buf)
	case packets.Puback:
		err = pk.PubackEncode(&buf)
	case packets.Subscribe:
		err = pk.SubscribeEncode(&buf)
	}
	if err != nil {
		return err
	}

	c.wmu.Lock()
	defer c.wmu.Unlock()
	if _, err := c.w.Write(buf.Bytes()); err != nil {
		return err
	}
	if flush || c.w.Buffered() > 32*1024 {
		return c.w.Flush()
	}
	return nil
}

func (c *client) flush() error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	return c.w.Flush()
}

func (c *client) read() (packets.Packet, error) {
	pk := packets.Packet{ProtocolVersion: 5}
	hb, err := c.r.ReadByte()
	if err != nil {
		return pk, err
	}
	if err := pk.FixedHeader.Decode(hb); err != nil {
		return pk, err
	}
	n, _, err := packets.DecodeLength(c.r)
	if err != nil {
		return pk, err
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(c.r, body); err != nil {
		return pk, err
	}

	switch pk.FixedHeader.Type {
	case packets.Connack:
		err = pk.ConnackDecode(body)
	case packets.Publish:
		err = pk.PublishDecode(body)
	case packets.Puback:
		err = pk.PubackDecode(body)
	case packets.Suback:
		err = pk.SubackDecode(body)
	}
	return pk, err
}

func (c *client) readLoop() {
	defer close(c.packets)
	for {
		pk, err := c.read()
		if err != nil {
			c.readErr.Store(err)
			return
		}

		switch pk.FixedHeader.Type {
		case packets.Puback:
			select {
			case <-c.inflight:
			default: // an ack for a publish the window isn't tracking
			}
		case packets.Publish:
			if pk.FixedHeader.Qos == 1 {
				// Acks are flushed once everything received so far has been read.
				_ = c.send(packets.Packet{FixedHeader: packets.FixedHeader{Type: packets.Puback}, PacketID: pk.PacketID}, c.r.Buffered() == 0)
			}
			c.packets <- pk
		default:
			c.packets <- pk
		}
	}
}

// next returns the next packet received.
func (c *client) next() (packets.Packet, error) {
	pk, ok := <-c.packets
	if !ok {
		return pk, fmt.Errorf("connection closed: %v", c.readErr.Load())
	}
	return pk, nil
}

func (c *client) subscribe(filter string, qos byte) error {
	if err := c.send(packets.Packet{
		FixedHeader: packets.FixedHeader{Type: packets.Subscribe, Qos: 1},
		PacketID:    1,
		Filters:     packets.Subscriptions{{Filter: filter, Qos: qos}},
	}, true); err != nil {
		return err
	}

	for { // the read loop delivers the suback
		pk, err := c.next()
		if err != nil {
			return err
		}
		if pk.FixedHeader.Type != packets.Suback {
			continue
		}
		if len(pk.ReasonCodes) != 1 || pk.ReasonCodes[0] >= 0x80 {
			return fmt.Errorf("subscribe refused")
		}
		return nil
	}
}

func (c *client) publish(topic string, payload []byte, qos byte) error {
	pk := packets.Packet{
		FixedHeader: packets.FixedHeader{Type: packets.Publish, Qos: qos},
		TopicName:   topic,
		Payload:     payload,
	}

	if qos > 0 {
		c.inflight <- struct{}{} // waits while the window is full
		c.nextID = c.nextID%65535 + 1
		pk.PacketID = c.nextID
		return c.send(pk, len(c.inflight) == cap(c.inflight))
	}

	return c.send(pk, false)
}

// waitAcks flushes, and waits for every qos 1 publish to be acknowledged.
func (c *client) waitAcks() {
	_ = c.flush()
	for len(c.inflight) > 0 {
		time.Sleep(time.Millisecond)
	}
}

func (c *client) close() {
	_ = c.flush()
	_ = c.conn.Close()
}
