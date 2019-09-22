package packets

import (
	"io"
)

// PingrespPacket contains the values of an MQTT PINGRESP packet.
type PingrespPacket struct {
	FixedHeader
}

// Encode encodes and writes the packet data values to the buffer.
func (pk *PingrespPacket) Encode(w io.Writer) error {

	out := pk.FixedHeader.encode()
	_, err := out.WriteTo(w)

	return err
}

// Decode extracts the data values from the packet.
func (pk *PingrespPacket) Decode(buf []byte) error {
	return nil
}

// Validate ensures the packet is compliant.
func (pk *PingrespPacket) Validate() (byte, error) {
	return Accepted, nil
}
