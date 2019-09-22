package packets

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewFixedHeader(t *testing.T) {

	tt := []struct {
		have byte
		fh   FixedHeader
	}{
		{
			have: Connect,
			fh:   FixedHeader{Type: Connect},
		},
		{
			have: Connack,
			fh:   FixedHeader{Type: Connack},
		},
		{
			have: Publish,
			fh:   FixedHeader{Type: Publish},
		},
		{
			have: Puback,
			fh:   FixedHeader{Type: Puback},
		},
		{
			have: Pubrec,
			fh:   FixedHeader{Type: Pubrec},
		},
		{
			have: Pubrel,
			fh:   FixedHeader{Type: Pubrel, Qos: 1},
		},
		{
			have: Pubcomp,
			fh:   FixedHeader{Type: Pubcomp},
		},
		{
			have: Subscribe,
			fh:   FixedHeader{Type: Subscribe, Qos: 1},
		},
		{
			have: Suback,
			fh:   FixedHeader{Type: Suback},
		},
		{
			have: Pingreq,
			fh:   FixedHeader{Type: Pingreq},
		},
		{
			have: Pingresp,
			fh:   FixedHeader{Type: Pingresp},
		},
		{
			have: Unsubscribe,
			fh:   FixedHeader{Type: Unsubscribe, Qos: 1},
		},
		{
			have: Unsuback,
			fh:   FixedHeader{Type: Unsuback},
		},
		{
			have: Disconnect,
			fh:   FixedHeader{Type: Disconnect},
		},
	}

	for i, wanted := range tt {
		fh := NewFixedHeader(wanted.have)
		require.Equal(t, wanted.fh, fh, "Returned fixedheader should match [i:%d] %s", i, wanted.have)
	}
}

func BenchmarkNewFixedHeader(b *testing.B) {
	for n := 0; n < b.N; n++ {
		NewFixedHeader(Publish)
	}
}

func TestNewPacket(t *testing.T) {
	// All the other packet types are handled and tested thorougly by the other tests.
	// Just check for out of range packet code.
	pk := newPacket(99)
	require.Equal(t, nil, pk, "Returned packet should be nil")
}

func BenchmarkNewPacket(b *testing.B) {
	for n := 0; n < b.N; n++ {
		newPacket(1)
	}
}
