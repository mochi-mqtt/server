package circ

import (
	"bytes"
	"github.com/stretchr/testify/require"
	"sync/atomic"
	"testing"
	"time"
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
		atomic.StoreInt64(&buf.done, 1)
		buf.rcond.L.Lock()
		buf.rcond.Broadcast()
		buf.rcond.L.Unlock()
	}()
	<-o
	require.Equal(t, []byte{'/', '/', '/', '/', '/', '/', '-', '-', '-', '-', '-', '-', '-', '-', '/', '/'}, buf.Get())
	require.Equal(t, int64(22), atomic.LoadInt64(&buf.head))
	require.Equal(t, 6, buf.Index(atomic.LoadInt64(&buf.head)))
}

/*
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

br.Reset([]byte{'-', '-', '-', '-', '/', '/', '/', '/'})
o := make(chan error)
go func() {
	_, err := buf.ReadFrom(br)
	o <- err
}()

atomic.StoreInt64(&buf.tail, 4)
<-o
require.Equal(t, int64(4), buf.head)
require.Equal(t, []byte{'/', '/', '/', '/', '-', '-', '-', '-', '-', '-', '-', '-', '-', '-', '-', '-'}, buf.buf)
}
*/
