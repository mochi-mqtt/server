package packets

import (
	"bytes"
)

// SubackPacket contains the values of an MQTT SUBACK packet.
type SubackPacket struct {
	FixedHeader

	PacketID    uint16
	ReturnCodes []byte
}

// Encode encodes and writes the packet data values to the buffer.
func (pk *SubackPacket) Encode(buf *bytes.Buffer) error {

	packetID := encodeUint16(pk.PacketID)
	pk.FixedHeader.Remaining = len(packetID) + len(pk.ReturnCodes) // Set length.
	pk.FixedHeader.encode(buf)

	buf.Write(packetID)       // Encode Packet ID.
	buf.Write(pk.ReturnCodes) // Encode granted QOS flags.

	return nil
}

// Decode extracts the data values from the packet.
func (pk *SubackPacket) Decode(buf []byte) error {
	var offset int
	var err error

	// Get Packet ID.
	pk.PacketID, offset, err = decodeUint16(buf, offset)
	if err != nil {
		return ErrMalformedPacketID
	}

	// Get Granted QOS flags.
	pk.ReturnCodes = buf[offset:]

	return nil
}

// Validate ensures the packet is compliant.
func (pk *SubackPacket) Validate() (byte, error) {
	return Accepted, nil
}
