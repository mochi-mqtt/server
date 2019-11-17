package packets

import (
	"bytes"
)

// FixedHeader contains the values of the fixed header portion of the MQTT packet.
type FixedHeader struct {

	// Type is the type of the packet (PUBLISH, SUBSCRIBE, etc) from bits 7 - 4 (byte 1).
	Type byte

	// Dup indicates if the packet is a duplicate.
	Dup bool

	// Qos byte indicates the quality of service expected.
	Qos byte

	// Retain indicates whether the message should be retained.
	Retain bool

	// Remaining is the number of remaining bytes in the payload.
	Remaining int
}

// encode encodes the FixedHeader and returns a bytes buffer.
func (fh *FixedHeader) Encode(buf *bytes.Buffer) {
	buf.WriteByte(fh.Type<<4 | encodeBool(fh.Dup)<<3 | fh.Qos<<1 | encodeBool(fh.Retain))
	encodeLength(buf, fh.Remaining)
}

// decode extracts the specification bits from the header byte.
func (fh *FixedHeader) Decode(headerByte byte) error {

	// Get the message type from the first 4 bytes.
	fh.Type = headerByte >> 4

	// @SPEC [MQTT-2.2.2-1]
	// Where a flag bit is marked as “Reserved” in Table 2.2 - Flag Bits,
	// it is reserved for future use and MUST be set to the value listed in that table.
	switch fh.Type {

	case Publish:
		fh.Dup = (headerByte>>3)&0x01 > 0 // Extract flags. Check if message is duplicate.
		fh.Qos = (headerByte >> 1) & 0x03 // Extract QoS flag.
		fh.Retain = headerByte&0x01 > 0   // Extract retain flag.

	case Pubrel:
		fh.Qos = (headerByte >> 1) & 0x03

	case Subscribe:
		fh.Qos = (headerByte >> 1) & 0x03

	case Unsubscribe:
		fh.Qos = (headerByte >> 1) & 0x03

	default:

		// [MQTT-2.2.2-2]
		// If invalid flags are received, the receiver MUST close the Network Connection.
		if (headerByte>>3)&0x01 > 0 || (headerByte>>1)&0x03 > 0 || headerByte&0x01 > 0 {
			return ErrInvalidFlags
		}
	}

	return nil

}

// encodeLength writes length bits for the header.
func encodeLength(buf *bytes.Buffer, length int) {
	for {
		digit := byte(length % 128)
		length /= 128
		if length > 0 {
			digit |= 0x80
		}
		buf.WriteByte(digit)
		if length == 0 {
			break
		}
	}
}
