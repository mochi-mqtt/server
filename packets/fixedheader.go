// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package packets

import (
	"bytes"
)

// FixedHeader contains the values of the fixed header portion of the MQTT packet.
type FixedHeader struct {
	Remaining int  `json:"remaining"` // the number of remaining bytes in the payload.
	Type      byte `json:"type"`      // the type of the packet (PUBLISH, SUBSCRIBE, etc) from bits 7 - 4 (byte 1).
	Qos       byte `json:"qos"`       // indicates the quality of service expected.
	Dup       bool `json:"dup"`       // indicates if the packet was already sent at an earlier time.
	Retain    bool `json:"retain"`    // whether the message should be retained.
}

// Encode encodes the FixedHeader and returns a bytes buffer.
func (fh *FixedHeader) Encode(buf *bytes.Buffer) {
	buf.WriteByte(fh.Type<<4 | encodeBool(fh.Dup)<<3 | fh.Qos<<1 | encodeBool(fh.Retain))
	encodeLength(buf, int64(fh.Remaining))
}

// Decode extracts the specification bits from the header byte.
func (fh *FixedHeader) Decode(hb byte) error {
	fh.Type = hb >> 4 // Get the message type from the first 4 bytes.

	switch fh.Type {
	case Publish:
		if (hb>>1)&0x01 > 0 && (hb>>1)&0x02 > 0 {
			return ErrProtocolViolationQosOutOfRange // [MQTT-3.3.1-4]
		}

		fh.Dup = (hb>>3)&0x01 > 0 // is duplicate
		fh.Qos = (hb >> 1) & 0x03 // qos flag
		fh.Retain = hb&0x01 > 0   // is retain flag
	case Pubrel:
		fallthrough
	case Subscribe:
		fallthrough
	case Unsubscribe:
		if (hb>>0)&0x01 != 0 || (hb>>1)&0x01 != 1 || (hb>>2)&0x01 != 0 || (hb>>3)&0x01 != 0 { // [MQTT-3.8.1-1] [MQTT-3.10.1-1]
			return ErrMalformedFlags
		}

		fh.Qos = (hb >> 1) & 0x03
	default:
		if (hb>>0)&0x01 != 0 ||
			(hb>>1)&0x01 != 0 ||
			(hb>>2)&0x01 != 0 ||
			(hb>>3)&0x01 != 0 { // [MQTT-3.8.3-5] [MQTT-3.14.1-1] [MQTT-3.15.1-1]
			return ErrMalformedFlags
		}
	}

	if fh.Qos == 0 && fh.Dup {
		return ErrProtocolViolationDupNoQos // [MQTT-3.3.1-2]
	}

	return nil
}
