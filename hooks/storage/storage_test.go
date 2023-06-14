// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-co
// SPDX-FileContributor: mochi-co

package storage

import (
	"testing"
	"time"

	"github.com/mochi-co/mqtt/v2/packets"
	"github.com/mochi-co/mqtt/v2/system"
	"github.com/stretchr/testify/require"
)

var (
	clientStruct = Client{
		ID:       "test",
		T:        "client",
		Remote:   "remote",
		Listener: "listener",
		Username: []byte("mochi"),
		Clean:    true,
		Properties: ClientProperties{
			SessionExpiryInterval:     2,
			SessionExpiryIntervalFlag: true,
			AuthenticationMethod:      "a",
			AuthenticationData:        []byte("test"),
			RequestProblemInfo:        1,
			RequestProblemInfoFlag:    true,
			RequestResponseInfo:       1,
			ReceiveMaximum:            128,
			TopicAliasMaximum:         256,
			User: []packets.UserProperty{
				{Key: "k", Val: "v"},
			},
			MaximumPacketSize: 120,
		},
		Will: ClientWill{
			Qos:               1,
			Payload:           []byte("abc"),
			TopicName:         "a/b/c",
			Flag:              1,
			Retain:            true,
			WillDelayInterval: 2,
			User: []packets.UserProperty{
				{Key: "k2", Val: "v2"},
			},
		},
	}
	clientJSON = []byte(`{"will":{"payload":"YWJj","user":[{"k":"k2","v":"v2"}],"topicName":"a/b/c","flag":1,"willDelayInterval":2,"qos":1,"retain":true},"properties":{"authenticationData":"dGVzdA==","user":[{"k":"k","v":"v"}],"authenticationMethod":"a","sessionExpiryInterval":2,"maximumPacketSize":120,"receiveMaximum":128,"topicAliasMaximum":256,"sessionExpiryIntervalFlag":true,"requestProblemInfo":1,"requestProblemInfoFlag":true,"requestResponseInfo":1},"username":"bW9jaGk=","id":"test","t":"client","remote":"remote","listener":"listener","protocolVersion":0,"clean":true}`)

	messageStruct = Message{
		T:       "message",
		Payload: []byte("payload"),
		FixedHeader: packets.FixedHeader{
			Remaining: 2,
			Type:      3,
			Qos:       1,
			Dup:       true,
			Retain:    true,
		},
		ID:        "id",
		Origin:    "mochi",
		TopicName: "topic",
		Properties: MessageProperties{
			PayloadFormat:          1,
			PayloadFormatFlag:      true,
			MessageExpiryInterval:  20,
			ContentType:            "type",
			ResponseTopic:          "a/b/r",
			CorrelationData:        []byte("r"),
			SubscriptionIdentifier: []int{1},
			TopicAlias:             2,
			User: []packets.UserProperty{
				{Key: "k2", Val: "v2"},
			},
		},
		Created:  time.Date(2019, time.September, 21, 1, 2, 3, 4, time.UTC).Unix(),
		Sent:     time.Date(2019, time.September, 21, 1, 2, 3, 4, time.UTC).Unix(),
		PacketID: 100,
	}
	messageJSON = []byte(`{"properties":{"correlationData":"cg==","subscriptionIdentifier":[1],"user":[{"k":"k2","v":"v2"}],"contentType":"type","responseTopic":"a/b/r","messageExpiry":20,"topicAlias":2,"payloadFormat":1,"payloadFormatFlag":true},"payload":"cGF5bG9hZA==","t":"message","id":"id","origin":"mochi","topic_name":"topic","fixedheader":{"remaining":2,"type":3,"qos":1,"dup":true,"retain":true},"created":1569027723,"sent":1569027723,"packet_id":100}`)

	subscriptionStruct = Subscription{
		T:      "subscription",
		ID:     "id",
		Client: "mochi",
		Filter: "a/b/c",
		Qos:    1,
	}
	subscriptionJSON = []byte(`{"t":"subscription","id":"id","client":"mochi","filter":"a/b/c","identifier":0,"retain_handling":0,"qos":1,"retain_as_pub":false,"no_local":false}`)

	sysInfoStruct = SystemInfo{
		T:  "info",
		ID: "id",
		Info: system.Info{
			Version:          "2.0.0",
			Started:          1,
			Uptime:           2,
			BytesReceived:    3,
			BytesSent:        4,
			ClientsConnected: 5,
			ClientsMaximum:   7,
			MessagesReceived: 10,
			MessagesSent:     11,
			MessagesDropped:  20,
			PacketsReceived:  12,
			PacketsSent:      13,
			Retained:         15,
			Inflight:         16,
			InflightDropped:  17,
		},
	}
	sysInfoJSON = []byte(`{"version":"2.0.0","started":1,"time":0,"uptime":2,"bytes_received":3,"bytes_sent":4,"clients_connected":5,"clients_disconnected":0,"clients_maximum":7,"clients_total":0,"messages_received":10,"messages_sent":11,"messages_dropped":20,"retained":15,"inflight":16,"inflight_dropped":17,"subscriptions":0,"packets_received":12,"packets_sent":13,"memory_alloc":0,"threads":0,"t":"info","id":"id"}`)
)

func TestClientMarshalBinary(t *testing.T) {
	data, err := clientStruct.MarshalBinary()
	require.NoError(t, err)
	require.Equal(t, clientJSON, data)
}

func TestClientUnmarshalBinary(t *testing.T) {
	d := clientStruct
	err := d.UnmarshalBinary(clientJSON)
	require.NoError(t, err)
	require.Equal(t, clientStruct, d)
}

func TestClientUnmarshalBinaryEmpty(t *testing.T) {
	d := Client{}
	err := d.UnmarshalBinary([]byte{})
	require.NoError(t, err)
	require.Equal(t, Client{}, d)
}

func TestMessageMarshalBinary(t *testing.T) {
	data, err := messageStruct.MarshalBinary()
	require.NoError(t, err)
	require.Equal(t, messageJSON, data)
}

func TestMessageUnmarshalBinary(t *testing.T) {
	d := messageStruct
	err := d.UnmarshalBinary(messageJSON)
	require.NoError(t, err)
	require.Equal(t, messageStruct, d)
}

func TestMessageUnmarshalBinaryEmpty(t *testing.T) {
	d := Message{}
	err := d.UnmarshalBinary([]byte{})
	require.NoError(t, err)
	require.Equal(t, Message{}, d)
}

func TestSubscriptionMarshalBinary(t *testing.T) {
	data, err := subscriptionStruct.MarshalBinary()
	require.NoError(t, err)
	require.Equal(t, subscriptionJSON, data)
}

func TestSubscriptionUnmarshalBinary(t *testing.T) {
	d := subscriptionStruct
	err := d.UnmarshalBinary(subscriptionJSON)
	require.NoError(t, err)
	require.Equal(t, subscriptionStruct, d)
}

func TestSubscriptionUnmarshalBinaryEmpty(t *testing.T) {
	d := Subscription{}
	err := d.UnmarshalBinary([]byte{})
	require.NoError(t, err)
	require.Equal(t, Subscription{}, d)
}

func TestSysInfoMarshalBinary(t *testing.T) {
	data, err := sysInfoStruct.MarshalBinary()
	require.NoError(t, err)
	require.Equal(t, sysInfoJSON, data)
}

func TestSysInfoUnmarshalBinary(t *testing.T) {
	d := sysInfoStruct
	err := d.UnmarshalBinary(sysInfoJSON)
	require.NoError(t, err)
	require.Equal(t, sysInfoStruct, d)
}

func TestSysInfoUnmarshalBinaryEmpty(t *testing.T) {
	d := SystemInfo{}
	err := d.UnmarshalBinary([]byte{})
	require.NoError(t, err)
	require.Equal(t, SystemInfo{}, d)
}

func TestMessageToPacket(t *testing.T) {
	d := messageStruct
	pk := d.ToPacket()

	require.Equal(t, packets.Packet{
		Payload: []byte("payload"),
		FixedHeader: packets.FixedHeader{
			Remaining: d.FixedHeader.Remaining,
			Type:      d.FixedHeader.Type,
			Qos:       d.FixedHeader.Qos,
			Dup:       d.FixedHeader.Dup,
			Retain:    d.FixedHeader.Retain,
		},
		Origin:    d.Origin,
		TopicName: d.TopicName,
		Properties: packets.Properties{
			PayloadFormat:          d.Properties.PayloadFormat,
			PayloadFormatFlag:      d.Properties.PayloadFormatFlag,
			MessageExpiryInterval:  d.Properties.MessageExpiryInterval,
			ContentType:            d.Properties.ContentType,
			ResponseTopic:          d.Properties.ResponseTopic,
			CorrelationData:        d.Properties.CorrelationData,
			SubscriptionIdentifier: d.Properties.SubscriptionIdentifier,
			TopicAlias:             d.Properties.TopicAlias,
			User:                   d.Properties.User,
		},
		PacketID: 100,
		Created:  d.Created,
	}, pk)

}
