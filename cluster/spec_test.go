// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package cluster

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/mochi-mqtt/server/v2/packets"
)

// Spec behaviour which must hold when clients are on different nodes. These were found
// by running the paho interoperability tests with each client on a different node.

// A delayed will is sent to subscribers on other nodes once its delay has passed
// [MQTT-3.1.3-9].
func TestClusterDelayedWillForwarded(t *testing.T) {
	tests := []struct {
		name          string
		delay, expiry uint32
	}{
		{"delay within session", 1, 60},
		{"session ends at disconnect", 5, 0}, // the will is sent when the session ends
		{"session ends before delay", 5, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes := newCluster(t, "n1", "n2")

			local := connect(t, nodes[0], "local-watcher", nil)
			local.subscribe("will/#", 0)
			remote := connect(t, nodes[1], "remote-watcher", nil)
			remote.subscribe("will/#", 0)
			waitRoutes(t, nodes[0], "n2", "will/#")

			o := connectOptions{
				persistent: tt.expiry > 0,
				expiry:     tt.expiry,
				will: &packets.ConnectParams{
					WillTopic:      "will/delayed",
					WillPayload:    []byte("gone"),
					WillProperties: packets.Properties{WillDelayInterval: tt.delay},
				},
			}
			dying, _ := connectWith(t, nodes[0], "delayed", o)
			start := time.Now()
			_ = dying.conn.Close()

			for _, c := range []*client{local, remote} {
				pk, ok := c.receive(time.Second * 8)
				require.True(t, ok, "%s: will not received after %s", c.id(), time.Since(start))
				require.Equal(t, "will/delayed", pk.TopicName)
				require.Equal(t, []byte("gone"), pk.Payload)
				t.Logf("%s received the will after %s", c.id(), time.Since(start).Round(time.Millisecond))
			}
		})
	}
}

// id returns the client's identifier, for messages.
func (c *client) id() string {
	return c.conn.LocalAddr().String()
}

// A message which waited for a client whose session moved to another node is sent with
// its remaining expiry interval, and not at all once it has expired [MQTT-3.3.2-5]
// [MQTT-3.3.2-6].
func TestSessionMoveKeepsMessageExpiry(t *testing.T) {
	nodes := newCluster(t, "n1", "n2", "n3")
	o := connectOptions{persistent: true, expiry: 60}

	c1, _ := connectWith(t, nodes[0], "mover", o)
	c1.subscribe("e/#", 1)
	waitRoutes(t, nodes[2], "n1", "e/#")
	_ = c1.conn.Close()
	require.Eventually(t, func() bool { return !liveSession(nodes[0], "mover") }, waitFor, tick)

	pub := connect(t, nodes[2], "pub", nil)
	pub.publishWith("e/short", []byte("expires"), 1, packets.Properties{MessageExpiryInterval: 1})
	pub.publishWith("e/long", []byte("keeps"), 1, packets.Properties{MessageExpiryInterval: 30})
	require.Eventually(t, func() bool {
		cl, ok := nodes[0].server.Clients.Get("mover")
		return ok && cl.State.Inflight.Len() == 2
	}, waitFor, tick)

	time.Sleep(time.Millisecond * 2100) // the short message expires, the long one ages

	c2, ack := connectWith(t, nodes[1], "mover", o)
	require.True(t, ack.SessionPresent)

	pk, ok := c2.receive(waitFor)
	require.True(t, ok)
	require.Equal(t, "e/long", pk.TopicName, "the expired message must not be sent")
	require.Positive(t, pk.Properties.MessageExpiryInterval)
	require.LessOrEqual(t, pk.Properties.MessageExpiryInterval, uint32(28), "expiry interval must count the time waited")

	_, ok = c2.receive(time.Millisecond * 300)
	require.False(t, ok, "only one message should be sent")
}

// publishWith publishes with properties, waiting for the PUBACK of a qos 1 publish.
func (c *client) publishWith(topic string, payload []byte, qos byte, props packets.Properties) {
	pk := packets.Packet{
		FixedHeader:     packets.FixedHeader{Type: packets.Publish, Qos: qos},
		ProtocolVersion: 5,
		TopicName:       topic,
		Payload:         payload,
		Properties:      props,
	}
	if qos > 0 {
		pk.PacketID = c.nextID()
	}
	var buf bytes.Buffer
	require.NoError(c.t, pk.PublishEncode(&buf))
	c.write(buf.Bytes())
	if qos == 1 {
		c.expect(packets.Puback)
	}
}

// With noLocal, a publisher doesn't receive its own message, while a subscriber on another
// node still does [MQTT-3.8.3-3].
func TestClusterNoLocalAcrossNodes(t *testing.T) {
	nodes := newCluster(t, "n1", "n2")

	a := connect(t, nodes[0], "a", nil)
	a.subscribeWith(packets.Subscription{Filter: "nl/#", Qos: 1, NoLocal: true})
	b := connect(t, nodes[1], "b", nil)
	b.subscribeWith(packets.Subscription{Filter: "nl/#", Qos: 1, NoLocal: true})
	waitRoutes(t, nodes[0], "n2", "nl/#")

	a.publish("nl/x", []byte("from a"), 1)

	pk, ok := b.receive(waitFor)
	require.True(t, ok, "the subscriber on the other node must receive it")
	require.Equal(t, []byte("from a"), pk.Payload)

	for {
		pk, ok := a.read(time.Millisecond * 300)
		if !ok {
			break
		}
		require.NotEqual(t, packets.Publish, pk.FixedHeader.Type, "noLocal: a received its own message")
	}
}

// subscribeWith subscribes with subscription options.
func (c *client) subscribeWith(sub packets.Subscription) {
	pk := packets.Packet{
		FixedHeader:     packets.FixedHeader{Type: packets.Subscribe, Qos: 1},
		ProtocolVersion: 5,
		PacketID:        c.nextID(),
		Filters:         packets.Subscriptions{sub},
	}
	var buf bytes.Buffer
	require.NoError(c.t, pk.SubscribeEncode(&buf))
	c.write(buf.Bytes())
	ack := c.expect(packets.Suback)
	require.Equal(c.t, []byte{sub.Qos}, ack.ReasonCodes)
}
