package pools

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewBytesBuffersPool(t *testing.T) {
	bpool := NewBytesBuffersPool()
	require.NotNil(t, bpool.pool)
}

func BenchmarkNewBytesBuffersPool(b *testing.B) {
	for n := 0; n < b.N; n++ {
		NewBytesBuffersPool()
	}
}

func TestNewBytesBuffersPoolGet(t *testing.T) {
	bpool := NewBytesBuffersPool()
	buf := bpool.Get()

	buf.WriteString("mochi")
	require.Equal(t, []byte{'m', 'o', 'c', 'h', 'i'}, buf.Bytes())
}

func BenchmarkNewBytesBuffersPoolGet(b *testing.B) {
	bpool := NewBytesBuffersPool()
	for n := 0; n < b.N; n++ {
		bpool.Get()
	}
}

func TestNewBytesBuffersPoolPut(t *testing.T) {
	bpool := NewBytesBuffersPool()
	buf := bpool.Get()

	buf.WriteString("mochi")
	require.Equal(t, []byte{'m', 'o', 'c', 'h', 'i'}, buf.Bytes())

	bpool.Put(buf)
	require.Equal(t, 0, buf.Len())
}

func BenchmarkNewBytesBuffersPoolPut(b *testing.B) {
	bpool := NewBytesBuffersPool()
	buf := bpool.Get()
	for n := 0; n < b.N; n++ {
		bpool.Put(buf)
	}
}
