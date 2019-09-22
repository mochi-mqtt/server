package packets

import (
	"bytes"
	"errors"
)

// SubackPacket contains the values of an MQTT SUBACK packet.
type SubackPacket struct {
	FixedHeader

	PacketID    uint16
	ReturnCodes []byte
}

// Encode encodes and writes the packet data values to the buffer.
func (pk *SubackPacket) Encode(buf *bytes.Buffer) error {
	pk.FixedHeader.encode(buf)

	bodyLen := buf.Len()

	buf.Write(encodeUint16(pk.PacketID)) // Encode Packet ID.
	buf.Write(pk.ReturnCodes)            // Encode granted QOS flags.

	pk.FixedHeader.Remaining = buf.Len() - bodyLen // Set length.

	return nil
}

// Decode extracts the data values from the packet.
func (pk *SubackPacket) Decode(buf []byte) error {

	var offset int
	var err error

	// Get Packet ID.
	pk.PacketID, offset, err = decodeUint16(buf, offset)
	if err != nil {
		return errors.New(ErrMalformedPacketID)
	}

	// Get Granted QOS flags.
	pk.ReturnCodes = buf[offset:]

	return nil
}

// Validate ensures the packet is compliant.
func (pk *SubackPacket) Validate() (byte, error) {
	return Accepted, nil
}
