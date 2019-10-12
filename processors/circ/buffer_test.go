package circ

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func init() {
	blockSize = 4
}

func TestNewBuffer(t *testing.T) {
	var size int64 = 16
	buf := newBuffer(size)

	require.NotNil(t, buf.buf)
	require.NotNil(t, buf.rcond)
	require.NotNil(t, buf.wcond)
	require.Equal(t, size, int64(len(buf.buf)))
	require.Equal(t, size, buf.size)
}

func TestAwaitCapacity(t *testing.T) {
	tt := []struct {
		tail  int64
		head  int64
		need  int64
		await int
		desc  string
	}{
		{0, 0, 4, 0, "OK 4, 0"},
		{6, 6, 4, 0, "OK 6, 10"},
		{12, 16, 4, 0, "OK 16, 4"},
		{16, 12, 4, 0, "OK 16 16"},
		{3, 6, 4, 0, "OK 3, 10"},
		{12, 0, 4, 0, "OK 16, 0"},
		{1, 16, 4, 4, "next is more than tail, wait for tail incr"},
		{7, 5, 4, 2, "tail is great than head, wrapped and caught up with tail, wait for tail incr"},
	}

	buf := newBuffer(16)
	for i, check := range tt {
		buf.tail, buf.head = check.tail, check.head
		o := make(chan []interface{})
		var start int64 = -1
		var err error
		go func() {
			start, err = buf.awaitCapacity(4)
			o <- []interface{}{start, err}
		}()

		time.Sleep(time.Millisecond) // atomic updates are super fast so wait a bit
		for i := 0; i < check.await; i++ {
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
		require.Equal(t, check.head, done[0].(int64), "Head-Start mismatch [i:%d] %s", i, check.desc)
		require.Nil(t, done[1], "Unexpected Error [i:%d] %s", i, check.desc)
	}
}
