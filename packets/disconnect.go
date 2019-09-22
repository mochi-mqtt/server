package packets

import (
	"io"
)

// DisconnectPacket contains the values of an MQTT DISCONNECT packet.
type DisconnectPacket struct {
	FixedHeader
}

// Encode encodes and writes the packet data values to the buffer.
func (pk *DisconnectPacket) Encode(w io.Writer) error {

	out := pk.FixedHeader.encode()
	_, err := out.WriteTo(w)

	return err
}

// Decode extracts the data values from the packet.
func (pk *DisconnectPacket) Decode(buf []byte) error {
	return nil
}

// Validate ensures the packet is compliant.
func (pk *DisconnectPacket) Validate() (byte, error) {
	return Accepted, nil
}
