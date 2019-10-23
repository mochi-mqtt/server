package mqtt

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mochi-co/mqtt/packets"
	"github.com/mochi-co/mqtt/processors/circ"
)

func TestNewProcessor(t *testing.T) {
	conn := new(MockNetConn)
	p := NewProcessor(conn, circ.NewReader(16), circ.NewWriter(16))
	require.NotNil(t, p.R)
}

func TestProcessorReadFixedHeader(t *testing.T) {
	conn := new(MockNetConn)

	p := NewProcessor(conn, circ.NewReader(16), circ.NewWriter(16))

	// Test null data.
	fh := new(packets.FixedHeader)
	//p.R.SetPos(0, 1)
	fmt.Printf("A %+v\n", p.R)
	err := p.ReadFixedHeader(fh)
	require.Error(t, err)

	// Test insufficient peeking.
	fh = new(packets.FixedHeader)
	p.R.Set([]byte{packets.Connect << 4}, 0, 1)
	p.R.SetPos(0, 1)
	fmt.Printf("B %+v\n", p.R)
	err = p.ReadFixedHeader(fh)
	require.Error(t, err)

	fh = new(packets.FixedHeader)
	p.R.Set([]byte{packets.Connect << 4, 0x00}, 0, 2)
	p.R.SetPos(0, 2)
	err = p.ReadFixedHeader(fh)
	require.NoError(t, err)
}
