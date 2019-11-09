package circ

import (
	"bufio"
	"bytes"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewWriter(t *testing.T) {
	var size = 16
	var block = 4
	buf := NewWriter(size, block)

	require.NotNil(t, buf.buf)
	require.Equal(t, size, len(buf.buf))
	require.Equal(t, size, buf.size)
	require.Equal(t, block, buf.block)
}

func TestWriteTo(t *testing.T) {
	tests := []struct {
		tail  int64
		head  int64
		bytes []byte
		await int
		total int
		err   error
		desc  string
	}{
		{tail: 0, head: 5, bytes: []byte{'a', 'b', 'c', 'd', 'e'}, desc: "0,5 OK"},
		{tail: 14, head: 21, bytes: []byte{'o', 'p', 'a', 'b', 'c', 'd', 'e'}, desc: "14,16(2) OK"},
	}

	for i, tt := range tests {
		bb := []byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p'}
		buf := NewWriter(16, 4)
		buf.Set(bb, 0, 16)
		buf.SetPos(tt.tail, tt.head)

		var b bytes.Buffer
		w := bufio.NewWriter(&b)

		nc := make(chan int)
		go func() {
			n, _ := buf.WriteTo(w)
			nc <- n
		}()

		time.Sleep(time.Millisecond * 100)
		atomic.StoreInt64(&buf.done, 1)
		buf.wcond.L.Lock()
		buf.wcond.Broadcast()
		buf.wcond.L.Unlock()

		w.Flush()
		require.Equal(t, tt.bytes, b.Bytes(), "Written bytes mismatch [i:%d] %s", i, tt.desc)
	}
}

func TestWriteToEndedFirst(t *testing.T) {
	buf := NewWriter(16, 4)
	buf.done = 1

	var b bytes.Buffer
	w := bufio.NewWriter(&b)
	_, err := buf.WriteTo(w)
	require.Error(t, err)
}

func TestWriteToBadWriter(t *testing.T) {
	bb := []byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p'}
	buf := NewWriter(16, 4)
	buf.Set(bb, 0, 16)
	buf.SetPos(0, 6)
	r, w := net.Pipe()

	w.Close()
	_, err := buf.WriteTo(w)
	require.Error(t, err)
	r.Close()
}
