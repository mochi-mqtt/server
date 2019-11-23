package packets

import (
	"bytes"
)

// ConnackPacket contains the values of an MQTT CONNACK packet.
type ConnackPacket struct {
	FixedHeader

	SessionPresent bool
	ReturnCode     byte
}

// Encode encodes and writes the packet data values to the buffer.
func (pk *ConnackPacket) Encode(buf *bytes.Buffer) error {
	pk.FixedHeader.Remaining = 2
	pk.FixedHeader.Encode(buf)
	buf.WriteByte(encodeBool(pk.SessionPresent))
	buf.WriteByte(pk.ReturnCode)

	return nil
}

// Decode extracts the data values from the packet.
func (pk *ConnackPacket) Decode(buf []byte) error {

	var offset int
	var err error

	// Unpack session present flag.
	pk.SessionPresent, offset, err = decodeByteBool(buf, 0)
	if err != nil {
		return ErrMalformedSessionPresent
	}

	// Unpack return code.
	pk.ReturnCode, offset, err = decodeByte(buf, offset)
	if err != nil {
		return ErrMalformedReturnCode
	}

	return nil

}

// Validate ensures the packet is compliant.
func (pk *ConnackPacket) Validate() (byte, error) {
	return Accepted, nil
}
