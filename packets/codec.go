// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-co
// SPDX-FileContributor: mochi-co

package packets

import (
	"bytes"
	"encoding/binary"
	"io"
	"unicode/utf8"
	"unsafe"
)

// bytesToString provides a zero-alloc no-copy byte to string conversion.
// via https://github.com/golang/go/issues/25484#issuecomment-391415660
func bytesToString(bs []byte) string {
	return *(*string)(unsafe.Pointer(&bs))
}

// decodeUint16 extracts the value of two bytes from a byte array.
func decodeUint16(buf []byte, offset int) (uint16, int, error) {
	if len(buf) < offset+2 {
		return 0, 0, ErrMalformedOffsetUintOutOfRange
	}

	return binary.BigEndian.Uint16(buf[offset : offset+2]), offset + 2, nil
}

// decodeUint32 extracts the value of four bytes from a byte array.
func decodeUint32(buf []byte, offset int) (uint32, int, error) {
	if len(buf) < offset+4 {
		return 0, 0, ErrMalformedOffsetUintOutOfRange
	}

	return binary.BigEndian.Uint32(buf[offset : offset+4]), offset + 4, nil
}

// decodeString extracts a string from a byte array, beginning at an offset.
func decodeString(buf []byte, offset int) (string, int, error) {
	b, n, err := decodeBytes(buf, offset)
	if err != nil {
		return "", 0, err
	}

	if !validUTF8(b) { // [MQTT-1.5.4-1] [MQTT-3.1.3-5]
		return "", 0, ErrMalformedInvalidUTF8
	}

	return bytesToString(b), n, nil
}

// validUTF8 checks if the byte array contains valid UTF-8 characters.
func validUTF8(b []byte) bool {
	return utf8.Valid(b) && bytes.IndexByte(b, 0x00) == -1 // [MQTT-1.5.4-1] [MQTT-1.5.4-2]
}

// decodeBytes extracts a byte array from a byte array, beginning at an offset. Used primarily for message payloads.
func decodeBytes(buf []byte, offset int) ([]byte, int, error) {
	length, next, err := decodeUint16(buf, offset)
	if err != nil {
		return make([]byte, 0), 0, err
	}

	if next+int(length) > len(buf) {
		return make([]byte, 0), 0, ErrMalformedOffsetBytesOutOfRange
	}

	return buf[next : next+int(length)], next + int(length), nil
}

// decodeByte extracts the value of a byte from a byte array.
func decodeByte(buf []byte, offset int) (byte, int, error) {
	if len(buf) <= offset {
		return 0, 0, ErrMalformedOffsetByteOutOfRange
	}
	return buf[offset], offset + 1, nil
}

// decodeByteBool extracts the value of a byte from a byte array and returns a bool.
func decodeByteBool(buf []byte, offset int) (bool, int, error) {
	if len(buf) <= offset {
		return false, 0, ErrMalformedOffsetBoolOutOfRange
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
	// In most circumstances the number of bytes being encoded is small.
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

// encodeUint32 encodes a uint16 value to a byte array.
func encodeUint32(val uint32) []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, val)
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

// encodeLength writes length bits for the header.
func encodeLength(b *bytes.Buffer, length int64) {
	// 1.5.5 Variable Byte Integer encode non-normative
	// https://docs.oasis-open.org/mqtt/mqtt/v5.0/os/mqtt-v5.0-os.html#_Toc3901027
	for {
		eb := byte(length % 128)
		length /= 128
		if length > 0 {
			eb |= 0x80
		}
		b.WriteByte(eb)
		if length == 0 {
			break // [MQTT-1.5.5-1]
		}
	}
}

func DecodeLength(b io.ByteReader) (n, bu int, err error) {
	// see 1.5.5 Variable Byte Integer decode non-normative
	// https://docs.oasis-open.org/mqtt/mqtt/v5.0/os/mqtt-v5.0-os.html#_Toc3901027
	var multiplier uint32
	var value uint32
	bu = 1
	for {
		eb, err := b.ReadByte()
		if err != nil {
			return 0, bu, err
		}

		value |= uint32(eb&127) << multiplier
		if value > 268435455 {
			return 0, bu, ErrMalformedVariableByteInteger
		}

		if (eb & 128) == 0 {
			break
		}

		multiplier += 7
		bu++
	}

	return int(value), bu, nil
}
