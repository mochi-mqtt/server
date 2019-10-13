package circ

import (
	"bytes"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewReader(t *testing.T) {
	var size int64 = 16
	buf := NewReader(size)

	require.NotNil(t, buf.buf)
	require.Equal(t, size, int64(len(buf.buf)))
	require.Equal(t, size, buf.size)
}

func TestReadFrom(t *testing.T) {
	buf := NewReader(16)

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

func TestPeek(t *testing.T) {
	buf := NewReader(16)
	buf.buf = []byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p'}

	tests := []struct {
		tail  int64
		head  int64
		want  int
		bytes []byte
		err   error
		desc  string
	}{
		{tail: 0, head: 4, want: 4, bytes: []byte{'a', 'b', 'c', 'd'}, err: nil, desc: "0,4 OK"},
		{tail: 3, head: 15, want: 8, bytes: []byte{'d', 'e', 'f', 'g', 'h', 'i', 'j', 'k'}, err: nil, desc: "3,15 OK"},
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
}

func TestRead(t *testing.T) {
	buf := NewReader(16)
	buf.buf = []byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p'}

	tests := []struct {
		tail  int64
		head  int64
		want  int
		bytes []byte
		err   error
		desc  string
	}{
		{tail: 0, head: 4, want: 4, bytes: []byte{'a', 'b', 'c', 'd'}, err: nil, desc: "0,4 OK"},
		{tail: 3, head: 15, want: 8, bytes: []byte{'d', 'e', 'f', 'g', 'h', 'i', 'j', 'k'}, err: nil, desc: "3,15 OK"},
		{tail: 14, head: 2, want: 6, bytes: []byte{'o', 'p', 'a', 'b', 'c', 'd'}, err: nil, desc: "14,2 wrapped OK"},
	}

	for i, tt := range tests {
		buf.tail, buf.head = tt.tail, tt.head
		o := make(chan []interface{})
		go func() {
			bs, err := buf.Read(int64(tt.want))
			o <- []interface{}{bs, err}
		}()

		atomic.StoreInt64(&buf.head, buf.head+int64(tt.want))

		buf.wcond.L.Lock()
		buf.wcond.Broadcast()
		buf.wcond.L.Unlock()

		time.Sleep(time.Millisecond) // wait for await capacity to actually exit
		done := <-o
		fmt.Println(done)
		fmt.Println(string(done[0].([]byte)))
		require.Nil(t, done[1], "Unexpected Error [i:%d] %s", i, tt.desc)
		require.Equal(t, tt.bytes, done[0].([]byte), "Peeked bytes mismatch [i:%d] %s", i, tt.desc)
	}

}
