// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package packets

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

type fixedHeaderTable struct {
	desc        string
	rawBytes    []byte
	header      FixedHeader
	packetError bool
	expect      error
}

var fixedHeaderExpected = []fixedHeaderTable{
	{
		desc:     "connect",
		rawBytes: []byte{Connect << 4, 0x00},
		header:   FixedHeader{Type: Connect, Dup: false, Qos: 0, Retain: false, Remaining: 0},
	},
	{
		desc:     "connack",
		rawBytes: []byte{Connack << 4, 0x00},
		header:   FixedHeader{Type: Connack, Dup: false, Qos: 0, Retain: false, Remaining: 0},
	},
	{
		desc:     "publish",
		rawBytes: []byte{Publish << 4, 0x00},
		header:   FixedHeader{Type: Publish, Dup: false, Qos: 0, Retain: false, Remaining: 0},
	},
	{
		desc:     "publish qos 1",
		rawBytes: []byte{Publish<<4 | 1<<1, 0x00},
		header:   FixedHeader{Type: Publish, Dup: false, Qos: 1, Retain: false, Remaining: 0},
	},
	{
		desc:     "publish qos 1 retain",
		rawBytes: []byte{Publish<<4 | 1<<1 | 1, 0x00},
		header:   FixedHeader{Type: Publish, Dup: false, Qos: 1, Retain: true, Remaining: 0},
	},
	{
		desc:     "publish qos 2",
		rawBytes: []byte{Publish<<4 | 2<<1, 0x00},
		header:   FixedHeader{Type: Publish, Dup: false, Qos: 2, Retain: false, Remaining: 0},
	},
	{
		desc:     "publish qos 2 retain",
		rawBytes: []byte{Publish<<4 | 2<<1 | 1, 0x00},
		header:   FixedHeader{Type: Publish, Dup: false, Qos: 2, Retain: true, Remaining: 0},
	},
	{
		desc:     "publish dup qos 0",
		rawBytes: []byte{Publish<<4 | 1<<3, 0x00},
		header:   FixedHeader{Type: Publish, Dup: true, Qos: 0, Retain: false, Remaining: 0},
		expect:   ErrProtocolViolationDupNoQos,
	},
	{
		desc:     "publish dup qos 0 retain",
		rawBytes: []byte{Publish<<4 | 1<<3 | 1, 0x00},
		header:   FixedHeader{Type: Publish, Dup: true, Qos: 0, Retain: true, Remaining: 0},
		expect:   ErrProtocolViolationDupNoQos,
	},
	{
		desc:     "publish dup qos 1 retain",
		rawBytes: []byte{Publish<<4 | 1<<3 | 1<<1 | 1, 0x00},
		header:   FixedHeader{Type: Publish, Dup: true, Qos: 1, Retain: true, Remaining: 0},
	},
	{
		desc:     "publish dup qos 2 retain",
		rawBytes: []byte{Publish<<4 | 1<<3 | 2<<1 | 1, 0x00},
		header:   FixedHeader{Type: Publish, Dup: true, Qos: 2, Retain: true, Remaining: 0},
	},
	{
		desc:     "puback",
		rawBytes: []byte{Puback << 4, 0x00},
		header:   FixedHeader{Type: Puback, Dup: false, Qos: 0, Retain: false, Remaining: 0},
	},
	{
		desc:     "pubrec",
		rawBytes: []byte{Pubrec << 4, 0x00},
		header:   FixedHeader{Type: Pubrec, Dup: false, Qos: 0, Retain: false, Remaining: 0},
	},
	{
		desc:     "pubrel",
		rawBytes: []byte{Pubrel<<4 | 1<<1, 0x00},
		header:   FixedHeader{Type: Pubrel, Dup: false, Qos: 1, Retain: false, Remaining: 0},
	},
	{
		desc:     "pubcomp",
		rawBytes: []byte{Pubcomp << 4, 0x00},
		header:   FixedHeader{Type: Pubcomp, Dup: false, Qos: 0, Retain: false, Remaining: 0},
	},
	{
		desc:     "subscribe",
		rawBytes: []byte{Subscribe<<4 | 1<<1, 0x00},
		header:   FixedHeader{Type: Subscribe, Dup: false, Qos: 1, Retain: false, Remaining: 0},
	},
	{
		desc:     "suback",
		rawBytes: []byte{Suback << 4, 0x00},
		header:   FixedHeader{Type: Suback, Dup: false, Qos: 0, Retain: false, Remaining: 0},
	},
	{
		desc:     "unsubscribe",
		rawBytes: []byte{Unsubscribe<<4 | 1<<1, 0x00},
		header:   FixedHeader{Type: Unsubscribe, Dup: false, Qos: 1, Retain: false, Remaining: 0},
	},
	{
		desc:     "unsuback",
		rawBytes: []byte{Unsuback << 4, 0x00},
		header:   FixedHeader{Type: Unsuback, Dup: false, Qos: 0, Retain: false, Remaining: 0},
	},
	{
		desc:     "pingreq",
		rawBytes: []byte{Pingreq << 4, 0x00},
		header:   FixedHeader{Type: Pingreq, Dup: false, Qos: 0, Retain: false, Remaining: 0},
	},
	{
		desc:     "pingresp",
		rawBytes: []byte{Pingresp << 4, 0x00},
		header:   FixedHeader{Type: Pingresp, Dup: false, Qos: 0, Retain: false, Remaining: 0},
	},
	{
		desc:     "disconnect",
		rawBytes: []byte{Disconnect << 4, 0x00},
		header:   FixedHeader{Type: Disconnect, Dup: false, Qos: 0, Retain: false, Remaining: 0},
	},
	{
		desc:     "auth",
		rawBytes: []byte{Auth << 4, 0x00},
		header:   FixedHeader{Type: Auth, Dup: false, Qos: 0, Retain: false, Remaining: 0},
	},

	// remaining length
	{
		desc:     "remaining length 10",
		rawBytes: []byte{Publish << 4, 0x0a},
		header:   FixedHeader{Type: Publish, Dup: false, Qos: 0, Retain: false, Remaining: 10},
	},
	{
		desc:     "remaining length 512",
		rawBytes: []byte{Publish << 4, 0x80, 0x04},
		header:   FixedHeader{Type: Publish, Dup: false, Qos: 0, Retain: false, Remaining: 512},
	},
	{
		desc:     "remaining length 978",
		rawBytes: []byte{Publish << 4, 0xd2, 0x07},
		header:   FixedHeader{Type: Publish, Dup: false, Qos: 0, Retain: false, Remaining: 978},
	},
	{
		desc:     "remaining length 20202",
		rawBytes: []byte{Publish << 4, 0x86, 0x9d, 0x01},
		header:   FixedHeader{Type: Publish, Dup: false, Qos: 0, Retain: false, Remaining: 20102},
	},
	{
		desc:        "remaining length oversize",
		rawBytes:    []byte{Publish << 4, 0xd5, 0x86, 0xf9, 0x9e, 0x01},
		header:      FixedHeader{Type: Publish, Dup: false, Qos: 0, Retain: false, Remaining: 333333333},
		packetError: true,
	},

	// Invalid flags for packet
	{
		desc:     "invalid type dup is true",
		rawBytes: []byte{Connect<<4 | 1<<3, 0x00},
		header:   FixedHeader{Type: Connect, Dup: true, Qos: 0, Retain: false, Remaining: 0},
		expect:   ErrMalformedFlags,
	},
	{
		desc:     "invalid type qos is 1",
		rawBytes: []byte{Connect<<4 | 1<<1, 0x00},
		header:   FixedHeader{Type: Connect, Dup: false, Qos: 1, Retain: false, Remaining: 0},
		expect:   ErrMalformedFlags,
	},
	{
		desc:     "invalid type retain is true",
		rawBytes: []byte{Connect<<4 | 1, 0x00},
		header:   FixedHeader{Type: Connect, Dup: false, Qos: 0, Retain: true, Remaining: 0},
		expect:   ErrMalformedFlags,
	},
	{
		desc:     "invalid publish qos bits 1 + 2 set",
		rawBytes: []byte{Publish<<4 | 1<<1 | 1<<2, 0x00},
		header:   FixedHeader{Type: Publish},
		expect:   ErrProtocolViolationQosOutOfRange,
	},
	{
		desc:     "invalid pubrel bits 3,2,1,0 should be 0,0,1,0",
		rawBytes: []byte{Pubrel<<4 | 1<<2 | 1<<0, 0x00},
		header:   FixedHeader{Type: Pubrel, Qos: 1},
		expect:   ErrMalformedFlags,
	},
	{
		desc:     "invalid subscribe bits 3,2,1,0 should be 0,0,1,0",
		rawBytes: []byte{Subscribe<<4 | 1<<2, 0x00},
		header:   FixedHeader{Type: Subscribe, Qos: 1},
		expect:   ErrMalformedFlags,
	},
}

func TestFixedHeaderEncode(t *testing.T) {
	for _, wanted := range fixedHeaderExpected {
		t.Run(wanted.desc, func(t *testing.T) {
			buf := new(bytes.Buffer)
			wanted.header.Encode(buf)
			if wanted.expect == nil {
				require.Equal(t, len(wanted.rawBytes), len(buf.Bytes()))
				require.EqualValues(t, wanted.rawBytes, buf.Bytes())
			}
		})
	}
}

func TestFixedHeaderDecode(t *testing.T) {
	for _, wanted := range fixedHeaderExpected {
		t.Run(wanted.desc, func(t *testing.T) {
			fh := new(FixedHeader)
			err := fh.Decode(wanted.rawBytes[0])
			if wanted.expect != nil {
				require.Equal(t, wanted.expect, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, wanted.header.Type, fh.Type)
				require.Equal(t, wanted.header.Dup, fh.Dup)
				require.Equal(t, wanted.header.Qos, fh.Qos)
				require.Equal(t, wanted.header.Retain, fh.Retain)
			}
		})
	}
}
