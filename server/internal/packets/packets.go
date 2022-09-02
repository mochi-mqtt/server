package packets

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
)

// All of the valid packet types and their packet identifier.
const (
	Reserved    byte = iota
	Connect          // 1
	Connack          // 2
	Publish          // 3
	Puback           // 4
	Pubrec           // 5
	Pubrel           // 6
	Pubcomp          // 7
	Subscribe        // 8
	Suback           // 9
	Unsubscribe      // 10
	Unsuback         // 11
	Pingreq          // 12
	Pingresp         // 13
	Disconnect       // 14

	Accepted                      byte = 0x00
	Failed                        byte = 0xFF
	CodeConnectBadProtocolVersion byte = 0x01
	CodeConnectBadClientID        byte = 0x02
	CodeConnectServerUnavailable  byte = 0x03
	CodeConnectBadAuthValues      byte = 0x04
	CodeConnectNotAuthorised      byte = 0x05
	CodeConnectNetworkError       byte = 0xFE
	CodeConnectProtocolViolation  byte = 0xFF
	ErrSubAckNetworkError         byte = 0x80
)

var (
	// CONNECT
	ErrMalformedProtocolName    = errors.New("malformed packet: protocol name")
	ErrMalformedProtocolVersion = errors.New("malformed packet: protocol version")
	ErrMalformedFlags           = errors.New("malformed packet: flags")
	ErrMalformedKeepalive       = errors.New("malformed packet: keepalive")
	ErrMalformedClientID        = errors.New("malformed packet: client id")
	ErrMalformedWillTopic       = errors.New("malformed packet: will topic")
	ErrMalformedWillMessage     = errors.New("malformed packet: will message")
	ErrMalformedUsername        = errors.New("malformed packet: username")
	ErrMalformedPassword        = errors.New("malformed packet: password")

	// CONNACK
	ErrMalformedSessionPresent = errors.New("malformed packet: session present")
	ErrMalformedReturnCode     = errors.New("malformed packet: return code")

	// PUBLISH
	ErrMalformedTopic    = errors.New("malformed packet: topic name")
	ErrMalformedPacketID = errors.New("malformed packet: packet id")

	// SUBSCRIBE
	ErrMalformedQoS = errors.New("malformed packet: qos")

	// PACKETS
	ErrProtocolViolation        = errors.New("protocol violation")
	ErrOffsetBytesOutOfRange    = errors.New("offset bytes out of range")
	ErrOffsetByteOutOfRange     = errors.New("offset byte out of range")
	ErrOffsetBoolOutOfRange     = errors.New("offset bool out of range")
	ErrOffsetUintOutOfRange     = errors.New("offset uint out of range")
	ErrOffsetStrInvalidUTF8     = errors.New("offset string invalid utf8")
	ErrInvalidFlags             = errors.New("invalid flags set for packet")
	ErrOversizedLengthIndicator = errors.New("protocol violation: oversized length indicator")
	ErrMissingPacketID          = errors.New("missing packet id")
	ErrSurplusPacketID          = errors.New("surplus packet id")
)

// Packet is an MQTT packet. Instead of providing a packet interface and variant
// packet structs, this is a single concrete packet type to cover all packet
// types, which allows us to take advantage of various compiler optimizations.
type Packet struct {
	FixedHeader      FixedHeader
	AllowClients     []string // For use with OnMessage event hook.
	Topics           []string
	ReturnCodes      []byte
	ProtocolName     []byte
	Qoss             []byte
	Payload          []byte
	Username         []byte
	Password         []byte
	WillMessage      []byte
	ClientIdentifier string
	TopicName        string
	WillTopic        string
	PacketID         uint16
	Keepalive        uint16
	ReturnCode       byte
	ProtocolVersion  byte
	WillQos          byte
	ReservedBit      byte
	CleanSession     bool
	WillFlag         bool
	WillRetain       bool
	UsernameFlag     bool
	PasswordFlag     bool
	SessionPresent   bool
}

// ConnectEncode encodes a connect packet.
func (pk *Packet) ConnectEncode(buf *bytes.Buffer) error {

	protoName := encodeBytes(pk.ProtocolName)
	protoVersion := pk.ProtocolVersion
	flag := encodeBool(pk.CleanSession)<<1 | encodeBool(pk.WillFlag)<<2 | pk.WillQos<<3 | encodeBool(pk.WillRetain)<<5 | encodeBool(pk.PasswordFlag)<<6 | encodeBool(pk.UsernameFlag)<<7
	keepalive := encodeUint16(pk.Keepalive)
	clientID := encodeString(pk.ClientIdentifier)

	var willTopic, willFlag, usernameFlag, passwordFlag []byte

	// If will flag is set, add topic and message.
	if pk.WillFlag {
		willTopic = encodeString(pk.WillTopic)
		willFlag = encodeBytes(pk.WillMessage)
	}

	// If username flag is set, add username.
	if pk.UsernameFlag {
		usernameFlag = encodeBytes(pk.Username)
	}

	// If password flag is set, add password.
	if pk.PasswordFlag {
		passwordFlag = encodeBytes(pk.Password)
	}

	// Get a length for the connect header. This is not super pretty, but it works.
	pk.FixedHeader.Remaining =
		len(protoName) + 1 + 1 + len(keepalive) + len(clientID) +
			len(willTopic) + len(willFlag) +
			len(usernameFlag) + len(passwordFlag)

	pk.FixedHeader.Encode(buf)

	// Eschew magic for readability.
	buf.Write(protoName)
	buf.WriteByte(protoVersion)
	buf.WriteByte(flag)
	buf.Write(keepalive)
	buf.Write(clientID)
	buf.Write(willTopic)
	buf.Write(willFlag)
	buf.Write(usernameFlag)
	buf.Write(passwordFlag)

	return nil
}

// ConnectDecode decodes a connect packet.
func (pk *Packet) ConnectDecode(buf []byte) error {
	var offset int
	var err error

	// Unpack protocol name and version.
	pk.ProtocolName, offset, err = decodeBytes(buf, 0)
	if err != nil {
		return fmt.Errorf("%s: %w", err, ErrMalformedProtocolName)
	}

	pk.ProtocolVersion, offset, err = decodeByte(buf, offset)
	if err != nil {
		return fmt.Errorf("%s: %w", err, ErrMalformedProtocolVersion)
	}
	// Unpack flags byte.
	flags, offset, err := decodeByte(buf, offset)
	if err != nil {
		return fmt.Errorf("%s: %w", err, ErrMalformedFlags)
	}
	pk.ReservedBit = 1 & flags
	pk.CleanSession = 1&(flags>>1) > 0
	pk.WillFlag = 1&(flags>>2) > 0
	pk.WillQos = 3 & (flags >> 3) // this one is not a bool
	pk.WillRetain = 1&(flags>>5) > 0
	pk.PasswordFlag = 1&(flags>>6) > 0
	pk.UsernameFlag = 1&(flags>>7) > 0

	// Get keepalive interval.
	pk.Keepalive, offset, err = decodeUint16(buf, offset)
	if err != nil {
		return fmt.Errorf("%s: %w", err, ErrMalformedKeepalive)
	}

	// Get client ID.
	pk.ClientIdentifier, offset, err = decodeString(buf, offset)
	if err != nil {
		return fmt.Errorf("%s: %w", err, ErrMalformedClientID)
	}

	// Get Last Will and Testament topic and message if applicable.
	if pk.WillFlag {
		pk.WillTopic, offset, err = decodeString(buf, offset)
		if err != nil {
			return fmt.Errorf("%s: %w", err, ErrMalformedWillTopic)
		}

		pk.WillMessage, offset, err = decodeBytes(buf, offset)
		if err != nil {
			return fmt.Errorf("%s: %w", err, ErrMalformedWillMessage)
		}
	}

	// Get username and password if applicable.
	if pk.UsernameFlag {
		pk.Username, offset, err = decodeBytes(buf, offset)
		if err != nil {
			return fmt.Errorf("%s: %w", err, ErrMalformedUsername)
		}
	}

	if pk.PasswordFlag {
		pk.Password, _, err = decodeBytes(buf, offset)
		if err != nil {
			return fmt.Errorf("%s: %w", err, ErrMalformedPassword)
		}
	}

	return nil

}

// ConnectValidate ensures the connect packet is compliant.
func (pk *Packet) ConnectValidate() (b byte, err error) {

	// End if protocol name is bad.
	if bytes.Compare(pk.ProtocolName, []byte{'M', 'Q', 'I', 's', 'd', 'p'}) != 0 &&
		bytes.Compare(pk.ProtocolName, []byte{'M', 'Q', 'T', 'T'}) != 0 {
		return CodeConnectProtocolViolation, ErrProtocolViolation
	}

	// End if protocol version is bad.
	if (bytes.Compare(pk.ProtocolName, []byte{'M', 'Q', 'I', 's', 'd', 'p'}) == 0 && pk.ProtocolVersion != 3) ||
		(bytes.Compare(pk.ProtocolName, []byte{'M', 'Q', 'T', 'T'}) == 0 && pk.ProtocolVersion != 4) {
		return CodeConnectBadProtocolVersion, ErrProtocolViolation
	}

	// End if reserved bit is not 0.
	if pk.ReservedBit != 0 {
		return CodeConnectProtocolViolation, ErrProtocolViolation
	}

	// End if ClientID is too long.
	if len(pk.ClientIdentifier) > 65535 {
		return CodeConnectProtocolViolation, ErrProtocolViolation
	}

	// End if password flag is set without a username.
	if pk.PasswordFlag && !pk.UsernameFlag {
		return CodeConnectProtocolViolation, ErrProtocolViolation
	}

	// End if Username or Password is too long.
	if len(pk.Username) > 65535 || len(pk.Password) > 65535 {
		return CodeConnectProtocolViolation, ErrProtocolViolation
	}

	// End if client id isn't set and clean session is false.
	if !pk.CleanSession && len(pk.ClientIdentifier) == 0 {
		return CodeConnectBadClientID, ErrProtocolViolation
	}

	return Accepted, nil
}

// ConnackEncode encodes a Connack packet.
func (pk *Packet) ConnackEncode(buf *bytes.Buffer) error {
	pk.FixedHeader.Remaining = 2
	pk.FixedHeader.Encode(buf)
	buf.WriteByte(encodeBool(pk.SessionPresent))
	buf.WriteByte(pk.ReturnCode)
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

	pk.ReturnCode, _, err = decodeByte(buf, offset)
	if err != nil {
		return fmt.Errorf("%s: %w", err, ErrMalformedReturnCode)
	}

	return nil
}

// DisconnectEncode encodes a Disconnect packet.
func (pk *Packet) DisconnectEncode(buf *bytes.Buffer) error {
	pk.FixedHeader.Encode(buf)
	return nil
}

// PingreqEncode encodes a Pingreq packet.
func (pk *Packet) PingreqEncode(buf *bytes.Buffer) error {
	pk.FixedHeader.Encode(buf)
	return nil
}

// PingrespEncode encodes a Pingresp packet.
func (pk *Packet) PingrespEncode(buf *bytes.Buffer) error {
	pk.FixedHeader.Encode(buf)
	return nil
}

// PubackEncode encodes a Puback packet.
func (pk *Packet) PubackEncode(buf *bytes.Buffer) error {
	pk.FixedHeader.Remaining = 2
	pk.FixedHeader.Encode(buf)
	buf.Write(encodeUint16(pk.PacketID))
	return nil
}

// PubackDecode decodes a Puback packet.
func (pk *Packet) PubackDecode(buf []byte) error {
	var err error
	pk.PacketID, _, err = decodeUint16(buf, 0)
	if err != nil {
		return fmt.Errorf("%s: %w", err, ErrMalformedPacketID)
	}
	return nil
}

// PubcompEncode encodes a Pubcomp packet.
func (pk *Packet) PubcompEncode(buf *bytes.Buffer) error {
	pk.FixedHeader.Remaining = 2
	pk.FixedHeader.Encode(buf)
	buf.Write(encodeUint16(pk.PacketID))
	return nil
}

// PubcompDecode decodes a Pubcomp packet.
func (pk *Packet) PubcompDecode(buf []byte) error {
	var err error
	pk.PacketID, _, err = decodeUint16(buf, 0)
	if err != nil {
		return fmt.Errorf("%s: %w", err, ErrMalformedPacketID)
	}
	return nil
}

// PublishEncode encodes a Publish packet.
func (pk *Packet) PublishEncode(buf *bytes.Buffer) error {
	topicName := encodeString(pk.TopicName)
	var packetID []byte

	// Add PacketID if QOS is set.
	// [MQTT-2.3.1-5] A PUBLISH Packet MUST NOT contain a Packet Identifier if its QoS value is set to 0.
	if pk.FixedHeader.Qos > 0 {

		// [MQTT-2.3.1-1] SUBSCRIBE, UNSUBSCRIBE, and PUBLISH (in cases where QoS > 0) Control Packets MUST contain a non-zero 16-bit Packet Identifier.
		if pk.PacketID == 0 {
			return ErrMissingPacketID
		}

		packetID = encodeUint16(pk.PacketID)
	}

	pk.FixedHeader.Remaining = len(topicName) + len(packetID) + len(pk.Payload)
	pk.FixedHeader.Encode(buf)
	buf.Write(topicName)
	buf.Write(packetID)
	buf.Write(pk.Payload)

	return nil
}

// PublishDecode extracts the data values from the packet.
func (pk *Packet) PublishDecode(buf []byte) error {
	var offset int
	var err error

	pk.TopicName, offset, err = decodeString(buf, 0)
	if err != nil {
		return fmt.Errorf("%s: %w", err, ErrMalformedTopic)
	}

	// If QOS decode Packet ID.
	if pk.FixedHeader.Qos > 0 {
		pk.PacketID, offset, err = decodeUint16(buf, offset)
		if err != nil {
			return fmt.Errorf("%s: %w", err, ErrMalformedPacketID)
		}
	}

	pk.Payload = buf[offset:]

	return nil
}

// PublishCopy creates a new instance of Publish packet bearing the
// same payload and destination topic, but with an empty header for
// inheriting new QoS flags, etc.
func (pk *Packet) PublishCopy() Packet {
	return Packet{
		FixedHeader: FixedHeader{
			Type:   Publish,
			Retain: pk.FixedHeader.Retain,
		},
		TopicName: pk.TopicName,
		Payload:   pk.Payload,
	}
}

// PublishValidate validates a publish packet.
func (pk *Packet) PublishValidate() (byte, error) {

	// @SPEC [MQTT-2.3.1-1]
	// SUBSCRIBE, UNSUBSCRIBE, and PUBLISH (in cases where QoS > 0) Control Packets MUST contain a non-zero 16-bit Packet Identifier.
	if pk.FixedHeader.Qos > 0 && pk.PacketID == 0 {
		return Failed, ErrMissingPacketID
	}

	// @SPEC [MQTT-2.3.1-5]
	// A PUBLISH Packet MUST NOT contain a Packet Identifier if its QoS value is set to 0.
	if pk.FixedHeader.Qos == 0 && pk.PacketID > 0 {
		return Failed, ErrSurplusPacketID
	}

	return Accepted, nil
}

// PubrecEncode encodes a Pubrec packet.
func (pk *Packet) PubrecEncode(buf *bytes.Buffer) error {
	pk.FixedHeader.Remaining = 2
	pk.FixedHeader.Encode(buf)
	buf.Write(encodeUint16(pk.PacketID))
	return nil
}

// PubrecDecode decodes a Pubrec packet.
func (pk *Packet) PubrecDecode(buf []byte) error {
	var err error
	pk.PacketID, _, err = decodeUint16(buf, 0)
	if err != nil {
		return fmt.Errorf("%s: %w", err, ErrMalformedPacketID)
	}

	return nil
}

// PubrelEncode encodes a Pubrel packet.
func (pk *Packet) PubrelEncode(buf *bytes.Buffer) error {
	pk.FixedHeader.Remaining = 2
	pk.FixedHeader.Encode(buf)
	buf.Write(encodeUint16(pk.PacketID))
	return nil
}

// PubrelDecode decodes a Pubrel packet.
func (pk *Packet) PubrelDecode(buf []byte) error {
	var err error
	pk.PacketID, _, err = decodeUint16(buf, 0)
	if err != nil {
		return fmt.Errorf("%s: %w", err, ErrMalformedPacketID)
	}
	return nil
}

// SubackEncode encodes a Suback packet.
func (pk *Packet) SubackEncode(buf *bytes.Buffer) error {
	packetID := encodeUint16(pk.PacketID)
	pk.FixedHeader.Remaining = len(packetID) + len(pk.ReturnCodes) // Set length.
	pk.FixedHeader.Encode(buf)

	buf.Write(packetID)       // Encode Packet ID.
	buf.Write(pk.ReturnCodes) // Encode granted QOS flags.

	return nil
}

// SubackDecode decodes a Suback packet.
func (pk *Packet) SubackDecode(buf []byte) error {
	var offset int
	var err error

	// Get Packet ID.
	pk.PacketID, offset, err = decodeUint16(buf, offset)
	if err != nil {
		return fmt.Errorf("%s: %w", err, ErrMalformedPacketID)
	}

	// Get Granted QOS flags.
	pk.ReturnCodes = buf[offset:]

	return nil
}

// SubscribeEncode encodes a Subscribe packet.
func (pk *Packet) SubscribeEncode(buf *bytes.Buffer) error {

	// Add the Packet ID.
	// [MQTT-2.3.1-1] SUBSCRIBE, UNSUBSCRIBE, and PUBLISH (in cases where QoS > 0) Control Packets MUST contain a non-zero 16-bit Packet Identifier.
	if pk.PacketID == 0 {
		return ErrMissingPacketID
	}

	packetID := encodeUint16(pk.PacketID)

	// Count topics lengths and associated QOS flags.
	var topicsLen int
	for _, topic := range pk.Topics {
		topicsLen += len(encodeString(topic)) + 1
	}

	pk.FixedHeader.Remaining = len(packetID) + topicsLen
	pk.FixedHeader.Encode(buf)
	buf.Write(packetID)

	// Add all provided topic names and associated QOS flags.
	for i, topic := range pk.Topics {
		buf.Write(encodeString(topic))
		buf.WriteByte(pk.Qoss[i])
	}

	return nil
}

// SubscribeDecode decodes a Subscribe packet.
func (pk *Packet) SubscribeDecode(buf []byte) error {
	var offset int
	var err error

	// Get the Packet ID.
	pk.PacketID, offset, err = decodeUint16(buf, 0)
	if err != nil {
		return fmt.Errorf("%s: %w", err, ErrMalformedPacketID)
	}

	// Keep decoding until there's no space left.
	for offset < len(buf) {

		// Decode Topic Name.
		var topic string
		topic, offset, err = decodeString(buf, offset)
		if err != nil {
			return fmt.Errorf("%s: %w", err, ErrMalformedTopic)
		}
		pk.Topics = append(pk.Topics, topic)

		// Decode QOS flag.
		var qos byte
		qos, offset, err = decodeByte(buf, offset)
		if err != nil {
			return fmt.Errorf("%s: %w", err, ErrMalformedQoS)
		}

		// Ensure QoS byte is within range.
		if !(qos >= 0 && qos <= 2) {
			//if !validateQoS(qos) {
			return ErrMalformedQoS
		}

		pk.Qoss = append(pk.Qoss, qos)
	}

	return nil
}

// SubscribeValidate ensures the packet is compliant.
func (pk *Packet) SubscribeValidate() (byte, error) {
	// @SPEC [MQTT-2.3.1-1].
	// SUBSCRIBE, UNSUBSCRIBE, and PUBLISH (in cases where QoS > 0) Control Packets MUST contain a non-zero 16-bit Packet Identifier.
	if pk.FixedHeader.Qos > 0 && pk.PacketID == 0 {
		return Failed, ErrMissingPacketID
	}

	return Accepted, nil
}

// UnsubackEncode encodes an Unsuback packet.
func (pk *Packet) UnsubackEncode(buf *bytes.Buffer) error {
	pk.FixedHeader.Remaining = 2
	pk.FixedHeader.Encode(buf)
	buf.Write(encodeUint16(pk.PacketID))
	return nil
}

// UnsubackDecode decodes an Unsuback packet.
func (pk *Packet) UnsubackDecode(buf []byte) error {
	var err error
	pk.PacketID, _, err = decodeUint16(buf, 0)
	if err != nil {
		return fmt.Errorf("%s: %w", err, ErrMalformedPacketID)
	}
	return nil
}

// UnsubscribeEncode encodes an Unsubscribe packet.
func (pk *Packet) UnsubscribeEncode(buf *bytes.Buffer) error {

	// Add the Packet ID.
	// [MQTT-2.3.1-1] SUBSCRIBE, UNSUBSCRIBE, and PUBLISH (in cases where QoS > 0) Control Packets MUST contain a non-zero 16-bit Packet Identifier.
	if pk.PacketID == 0 {
		return ErrMissingPacketID
	}

	packetID := encodeUint16(pk.PacketID)

	// Count topics lengths.
	var topicsLen int
	for _, topic := range pk.Topics {
		topicsLen += len(encodeString(topic))
	}

	pk.FixedHeader.Remaining = len(packetID) + topicsLen
	pk.FixedHeader.Encode(buf)
	buf.Write(packetID)

	// Add all provided topic names.
	for _, topic := range pk.Topics {
		buf.Write(encodeString(topic))
	}

	return nil
}

// UnsubscribeDecode decodes an Unsubscribe packet.
func (pk *Packet) UnsubscribeDecode(buf []byte) error {
	var offset int
	var err error

	// Get the Packet ID.
	pk.PacketID, offset, err = decodeUint16(buf, 0)
	if err != nil {
		return fmt.Errorf("%s: %w", err, ErrMalformedPacketID)
	}

	// Keep decoding until there's no space left.
	for offset < len(buf) {
		var t string
		t, offset, err = decodeString(buf, offset) // Decode Topic Name.
		if err != nil {
			return fmt.Errorf("%s: %w", err, ErrMalformedTopic)
		}

		if len(t) > 0 {
			pk.Topics = append(pk.Topics, t)
		}
	}

	return nil

}

// UnsubscribeValidate validates an Unsubscribe packet.
func (pk *Packet) UnsubscribeValidate() (byte, error) {
	// @SPEC [MQTT-2.3.1-1].
	// SUBSCRIBE, UNSUBSCRIBE, and PUBLISH (in cases where QoS > 0) Control Packets MUST contain a non-zero 16-bit Packet Identifier.
	if pk.FixedHeader.Qos > 0 && pk.PacketID == 0 {
		return Failed, ErrMissingPacketID
	}

	return Accepted, nil
}

// FormatID returns the PacketID field as a decimal integer.
func (pk *Packet) FormatID() string {
	return strconv.FormatUint(uint64(pk.PacketID), 10)
}
