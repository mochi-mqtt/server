package packets

import (
	"bytes"
	"errors"
	"io"
)

// PubrelPacket contains the values of an MQTT PUBREL packet.
type PubrelPacket struct {
	FixedHeader

	PacketID uint16
}

// Encode encodes and writes the packet data values to the buffer.
func (pk *PubrelPacket) Encode(w io.Writer) error {

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
func (pk *PubrelPacket) Decode(buf []byte) error {

	var err error

	pk.PacketID, _, err = decodeUint16(buf, 0)
	if err != nil {
		return errors.New(ErrMalformedPacketID)
	}

	return nil
}

// Validate ensures the packet is compliant.
func (pk *PubrelPacket) Validate() (byte, error) {
	return Accepted, nil
}
