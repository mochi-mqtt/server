package packets

import (
	"bytes"
)

// FixedHeader contains the values of the fixed header portion of the MQTT packet.
type FixedHeader struct {
	Remaining int  // the number of remaining bytes in the payload.
	Type      byte // the type of the packet (PUBLISH, SUBSCRIBE, etc) from bits 7 - 4 (byte 1).
	Qos       byte // indicates the quality of service expected.
	Dup       bool // indicates if the packet was already sent at an earlier time.
	Retain    bool // whether the message should be retained.
}

// Encode encodes the FixedHeader and returns a bytes buffer.
func (fh *FixedHeader) Encode(buf *bytes.Buffer) {
	buf.WriteByte(fh.Type<<4 | encodeBool(fh.Dup)<<3 | fh.Qos<<1 | encodeBool(fh.Retain))
	encodeLength(buf, int64(fh.Remaining))
}

// Decode extracts the specification bits from the header byte.
func (fh *FixedHeader) Decode(headerByte byte) error {
	fh.Type = headerByte >> 4 // Get the message type from the first 4 bytes.

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
		if (headerByte>>3)&0x01 > 0 || (headerByte>>1)&0x03 > 0 || headerByte&0x01 > 0 {
			return ErrInvalidFlags
		}
	}

	return nil
}

// encodeLength writes length bits for the header.
func encodeLength(buf *bytes.Buffer, length int64) {
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
