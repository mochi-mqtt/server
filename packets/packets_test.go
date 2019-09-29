package packets

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
