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
		header:   FixedHeader{Connect, false, 0, false, 0}, // Type byte, Dup bool, Qos byte, Retain bool, Remaining int
	},
	{
		rawBytes: []byte{Connack << 4, 0x00},
		header:   FixedHeader{Connack, false, 0, false, 0},
	},
	{
		rawBytes: []byte{Publish << 4, 0x00},
		header:   FixedHeader{Publish, false, 0, false, 0},
	},
	{
		rawBytes: []byte{Publish<<4 | 1<<1, 0x00},
		header:   FixedHeader{Publish, false, 1, false, 0},
	},
	{
		rawBytes: []byte{Publish<<4 | 1<<1 | 1, 0x00},
		header:   FixedHeader{Publish, false, 1, true, 0},
	},
	{
		rawBytes: []byte{Publish<<4 | 2<<1, 0x00},
		header:   FixedHeader{Publish, false, 2, false, 0},
	},
	{
		rawBytes: []byte{Publish<<4 | 2<<1 | 1, 0x00},
		header:   FixedHeader{Publish, false, 2, true, 0},
	},
	{
		rawBytes: []byte{Publish<<4 | 1<<3, 0x00},
		header:   FixedHeader{Publish, true, 0, false, 0},
	},
	{
		rawBytes: []byte{Publish<<4 | 1<<3 | 1, 0x00},
		header:   FixedHeader{Publish, true, 0, true, 0},
	},
	{
		rawBytes: []byte{Publish<<4 | 1<<3 | 1<<1 | 1, 0x00},
		header:   FixedHeader{Publish, true, 1, true, 0},
	},
	{
		rawBytes: []byte{Publish<<4 | 1<<3 | 2<<1 | 1, 0x00},
		header:   FixedHeader{Publish, true, 2, true, 0},
	},
	{
		rawBytes: []byte{Puback << 4, 0x00},
		header:   FixedHeader{Puback, false, 0, false, 0},
	},
	{
		rawBytes: []byte{Pubrec << 4, 0x00},
		header:   FixedHeader{Pubrec, false, 0, false, 0},
	},
	{
		rawBytes: []byte{Pubrel<<4 | 1<<1, 0x00},
		header:   FixedHeader{Pubrel, false, 1, false, 0},
	},
	{
		rawBytes: []byte{Pubcomp << 4, 0x00},
		header:   FixedHeader{Pubcomp, false, 0, false, 0},
	},
	{
		rawBytes: []byte{Subscribe<<4 | 1<<1, 0x00},
		header:   FixedHeader{Subscribe, false, 1, false, 0},
	},
	{
		rawBytes: []byte{Suback << 4, 0x00},
		header:   FixedHeader{Suback, false, 0, false, 0},
	},
	{
		rawBytes: []byte{Unsubscribe<<4 | 1<<1, 0x00},
		header:   FixedHeader{Unsubscribe, false, 1, false, 0},
	},
	{
		rawBytes: []byte{Unsuback << 4, 0x00},
		header:   FixedHeader{Unsuback, false, 0, false, 0},
	},
	{
		rawBytes: []byte{Pingreq << 4, 0x00},
		header:   FixedHeader{Pingreq, false, 0, false, 0},
	},
	{
		rawBytes: []byte{Pingresp << 4, 0x00},
		header:   FixedHeader{Pingresp, false, 0, false, 0},
	},
	{
		rawBytes: []byte{Disconnect << 4, 0x00},
		header:   FixedHeader{Disconnect, false, 0, false, 0},
	},

	// remaining length
	{
		rawBytes: []byte{Publish << 4, 0x0a},
		header:   FixedHeader{Publish, false, 0, false, 10},
	},
	{
		rawBytes: []byte{Publish << 4, 0x80, 0x04},
		header:   FixedHeader{Publish, false, 0, false, 512},
	},
	{
		rawBytes: []byte{Publish << 4, 0xd2, 0x07},
		header:   FixedHeader{Publish, false, 0, false, 978},
	},
	{
		rawBytes: []byte{Publish << 4, 0x86, 0x9d, 0x01},
		header:   FixedHeader{Publish, false, 0, false, 20102},
	},
	{
		rawBytes:    []byte{Publish << 4, 0xd5, 0x86, 0xf9, 0x9e, 0x01},
		header:      FixedHeader{Publish, false, 0, false, 333333333},
		packetError: true,
	},

	// Invalid flags for packet
	{
		rawBytes:  []byte{Connect<<4 | 1<<3, 0x00},
		header:    FixedHeader{Connect, true, 0, false, 0},
		flagError: true,
	},
	{
		rawBytes:  []byte{Connect<<4 | 1<<1, 0x00},
		header:    FixedHeader{Connect, false, 1, false, 0},
		flagError: true,
	},
	{
		rawBytes:  []byte{Connect<<4 | 1, 0x00},
		header:    FixedHeader{Connect, false, 0, true, 0},
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
		have int
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
