package pools

/*
import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewBufioReadersPool(t *testing.T) {
	bpool := NewBufioReadersPool(16)
	require.NotNil(t, bpool.pool)
}

func BenchmarkNewBufioReadersPool(b *testing.B) {
	for n := 0; n < b.N; n++ {
		NewBufioReadersPool(16)
	}
}

func TestNewBufioReadersGetPut(t *testing.T) {
	bpool := NewBufioReadersPool(16)
	r := bufio.NewReader(strings.NewReader("mochi"))
	out := make([]byte, 5)
	buf := bpool.Get(r)
	n, err := buf.Read(out)
	require.Equal(t, 5, n)
	require.NoError(t, err)

	require.Equal(t, []byte{'m', 'o', 'c', 'h', 'i'}, out)

	bpool.Put(buf)
	require.Panics(t, func() {
		buf.ReadByte()
	})
}

func BenchmarkBufioReadersGet(b *testing.B) {
	bpool := NewBufioReadersPool(16)
	r := bufio.NewReader(strings.NewReader("mochi"))

	for n := 0; n < b.N; n++ {
		bpool.Get(r)
	}
}

func BenchmarkBufioReadersPut(b *testing.B) {
	bpool := NewBufioReadersPool(16)
	r := bufio.NewReader(strings.NewReader("mochi"))
	buf := bpool.Get(r)
	for n := 0; n < b.N; n++ {
		bpool.Put(buf)
	}
}

func TestNewBufioWritersPool(t *testing.T) {
	bpool := NewBufioWritersPool(16)
	require.NotNil(t, bpool.pool)
}

func BenchmarkNewBufioWritersPool(b *testing.B) {
	for n := 0; n < b.N; n++ {
		NewBufioWritersPool(16)
	}
}

func TestNewBufioWritersGetPut(t *testing.T) {
	payload := []byte{'m', 'o', 'c', 'h', 'i'}

	bpool := NewBufioWritersPool(16)

	buf := new(bytes.Buffer)
	bw := bufio.NewWriter(buf)
	require.NotNil(t, bw)
	w := bpool.Get(bw)
	require.NotNil(t, w)

	n, err := w.Write(payload)
	require.NoError(t, err)
	require.Equal(t, 5, n)

	w.Flush()
	bw.Flush()
	require.Equal(t, payload, buf.Bytes())

	bpool.Put(w)
	_, err = w.Write(payload)
	require.NoError(t, err)
	require.Panics(t, func() {
		w.Flush()
	})
}

func BenchmarkBufioWritersGet(b *testing.B) {
	bpool := NewBufioWritersPool(16)
	buf := new(bytes.Buffer)
	bw := bufio.NewWriter(buf)

	for n := 0; n < b.N; n++ {
		bpool.Get(bw)
	}
}

func BenchmarkBufioWritersPut(b *testing.B) {
	bpool := NewBufioWritersPool(16)
	w := bpool.Get(bufio.NewWriter(new(bytes.Buffer)))

	for n := 0; n < b.N; n++ {
		bpool.Put(w)
	}
}
*/
