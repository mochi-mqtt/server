package circ

import (
	//"fmt"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/require"
)

func TestNewBuffer(t *testing.T) {
	var size int = 16
	var block int = 4
	buf := NewBuffer(size, block)

	require.NotNil(t, buf.buf)
	require.NotNil(t, buf.rcond)
	require.NotNil(t, buf.wcond)
	require.Equal(t, size, len(buf.buf))
	require.Equal(t, size, buf.size)
	require.Equal(t, block, buf.block)
}

func TestNewBuffer0Size(t *testing.T) {
	buf := NewBuffer(0, 0)
	require.NotNil(t, buf.buf)
	require.Equal(t, DefaultBufferSize, buf.size)
	require.Equal(t, DefaultBlockSize, buf.block)
}

func TestNewBufferUndersize(t *testing.T) {
	buf := NewBuffer(DefaultBlockSize+10, DefaultBlockSize)
	require.NotNil(t, buf.buf)
	require.Equal(t, DefaultBlockSize*2, buf.size)
	require.Equal(t, DefaultBlockSize, buf.block)
}

func TestNewBufferFromSlice(t *testing.T) {
	b := NewBytesPool(256)
	buf := NewBufferFromSlice(DefaultBlockSize, b.Get())
	require.NotNil(t, buf.buf)
	require.Equal(t, 256, cap(buf.buf))
}

func TestNewBufferFromSlice0Size(t *testing.T) {
	b := NewBytesPool(256)
	buf := NewBufferFromSlice(0, b.Get())
	require.NotNil(t, buf.buf)
	require.Equal(t, 256, cap(buf.buf))
}

func TestAtomicAlignment(t *testing.T) {
	var b Buffer

	offset := unsafe.Offsetof(b.head)
	require.Equalf(t, uintptr(0), offset%8,
		"head requires 64-bit alignment for atomic: offset %d", offset)

	offset = unsafe.Offsetof(b.tail)
	require.Equalf(t, uintptr(0), offset%8,
		"tail requires 64-bit alignment for atomic: offset %d", offset)
}

func TestGetPos(t *testing.T) {
	buf := NewBuffer(16, 4)
	tail, head := buf.GetPos()
	require.Equal(t, int64(0), tail)
	require.Equal(t, int64(0), head)

	atomic.StoreInt64(&buf.tail, 3)
	atomic.StoreInt64(&buf.head, 11)

	tail, head = buf.GetPos()
	require.Equal(t, int64(3), tail)
	require.Equal(t, int64(11), head)
}

func TestGet(t *testing.T) {
	buf := NewBuffer(16, 4)
	require.Equal(t, make([]byte, 16), buf.Get())

	buf.buf[0] = 1
	buf.buf[15] = 1
	require.Equal(t, []byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}, buf.Get())
}

func TestSetPos(t *testing.T) {
	buf := NewBuffer(16, 4)
	require.Equal(t, int64(0), atomic.LoadInt64(&buf.tail))
	require.Equal(t, int64(0), atomic.LoadInt64(&buf.head))

	buf.SetPos(4, 8)
	require.Equal(t, int64(4), atomic.LoadInt64(&buf.tail))
	require.Equal(t, int64(8), atomic.LoadInt64(&buf.head))
}

func TestSet(t *testing.T) {
	buf := NewBuffer(16, 4)
	err := buf.Set([]byte{1, 1, 1, 1}, 17, 19)
	require.Error(t, err)

	err = buf.Set([]byte{1, 1, 1, 1}, 4, 8)
	require.NoError(t, err)
	require.Equal(t, []byte{0, 0, 0, 0, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0}, buf.buf)
}

func TestIndex(t *testing.T) {
	buf := NewBuffer(1024, 4)
	require.Equal(t, 512, buf.Index(512))
	require.Equal(t, 0, buf.Index(1024))
	require.Equal(t, 6, buf.Index(1030))
	require.Equal(t, 6, buf.Index(61446))
}

func TestAwaitFilled(t *testing.T) {
	tests := []struct {
		tail  int64
		head  int64
		n     int
		await int
		desc  string
	}{
		{tail: 0, head: 4, n: 4, await: 1, desc: "OK 0, 4"},
		{tail: 8, head: 11, n: 4, await: 1, desc: "OK 8, 11"},
		{tail: 102, head: 103, n: 4, await: 3, desc: "OK 102, 103"},
	}

	for i, tt := range tests {
		//fmt.Println(i)
		buf := NewBuffer(16, 4)
		buf.SetPos(tt.tail, tt.head)
		o := make(chan error)
		go func() {
			o <- buf.awaitFilled(4)
		}()

		time.Sleep(time.Millisecond)
		atomic.AddInt64(&buf.head, int64(tt.await))
		buf.wcond.L.Lock()
		buf.wcond.Broadcast()
		buf.wcond.L.Unlock()

		require.NoError(t, <-o, "Unexpected Error [i:%d] %s", i, tt.desc)
	}
}

func TestAwaitFilledEnded(t *testing.T) {
	buf := NewBuffer(16, 4)
	o := make(chan error)
	go func() {
		o <- buf.awaitFilled(4)
	}()
	time.Sleep(time.Millisecond)
	atomic.StoreUint32(&buf.done, 1)
	buf.wcond.L.Lock()
	buf.wcond.Broadcast()
	buf.wcond.L.Unlock()

	require.Error(t, <-o)
}

func TestAwaitEmptyOK(t *testing.T) {
	tests := []struct {
		tail  int64
		head  int64
		await int
		desc  string
	}{
		{tail: 0, head: 0, await: 0, desc: "OK 0, 0"},
		{tail: 0, head: 5, await: 0, desc: "OK 0, 5"},
		{tail: 0, head: 14, await: 3, desc: "OK wrap 0, 14 "},
		{tail: 22, head: 35, await: 2, desc: "OK wrap 0, 14 "},
		{tail: 15, head: 17, await: 7, desc: "OK 15,2"},
		{tail: 0, head: 10, await: 2, desc: "OK 0, 10"},
		{tail: 1, head: 15, await: 4, desc: "OK 2, 14"},
	}

	for i, tt := range tests {
		buf := NewBuffer(16, 4)
		buf.SetPos(tt.tail, tt.head)
		o := make(chan error)
		go func() {
			o <- buf.awaitEmpty(4)
		}()

		time.Sleep(time.Millisecond)
		atomic.AddInt64(&buf.tail, int64(tt.await))
		buf.rcond.L.Lock()
		buf.rcond.Broadcast()
		buf.rcond.L.Unlock()

		require.NoError(t, <-o, "Unexpected Error [i:%d] %s", i, tt.desc)
	}
}

func TestAwaitEmptyEnded(t *testing.T) {
	buf := NewBuffer(16, 4)
	buf.SetPos(1, 15)
	o := make(chan error)
	go func() {
		o <- buf.awaitEmpty(4)
	}()
	time.Sleep(time.Millisecond)
	atomic.StoreUint32(&buf.done, 1)
	buf.rcond.L.Lock()
	buf.rcond.Broadcast()
	buf.rcond.L.Unlock()

	require.Error(t, <-o)
}

func TestCheckEmpty(t *testing.T) {
	buf := NewBuffer(16, 4)

	tests := []struct {
		head int64
		tail int64
		want bool
		desc string
	}{
		{tail: 0, head: 0, want: true, desc: "0, 0 true"},
		{tail: 3, head: 4, want: true, desc: "4, 3 true"},
		{tail: 15, head: 17, want: true, desc: "15, 17(1) true"},
		{tail: 1, head: 30, want: false, desc: "1, 30(14) false"},
		{tail: 15, head: 30, want: false, desc: "15, 30(14) false; head has caught up to tail"},
	}
	for i, tt := range tests {
		buf.SetPos(tt.tail, tt.head)
		require.Equal(t, tt.want, buf.checkEmpty(4), "Mismatched bool wanted [i:%d] %s", i, tt.desc)
	}
}

func TestCheckFilled(t *testing.T) {
	buf := NewBuffer(16, 4)

	tests := []struct {
		head int64
		tail int64
		want bool
		desc string
	}{
		{tail: 0, head: 0, want: false, desc: "0, 0 false"},
		{tail: 0, head: 4, want: true, desc: "0, 4 true"},
		{tail: 14, head: 16, want: false, desc: "14,16 false"},
		{tail: 14, head: 18, want: true, desc: "14,16 true"},
	}

	for i, tt := range tests {
		buf.SetPos(tt.tail, tt.head)
		require.Equal(t, tt.want, buf.checkFilled(4), "Mismatched bool wanted [i:%d] %s", i, tt.desc)
	}

}

func TestCommitTail(t *testing.T) {
	tests := []struct {
		tail  int64
		head  int64
		n     int
		next  int64
		await int
		desc  string
	}{
		{tail: 0, head: 5, n: 4, next: 4, await: 0, desc: "OK 0, 4"},
		{tail: 0, head: 5, n: 6, next: 6, await: 1, desc: "OK 0, 5"},
	}

	for i, tt := range tests {
		buf := NewBuffer(16, 4)
		buf.SetPos(tt.tail, tt.head)
		go func() {
			buf.CommitTail(tt.n)
		}()

		time.Sleep(time.Millisecond)
		for j := 0; j < tt.await; j++ {
			atomic.AddInt64(&buf.head, 1)
			buf.wcond.L.Lock()
			buf.wcond.Broadcast()
			buf.wcond.L.Unlock()
		}
		require.Equal(t, tt.next, atomic.LoadInt64(&buf.tail), "Next tail mismatch [i:%d] %s", i, tt.desc)
	}
}

/*
func TestCommitTailEnded(t *testing.T) {
	buf := NewBuffer(16, 4)
	o := make(chan error)
	go func() {
		o <- buf.CommitTail(5)
	}()
	time.Sleep(time.Millisecond)
	atomic.StoreUint32(&buf.done, 1)
	buf.wcond.L.Lock()
	buf.wcond.Broadcast()
	buf.wcond.L.Unlock()

	require.Error(t, <-o)
}
*/
func TestCapDelta(t *testing.T) {
	buf := NewBuffer(16, 4)

	require.Equal(t, 0, buf.CapDelta())

	buf.SetPos(10, 15)
	require.Equal(t, 5, buf.CapDelta())
}

func TestStop(t *testing.T) {
	buf := NewBuffer(16, 4)
	buf.Stop()
	require.Equal(t, uint32(1), buf.done)
}
