package packets

import (
	"bytes"
	"errors"
	"io"
)

// UnsubackPacket contains the values of an MQTT UNSUBACK packet.
type UnsubackPacket struct {
	FixedHeader

	PacketID uint16
}

// Encode encodes and writes the packet data values to the buffer.
func (pk *UnsubackPacket) Encode(w io.Writer) error {

	var body bytes.Buffer

	// Add the Packet ID.
	body.Write(encodeUint16(pk.PacketID))
	pk.Remaining = 2

	// Write header and packet to output.
	out := pk.FixedHeader.encode()
	out.Write(body.Bytes())
	_, err := out.WriteTo(w)

	return err

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
