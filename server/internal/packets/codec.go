package packets

import (
	"encoding/binary"
	"unicode/utf8"
	"unsafe"
)

// bytesToString provides a zero-alloc, no-copy byte to string conversion.
// via https://github.com/golang/go/issues/25484#issuecomment-391415660
func bytesToString(bs []byte) string {
	return *(*string)(unsafe.Pointer(&bs))
}

// decodeUint16 extracts the value of two bytes from a byte array.
func decodeUint16(buf []byte, offset int) (uint16, int, error) {
	if len(buf) < offset+2 {
		return 0, 0, ErrOffsetUintOutOfRange
	}

	return binary.BigEndian.Uint16(buf[offset : offset+2]), offset + 2, nil
}

// decodeString extracts a string from a byte array, beginning at an offset.
func decodeString(buf []byte, offset int) (string, int, error) {
	b, n, err := decodeBytes(buf, offset)
	if err != nil {
		return "", 0, err
	}

	if !validUTF8(b) {
		return "", 0, ErrOffsetStrInvalidUTF8
	}

	return bytesToString(b), n, nil
}

// decodeBytes extracts a byte array from a byte array, beginning at an offset. Used primarily for message payloads.
func decodeBytes(buf []byte, offset int) ([]byte, int, error) {
	length, next, err := decodeUint16(buf, offset)
	if err != nil {
		return make([]byte, 0, 0), 0, err
	}

	if next+int(length) > len(buf) {
		return make([]byte, 0, 0), 0, ErrOffsetBytesOutOfRange
	}

	// Note: there is no validUTF8() test for []byte payloads

	return buf[next : next+int(length)], next + int(length), nil
}

// decodeByte extracts the value of a byte from a byte array.
func decodeByte(buf []byte, offset int) (byte, int, error) {
	if len(buf) <= offset {
		return 0, 0, ErrOffsetByteOutOfRange
	}
	return buf[offset], offset + 1, nil
}

// decodeByteBool extracts the value of a byte from a byte array and returns a bool.
func decodeByteBool(buf []byte, offset int) (bool, int, error) {
	if len(buf) <= offset {
		return false, 0, ErrOffsetBoolOutOfRange
	}
	return 1&buf[offset] > 0, offset + 1, nil
}

// encodeBool returns a byte instead of a bool.
func encodeBool(b bool) byte {
	if b {
		return 1
	}
	return 0
}

// encodeBytes encodes a byte array to a byte array. Used primarily for message payloads.
func encodeBytes(val []byte) []byte {
	// In many circumstances the number of bytes being encoded is small.
	// Setting the cap to a low amount allows us to account for those without
	// triggering allocation growth on append unless we need to.
	buf := make([]byte, 2, 32)
	binary.BigEndian.PutUint16(buf, uint16(len(val)))
	return append(buf, val...)
}

// encodeUint16 encodes a uint16 value to a byte array.
func encodeUint16(val uint16) []byte {
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, val)
	return buf
}

// encodeString encodes a string to a byte array.
func encodeString(val string) []byte {
	// Like encodeBytes, we set the cap to a small number to avoid
	// triggering allocation growth on append unless we absolutely need to.
	buf := make([]byte, 2, 32)
	binary.BigEndian.PutUint16(buf, uint16(len(val)))
	return append(buf, []byte(val)...)
}

// validUTF8 checks if the byte array contains valid UTF-8 characters, specifically
// conforming to the MQTT specification requirements.
func validUTF8(b []byte) bool {
	// [MQTT-1.4.0-1] The character data in a UTF-8 encoded string MUST be well-formed UTF-8...
	if !utf8.Valid(b) {
		return false
	}

	// [MQTT-1.4.0-2] A UTF-8 encoded string MUST NOT include an encoding of the null character U+0000...
	// ...
	return true

}
