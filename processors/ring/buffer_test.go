package ring

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func init() {
	blockSize = 4
}

func TestNewBuffer(t *testing.T) {
	var size int64 = 256
	buf := NewBuffer(size)

	require.NotNil(t, buf.buffer)
	require.Equal(t, size, int64(len(buf.buffer)))
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
}

func TestReadFrom(t *testing.T) {
	buf := NewBuffer(16)

	b4 := bytes.Repeat([]byte{'-'}, 4)
	br := bytes.NewReader(b4)
	_, err := buf.ReadFrom(br)
	require.NoError(t, err)
	require.Equal(t, bytes.Repeat([]byte{'-'}, 4), buf.buffer[:4])
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
	require.Equal(t, []byte{'/', '/', '/', '/', '-', '-', '-', '-', '-', '-', '-', '-', '-', '-', '-', '-'}, buf.buffer)
}

func TestPeek(t *testing.T) {

	buf := NewBuffer(16)
	buf.buffer = []byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p'}

	tests := []struct {
		tail  int64
		head  int64
		want  int
		bytes []byte
		err   error
		desc  string
	}{
		{tail: 0, head: 4, want: 4, bytes: []byte{'a', 'b', 'c', 'd'}, err: nil, desc: "0,4: OK"},
		{tail: 3, head: 15, want: 8, bytes: []byte{'d', 'e', 'f', 'g', 'h', 'i', 'j', 'k'}, err: nil, desc: "0,4: OK"},
		{tail: 14, head: 5, want: 6, bytes: []byte{'o', 'p', 'a', 'b', 'c', 'd'}, err: nil, desc: "14,5 OK"},
		{tail: 14, head: 1, want: 6, bytes: nil, err: ErrInsufficientBytes, desc: "14,1 insufficient bytes"},
	}

	for i, tt := range tests {
		buf.tail, buf.head = tt.tail, tt.head
		o := make(chan []interface{})
		go func() {
			bs, err := buf.Peek(int64(tt.want))
			o <- []interface{}{bs, err}
		}()

		time.Sleep(time.Millisecond)
		atomic.StoreInt64(&buf.head, buf.head+int64(tt.want))
		time.Sleep(time.Millisecond * 10)

		buf.wcond.L.Lock()
		buf.wcond.Broadcast()
		buf.wcond.L.Unlock()

		time.Sleep(time.Millisecond) // wait for await capacity to actually exit
		done := <-o
		if tt.err != nil {
			require.Error(t, done[1].(error), "Expected Error [i:%d] %s", i, tt.desc)
		} else {
			require.Nil(t, done[1], "Unexpected Error [i:%d] %s", i, tt.desc)
		}

		require.Equal(t, tt.bytes, done[0].([]byte), "Peeked bytes mismatch [i:%d] %s", i, tt.desc)
	}

	fmt.Println("done")

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
		buf := NewBuffer(16)
		buf.buffer = bb
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
		if tt.err != nil {
			require.Error(t, err, "Expected Error [i:%d] %s", i, tt.desc)
		} else {
			require.NoError(t, err, "Unexpected Error [i:%d] %s", i, tt.desc)
		}
		require.Equal(t, tt.bytes, <-recv, "Written bytes mismatch [i:%d] %s", i, tt.desc)

		r.Close()
	}

}
