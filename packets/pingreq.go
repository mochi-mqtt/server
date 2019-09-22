package packets

import (
	"io"
)

// PingreqPacket contains the values of an MQTT PINGREQ packet.
type PingreqPacket struct {
	FixedHeader
}

// Encode encodes and writes the packet data values to the buffer.
func (pk *PingreqPacket) Encode(w io.Writer) error {

	out := pk.FixedHeader.encode()
	_, err := out.WriteTo(w)

	return err
}

// Decode extracts the data values from the packet.
func (pk *PingreqPacket) Decode(buf []byte) error {
	return nil
}

// Validate ensures the packet is compliant.
func (pk *PingreqPacket) Validate() (byte, error) {
	return Accepted, nil
}
