package packets

import (
	"bytes"
	"errors"
	"io"
)

// SubackPacket contains the values of an MQTT SUBACK packet.
type SubackPacket struct {
	FixedHeader

	PacketID    uint16
	ReturnCodes []byte
}

// Encode encodes and writes the packet data values to the buffer.
func (pk *SubackPacket) Encode(w io.Writer) error {

	var body bytes.Buffer

	// Encode Packet ID.
	body.Write(encodeUint16(pk.PacketID))

	// Encode granted QOS flags.
	body.Write(pk.ReturnCodes)

	// Set length.
	pk.FixedHeader.Remaining = body.Len()

	// Write header and packet to output.
	out := pk.FixedHeader.encode()
	out.Write(body.Bytes())
	_, err := out.WriteTo(w)

	return err
}

// Decode extracts the data values from the packet.
func (pk *SubackPacket) Decode(buf []byte) error {

	var offset int
	var err error

	// Get Packet ID.
	pk.PacketID, offset, err = decodeUint16(buf, offset)
	if err != nil {
		return errors.New(ErrMalformedPacketID)
	}

	// Get Granted QOS flags.
	pk.ReturnCodes = buf[offset:]

	return nil
}

// Validate ensures the packet is compliant.
func (pk *SubackPacket) Validate() (byte, error) {
	return Accepted, nil
}
