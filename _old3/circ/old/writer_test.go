package circ

import (
	"io"
	"io/ioutil"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewWriter(t *testing.T) {
	var size int64 = 16
	var block int64 = 4
	buf := NewWriter(size, block)

	require.NotNil(t, buf.buf)
	require.Equal(t, size, int64(len(buf.buf)))
	require.Equal(t, size, buf.size)
	require.Equal(t, block, buf.block)
}

func TestWriteTo(t *testing.T) {
	bb := []byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p'}

	tests := []struct {
		tail  int64
		next  int64
		head  int64
		err   error
		bytes []byte
		total int64
		desc  string
	}{
		{tail: 0, next: 4, head: 4, err: io.EOF, bytes: []byte{'a', 'b', 'c', 'd'}, total: 4, desc: "0, 4 OK"},
		{tail: 14, next: 2, head: 2, err: io.EOF, bytes: []byte{'o', 'p', 'a', 'b'}, total: 4, desc: "14, 2 wrap"},
		{tail: 0, next: 0, head: 2, err: ErrInsufficientBytes, bytes: []byte{}, total: 0, desc: "0, 2 insufficient"},
		{tail: 14, next: 2, head: 3, err: ErrInsufficientBytes, bytes: []byte{'o', 'p', 'a', 'b'}, total: 4, desc: "0, 3 OK > insufficient wrap"},
	}

	for i, tt := range tests {
		buf := NewWriter(16, 4)
		buf.buf = bb
		buf.tail, buf.head = tt.tail, tt.head
		r, w := net.Pipe()
		go func() {
			time.Sleep(time.Millisecond)
			atomic.StoreInt64(&buf.done, 1)
			buf.wcond.L.Lock()
			buf.wcond.Broadcast()
			buf.wcond.L.Unlock()
			w.Close()
		}()

		recv := make(chan []byte)
		go func() {
			buf, err := ioutil.ReadAll(r)
			if err != nil {
				panic(err)
			}
			recv <- buf
		}()

		total, err := buf.WriteTo(w)
		require.Equal(t, tt.next, buf.tail, "Tail placement mismatched [i:%d] %s", i, tt.desc)
		require.Equal(t, tt.total, total, "Total bytes written mismatch [i:%d] %s", i, tt.desc)
		require.Equal(t, tt.bytes, <-recv, "Written bytes mismatch [i:%d] %s", i, tt.desc)
		if tt.err != nil {
			require.Error(t, err, "Expected Error [i:%d] %s", i, tt.desc)
		} else {
			require.NoError(t, err, "Unexpected Error [i:%d] %s", i, tt.desc)
		}

		r.Close()
	}

}

func TestWrite(t *testing.T) {

	buf := NewWriter(16, 4)

	tests := []struct {
		tail  int64
		head  int64
		bytes []byte
		want  int
		err   error
		desc  string
	}{
		{tail: 0, head: 4, bytes: []byte{'a', 'b', 'c', 'd'}, want: 4, err: nil, desc: "0,4 OK"},
	}

	for i, tt := range tests {
		buf.tail, buf.head = tt.tail, tt.head

		o := make(chan []interface{})
		go func() {
			nn, err := buf.Write(tt.bytes)
			o <- []interface{}{nn, err}
		}()

		done := <-o
		require.Equal(t, tt.want, done[0].(int), "Wanted written mismatch [i:%d] %s", i, tt.desc)
		require.Nil(t, done[1], "Unexpected Error [i:%d] %s", i, tt.desc)

	}

}

func TestWriteSequential(t *testing.T) {
	buf := NewWriter(8, 4)

	a := []byte{'a', 'b', 'c', 'd'}
	b := []byte{'e', 'f', 'g', 'h'}

	buf.head = 4
	nn, err := buf.Write(a)
	require.NoError(t, err)
	require.Equal(t, 4, nn)

	buf.tail = (buf.tail + int64(nn)) % buf.size
	require.Equal(t, int64(4), buf.tail)
	buf.head = 8
	nn, err = buf.Write(b)
	require.Equal(t, 4, nn)
	buf.tail = (buf.tail + int64(nn)) % buf.size
	require.Equal(t, int64(0), buf.tail)
	require.Equal(t, []byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'}, buf.buf)
}

func TestWriteBytes(t *testing.T) {

	tests := []struct {
		tail  int64
		bytes []byte
		want  []byte
		start int
		desc  string
	}{
		{tail: 0, bytes: []byte{'a', 'b', 'c', 'd'}, want: []byte{'a', 'b', 'c', 'd', 0, 0, 0, 0}, start: 4, desc: "0,4 OK"},
		{tail: 6, bytes: []byte{'a', 'b', 'c', 'd'}, want: []byte{'c', 'd', 0, 0, 0, 0, 'a', 'b'}, start: 4, desc: "6,2 OK wrapped"},
	}

	for i, tt := range tests {
		buf := NewWriter(8, 4)
		buf.tail = tt.tail
		ns := buf.writeBytes(tt.bytes)
		require.Equal(t, tt.want, buf.buf, "Buffer mistmatch [i:%d] %s", i, tt.desc)
		require.Equal(t, tt.start, ns, "Unexpected new start position [i:%d] %s", i, tt.desc)
	}

}
