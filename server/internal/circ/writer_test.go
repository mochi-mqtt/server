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

func TestNewWriterFromSlice(t *testing.T) {
	b := NewBytesPool(256)
	buf := NewWriterFromSlice(DefaultBlockSize, b.Get())
	require.NotNil(t, buf.buf)
	require.Equal(t, 256, cap(buf.buf))
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

		nc := make(chan int64)
		go func() {
			n, _ := buf.WriteTo(w)
			nc <- n
		}()

		time.Sleep(time.Millisecond * 100)
		atomic.StoreUint32(&buf.done, 1)
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

func TestWrite(t *testing.T) {
	tests := []struct {
		tail  int64
		head  int64
		rHead int64
		bytes []byte
		want  []byte
		desc  string
	}{
		{tail: 0, head: 0, rHead: 4, bytes: []byte{'a', 'b', 'c', 'd'}, want: []byte{'a', 'b', 'c', 'd', 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, desc: "0>4 OK"},
		{tail: 4, head: 14, rHead: 2, bytes: []byte{'a', 'b', 'c', 'd'}, want: []byte{'c', 'd', 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 'a', 'b'}, desc: "14>2 OK"},
	}

	for i, tt := range tests {
		buf := NewWriter(16, 4)
		buf.SetPos(tt.tail, tt.head)

		o := make(chan []interface{})
		go func() {
			nn, err := buf.Write(tt.bytes)
			o <- []interface{}{nn, err}
		}()

		done := <-o
		require.Equal(t, tt.want, buf.buf, "Wanted written mismatch [i:%d] %s", i, tt.desc)
		require.Nil(t, done[1], "Unexpected Error [i:%d] %s", i, tt.desc)
	}
}

func TestWriteEnded(t *testing.T) {
	buf := NewWriter(16, 4)
	buf.SetPos(15, 30)
	buf.done = 1

	_, err := buf.Write([]byte{'a', 'b', 'c', 'd'})
	require.Error(t, err)
}

func TestWriteBytes(t *testing.T) {
	tests := []struct {
		tail  int64
		head  int64
		bytes []byte
		want  []byte
		start int
		desc  string
	}{
		{tail: 0, head: 0, bytes: []byte{'a', 'b', 'c', 'd'}, want: []byte{'a', 'b', 'c', 'd', 0, 0, 0, 0}, desc: "0,4 OK"},
		{tail: 6, head: 6, bytes: []byte{'a', 'b', 'c', 'd'}, want: []byte{'c', 'd', 0, 0, 0, 0, 'a', 'b'}, desc: "6,2 OK wrapped"},
	}

	for i, tt := range tests {
		buf := NewWriter(8, 4)
		buf.SetPos(tt.tail, tt.head)
		n := buf.writeBytes(tt.bytes)

		require.Equal(t, tt.want, buf.buf, "Buffer mistmatch [i:%d] %s", i, tt.desc)
		require.Equal(t, len(tt.bytes), n)
	}

}
