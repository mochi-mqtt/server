package mempool

import (
	"reflect"
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewBuffer(t *testing.T) {
	defer debug.SetGCPercent(debug.SetGCPercent(-1))
	bp := NewBuffer(1000)
	require.Equal(t, "*mempool.BufferWithCap", reflect.TypeOf(bp).String())

	bp = NewBuffer(0)
	require.Equal(t, "*mempool.Buffer", reflect.TypeOf(bp).String())

	bp = NewBuffer(-1)
	require.Equal(t, "*mempool.Buffer", reflect.TypeOf(bp).String())
}

func TestBuffer(t *testing.T) {
	defer debug.SetGCPercent(debug.SetGCPercent(-1))
	Size := 101

	bp := NewBuffer(0)
	buf := bp.Get()

	for i := 0; i < Size; i++ {
		buf.WriteByte('a')
	}

	bp.Put(buf)
	buf = bp.Get()
	require.Equal(t, 0, buf.Len())
}

func TestBufferWithCap(t *testing.T) {
	defer debug.SetGCPercent(debug.SetGCPercent(-1))
	Size := 101
	bp := NewBuffer(100)
	buf := bp.Get()

	for i := 0; i < Size; i++ {
		buf.WriteByte('a')
	}

	bp.Put(buf)
	buf = bp.Get()
	require.Equal(t, 0, buf.Len())
	require.Equal(t, 0, buf.Cap())
}
