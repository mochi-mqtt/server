package circ

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewBuffer(t *testing.T) {
	var size int64 = 16
	var block int64 = 4
	buf := newBuffer(size, block)

	require.NotNil(t, buf.buf)
	require.NotNil(t, buf.rcond)
	require.NotNil(t, buf.wcond)
	require.Equal(t, size, int64(len(buf.buf)))
	require.Equal(t, size, buf.size)
	require.Equal(t, block, buf.block)
}

func TestSet(t *testing.T) {
	buf := newBuffer(8, 4)
	require.Equal(t, make([]byte, 4), buf.buf[0:4])
	p := []byte{'1', '2', '3', '4'}
	buf.Set(p, 0, 4)
	require.Equal(t, p, buf.buf[0:4])

	buf.Set(p, 2, 6)
	require.Equal(t, p, buf.buf[2:6])
}

func TestGetPos(t *testing.T) {
	buf := newBuffer(8, 4)
	buf.tail, buf.head = 1, 3
	tail, head := buf.GetPos()
	require.Equal(t, int64(1), tail)
	require.Equal(t, int64(3), head)
}

func TestSetPos(t *testing.T) {
	buf := newBuffer(8, 4)
	buf.SetPos(1, 3)
	require.Equal(t, int64(3), buf.head)
	require.Equal(t, int64(1), buf.tail)
}

func TestCommitHead(t *testing.T) {
	//buf := newBuffer(16, 4)

}

func TestAwaitCapacity(t *testing.T) {
	tests := []struct {
		tail  int64
		head  int64
		await int
		desc  string
	}{
		{0, 0, 0, "OK 4, 0"},
		{6, 6, 0, "OK 6, 10"},
		{12, 16, 0, "OK 16, 4"},
		{16, 12, 0, "OK 16 16"},
		{3, 6, 0, "OK 3, 10"},
		{12, 0, 0, "OK 16, 0"},
		{1, 16, 4, "next is more than tail, wait for tail incr"},
		{7, 5, 2, "tail is great than head, wrapped and caught up with tail, wait for tail incr"},
	}

	buf := newBuffer(16, 4)
	for i, tt := range tests {
		buf.tail, buf.head = tt.tail, tt.head
		o := make(chan []interface{})
		var start int64 = -1
		var err error
		go func() {
			start, err = buf.awaitCapacity(4)
			o <- []interface{}{start, err}
		}()

		time.Sleep(time.Millisecond)
		for j := 0; j < tt.await; j++ {
			atomic.AddInt64(&buf.tail, 1)
			buf.rcond.L.Lock()
			buf.rcond.Broadcast()
			buf.rcond.L.Unlock()
		}

		time.Sleep(time.Millisecond) // wait for await capacity to actually exit
		if start == -1 {
			atomic.StoreInt64(&buf.done, 1)
			buf.rcond.L.Lock()
			buf.rcond.Broadcast()
			buf.rcond.L.Unlock()
		}
		done := <-o
		require.Equal(t, tt.head, done[0].(int64), "Head-Start mismatch [i:%d] %s", i, tt.desc)
		require.Nil(t, done[1], "Unexpected Error [i:%d] %s", i, tt.desc)
	}
}

func TestAwaitFilled(t *testing.T) {
	tests := []struct {
		tail  int64
		head  int64
		next  int64
		await int
		desc  string
	}{
		{0, 4, 4, 0, "OK 0, 4"},
		{6, 10, 10, 0, "OK 6, 10"},
		{14, 2, 2, 0, "OK 14, 2 wrapped"},
		{6, 8, 10, 2, "Wait 6, 8"},
		{14, 1, 4, 3, "Wait 14, 1 wrapped"},
	}

	buf := newBuffer(16, 4)
	for i, tt := range tests {
		buf.tail, buf.head = tt.tail, tt.head
		o := make(chan []interface{})
		var start int64 = -1
		var err error
		go func() {
			start, err = buf.awaitFilled(4)
			o <- []interface{}{start, err}
		}()

		time.Sleep(time.Millisecond)
		for j := 0; j < tt.await; j++ {
			atomic.AddInt64(&buf.head, 1)
			buf.wcond.L.Lock()
			buf.wcond.Broadcast()
			buf.wcond.L.Unlock()
		}

		done := <-o
		require.Equal(t, tt.tail, done[0].(int64), "tail start [i:%d] %s", i, tt.desc)
		require.Equal(t, tt.next, buf.head, "Head-next mismatch [i:%d] %s", i, tt.desc)
		require.Nil(t, done[1], "Unexpected Error [i:%d] %s", i, tt.desc)
	}
}

func TestCommitTail(t *testing.T) {
	tests := []struct {
		tail  int64
		head  int64
		next  int64
		await int
		desc  string
	}{
		{0, 5, 4, 0, "OK 0, 4"},
		{6, 10, 10, 0, "OK 6, 10"},
		{14, 2, 2, 0, "OK 14, 2 wrapped"},
		{6, 8, 10, 2, "Wait 6, 8"},
		{14, 1, 2, 2, "Wait 14, 1 wrapped"},
	}

	buf := newBuffer(16, 4)
	for i, tt := range tests {
		buf.tail, buf.head = tt.tail, tt.head
		o := make(chan error)
		go func() {
			o <- buf.CommitTail(4)
		}()

		time.Sleep(time.Millisecond)
		for j := 0; j < tt.await; j++ {
			atomic.AddInt64(&buf.head, 1)
			buf.wcond.L.Lock()
			buf.wcond.Broadcast()
			buf.wcond.L.Unlock()
		}
		require.NoError(t, <-o, "Unexpected Error [i:%d] %s", i, tt.desc)
		require.Equal(t, tt.next, buf.tail, "Tail-next mismatch [i:%d] %s", i, tt.desc)
	}
}
