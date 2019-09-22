package packets

import (
	"bytes"
	"errors"
)

// PublishPacket contains the values of an MQTT PUBLISH packet.
type PublishPacket struct {
	FixedHeader

	TopicName string
	PacketID  uint16
	Payload   []byte
}

// Encode encodes and writes the packet data values to the buffer.
func (pk *PublishPacket) Encode(buf *bytes.Buffer) error {
	var body bytes.Buffer

	body.Write(encodeString(pk.TopicName)) // Write topic name.

	// Add PacketID if QOS is set.
	// [MQTT-2.3.1-5] A PUBLISH Packet MUST NOT contain a Packet Identifier if its QoS value is set to 0.
	if pk.Qos > 0 {

		// [MQTT-2.3.1-1] SUBSCRIBE, UNSUBSCRIBE, and PUBLISH (in cases where QoS > 0) Control Packets MUST contain a non-zero 16-bit Packet Identifier.
		if pk.PacketID == 0 {
			return errors.New(ErrMissingPacketID)
		}

		body.Write(encodeUint16(pk.PacketID))
	}

	// Set remaining length.
	pk.FixedHeader.Remaining = body.Len() + len(pk.Payload)
	pk.FixedHeader.encode(buf)
	buf.Write(body.Bytes())
	buf.Write(pk.Payload)

	return nil
}

// Decode extracts the data values from the packet.
func (pk *PublishPacket) Decode(buf []byte) error {
	var offset int
	var err error

	pk.TopicName, offset, err = decodeString(buf, 0)
	if err != nil {
		return errors.New(ErrMalformedTopic)
	}

	// If QOS decode Packet ID.
	if pk.Qos > 0 {
		pk.PacketID, offset, err = decodeUint16(buf, offset)
		if err != nil {
			return errors.New(ErrMalformedPacketID)
		}
	}

	pk.Payload = buf[offset:]

	return nil
}

// Copy creates a new instance of PublishPacket bearing the same payload and
// destination topic, but with an empty header for inheriting new QoS etc flags.
func (pk *PublishPacket) Copy() *PublishPacket {
	cc := new(PublishPacket)
	cc.FixedHeader = NewFixedHeader(Publish)
	cc.TopicName = pk.TopicName
	cc.Payload = pk.Payload

	return cc
}

// Validate ensures the packet is compliant.
func (pk *PublishPacket) Validate() (byte, error) {

	// @SPEC [MQTT-2.3.1-1]
	// SUBSCRIBE, UNSUBSCRIBE, and PUBLISH (in cases where QoS > 0) Control Packets MUST contain a non-zero 16-bit Packet Identifier.
	if pk.FixedHeader.Qos > 0 && pk.PacketID == 0 {
		return Failed, errors.New(ErrMissingPacketID)
	}

	// @SPEC [MQTT-2.3.1-5]
	// A PUBLISH Packet MUST NOT contain a Packet Identifier if its QoS value is set to 0.
	if pk.FixedHeader.Qos == 0 && pk.PacketID > 0 {
		return Failed, errors.New(ErrSurplusPacketID)
	}

	return Accepted, nil
}
