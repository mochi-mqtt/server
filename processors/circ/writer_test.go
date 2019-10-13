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
		buf := NewWriter(16)
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

		//require.NoError(t, err)
		/*require.Equal(t, tt.next, buf.tail, "Tail placement mismatched [i:%d] %s", i, tt.desc)
		require.Equal(t, tt.total, total, "Total bytes written mismatch [i:%d] %s", i, tt.desc)
		if tt.err != nil {
			require.Error(t, err, "Expected Error [i:%d] %s", i, tt.desc)
		} else {
			require.NoError(t, err, "Unexpected Error [i:%d] %s", i, tt.desc)
		}

		*/
		r.Close()
	}

}
