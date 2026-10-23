// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

//go:build chaos

package chaos

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mochi-mqtt/server/v2/packets"
)

var errClosed = errors.New("connection closed")

// conn is a minimal mqtt v5 client connection, which records when it was established
// and when the server closed it.
type conn struct {
	id   string
	node string
	c    net.Conn
	r    *bufio.Reader

	connacked time.Time    // when the connack was received
	present   bool         // the session was present
	lastRx    atomic.Int64 // unix nanoseconds when a message was last received

	wmu      sync.Mutex
	nextID   atomic.Uint32
	acks     sync.Map // packet id -> chan packets.Packet
	messages chan packets.Packet

	closeOnce sync.Once
	done      chan struct{}
	closedAt  time.Time
	reason    byte // disconnect reason code, if the server sent one
}

// dial connects a client to a node. A refused connection returns its connack reason code.
func dial(n node, id string, persistent bool, timeout time.Duration) (*conn, byte, error) {
	nc, err := net.DialTimeout("tcp", n.mqtt, timeout)
	if err != nil {
		return nil, 0, err
	}

	pk := packets.Packet{
		FixedHeader:     packets.FixedHeader{Type: packets.Connect},
		ProtocolVersion: 5,
		Connect: packets.ConnectParams{
			ProtocolName:     []byte("MQTT"),
			Clean:            !persistent,
			Keepalive:        30,
			ClientIdentifier: id,
		},
	}
	if persistent {
		pk.Properties.SessionExpiryInterval = 300
		pk.Properties.SessionExpiryIntervalFlag = true
	}

	var buf bytes.Buffer
	if err := pk.ConnectEncode(&buf); err != nil {
		return nil, 0, err
	}

	_ = nc.SetDeadline(time.Now().Add(timeout))
	if _, err := nc.Write(buf.Bytes()); err != nil {
		_ = nc.Close()
		return nil, 0, err
	}

	c := &conn{
		id:       id,
		node:     n.name,
		c:        nc,
		r:        bufio.NewReader(nc),
		messages: make(chan packets.Packet, 4096),
		done:     make(chan struct{}),
	}

	ack, err := c.readPacket()
	if err != nil {
		_ = nc.Close()
		return nil, 0, err
	}
	if ack.FixedHeader.Type != packets.Connack {
		_ = nc.Close()
		return nil, 0, fmt.Errorf("expected connack, got %d", ack.FixedHeader.Type)
	}
	if ack.ReasonCode != 0 {
		_ = nc.Close()
		return nil, ack.ReasonCode, nil
	}

	c.connacked = time.Now()
	c.present = ack.SessionPresent
	_ = nc.SetDeadline(time.Time{})
	go c.readLoop()
	return c, 0, nil
}

func (c *conn) readPacket() (packets.Packet, error) {
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
	pk.FixedHeader.Remaining = n

	switch pk.FixedHeader.Type {
	case packets.Connack:
		err = pk.ConnackDecode(body)
	case packets.Publish:
		err = pk.PublishDecode(body)
	case packets.Puback:
		err = pk.PubackDecode(body)
	case packets.Suback:
		err = pk.SubackDecode(body)
	case packets.Disconnect:
		err = pk.DisconnectDecode(body)
	}
	return pk, err
}

func (c *conn) readLoop() {
	defer c.markClosed(0)
	for {
		pk, err := c.readPacket()
		if err != nil {
			return
		}

		switch pk.FixedHeader.Type {
		case packets.Disconnect:
			c.markClosed(pk.ReasonCode)
			return
		case packets.Publish:
			c.lastRx.Store(time.Now().UnixNano())
			if pk.FixedHeader.Qos == 1 {
				_ = c.write(packets.Packet{FixedHeader: packets.FixedHeader{Type: packets.Puback}, PacketID: pk.PacketID})
			}
			select {
			case c.messages <- pk:
			default: // the test isn't reading this client's messages
			}
		case packets.Puback, packets.Suback:
			if ch, ok := c.acks.LoadAndDelete(pk.PacketID); ok {
				ch.(chan packets.Packet) <- pk
			}
		}
	}
}

// markClosed records when the connection closed.
func (c *conn) markClosed(reason byte) {
	c.closeOnce.Do(func() {
		c.closedAt = time.Now()
		c.reason = reason
		close(c.done)
		_ = c.c.Close()
	})
}

// close closes the connection from the client side.
func (c *conn) close() {
	c.markClosed(0)
}

// closed returns true if the connection has closed, and when.
func (c *conn) closedTime() (time.Time, bool) {
	select {
	case <-c.done:
		return c.closedAt, true
	default:
		return time.Time{}, false
	}
}

func (c *conn) write(pk packets.Packet) error {
	pk.ProtocolVersion = 5
	var buf bytes.Buffer
	var err error
	switch pk.FixedHeader.Type {
	case packets.Publish:
		err = pk.PublishEncode(&buf)
	case packets.Puback:
		err = pk.PubackEncode(&buf)
	case packets.Subscribe:
		err = pk.SubscribeEncode(&buf)
	default:
		return fmt.Errorf("can't write packet type %d", pk.FixedHeader.Type)
	}
	if err != nil {
		return err
	}

	c.wmu.Lock()
	defer c.wmu.Unlock()
	_ = c.c.SetWriteDeadline(time.Now().Add(time.Second * 5))
	_, err = c.c.Write(buf.Bytes())
	return err
}

// request writes a packet and waits for its ack.
func (c *conn) request(pk packets.Packet, timeout time.Duration) (packets.Packet, error) {
	id := uint16(c.nextID.Add(1)%65535 + 1)
	pk.PacketID = id
	ch := make(chan packets.Packet, 1)
	c.acks.Store(id, ch)
	defer c.acks.Delete(id)

	if err := c.write(pk); err != nil {
		return pk, err
	}

	select {
	case ack := <-ch:
		return ack, nil
	case <-c.done:
		return pk, errClosed
	case <-time.After(timeout):
		return pk, fmt.Errorf("timed out waiting for ack")
	}
}

// publish publishes a message, waiting for the puback at qos 1.
func (c *conn) publish(topic string, payload []byte, qos byte, retain bool) error {
	pk := packets.Packet{
		FixedHeader: packets.FixedHeader{Type: packets.Publish, Qos: qos, Retain: retain},
		TopicName:   topic,
		Payload:     payload,
	}
	if qos == 0 {
		return c.write(pk)
	}

	ack, err := c.request(pk, time.Second*5)
	if err != nil {
		return err
	}
	if ack.ReasonCode >= 0x80 {
		return fmt.Errorf("publish refused: %#x", ack.ReasonCode)
	}
	return nil
}

// subscribe subscribes to a filter.
func (c *conn) subscribe(filter string, qos byte) error {
	ack, err := c.request(packets.Packet{
		FixedHeader: packets.FixedHeader{Type: packets.Subscribe, Qos: 1},
		Filters:     packets.Subscriptions{{Filter: filter, Qos: qos}},
	}, time.Second*5)
	if err != nil {
		return err
	}
	if len(ack.ReasonCodes) != 1 || ack.ReasonCodes[0] >= 0x80 {
		return fmt.Errorf("subscribe refused: %v", ack.ReasonCodes)
	}
	return nil
}
