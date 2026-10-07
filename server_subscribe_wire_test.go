// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
// SPDX-FileContributor: Barış Velioğlu (MaestroHub)

package mqtt

// Three subscription-wire gaps found while testing an MQTT 5 client that
// routes retained replays by subscription identifier against this broker.
// Each test names the spec clause it pins.

import (
	"bytes"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/mochi-mqtt/server/v2/packets"
)

// A client's SUBSCRIBE whose packet identifier equals one the SERVER chose
// for a publish still in flight to that client is not "in use": the two
// identifier spaces are independent (2.2.1). Before the fix the SUBSCRIBE was
// refused with 0x91 — with both sides counting from 1, most of the time right
// after a retained replay at QoS 1.
func TestServerProcessPacketSubscribeOutboundInflightIDIsNotInUse(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5
	// The server's own QoS 1 publish to this client, id 15, unacked.
	cl.State.Inflight.Set(packets.Packet{PacketID: 15, FixedHeader: packets.FixedHeader{Type: packets.Publish, Qos: 1}})

	pkx := *packets.TPacketData[packets.Subscribe].Get(packets.TSubscribeMqtt5).Packet
	pkx.PacketID = 15
	go func() {
		err := s.processPacket(cl, pkx)
		require.NoError(t, err)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Suback].Get(packets.TSubackMqtt5).RawBytes, buf, "granted, not 0x91")
	require.Equal(t, int64(1), atomic.LoadInt64(&s.Info.Subscriptions))
}

func TestServerProcessPacketUnsubscribeOutboundInflightIDIsNotInUse(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5
	cl.State.Inflight.Set(packets.Packet{PacketID: 15, FixedHeader: packets.FixedHeader{Type: packets.Publish, Qos: 1}})
	s.Topics.Subscribe(cl.ID, packets.Subscription{Filter: "a/b", Qos: 0}) // the filter the v5 fixture unsubscribes

	go func() {
		err := s.processPacket(cl, *packets.TPacketData[packets.Unsubscribe].Get(packets.TUnsubscribeMqtt5).Packet)
		require.NoError(t, err)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, packets.TPacketData[packets.Unsuback].Get(packets.TUnsubackMqtt5).RawBytes, buf, "unsubscribed, not 0x91")
	require.Equal(t, int64(-1), atomic.LoadInt64(&s.Info.Subscriptions))
}

// A retained message re-sent for a SUBSCRIBE carries that subscription's
// identifier [MQTT-3.3.4-3], as a live delivery does. Before the fix the
// replay carried none, so a client routing by identifier could not tell
// which subscription the replay was for.
func TestServerProcessSubscribeRetainedReplayCarriesSubscriptionIdentifier(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5

	retained := s.Topics.RetainMessage(packets.Packet{
		ProtocolVersion: 5,
		FixedHeader:     packets.FixedHeader{Type: packets.Publish, Retain: true},
		TopicName:       "a/b/c",
		Payload:         []byte("hello mochi"),
	})
	require.Equal(t, int64(1), retained)

	// The decoded SUBSCRIBE entry: QoS 0 (so the replay needs no packet id),
	// retain handling 0, identifier 322122 as the v5 fixture carries.
	pkx := *packets.TPacketData[packets.Subscribe].Get(packets.TSubscribeMqtt5).Packet
	pkx.Filters = packets.Subscriptions{{Filter: "a/b/c", Qos: 0, Identifier: 322122}}
	go func() {
		err := s.processPacket(cl, pkx)
		require.NoError(t, err)
		time.Sleep(time.Millisecond)
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)

	suback := append([]byte(nil), packets.TPacketData[packets.Suback].Get(packets.TSubackMqtt5).RawBytes...)
	suback[len(suback)-1] = packets.CodeGrantedQos0.Code

	var replay bytes.Buffer
	expected := packets.Packet{
		ProtocolVersion: 5,
		FixedHeader:     packets.FixedHeader{Type: packets.Publish, Retain: true},
		TopicName:       "a/b/c",
		Payload:         []byte("hello mochi"),
		Properties:      packets.Properties{SubscriptionIdentifier: []int{322122}},
	}
	require.NoError(t, expected.PublishEncode(&replay))

	require.Equal(t, append(suback, replay.Bytes()...), buf,
		"SUBACK, then the retained message with the subscription identifier")
}

// A v5 client that subscribes with no identifier still gets the replay, as
// before (no empty property is written).
func TestServerProcessSubscribeRetainedReplayWithoutIdentifierIsUnchanged(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5

	s.Topics.RetainMessage(packets.Packet{
		ProtocolVersion: 5,
		FixedHeader:     packets.FixedHeader{Type: packets.Publish, Retain: true},
		TopicName:       "a/b/c",
		Payload:         []byte("hello mochi"),
	})
	pkx := *packets.TPacketData[packets.Subscribe].Get(packets.TSubscribeMqtt5).Packet
	pkx.Filters = packets.Subscriptions{{Filter: "a/b/c", Qos: 0}}
	go func() {
		require.NoError(t, s.processPacket(cl, pkx))
		time.Sleep(time.Millisecond)
		_ = w.Close()
	}()
	buf, err := io.ReadAll(r)
	require.NoError(t, err)

	var replay bytes.Buffer
	expected := packets.Packet{
		ProtocolVersion: 5,
		FixedHeader:     packets.FixedHeader{Type: packets.Publish, Retain: true},
		TopicName:       "a/b/c",
		Payload:         []byte("hello mochi"),
	}
	require.NoError(t, expected.PublishEncode(&replay))
	require.True(t, bytes.HasSuffix(buf, replay.Bytes()), "the replay carries no identifier property")
}

// A server without subscription identifiers or shared subscriptions says so
// in the CONNACK (3.2.2.3.12, 3.2.2.3.13); absent means available. Before
// the fix the Capabilities were never written, so a client assumed both.
func TestServerSendConnackAnnouncesMissingCapabilities(t *testing.T) {
	s := newServer()
	s.Options.Capabilities.SubIDAvailable = 0
	s.Options.Capabilities.SharedSubAvailable = 0
	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5
	go func() {
		require.NoError(t, s.SendConnack(cl, packets.CodeSuccess, false, nil))
		_ = w.Close()
	}()
	buf, err := io.ReadAll(r)
	require.NoError(t, err)

	pk := packets.Packet{ProtocolVersion: 5, FixedHeader: packets.FixedHeader{Type: packets.Connack}}
	require.NoError(t, pk.ConnackDecode(buf[2:]))
	require.True(t, pk.Properties.SubIDAvailableFlag, "the property is present")
	require.Equal(t, byte(0), pk.Properties.SubIDAvailable)
	require.True(t, pk.Properties.SharedSubAvailableFlag)
	require.Equal(t, byte(0), pk.Properties.SharedSubAvailable)
}

// A default server (both available) writes neither property: the CONNACK
// of every existing fixture is unchanged.
func TestServerSendConnackDefaultCapabilitiesWriteNothing(t *testing.T) {
	s := newServer()
	cl, r, w := newTestClient()
	cl.Properties.ProtocolVersion = 5
	go func() {
		require.NoError(t, s.SendConnack(cl, packets.CodeSuccess, false, nil))
		_ = w.Close()
	}()
	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	pk := packets.Packet{ProtocolVersion: 5, FixedHeader: packets.FixedHeader{Type: packets.Connack}}
	require.NoError(t, pk.ConnackDecode(buf[2:]))
	require.False(t, pk.Properties.SubIDAvailableFlag)
	require.False(t, pk.Properties.SharedSubAvailableFlag)
}

// With the capability off, a SUBSCRIBE that uses it is refused with the
// reason the spec names, and nothing is subscribed.
func TestServerProcessSubscribeRefusesUnavailableCapabilities(t *testing.T) {
	cases := []struct {
		name   string
		setup  func(s *Server)
		filter packets.Subscription
		code   byte
	}{
		{"subscription identifiers", func(s *Server) { s.Options.Capabilities.SubIDAvailable = 0 },
			packets.Subscription{Filter: "a/b/c", Qos: 2, Identifier: 322122}, packets.ErrSubscriptionIdentifiersNotSupported.Code},
		{"shared subscriptions", func(s *Server) { s.Options.Capabilities.SharedSubAvailable = 0 },
			packets.Subscription{Filter: "$share/g/a/b/c", Qos: 2}, packets.ErrSharedSubscriptionsNotSupported.Code},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newServer()
			tc.setup(s)
			cl, r, w := newTestClient()
			cl.Properties.ProtocolVersion = 5
			pkx := *packets.TPacketData[packets.Subscribe].Get(packets.TSubscribeMqtt5).Packet
			pkx.Filters = packets.Subscriptions{tc.filter}
			go func() {
				require.NoError(t, s.processPacket(cl, pkx))
				_ = w.Close()
			}()
			buf, err := io.ReadAll(r)
			require.NoError(t, err)
			expected := append([]byte(nil), packets.TPacketData[packets.Suback].Get(packets.TSubackMqtt5).RawBytes...)
			expected[len(expected)-1] = tc.code
			require.Equal(t, expected, buf)
			require.Equal(t, int64(0), atomic.LoadInt64(&s.Info.Subscriptions), "nothing subscribed")
		})
	}
}
