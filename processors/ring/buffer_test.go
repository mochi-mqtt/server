package ring

import (
	//"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewBuffer(t *testing.T) {
	var size int64 = 256
	buf := NewBuffer(size)

	require.NotNil(t, buf.buffer)
	require.Equal(t, size, len(buf.buffer))
	require.Equal(t, size, buf.size)
}

/*
func TestReadFromIO(t *testing.T) {
	buf := NewBuffer(128)
	in := bytes.Repeat([]byte{'-'}, 32)
	n, err := buf.Read(bytes.NewReader(in))
	require.Equal(t, 3, n)
	require.Equal(t, err, io.EOF)
}
*/

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
		{1, 16, 4, 4, "next is more than tail, wait for tail incr"},
		{7, 5, 4, 2, "tail is great than head, wrapped and caught up with tail, wait for tail incr"},
	}

	buf := NewBuffer(16)
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

	/*buf.tail, buf.head = 0, 0 // OK 4, 0
	start, err := buf.awaitCapacity(4)
	require.NoError(t, err)

	buf.tail, buf.head = 6, 6 // OK 6, 10
	start, err := buf.awaitCapacity(4)
	require.NoError(t, err)

	buf.tail, buf.head = 12, 16 // OK 16, 4
	buf.awaitCapacity(4)

	buf.tail, buf.head = 16, 12 // OK 16, 16
	buf.awaitCapacity(4)

	// Plenty of room for next block
	buf.tail, buf.head = 3, 6 // OK 3, 10
	buf.awaitCapacity(4)

	fmt.Println("---")

	// head + n > tail wait will wrap and overrun the tail! 4...4
	buf.tail, buf.head = 1, 16
	buf.awaitCapacity(4)

	// Tail is greater than head - we've wrapped and caught up to the tail. 9...9
	buf.tail, buf.head = 7, 5
	buf.awaitCapacity(4)
	*/
}
