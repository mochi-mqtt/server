package packets

import (
	"bytes"
	"errors"
)

// UnsubackPacket contains the values of an MQTT UNSUBACK packet.
type UnsubackPacket struct {
	FixedHeader

	PacketID uint16
}

// Encode encodes and writes the packet data values to the buffer.
func (pk *UnsubackPacket) Encode(buf *bytes.Buffer) error {
	pk.FixedHeader.Remaining = 2
	pk.FixedHeader.encode(buf)
	buf.Write(encodeUint16(pk.PacketID))
	return nil
}

// Decode extracts the data values from the packet.
func (pk *UnsubackPacket) Decode(buf []byte) error {

	var err error

	pk.PacketID, _, err = decodeUint16(buf, 0)
	if err != nil {
		return errors.New(ErrMalformedPacketID)
	}

	return nil
}

// Validate ensures the packet is compliant.
func (pk *UnsubackPacket) Validate() (byte, error) {
	return Accepted, nil
}
