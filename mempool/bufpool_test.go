package mempool

import (
	"bytes"
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

func BenchmarkBufferPool(b *testing.B) {
	bp := NewBuffer(0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b := bp.Get()
		b.WriteString("this is a test")
		bp.Put(b)
	}
}

func BenchmarkBufferPoolWithCapLarger(b *testing.B) {
	bp := NewBuffer(64 * 1024)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b := bp.Get()
		b.WriteString("this is a test")
		bp.Put(b)
	}
}

func BenchmarkBufferPoolWithCapLesser(b *testing.B) {
	bp := NewBuffer(10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b := bp.Get()
		b.WriteString("this is a test")
		bp.Put(b)
	}
}

func BenchmarkBufferWithoutPool(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b := new(bytes.Buffer)
		b.WriteString("this is a test")
		_ = b
	}
}
