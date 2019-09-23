package packets

import (
	"bytes"
)

// SubscribePacket contains the values of an MQTT SUBSCRIBE packet.
type SubscribePacket struct {
	FixedHeader

	PacketID uint16
	Topics   []string
	Qoss     []byte
}

// Encode encodes and writes the packet data values to the buffer.
func (pk *SubscribePacket) Encode(buf *bytes.Buffer) error {

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
	pk.FixedHeader.encode(buf)
	buf.Write(packetID)

	// Add all provided topic names and associated QOS flags.
	for i, topic := range pk.Topics {
		buf.Write(encodeString(topic))
		buf.WriteByte(pk.Qoss[i])
	}

	return nil
}

// Decode extracts the data values from the packet.
func (pk *SubscribePacket) Decode(buf []byte) error {
	var offset int
	var err error

	// Get the Packet ID.
	pk.PacketID, offset, err = decodeUint16(buf, 0)
	if err != nil {
		return ErrMalformedPacketID
	}

	// Keep decoding until there's no space left.
	for offset < len(buf) {

		// Decode Topic Name.
		var topic string
		topic, offset, err = decodeString(buf, offset)
		if err != nil {
			return ErrMalformedTopic
		}
		pk.Topics = append(pk.Topics, topic)

		// Decode QOS flag.
		var qos byte
		qos, offset, err = decodeByte(buf, offset)
		if err != nil {
			return ErrMalformedQoS
		}

		if !validateQoS(qos) {
			return ErrMalformedQoS
		}

		pk.Qoss = append(pk.Qoss, qos)
	}

	return nil
}

// Validate ensures the packet is compliant.
func (pk *SubscribePacket) Validate() (byte, error) {

	// @SPEC [MQTT-2.3.1-1].
	// SUBSCRIBE, UNSUBSCRIBE, and PUBLISH (in cases where QoS > 0) Control Packets MUST contain a non-zero 16-bit Packet Identifier.
	if pk.FixedHeader.Qos > 0 && pk.PacketID == 0 {
		return Failed, ErrMissingPacketID
	}

	return Accepted, nil
}
