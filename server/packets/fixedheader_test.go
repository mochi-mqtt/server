package packets

import (
	"bytes"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

type fixedHeaderTable struct {
	rawBytes    []byte
	header      FixedHeader
	packetError bool
	flagError   bool
}

var fixedHeaderExpected = []fixedHeaderTable{
	{
		rawBytes: []byte{Connect << 4, 0x00},
		header:   FixedHeader{Type: Connect, Dup: false, Qos: 0, Retain: false, Remaining: 0},
	},
	{
		rawBytes: []byte{Connack << 4, 0x00},
		header:   FixedHeader{Type: Connack, Dup: false, Qos: 0, Retain: false, Remaining: 0},
	},
	{
		rawBytes: []byte{Publish << 4, 0x00},
		header:   FixedHeader{Type: Publish, Dup: false, Qos: 0, Retain: false, Remaining: 0},
	},
	{
		rawBytes: []byte{Publish<<4 | 1<<1, 0x00},
		header:   FixedHeader{Type: Publish, Dup: false, Qos: 1, Retain: false, Remaining: 0},
	},
	{
		rawBytes: []byte{Publish<<4 | 1<<1 | 1, 0x00},
		header:   FixedHeader{Type: Publish, Dup: false, Qos: 1, Retain: true, Remaining: 0},
	},
	{
		rawBytes: []byte{Publish<<4 | 2<<1, 0x00},
		header:   FixedHeader{Type: Publish, Dup: false, Qos: 2, Retain: false, Remaining: 0},
	},
	{
		rawBytes: []byte{Publish<<4 | 2<<1 | 1, 0x00},
		header:   FixedHeader{Type: Publish, Dup: false, Qos: 2, Retain: true, Remaining: 0},
	},
	{
		rawBytes: []byte{Publish<<4 | 1<<3, 0x00},
		header:   FixedHeader{Type: Publish, Dup: true, Qos: 0, Retain: false, Remaining: 0},
	},
	{
		rawBytes: []byte{Publish<<4 | 1<<3 | 1, 0x00},
		header:   FixedHeader{Type: Publish, Dup: true, Qos: 0, Retain: true, Remaining: 0},
	},
	{
		rawBytes: []byte{Publish<<4 | 1<<3 | 1<<1 | 1, 0x00},
		header:   FixedHeader{Type: Publish, Dup: true, Qos: 1, Retain: true, Remaining: 0},
	},
	{
		rawBytes: []byte{Publish<<4 | 1<<3 | 2<<1 | 1, 0x00},
		header:   FixedHeader{Type: Publish, Dup: true, Qos: 2, Retain: true, Remaining: 0},
	},
	{
		rawBytes: []byte{Puback << 4, 0x00},
		header:   FixedHeader{Type: Puback, Dup: false, Qos: 0, Retain: false, Remaining: 0},
	},
	{
		rawBytes: []byte{Pubrec << 4, 0x00},
		header:   FixedHeader{Type: Pubrec, Dup: false, Qos: 0, Retain: false, Remaining: 0},
	},
	{
		rawBytes: []byte{Pubrel<<4 | 1<<1, 0x00},
		header:   FixedHeader{Type: Pubrel, Dup: false, Qos: 1, Retain: false, Remaining: 0},
	},
	{
		rawBytes: []byte{Pubcomp << 4, 0x00},
		header:   FixedHeader{Type: Pubcomp, Dup: false, Qos: 0, Retain: false, Remaining: 0},
	},
	{
		rawBytes: []byte{Subscribe<<4 | 1<<1, 0x00},
		header:   FixedHeader{Type: Subscribe, Dup: false, Qos: 1, Retain: false, Remaining: 0},
	},
	{
		rawBytes: []byte{Suback << 4, 0x00},
		header:   FixedHeader{Type: Suback, Dup: false, Qos: 0, Retain: false, Remaining: 0},
	},
	{
		rawBytes: []byte{Unsubscribe<<4 | 1<<1, 0x00},
		header:   FixedHeader{Type: Unsubscribe, Dup: false, Qos: 1, Retain: false, Remaining: 0},
	},
	{
		rawBytes: []byte{Unsuback << 4, 0x00},
		header:   FixedHeader{Type: Unsuback, Dup: false, Qos: 0, Retain: false, Remaining: 0},
	},
	{
		rawBytes: []byte{Pingreq << 4, 0x00},
		header:   FixedHeader{Type: Pingreq, Dup: false, Qos: 0, Retain: false, Remaining: 0},
	},
	{
		rawBytes: []byte{Pingresp << 4, 0x00},
		header:   FixedHeader{Type: Pingresp, Dup: false, Qos: 0, Retain: false, Remaining: 0},
	},
	{
		rawBytes: []byte{Disconnect << 4, 0x00},
		header:   FixedHeader{Type: Disconnect, Dup: false, Qos: 0, Retain: false, Remaining: 0},
	},

	// remaining length
	{
		rawBytes: []byte{Publish << 4, 0x0a},
		header:   FixedHeader{Type: Publish, Dup: false, Qos: 0, Retain: false, Remaining: 10},
	},
	{
		rawBytes: []byte{Publish << 4, 0x80, 0x04},
		header:   FixedHeader{Type: Publish, Dup: false, Qos: 0, Retain: false, Remaining: 512},
	},
	{
		rawBytes: []byte{Publish << 4, 0xd2, 0x07},
		header:   FixedHeader{Type: Publish, Dup: false, Qos: 0, Retain: false, Remaining: 978},
	},
	{
		rawBytes: []byte{Publish << 4, 0x86, 0x9d, 0x01},
		header:   FixedHeader{Type: Publish, Dup: false, Qos: 0, Retain: false, Remaining: 20102},
	},
	{
		rawBytes:    []byte{Publish << 4, 0xd5, 0x86, 0xf9, 0x9e, 0x01},
		header:      FixedHeader{Type: Publish, Dup: false, Qos: 0, Retain: false, Remaining: 333333333},
		packetError: true,
	},

	// Invalid flags for packet
	{
		rawBytes:  []byte{Connect<<4 | 1<<3, 0x00},
		header:    FixedHeader{Type: Connect, Dup: true, Qos: 0, Retain: false, Remaining: 0},
		flagError: true,
	},
	{
		rawBytes:  []byte{Connect<<4 | 1<<1, 0x00},
		header:    FixedHeader{Type: Connect, Dup: false, Qos: 1, Retain: false, Remaining: 0},
		flagError: true,
	},
	{
		rawBytes:  []byte{Connect<<4 | 1, 0x00},
		header:    FixedHeader{Type: Connect, Dup: false, Qos: 0, Retain: true, Remaining: 0},
		flagError: true,
	},
}

func TestFixedHeaderEncode(t *testing.T) {
	for i, wanted := range fixedHeaderExpected {
		buf := new(bytes.Buffer)
		wanted.header.Encode(buf)
		if wanted.flagError == false {
			require.Equal(t, len(wanted.rawBytes), len(buf.Bytes()), "Mismatched fixedheader length [i:%d] %v", i, wanted.rawBytes)
			require.EqualValues(t, wanted.rawBytes, buf.Bytes(), "Mismatched byte values [i:%d] %v", i, wanted.rawBytes)
		}
	}
}

func BenchmarkFixedHeaderEncode(b *testing.B) {
	buf := new(bytes.Buffer)
	for n := 0; n < b.N; n++ {
		fixedHeaderExpected[0].header.Encode(buf)
	}
}

func TestFixedHeaderDecode(t *testing.T) {
	for i, wanted := range fixedHeaderExpected {
		fh := new(FixedHeader)
		err := fh.Decode(wanted.rawBytes[0])
		if wanted.flagError {
			require.Error(t, err, "Expected error reading fixedheader [i:%d] %v", i, wanted.rawBytes)
		} else {
			require.NoError(t, err, "Error reading fixedheader [i:%d] %v", i, wanted.rawBytes)
			require.Equal(t, wanted.header.Type, fh.Type, "Mismatched fixedheader type [i:%d] %v", i, wanted.rawBytes)
			require.Equal(t, wanted.header.Dup, fh.Dup, "Mismatched fixedheader dup [i:%d] %v", i, wanted.rawBytes)
			require.Equal(t, wanted.header.Qos, fh.Qos, "Mismatched fixedheader qos [i:%d] %v", i, wanted.rawBytes)
			require.Equal(t, wanted.header.Retain, fh.Retain, "Mismatched fixedheader retain [i:%d] %v", i, wanted.rawBytes)
		}
	}
}

func BenchmarkFixedHeaderDecode(b *testing.B) {
	fh := new(FixedHeader)
	for n := 0; n < b.N; n++ {
		err := fh.Decode(fixedHeaderExpected[0].rawBytes[0])
		if err != nil {
			panic(err)
		}
	}
}

func TestEncodeLength(t *testing.T) {
	tt := []struct {
		have int64
		want []byte
	}{
		{
			120,
			[]byte{0x78},
		},
		{
			math.MaxInt64,
			[]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7f},
		},
	}

	for i, wanted := range tt {
		buf := new(bytes.Buffer)
		encodeLength(buf, wanted.have)
		require.Equal(t, wanted.want, buf.Bytes(), "Returned bytes should match length [i:%d] %s", i, wanted.have)
	}
}

func BenchmarkEncodeLength(b *testing.B) {
	buf := new(bytes.Buffer)
	for n := 0; n < b.N; n++ {
		encodeLength(buf, 120)
	}
}
