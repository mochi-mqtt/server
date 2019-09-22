package packets

import (
	"bytes"
	"errors"
)

// PubackPacket contains the values of an MQTT PUBACK packet.
type PubackPacket struct {
	FixedHeader

	PacketID uint16
}

// Encode encodes and writes the packet data values to the buffer.
func (pk *PubackPacket) Encode(buf *bytes.Buffer) error {
	pk.FixedHeader.Remaining = 2
	pk.FixedHeader.encode(buf)
	buf.Write(encodeUint16(pk.PacketID))
	return nil
}

// Decode extracts the data values from the packet.
func (pk *PubackPacket) Decode(buf []byte) error {

	var err error

	pk.PacketID, _, err = decodeUint16(buf, 0)
	if err != nil {
		return errors.New(ErrMalformedPacketID)
	}

	return nil
}

// Validate ensures the packet is compliant.
func (pk *PubackPacket) Validate() (byte, error) {
	return Accepted, nil
}
