package packets

import (
	"bytes"
	"errors"
	"io"
)

// ConnackPacket contains the values of an MQTT CONNACK packet.
type ConnackPacket struct {
	FixedHeader

	SessionPresent bool
	ReturnCode     byte
}

// Encode encodes and writes the packet data values to the buffer.
func (pk *ConnackPacket) Encode(w io.Writer) error {
	var body bytes.Buffer

	// Write flags to packet body.
	body.WriteByte(encodeBool(pk.SessionPresent))
	body.WriteByte(pk.ReturnCode)

	// Increase remaining length.
	pk.FixedHeader.Remaining = 2

	// Write header and packet to output.
	out := pk.FixedHeader.encode()
	out.Write(body.Bytes())

	_, err := out.WriteTo(w)

	return err

}

// Decode extracts the data values from the packet.
func (pk *ConnackPacket) Decode(buf []byte) error {

	var offset int
	var err error

	// Unpack session present flag.
	pk.SessionPresent, offset, err = decodeByteBool(buf, 0)
	if err != nil {
		return errors.New(ErrMalformedSessionPresent)
	}

	// Unpack return code.
	pk.ReturnCode, offset, err = decodeByte(buf, offset)
	if err != nil {
		return errors.New(ErrMalformedReturnCode)
	}

	return nil

}

// Validate ensures the packet is compliant.
func (pk *ConnackPacket) Validate() (byte, error) {
	return Accepted, nil
}
