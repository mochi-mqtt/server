// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-co
// SPDX-FileContributor: mochi-co

package storage

import (
	"encoding/json"
	"errors"

	"github.com/mochi-co/mqtt/v2/packets"
	"github.com/mochi-co/mqtt/v2/system"
)

const (
	SubscriptionKey = "SUB" // unique key to denote Subscriptions in a store
	SysInfoKey      = "SYS" // unique key to denote server system information in a store
	RetainedKey     = "RET" // unique key to denote retained messages in a store
	InflightKey     = "IFM" // unique key to denote inflight messages in a store
	ClientKey       = "CL"  // unique key to denote clients in a store
)

var (
	// ErrDBFileNotOpen indicates that the file database (e.g. bolt/badger) wasn't open for reading.
	ErrDBFileNotOpen = errors.New("db file not open")
)

// Client is a storable representation of an mqtt client.
type Client struct {
	Will            ClientWill       `json:"will"`            // will topic and payload data if applicable
	Properties      ClientProperties `json:"properties"`      // the connect properties for the client
	Username        []byte           `json:"username"`        // the username of the client
	ID              string           `json:"id" storm:"id"`   // the client id / storage key
	T               string           `json:"t"`               // the data type (client)
	Remote          string           `json:"remote"`          // the remote address of the client
	Listener        string           `json:"listener"`        // the listener the client connected on
	ProtocolVersion byte             `json:"protocolVersion"` // mqtt protocol version of the client
	Clean           bool             `json:"clean"`           // if the client requested a clean start/session
}

// ClientProperties contains a limited set of the mqtt v5 properties specific to a client connection.
type ClientProperties struct {
	AuthenticationData        []byte                 `json:"authenticationData"`
	User                      []packets.UserProperty `json:"user"`
	AuthenticationMethod      string                 `json:"authenticationMethod"`
	SessionExpiryInterval     uint32                 `json:"sessionExpiryInterval"`
	MaximumPacketSize         uint32                 `json:"maximumPacketSize"`
	ReceiveMaximum            uint16                 `json:"receiveMaximum"`
	TopicAliasMaximum         uint16                 `json:"topicAliasMaximum"`
	SessionExpiryIntervalFlag bool                   `json:"sessionExpiryIntervalFlag"`
	RequestProblemInfo        byte                   `json:"requestProblemInfo"`
	RequestProblemInfoFlag    bool                   `json:"requestProblemInfoFlag"`
	RequestResponseInfo       byte                   `json:"requestResponseInfo"`
}

// ClientWill contains a will message for a client, and limited mqtt v5 properties.
type ClientWill struct {
	Payload           []byte                 `json:"payload"`
	User              []packets.UserProperty `json:"user"`
	TopicName         string                 `json:"topicName"`
	Flag              uint32                 `json:"flag"`
	WillDelayInterval uint32                 `json:"willDelayInterval"`
	Qos               byte                   `json:"qos"`
	Retain            bool                   `json:"retain"`
}

// MarshalBinary encodes the values into a json string.
func (d Client) MarshalBinary() (data []byte, err error) {
	return json.Marshal(d)
}

// UnmarshalBinary decodes a json string into a struct.
func (d *Client) UnmarshalBinary(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, d)
}

// Message is a storable representation of an MQTT message (specifically publish).
type Message struct {
	Properties  MessageProperties   `json:"properties"`    // -
	Payload     []byte              `json:"payload"`       // the message payload (if retained)
	T           string              `json:"t"`             // the data type
	ID          string              `json:"id" storm:"id"` // the storage key
	Origin      string              `json:"origin"`        // the id of the client who sent the message
	TopicName   string              `json:"topic_name"`    // the topic the message was sent to (if retained)
	FixedHeader packets.FixedHeader `json:"fixedheader"`   // the header properties of the message
	Created     int64               `json:"created"`       // the time the message was created in unixtime
	Sent        int64               `json:"sent"`          // the last time the message was sent (for retries) in unixtime (if inflight)
	PacketID    uint16              `json:"packet_id"`     // the unique id of the packet (if inflight)
}

// MessageProperties contains a limited subset of mqtt v5 properties specific to publish messages.
type MessageProperties struct {
	CorrelationData        []byte                 `json:"correlationData"`
	SubscriptionIdentifier []int                  `json:"subscriptionIdentifier"`
	User                   []packets.UserProperty `json:"user"`
	ContentType            string                 `json:"contentType"`
	ResponseTopic          string                 `json:"responseTopic"`
	MessageExpiryInterval  uint32                 `json:"messageExpiry"`
	TopicAlias             uint16                 `json:"topicAlias"`
	PayloadFormat          byte                   `json:"payloadFormat"`
	PayloadFormatFlag      bool                   `json:"payloadFormatFlag"`
}

// MarshalBinary encodes the values into a json string.
func (d Message) MarshalBinary() (data []byte, err error) {
	return json.Marshal(d)
}

// UnmarshalBinary decodes a json string into a struct.
func (d *Message) UnmarshalBinary(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, d)
}

// ToPacket converts a storage.Message to a standard packet.
func (d *Message) ToPacket() packets.Packet {
	pk := packets.Packet{
		FixedHeader: d.FixedHeader,
		PacketID:    d.PacketID,
		TopicName:   d.TopicName,
		Payload:     d.Payload,
		Origin:      d.Origin,
		Created:     d.Created,
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
	}

	// Return a deep copy of the packet data otherwise the slices will
	// continue pointing at the values from the storage packet.
	pk = pk.Copy(true)
	pk.FixedHeader.Dup = d.FixedHeader.Dup

	return pk
}

// Subscription is a storable representation of an mqtt subscription.
type Subscription struct {
	T                 string `json:"t"`
	ID                string `json:"id" storm:"id"`
	Client            string `json:"client"`
	Filter            string `json:"filter"`
	Identifier        int    `json:"identifier"`
	RetainHandling    byte   `json:"retain_handling"`
	Qos               byte   `json:"qos"`
	RetainAsPublished bool   `json:"retain_as_pub"`
	NoLocal           bool   `json:"no_local"`
}

// MarshalBinary encodes the values into a json string.
func (d Subscription) MarshalBinary() (data []byte, err error) {
	return json.Marshal(d)
}

// UnmarshalBinary decodes a json string into a struct.
func (d *Subscription) UnmarshalBinary(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, d)
}

// SystemInfo is a storable representation of the system information values.
type SystemInfo struct {
	system.Info        // embed the system info struct
	T           string `json:"t"`             // the data type
	ID          string `json:"id" storm:"id"` // the storage key
}

// MarshalBinary encodes the values into a json string.
func (d SystemInfo) MarshalBinary() (data []byte, err error) {
	return json.Marshal(d)
}

// UnmarshalBinary decodes a json string into a struct.
func (d *SystemInfo) UnmarshalBinary(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, d)
}
