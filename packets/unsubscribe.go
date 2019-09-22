package packets

import (
	"bytes"
	"errors"
	"io"
)

// UnsubscribePacket contains the values of an MQTT UNSUBSCRIBE packet.
type UnsubscribePacket struct {
	FixedHeader

	PacketID uint16
	Topics   []string
}

// Encode encodes and writes the packet data values to the buffer.
func (pk *UnsubscribePacket) Encode(w io.Writer) error {

	var body bytes.Buffer

	// Add the Packet ID.
	// [MQTT-2.3.1-1] SUBSCRIBE, UNSUBSCRIBE, and PUBLISH (in cases where QoS > 0) Control Packets MUST contain a non-zero 16-bit Packet Identifier.
	if pk.PacketID == 0 {
		return errors.New(ErrMissingPacketID)
	}

	body.Write(encodeUint16(pk.PacketID))

	// Add all provided topic names and associated QOS flags.
	for _, topic := range pk.Topics {
		body.Write(encodeString(topic))
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
func (pk *UnsubscribePacket) Decode(buf []byte) error {

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
		var t string
		t, offset, err = decodeString(buf, offset)
		if err != nil {
			return errors.New(ErrMalformedTopic)
		}

		if t != "" {
			pk.Topics = append(pk.Topics, t)
		}

	}

	return nil

}

// Validate ensures the packet is compliant.
func (pk *UnsubscribePacket) Validate() (byte, error) {

	// @SPEC [MQTT-2.3.1-1].
	// SUBSCRIBE, UNSUBSCRIBE, and PUBLISH (in cases where QoS > 0) Control Packets MUST contain a non-zero 16-bit Packet Identifier.
	if pk.FixedHeader.Qos > 0 && pk.PacketID == 0 {
		return Failed, errors.New(ErrMissingPacketID)
	}

	return Accepted, nil
}
