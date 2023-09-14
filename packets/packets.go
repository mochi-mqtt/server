// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package packets

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
)

// All valid packet types and their packet identifiers.
const (
	Reserved       byte = iota // 0 - we use this in packet tests to indicate special-test or all packets.
	Connect                    // 1
	Connack                    // 2
	Publish                    // 3
	Puback                     // 4
	Pubrec                     // 5
	Pubrel                     // 6
	Pubcomp                    // 7
	Subscribe                  // 8
	Suback                     // 9
	Unsubscribe                // 10
	Unsuback                   // 11
	Pingreq                    // 12
	Pingresp                   // 13
	Disconnect                 // 14
	Auth                       // 15
	WillProperties byte = 99   // Special byte for validating Will Properties.
)

var (
	// ErrNoValidPacketAvailable indicates the packet type byte provided does not exist in the mqtt specification.
	ErrNoValidPacketAvailable = errors.New("no valid packet available")

	// PacketNames is a map of packet bytes to human-readable names, for easier debugging.
	PacketNames = map[byte]string{
		0:  "Reserved",
		1:  "Connect",
		2:  "Connack",
		3:  "Publish",
		4:  "Puback",
		5:  "Pubrec",
		6:  "Pubrel",
		7:  "Pubcomp",
		8:  "Subscribe",
		9:  "Suback",
		10: "Unsubscribe",
		11: "Unsuback",
		12: "Pingreq",
		13: "Pingresp",
		14: "Disconnect",
		15: "Auth",
	}
)

// Packets is a concurrency safe map of packets.
type Packets struct {
	internal map[string]Packet
	sync.RWMutex
}

// NewPackets returns a new instance of Packets.
func NewPackets() *Packets {
	return &Packets{
		internal: map[string]Packet{},
	}
}

// Add adds a new packet to the map.
func (p *Packets) Add(id string, val Packet) {
	p.Lock()
	defer p.Unlock()
	p.internal[id] = val
}

// GetAll returns all packets in the map.
func (p *Packets) GetAll() map[string]Packet {
	p.RLock()
	defer p.RUnlock()
	m := map[string]Packet{}
	for k, v := range p.internal {
		m[k] = v
	}
	return m
}

// Get returns a specific packet in the map by packet id.
func (p *Packets) Get(id string) (val Packet, ok bool) {
	p.RLock()
	defer p.RUnlock()
	val, ok = p.internal[id]
	return val, ok
}

// Len returns the number of packets in the map.
func (p *Packets) Len() int {
	p.RLock()
	defer p.RUnlock()
	val := len(p.internal)
	return val
}

// Delete removes a packet from the map by packet id.
func (p *Packets) Delete(id string) {
	p.Lock()
	defer p.Unlock()
	delete(p.internal, id)
}

// Packet represents an MQTT packet. Instead of providing a packet interface
// variant packet structs, this is a single concrete packet type to cover all packet
// types, which allows us to take advantage of various compiler optimizations. It
// contains a combination of mqtt spec values and internal broker control codes.
type Packet struct {
	Connect         ConnectParams // parameters for connect packets (just for organisation)
	Properties      Properties    // all mqtt v5 packet properties
	Payload         []byte        // a message/payload for publish packets
	ReasonCodes     []byte        // one or more reason codes for multi-reason responses (suback, etc)
	Filters         Subscriptions // a list of subscription filters and their properties (subscribe, unsubscribe)
	TopicName       string        // the topic a payload is being published to
	Origin          string        // client id of the client who is issuing the packet (mostly internal use)
	FixedHeader     FixedHeader   // -
	Created         int64         // unix timestamp indicating time packet was created/received on the server
	Expiry          int64         // unix timestamp indicating when the packet will expire and should be deleted
	Mods            Mods          // internal broker control values for controlling certain mqtt v5 compliance
	PacketID        uint16        // packet id for the packet (publish, qos, etc)
	ProtocolVersion byte          // protocol version of the client the packet belongs to
	SessionPresent  bool          // session existed for connack
	ReasonCode      byte          // reason code for a packet response (acks, etc)
	ReservedBit     byte          // reserved, do not use (except in testing)
	Ignore          bool          // if true, do not perform any message forwarding operations
}

// Mods specifies certain values required for certain mqtt v5 compliance within packet encoding/decoding.
type Mods struct {
	MaxSize             uint32 // the maximum packet size specified by the client / server
	DisallowProblemInfo bool   // if problem info is disallowed
	AllowResponseInfo   bool   // if response info is disallowed
}

// ConnectParams contains packet values which are specifically related to connect packets.
type ConnectParams struct {
	WillProperties   Properties `json:"willProperties"` // -
	Password         []byte     `json:"password"`       // -
	Username         []byte     `json:"username"`       // -
	ProtocolName     []byte     `json:"protocolName"`   // -
	WillPayload      []byte     `json:"willPayload"`    // -
	ClientIdentifier string     `json:"clientId"`       // -
	WillTopic        string     `json:"willTopic"`      // -
	Keepalive        uint16     `json:"keepalive"`      // -
	PasswordFlag     bool       `json:"passwordFlag"`   // -
	UsernameFlag     bool       `json:"usernameFlag"`   // -
	WillQos          byte       `json:"willQos"`        // -
	WillFlag         bool       `json:"willFlag"`       // -
	WillRetain       bool       `json:"willRetain"`     // -
	Clean            bool       `json:"clean"`          // CleanSession in v3.1.1, CleanStart in v5
}

// Subscriptions is a slice of Subscription.
type Subscriptions []Subscription // must be a slice to retain order.

// Subscription contains details about a client subscription to a topic filter.
type Subscription struct {
	ShareName         []string
	Filter            string
	Identifier        int
	Identifiers       map[string]int
	RetainHandling    byte
	Qos               byte
	RetainAsPublished bool
	NoLocal           bool
	FwdRetainedFlag   bool // true if the subscription forms part of a publish response to a client subscription and packet is retained.
}

// Copy creates a new instance of a packet, but with an empty header for inheriting new QoS flags, etc.
func (pk *Packet) Copy(allowTransfer bool) Packet {
	p := Packet{
		FixedHeader: FixedHeader{
			Remaining: pk.FixedHeader.Remaining,
			Type:      pk.FixedHeader.Type,
			Retain:    pk.FixedHeader.Retain,
			Dup:       false, // [MQTT-4.3.1-1] [MQTT-4.3.2-2]
			Qos:       pk.FixedHeader.Qos,
		},
		Mods: Mods{
			MaxSize: pk.Mods.MaxSize,
		},
		ReservedBit:     pk.ReservedBit,
		ProtocolVersion: pk.ProtocolVersion,
		Connect: ConnectParams{
			ClientIdentifier: pk.Connect.ClientIdentifier,
			Keepalive:        pk.Connect.Keepalive,
			WillQos:          pk.Connect.WillQos,
			WillTopic:        pk.Connect.WillTopic,
			WillFlag:         pk.Connect.WillFlag,
			WillRetain:       pk.Connect.WillRetain,
			WillProperties:   pk.Connect.WillProperties.Copy(allowTransfer),
			Clean:            pk.Connect.Clean,
		},
		TopicName:      pk.TopicName,
		Properties:     pk.Properties.Copy(allowTransfer),
		SessionPresent: pk.SessionPresent,
		ReasonCode:     pk.ReasonCode,
		Filters:        pk.Filters,
		Created:        pk.Created,
		Expiry:         pk.Expiry,
		Origin:         pk.Origin,
	}

	if allowTransfer {
		p.PacketID = pk.PacketID
	}

	if len(pk.Connect.ProtocolName) > 0 {
		p.Connect.ProtocolName = append([]byte{}, pk.Connect.ProtocolName...)
	}

	if len(pk.Connect.Password) > 0 {
		p.Connect.PasswordFlag = true
		p.Connect.Password = append([]byte{}, pk.Connect.Password...)
	}

	if len(pk.Connect.Username) > 0 {
		p.Connect.UsernameFlag = true
		p.Connect.Username = append([]byte{}, pk.Connect.Username...)
	}

	if len(pk.Connect.WillPayload) > 0 {
		p.Connect.WillPayload = append([]byte{}, pk.Connect.WillPayload...)
	}

	if len(pk.Payload) > 0 {
		p.Payload = append([]byte{}, pk.Payload...)
	}

	if len(pk.ReasonCodes) > 0 {
		p.ReasonCodes = append([]byte{}, pk.ReasonCodes...)
	}

	return p
}

// Merge merges a new subscription with a base subscription, preserving the highest
// qos value, matched identifiers and any special properties.
func (s Subscription) Merge(n Subscription) Subscription {
	if s.Identifiers == nil {
		s.Identifiers = map[string]int{
			s.Filter: s.Identifier,
		}
	}

	if n.Identifier > 0 {
		s.Identifiers[n.Filter] = n.Identifier
	}

	if n.Qos > s.Qos {
		s.Qos = n.Qos // [MQTT-3.3.4-2]
	}

	if n.NoLocal {
		s.NoLocal = true // [MQTT-3.8.3-3]
	}

	return s
}

// encode encodes a subscription and properties into bytes.
func (s Subscription) encode() byte {
	var flag byte
	flag |= s.Qos

	if s.NoLocal {
		flag |= 1 << 2
	}

	if s.RetainAsPublished {
		flag |= 1 << 3
	}

	flag |= s.RetainHandling << 4
	return flag
}

// decode decodes subscription bytes into a subscription struct.
func (s *Subscription) decode(b byte) {
	s.Qos = b & 3                      // byte
	s.NoLocal = 1&(b>>2) > 0           // bool
	s.RetainAsPublished = 1&(b>>3) > 0 // bool
	s.RetainHandling = 3 & (b >> 4)    // byte
}

// ConnectEncode encodes a connect packet.
func (pk *Packet) ConnectEncode(buf *bytes.Buffer) error {
	nb := bytes.NewBuffer([]byte{})
	nb.Write(encodeBytes(pk.Connect.ProtocolName))
	nb.WriteByte(pk.ProtocolVersion)

	nb.WriteByte(
		encodeBool(pk.Connect.Clean)<<1 |
			encodeBool(pk.Connect.WillFlag)<<2 |
			pk.Connect.WillQos<<3 |
			encodeBool(pk.Connect.WillRetain)<<5 |
			encodeBool(pk.Connect.PasswordFlag)<<6 |
			encodeBool(pk.Connect.UsernameFlag)<<7 |
			0, // [MQTT-2.1.3-1]
	)

	nb.Write(encodeUint16(pk.Connect.Keepalive))

	if pk.ProtocolVersion == 5 {
		pb := bytes.NewBuffer([]byte{})
		(&pk.Properties).Encode(pk.FixedHeader.Type, pk.Mods, pb, 0)
		nb.Write(pb.Bytes())
	}

	nb.Write(encodeString(pk.Connect.ClientIdentifier))

	if pk.Connect.WillFlag {
		if pk.ProtocolVersion == 5 {
			pb := bytes.NewBuffer([]byte{})
			(&pk.Connect).WillProperties.Encode(WillProperties, pk.Mods, pb, 0)
			nb.Write(pb.Bytes())
		}

		nb.Write(encodeString(pk.Connect.WillTopic))
		nb.Write(encodeBytes(pk.Connect.WillPayload))
	}

	if pk.Connect.UsernameFlag {
		nb.Write(encodeBytes(pk.Connect.Username))
	}

	if pk.Connect.PasswordFlag {
		nb.Write(encodeBytes(pk.Connect.Password))
	}

	pk.FixedHeader.Remaining = nb.Len()
	pk.FixedHeader.Encode(buf)
	_, _ = nb.WriteTo(buf)

	return nil
}

// ConnectDecode decodes a connect packet.
func (pk *Packet) ConnectDecode(buf []byte) error {
	var offset int
	var err error

	pk.Connect.ProtocolName, offset, err = decodeBytes(buf, 0)
	if err != nil {
		return ErrMalformedProtocolName
	}

	pk.ProtocolVersion, offset, err = decodeByte(buf, offset)
	if err != nil {
		return ErrMalformedProtocolVersion
	}

	flags, offset, err := decodeByte(buf, offset)
	if err != nil {
		return ErrMalformedFlags
	}

	pk.ReservedBit = 1 & flags
	pk.Connect.Clean = 1&(flags>>1) > 0
	pk.Connect.WillFlag = 1&(flags>>2) > 0
	pk.Connect.WillQos = 3 & (flags >> 3) // this one is not a bool
	pk.Connect.WillRetain = 1&(flags>>5) > 0
	pk.Connect.PasswordFlag = 1&(flags>>6) > 0
	pk.Connect.UsernameFlag = 1&(flags>>7) > 0

	pk.Connect.Keepalive, offset, err = decodeUint16(buf, offset)
	if err != nil {
		return ErrMalformedKeepalive
	}

	if pk.ProtocolVersion == 5 {
		n, err := pk.Properties.Decode(pk.FixedHeader.Type, bytes.NewBuffer(buf[offset:]))
		if err != nil {
			return fmt.Errorf("%s: %w", err, ErrMalformedProperties)
		}
		offset += n
	}

	pk.Connect.ClientIdentifier, offset, err = decodeString(buf, offset) // [MQTT-3.1.3-1] [MQTT-3.1.3-2] [MQTT-3.1.3-3] [MQTT-3.1.3-4]
	if err != nil {
		return ErrClientIdentifierNotValid // [MQTT-3.1.3-8]
	}

	if pk.Connect.WillFlag { // [MQTT-3.1.2-7]
		if pk.ProtocolVersion == 5 {
			n, err := pk.Connect.WillProperties.Decode(WillProperties, bytes.NewBuffer(buf[offset:]))
			if err != nil {
				return ErrMalformedWillProperties
			}
			offset += n
		}

		pk.Connect.WillTopic, offset, err = decodeString(buf, offset)
		if err != nil {
			return ErrMalformedWillTopic
		}

		pk.Connect.WillPayload, offset, err = decodeBytes(buf, offset)
		if err != nil {
			return ErrMalformedWillPayload
		}
	}

	if pk.Connect.UsernameFlag { // [MQTT-3.1.3-12]
		if offset >= len(buf) { // we are at the end of the packet
			return ErrProtocolViolationFlagNoUsername // [MQTT-3.1.2-17]
		}

		pk.Connect.Username, offset, err = decodeBytes(buf, offset)
		if err != nil {
			return ErrMalformedUsername
		}
	}

	if pk.Connect.PasswordFlag {
		pk.Connect.Password, _, err = decodeBytes(buf, offset)
		if err != nil {
			return ErrMalformedPassword
		}
	}

	return nil
}

// ConnectValidate ensures the connect packet is compliant.
func (pk *Packet) ConnectValidate() Code {
	if !bytes.Equal(pk.Connect.ProtocolName, []byte{'M', 'Q', 'I', 's', 'd', 'p'}) && !bytes.Equal(pk.Connect.ProtocolName, []byte{'M', 'Q', 'T', 'T'}) {
		return ErrProtocolViolationProtocolName // [MQTT-3.1.2-1]
	}

	if (bytes.Equal(pk.Connect.ProtocolName, []byte{'M', 'Q', 'I', 's', 'd', 'p'}) && pk.ProtocolVersion != 3) ||
		(bytes.Equal(pk.Connect.ProtocolName, []byte{'M', 'Q', 'T', 'T'}) && pk.ProtocolVersion != 4 && pk.ProtocolVersion != 5) {
		return ErrProtocolViolationProtocolVersion // [MQTT-3.1.2-2]
	}

	if pk.ReservedBit != 0 {
		return ErrProtocolViolationReservedBit // [MQTT-3.1.2-3]
	}

	if len(pk.Connect.Password) > math.MaxUint16 {
		return ErrProtocolViolationPasswordTooLong
	}

	if len(pk.Connect.Username) > math.MaxUint16 {
		return ErrProtocolViolationUsernameTooLong
	}

	if !pk.Connect.UsernameFlag && len(pk.Connect.Username) > 0 {
		return ErrProtocolViolationUsernameNoFlag // [MQTT-3.1.2-16]
	}

	if pk.Connect.PasswordFlag && len(pk.Connect.Password) == 0 {
		return ErrProtocolViolationFlagNoPassword // [MQTT-3.1.2-19]
	}

	if !pk.Connect.PasswordFlag && len(pk.Connect.Password) > 0 {
		return ErrProtocolViolationPasswordNoFlag // [MQTT-3.1.2-18]
	}

	if len(pk.Connect.ClientIdentifier) > math.MaxUint16 {
		return ErrClientIdentifierNotValid
	}

	if pk.Connect.WillFlag {
		if len(pk.Connect.WillPayload) == 0 || pk.Connect.WillTopic == "" {
			return ErrProtocolViolationWillFlagNoPayload // [MQTT-3.1.2-9]
		}

		if pk.Connect.WillQos > 2 {
			return ErrProtocolViolationQosOutOfRange // [MQTT-3.1.2-12]
		}
	}

	if !pk.Connect.WillFlag && pk.Connect.WillRetain {
		return ErrProtocolViolationWillFlagSurplusRetain // [MQTT-3.1.2-13]
	}

	return CodeSuccess
}

// ConnackEncode encodes a Connack packet.
func (pk *Packet) ConnackEncode(buf *bytes.Buffer) error {
	nb := bytes.NewBuffer([]byte{})
	nb.WriteByte(encodeBool(pk.SessionPresent))
	nb.WriteByte(pk.ReasonCode)

	if pk.ProtocolVersion == 5 {
		pb := bytes.NewBuffer([]byte{})
		pk.Properties.Encode(pk.FixedHeader.Type, pk.Mods, pb, nb.Len()+2) // +SessionPresent +ReasonCode
		nb.Write(pb.Bytes())
	}

	pk.FixedHeader.Remaining = nb.Len()
	pk.FixedHeader.Encode(buf)
	_, _ = nb.WriteTo(buf)
	return nil
}

// ConnackDecode decodes a Connack packet.
func (pk *Packet) ConnackDecode(buf []byte) error {
	var offset int
	var err error

	pk.SessionPresent, offset, err = decodeByteBool(buf, 0)
	if err != nil {
		return fmt.Errorf("%s: %w", err, ErrMalformedSessionPresent)
	}

	pk.ReasonCode, offset, err = decodeByte(buf, offset)
	if err != nil {
		return fmt.Errorf("%s: %w", err, ErrMalformedReasonCode)
	}

	if pk.ProtocolVersion == 5 {
		_, err := pk.Properties.Decode(pk.FixedHeader.Type, bytes.NewBuffer(buf[offset:]))
		if err != nil {
			return fmt.Errorf("%s: %w", err, ErrMalformedProperties)
		}
	}

	return nil
}

// DisconnectEncode encodes a Disconnect packet.
func (pk *Packet) DisconnectEncode(buf *bytes.Buffer) error {
	nb := bytes.NewBuffer([]byte{})

	if pk.ProtocolVersion == 5 {
		nb.WriteByte(pk.ReasonCode)

		pb := bytes.NewBuffer([]byte{})
		pk.Properties.Encode(pk.FixedHeader.Type, pk.Mods, pb, nb.Len())
		nb.Write(pb.Bytes())
	}

	pk.FixedHeader.Remaining = nb.Len()
	pk.FixedHeader.Encode(buf)
	_, _ = nb.WriteTo(buf)

	return nil
}

// DisconnectDecode decodes a Disconnect packet.
func (pk *Packet) DisconnectDecode(buf []byte) error {
	if pk.ProtocolVersion == 5 && pk.FixedHeader.Remaining > 1 {
		var err error
		var offset int
		pk.ReasonCode, offset, err = decodeByte(buf, offset)
		if err != nil {
			return fmt.Errorf("%s: %w", err, ErrMalformedReasonCode)
		}

		if pk.FixedHeader.Remaining > 2 {
			_, err = pk.Properties.Decode(pk.FixedHeader.Type, bytes.NewBuffer(buf[offset:]))
			if err != nil {
				return fmt.Errorf("%s: %w", err, ErrMalformedProperties)
			}
		}
	}

	return nil
}

// PingreqEncode encodes a Pingreq packet.
func (pk *Packet) PingreqEncode(buf *bytes.Buffer) error {
	pk.FixedHeader.Encode(buf)
	return nil
}

// PingreqDecode decodes a Pingreq packet.
func (pk *Packet) PingreqDecode(buf []byte) error {
	return nil
}

// PingrespEncode encodes a Pingresp packet.
func (pk *Packet) PingrespEncode(buf *bytes.Buffer) error {
	pk.FixedHeader.Encode(buf)
	return nil
}

// PingrespDecode decodes a Pingres packet.
func (pk *Packet) PingrespDecode(buf []byte) error {
	return nil
}

// PublishEncode encodes a Publish packet.
func (pk *Packet) PublishEncode(buf *bytes.Buffer) error {
	nb := bytes.NewBuffer([]byte{})

	nb.Write(encodeString(pk.TopicName)) // [MQTT-3.3.2-1]

	if pk.FixedHeader.Qos > 0 {
		if pk.PacketID == 0 {
			return ErrProtocolViolationNoPacketID // [MQTT-2.2.1-2]
		}
		nb.Write(encodeUint16(pk.PacketID))
	}

	if pk.ProtocolVersion == 5 {
		pb := bytes.NewBuffer([]byte{})
		pk.Properties.Encode(pk.FixedHeader.Type, pk.Mods, pb, nb.Len()+len(pk.Payload))
		nb.Write(pb.Bytes())
	}

	nb.Write(pk.Payload)

	pk.FixedHeader.Remaining = nb.Len()
	pk.FixedHeader.Encode(buf)
	_, _ = nb.WriteTo(buf)

	return nil
}

// PublishDecode extracts the data values from the packet.
func (pk *Packet) PublishDecode(buf []byte) error {
	var offset int
	var err error

	pk.TopicName, offset, err = decodeString(buf, 0) // [MQTT-3.3.2-1]
	if err != nil {
		return fmt.Errorf("%s: %w", err, ErrMalformedTopic)
	}

	if pk.FixedHeader.Qos > 0 {
		pk.PacketID, offset, err = decodeUint16(buf, offset)
		if err != nil {
			return fmt.Errorf("%s: %w", err, ErrMalformedPacketID)
		}
	}

	if pk.ProtocolVersion == 5 {
		n, err := pk.Properties.Decode(pk.FixedHeader.Type, bytes.NewBuffer(buf[offset:]))
		if err != nil {
			return fmt.Errorf("%s: %w", err, ErrMalformedProperties)
		}

		offset += n
	}

	pk.Payload = buf[offset:]

	return nil
}

// PublishValidate validates a publish packet.
func (pk *Packet) PublishValidate(topicAliasMaximum uint16) Code {
	if pk.FixedHeader.Qos > 0 && pk.PacketID == 0 {
		return ErrProtocolViolationNoPacketID // [MQTT-2.2.1-3] [MQTT-2.2.1-4]
	}

	if pk.FixedHeader.Qos == 0 && pk.PacketID > 0 {
		return ErrProtocolViolationSurplusPacketID // [MQTT-2.2.1-2]
	}

	if strings.ContainsAny(pk.TopicName, "+#") {
		return ErrProtocolViolationSurplusWildcard // [MQTT-3.3.2-2]
	}

	if pk.Properties.TopicAlias > topicAliasMaximum {
		return ErrTopicAliasInvalid // [MQTT-3.2.2-17] [MQTT-3.3.2-9] ~[MQTT-3.3.2-10] [MQTT-3.3.2-12]
	}

	if pk.TopicName == "" && pk.Properties.TopicAlias == 0 {
		return ErrProtocolViolationNoTopic // ~[MQTT-3.3.2-8]
	}

	if pk.Properties.TopicAliasFlag && pk.Properties.TopicAlias == 0 {
		return ErrTopicAliasInvalid // [MQTT-3.3.2-8]
	}

	if len(pk.Properties.SubscriptionIdentifier) > 0 {
		return ErrProtocolViolationSurplusSubID // [MQTT-3.3.4-6]
	}

	return CodeSuccess
}

// encodePubAckRelRecComp encodes a Puback, Pubrel, Pubrec, or Pubcomp packet.
func (pk *Packet) encodePubAckRelRecComp(buf *bytes.Buffer) error {
	nb := bytes.NewBuffer([]byte{})
	nb.Write(encodeUint16(pk.PacketID))

	if pk.ProtocolVersion == 5 {
		pb := bytes.NewBuffer([]byte{})
		pk.Properties.Encode(pk.FixedHeader.Type, pk.Mods, pb, nb.Len())
		if pk.ReasonCode >= ErrUnspecifiedError.Code || pb.Len() > 1 {
			nb.WriteByte(pk.ReasonCode)
		}

		if pb.Len() > 1 {
			nb.Write(pb.Bytes())
		}
	}

	pk.FixedHeader.Remaining = nb.Len()
	pk.FixedHeader.Encode(buf)
	_, _ = nb.WriteTo(buf)
	return nil
}

// decode extracts the data values from a Puback, Pubrel, Pubrec, or Pubcomp packet.
func (pk *Packet) decodePubAckRelRecComp(buf []byte) error {
	var offset int
	var err error
	pk.PacketID, offset, err = decodeUint16(buf, offset)
	if err != nil {
		return fmt.Errorf("%s: %w", err, ErrMalformedPacketID)
	}

	if pk.ProtocolVersion == 5 && pk.FixedHeader.Remaining > 2 {
		pk.ReasonCode, offset, err = decodeByte(buf, offset)
		if err != nil {
			return fmt.Errorf("%s: %w", err, ErrMalformedReasonCode)
		}

		if pk.FixedHeader.Remaining > 3 {
			_, err = pk.Properties.Decode(pk.FixedHeader.Type, bytes.NewBuffer(buf[offset:]))
			if err != nil {
				return fmt.Errorf("%s: %w", err, ErrMalformedProperties)
			}
		}
	}

	return nil
}

// PubackEncode encodes a Puback packet.
func (pk *Packet) PubackEncode(buf *bytes.Buffer) error {
	return pk.encodePubAckRelRecComp(buf)
}

// PubackDecode decodes a Puback packet.
func (pk *Packet) PubackDecode(buf []byte) error {
	return pk.decodePubAckRelRecComp(buf)
}

// PubcompEncode encodes a Pubcomp packet.
func (pk *Packet) PubcompEncode(buf *bytes.Buffer) error {
	return pk.encodePubAckRelRecComp(buf)
}

// PubcompDecode decodes a Pubcomp packet.
func (pk *Packet) PubcompDecode(buf []byte) error {
	return pk.decodePubAckRelRecComp(buf)
}

// PubrecEncode encodes a Pubrec packet.
func (pk *Packet) PubrecEncode(buf *bytes.Buffer) error {
	return pk.encodePubAckRelRecComp(buf)
}

// PubrecDecode decodes a Pubrec packet.
func (pk *Packet) PubrecDecode(buf []byte) error {
	return pk.decodePubAckRelRecComp(buf)
}

// PubrelEncode encodes a Pubrel packet.
func (pk *Packet) PubrelEncode(buf *bytes.Buffer) error {
	return pk.encodePubAckRelRecComp(buf)
}

// PubrelDecode decodes a Pubrel packet.
func (pk *Packet) PubrelDecode(buf []byte) error {
	return pk.decodePubAckRelRecComp(buf)
}

// ReasonCodeValid returns true if the provided reason code is valid for the packet type.
func (pk *Packet) ReasonCodeValid() bool {
	switch pk.FixedHeader.Type {
	case Pubrec:
		return bytes.Contains([]byte{
			CodeSuccess.Code,
			CodeNoMatchingSubscribers.Code,
			ErrUnspecifiedError.Code,
			ErrImplementationSpecificError.Code,
			ErrNotAuthorized.Code,
			ErrTopicNameInvalid.Code,
			ErrPacketIdentifierInUse.Code,
			ErrQuotaExceeded.Code,
			ErrPayloadFormatInvalid.Code,
		}, []byte{pk.ReasonCode})
	case Pubrel:
		fallthrough
	case Pubcomp:
		return bytes.Contains([]byte{
			CodeSuccess.Code,
			ErrPacketIdentifierNotFound.Code,
		}, []byte{pk.ReasonCode})
	case Suback:
		return bytes.Contains([]byte{
			CodeGrantedQos0.Code,
			CodeGrantedQos1.Code,
			CodeGrantedQos2.Code,
			ErrUnspecifiedError.Code,
			ErrImplementationSpecificError.Code,
			ErrNotAuthorized.Code,
			ErrTopicFilterInvalid.Code,
			ErrPacketIdentifierInUse.Code,
			ErrQuotaExceeded.Code,
			ErrSharedSubscriptionsNotSupported.Code,
			ErrSubscriptionIdentifiersNotSupported.Code,
			ErrWildcardSubscriptionsNotSupported.Code,
		}, []byte{pk.ReasonCode})
	case Unsuback:
		return bytes.Contains([]byte{
			CodeSuccess.Code,
			CodeNoSubscriptionExisted.Code,
			ErrUnspecifiedError.Code,
			ErrImplementationSpecificError.Code,
			ErrNotAuthorized.Code,
			ErrTopicFilterInvalid.Code,
			ErrPacketIdentifierInUse.Code,
		}, []byte{pk.ReasonCode})
	}

	return true
}

// SubackEncode encodes a Suback packet.
func (pk *Packet) SubackEncode(buf *bytes.Buffer) error {
	nb := bytes.NewBuffer([]byte{})
	nb.Write(encodeUint16(pk.PacketID))

	if pk.ProtocolVersion == 5 {
		pb := bytes.NewBuffer([]byte{})
		pk.Properties.Encode(pk.FixedHeader.Type, pk.Mods, pb, nb.Len()+len(pk.ReasonCodes))
		nb.Write(pb.Bytes())
	}

	nb.Write(pk.ReasonCodes)

	pk.FixedHeader.Remaining = nb.Len()
	pk.FixedHeader.Encode(buf)
	_, _ = nb.WriteTo(buf)

	return nil
}

// SubackDecode decodes a Suback packet.
func (pk *Packet) SubackDecode(buf []byte) error {
	var offset int
	var err error

	pk.PacketID, offset, err = decodeUint16(buf, offset)
	if err != nil {
		return fmt.Errorf("%s: %w", err, ErrMalformedPacketID)
	}

	if pk.ProtocolVersion == 5 {
		n, err := pk.Properties.Decode(pk.FixedHeader.Type, bytes.NewBuffer(buf[offset:]))
		if err != nil {
			return fmt.Errorf("%s: %w", err, ErrMalformedProperties)
		}
		offset += n
	}

	pk.ReasonCodes = buf[offset:]

	return nil
}

// SubscribeEncode encodes a Subscribe packet.
func (pk *Packet) SubscribeEncode(buf *bytes.Buffer) error {
	if pk.PacketID == 0 {
		return ErrProtocolViolationNoPacketID
	}

	nb := bytes.NewBuffer([]byte{})
	nb.Write(encodeUint16(pk.PacketID))

	xb := bytes.NewBuffer([]byte{}) // capture and write filters after length checks
	for _, opts := range pk.Filters {
		xb.Write(encodeString(opts.Filter)) // [MQTT-3.8.3-1]
		if pk.ProtocolVersion == 5 {
			xb.WriteByte(opts.encode())
		} else {
			xb.WriteByte(opts.Qos)
		}
	}

	if pk.ProtocolVersion == 5 {
		pb := bytes.NewBuffer([]byte{})
		pk.Properties.Encode(pk.FixedHeader.Type, pk.Mods, pb, nb.Len()+xb.Len())
		nb.Write(pb.Bytes())
	}

	nb.Write(xb.Bytes())

	pk.FixedHeader.Remaining = nb.Len()
	pk.FixedHeader.Encode(buf)
	_, _ = nb.WriteTo(buf)

	return nil
}

// SubscribeDecode decodes a Subscribe packet.
func (pk *Packet) SubscribeDecode(buf []byte) error {
	var offset int
	var err error

	pk.PacketID, offset, err = decodeUint16(buf, offset)
	if err != nil {
		return ErrMalformedPacketID
	}

	if pk.ProtocolVersion == 5 {
		n, err := pk.Properties.Decode(pk.FixedHeader.Type, bytes.NewBuffer(buf[offset:]))
		if err != nil {
			return fmt.Errorf("%s: %w", err, ErrMalformedProperties)
		}
		offset += n
	}

	var filter string
	pk.Filters = Subscriptions{}
	for offset < len(buf) {
		filter, offset, err = decodeString(buf, offset) // [MQTT-3.8.3-1]
		if err != nil {
			return ErrMalformedTopic
		}

		var option byte
		sub := &Subscription{
			Filter: filter,
		}

		if pk.ProtocolVersion == 5 {
			sub.decode(buf[offset])
			offset += 1
		} else {
			option, offset, err = decodeByte(buf, offset)
			if err != nil {
				return ErrMalformedQos
			}
			sub.Qos = option
		}

		if len(pk.Properties.SubscriptionIdentifier) > 0 {
			sub.Identifier = pk.Properties.SubscriptionIdentifier[0]
		}

		if sub.Qos > 2 {
			return ErrProtocolViolationQosOutOfRange
		}

		pk.Filters = append(pk.Filters, *sub)
	}

	return nil
}

// SubscribeValidate ensures the packet is compliant.
func (pk *Packet) SubscribeValidate() Code {
	if pk.FixedHeader.Qos > 0 && pk.PacketID == 0 {
		return ErrProtocolViolationNoPacketID // [MQTT-2.2.1-3] [MQTT-2.2.1-4]
	}

	if len(pk.Filters) == 0 {
		return ErrProtocolViolationNoFilters // [MQTT-3.10.3-2]
	}

	for _, v := range pk.Filters {
		if v.Identifier > 268435455 { // 3.3.2.3.8 The Subscription Identifier can have the value of 1 to 268,435,455.
			return ErrProtocolViolationOversizeSubID //
		}
	}

	return CodeSuccess
}

// UnsubackEncode encodes an Unsuback packet.
func (pk *Packet) UnsubackEncode(buf *bytes.Buffer) error {
	nb := bytes.NewBuffer([]byte{})
	nb.Write(encodeUint16(pk.PacketID))

	if pk.ProtocolVersion == 5 {
		pb := bytes.NewBuffer([]byte{})
		pk.Properties.Encode(pk.FixedHeader.Type, pk.Mods, pb, nb.Len())
		nb.Write(pb.Bytes())
	}

	nb.Write(pk.ReasonCodes)

	pk.FixedHeader.Remaining = nb.Len()
	pk.FixedHeader.Encode(buf)
	_, _ = nb.WriteTo(buf)

	return nil
}

// UnsubackDecode decodes an Unsuback packet.
func (pk *Packet) UnsubackDecode(buf []byte) error {
	var offset int
	var err error

	pk.PacketID, offset, err = decodeUint16(buf, offset)
	if err != nil {
		return fmt.Errorf("%s: %w", err, ErrMalformedPacketID)
	}

	if pk.ProtocolVersion == 5 {
		n, err := pk.Properties.Decode(pk.FixedHeader.Type, bytes.NewBuffer(buf[offset:]))
		if err != nil {
			return fmt.Errorf("%s: %w", err, ErrMalformedProperties)
		}

		offset += n

		pk.ReasonCodes = buf[offset:]
	}

	return nil
}

// UnsubscribeEncode encodes an Unsubscribe packet.
func (pk *Packet) UnsubscribeEncode(buf *bytes.Buffer) error {
	if pk.PacketID == 0 {
		return ErrProtocolViolationNoPacketID
	}

	nb := bytes.NewBuffer([]byte{})
	nb.Write(encodeUint16(pk.PacketID))

	xb := bytes.NewBuffer([]byte{}) // capture filters and write after length checks
	for _, sub := range pk.Filters {
		xb.Write(encodeString(sub.Filter)) // [MQTT-3.10.3-1]
	}

	if pk.ProtocolVersion == 5 {
		pb := bytes.NewBuffer([]byte{})
		pk.Properties.Encode(pk.FixedHeader.Type, pk.Mods, pb, nb.Len()+xb.Len())
		nb.Write(pb.Bytes())
	}

	nb.Write(xb.Bytes())

	pk.FixedHeader.Remaining = nb.Len()
	pk.FixedHeader.Encode(buf)
	_, _ = nb.WriteTo(buf)

	return nil
}

// UnsubscribeDecode decodes an Unsubscribe packet.
func (pk *Packet) UnsubscribeDecode(buf []byte) error {
	var offset int
	var err error

	pk.PacketID, offset, err = decodeUint16(buf, offset)
	if err != nil {
		return fmt.Errorf("%s: %w", err, ErrMalformedPacketID)
	}

	if pk.ProtocolVersion == 5 {
		n, err := pk.Properties.Decode(pk.FixedHeader.Type, bytes.NewBuffer(buf[offset:]))
		if err != nil {
			return fmt.Errorf("%s: %w", err, ErrMalformedProperties)
		}
		offset += n
	}

	var filter string
	pk.Filters = Subscriptions{}
	for offset < len(buf) {
		filter, offset, err = decodeString(buf, offset) // [MQTT-3.10.3-1]
		if err != nil {
			return fmt.Errorf("%s: %w", err, ErrMalformedTopic)
		}
		pk.Filters = append(pk.Filters, Subscription{Filter: filter})
	}

	return nil
}

// UnsubscribeValidate validates an Unsubscribe packet.
func (pk *Packet) UnsubscribeValidate() Code {
	if pk.FixedHeader.Qos > 0 && pk.PacketID == 0 {
		return ErrProtocolViolationNoPacketID // [MQTT-2.2.1-3] [MQTT-2.2.1-4]
	}

	if len(pk.Filters) == 0 {
		return ErrProtocolViolationNoFilters // [MQTT-3.10.3-2]
	}

	return CodeSuccess
}

// AuthEncode encodes an Auth packet.
func (pk *Packet) AuthEncode(buf *bytes.Buffer) error {
	nb := bytes.NewBuffer([]byte{})
	nb.WriteByte(pk.ReasonCode)

	pb := bytes.NewBuffer([]byte{})
	pk.Properties.Encode(pk.FixedHeader.Type, pk.Mods, pb, nb.Len())
	nb.Write(pb.Bytes())

	pk.FixedHeader.Remaining = nb.Len()
	pk.FixedHeader.Encode(buf)
	_, _ = nb.WriteTo(buf)
	return nil
}

// AuthDecode decodes an Auth packet.
func (pk *Packet) AuthDecode(buf []byte) error {
	var offset int
	var err error

	pk.ReasonCode, offset, err = decodeByte(buf, offset)
	if err != nil {
		return fmt.Errorf("%s: %w", err, ErrMalformedReasonCode)
	}

	_, err = pk.Properties.Decode(pk.FixedHeader.Type, bytes.NewBuffer(buf[offset:]))
	if err != nil {
		return fmt.Errorf("%s: %w", err, ErrMalformedProperties)
	}

	return nil
}

// AuthValidate returns success if the auth packet is valid.
func (pk *Packet) AuthValidate() Code {
	if pk.ReasonCode != CodeSuccess.Code &&
		pk.ReasonCode != CodeContinueAuthentication.Code &&
		pk.ReasonCode != CodeReAuthenticate.Code {
		return ErrProtocolViolationInvalidReason // [MQTT-3.15.2-1]
	}

	return CodeSuccess
}

// FormatID returns the PacketID field as a decimal integer.
func (pk *Packet) FormatID() string {
	return strconv.FormatUint(uint64(pk.PacketID), 10)
}
