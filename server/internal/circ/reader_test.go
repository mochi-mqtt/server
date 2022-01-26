package circ

import (
	"bytes"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewReader(t *testing.T) {
	var size = 16
	var block = 4
	buf := NewReader(size, block)

	require.NotNil(t, buf.buf)
	require.Equal(t, size, len(buf.buf))
	require.Equal(t, size, buf.size)
	require.Equal(t, block, buf.block)
}

func TestNewReaderFromSlice(t *testing.T) {
	b := NewBytesPool(256)
	buf := NewReaderFromSlice(DefaultBlockSize, b.Get())
	require.NotNil(t, buf.buf)
	require.Equal(t, 256, cap(buf.buf))
}

func TestReadFrom(t *testing.T) {
	buf := NewReader(16, 4)

	b4 := bytes.Repeat([]byte{'-'}, 4)
	br := bytes.NewReader(b4)

	_, err := buf.ReadFrom(br)
	require.NoError(t, err)
	require.Equal(t, bytes.Repeat([]byte{'-'}, 4), buf.buf[:4])
	require.Equal(t, int64(4), buf.head)

	br.Reset(b4)
	_, err = buf.ReadFrom(br)
	require.Equal(t, int64(8), buf.head)

	br.Reset(b4)
	_, err = buf.ReadFrom(br)
	require.Equal(t, int64(12), buf.head)
}

func TestReadFromWrap(t *testing.T) {
	buf := NewReader(16, 4)
	buf.buf = bytes.Repeat([]byte{'-'}, 16)
	buf.SetPos(8, 14)
	br := bytes.NewReader(bytes.Repeat([]byte{'/'}, 8))

	o := make(chan error)
	go func() {
		_, err := buf.ReadFrom(br)
		o <- err
	}()
	time.Sleep(time.Millisecond * 100)
	go func() {
		atomic.StoreUint32(&buf.done, 1)
		buf.rcond.L.Lock()
		buf.rcond.Broadcast()
		buf.rcond.L.Unlock()
	}()
	<-o
	require.Equal(t, []byte{'/', '/', '/', '/', '/', '/', '-', '-', '-', '-', '-', '-', '-', '-', '/', '/'}, buf.Get())
	require.Equal(t, int64(22), atomic.LoadInt64(&buf.head))
	require.Equal(t, 6, buf.Index(atomic.LoadInt64(&buf.head)))
}

func TestReadOK(t *testing.T) {
	buf := NewReader(16, 4)
	buf.buf = []byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p'}

	tests := []struct {
		tail  int64
		head  int64
		n     int
		bytes []byte
		desc  string
	}{
		{tail: 0, head: 4, n: 4, bytes: []byte{'a', 'b', 'c', 'd'}, desc: "0, 4 OK"},
		{tail: 3, head: 15, n: 8, bytes: []byte{'d', 'e', 'f', 'g', 'h', 'i', 'j', 'k'}, desc: "3, 15 OK"},
		{tail: 14, head: 15, n: 6, bytes: []byte{'o', 'p', 'a', 'b', 'c', 'd'}, desc: "14, 2 wrapped OK"},
	}

	for i, tt := range tests {
		buf.SetPos(tt.tail, tt.head)
		o := make(chan []byte)
		go func() {
			p, _ := buf.Read(tt.n)
			o <- p
		}()

		time.Sleep(time.Millisecond)
		atomic.StoreInt64(&buf.head, buf.head+int64(tt.n))

		buf.wcond.L.Lock()
		buf.wcond.Broadcast()
		buf.wcond.L.Unlock()

		done := <-o
		require.Equal(t, tt.bytes, done, "Peeked bytes mismatch [i:%d] %s", i, tt.desc)

	}
}

func TestReadEnded(t *testing.T) {
	buf := NewBuffer(16, 4)
	o := make(chan error)
	go func() {
		_, err := buf.Read(4)
		o <- err
	}()
	time.Sleep(time.Millisecond)
	atomic.StoreUint32(&buf.done, 1)
	buf.wcond.L.Lock()
	buf.wcond.Broadcast()
	buf.wcond.L.Unlock()

	require.Error(t, <-o)
}
