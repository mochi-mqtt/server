package packets

import (
	"bytes"
)

// PingreqPacket contains the values of an MQTT PINGREQ packet.
type PingreqPacket struct {
	FixedHeader
}

// Encode encodes and writes the packet data values to the buffer.
func (pk *PingreqPacket) Encode(buf *bytes.Buffer) error {
	pk.FixedHeader.Encode(buf)
	return nil
}

// Decode extracts the data values from the packet.
func (pk *PingreqPacket) Decode(buf []byte) error {
	return nil
}

// Validate ensures the packet is compliant.
func (pk *PingreqPacket) Validate() (byte, error) {
	return Accepted, nil
}
