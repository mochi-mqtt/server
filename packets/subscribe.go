package packets

import (
	"bytes"
	"errors"
	"io"
)

// SubscribePacket contains the values of an MQTT SUBSCRIBE packet.
type SubscribePacket struct {
	FixedHeader

	PacketID uint16
	Topics   []string
	Qoss     []byte
}

// Encode encodes and writes the packet data values to the buffer.
func (pk *SubscribePacket) Encode(w io.Writer) error {

	var body bytes.Buffer

	// Add the Packet ID.
	// [MQTT-2.3.1-1] SUBSCRIBE, UNSUBSCRIBE, and PUBLISH (in cases where QoS > 0) Control Packets MUST contain a non-zero 16-bit Packet Identifier.
	if pk.PacketID == 0 {
		return errors.New(ErrMissingPacketID)
	}

	body.Write(encodeUint16(pk.PacketID))

	// Add all provided topic names and associated QOS flags.
	for i, topic := range pk.Topics {
		body.Write(encodeString(topic))
		body.WriteByte(pk.Qoss[i])
	}

	// Set length.
	pk.FixedHeader.Remaining = body.Len()

	// Write header and packet to output.
	out := pk.FixedHeader.encode()
	out.Write(body.Bytes())
	_, err := out.WriteTo(w)

	return err

}

// Decode extracts the data values from the packet.
func (pk *SubscribePacket) Decode(buf []byte) error {
	var offset int
	var err error

	// Get the Packet ID.
	pk.PacketID, offset, err = decodeUint16(buf, 0)
	if err != nil {
		return errors.New(ErrMalformedPacketID)
	}

	// Keep decoding until there's no space left.
	for offset < len(buf) {

		// Decode Topic Name.
		var topic string
		topic, offset, err = decodeString(buf, offset)
		if err != nil {
			return errors.New(ErrMalformedTopic)
		}
		pk.Topics = append(pk.Topics, topic)

		// Decode QOS flag.
		var qos byte
		qos, offset, err = decodeByte(buf, offset)
		if err != nil {
			return errors.New(ErrMalformedQoS)
		}

		if !validateQoS(qos) {
			return errors.New(ErrMalformedQoS)
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
		return Failed, errors.New(ErrMissingPacketID)
	}

	return Accepted, nil
}
