package packets

import (
	"bytes"
)

// PubcompPacket contains the values of an MQTT PUBCOMP packet.
type PubcompPacket struct {
	FixedHeader

	PacketID uint16
}

// Encode encodes and writes the packet data values to the buffer.
func (pk *PubcompPacket) Encode(buf *bytes.Buffer) error {
	pk.FixedHeader.Remaining = 2
	pk.FixedHeader.encode(buf)
	buf.Write(encodeUint16(pk.PacketID))
	return nil
}

// Decode extracts the data values from the packet.
func (pk *PubcompPacket) Decode(buf []byte) error {
	var err error
	pk.PacketID, _, err = decodeUint16(buf, 0)
	if err != nil {
		return ErrMalformedPacketID
	}
	return nil
}

// Validate ensures the packet is compliant.
func (pk *PubcompPacket) Validate() (byte, error) {
	return Accepted, nil
}
