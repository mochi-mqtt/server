package circ

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

var (
	blockSize int64 = 8192
)

// buffer contains core values and methods to be included in a reader or writer.
type buffer struct {

	// size is the size of the buffer.
	size int64

	// buffer is the circular byte buffer.
	buf []byte

	// tmp is a temporary buffer.
	tmp []byte

	// head is the current position in the sequence.
	head int64

	// tail is the committed position in the sequence, I.E. where we
	// have successfully consumed and processed to.
	tail int64

	// rcond is the sync condition for the reader.
	rcond *sync.Cond

	// done indicates that the buffer is closed.
	done int64

	// wcond is the sync condition for the writer.
	wcond *sync.Cond
}

// newBuffer returns a new instance of buffer.
func newBuffer(size int64) buffer {
	return buffer{
		size:  size,
		buf:   make([]byte, size),
		tmp:   make([]byte, size),
		rcond: sync.NewCond(new(sync.Mutex)),
		wcond: sync.NewCond(new(sync.Mutex)),
	}
}

// awaitCapacity will hold until there are n bytes free in the buffer, blocking
// until there are at least n bytes to write without overrunning the tail.
func (b *buffer) awaitCapacity(n int64) (int64, error) {
	tail := atomic.LoadInt64(&b.tail)
	head := atomic.LoadInt64(&b.head)
	next := head + n
	wrapped := next - b.size

	b.rcond.L.Lock()
	for ; wrapped > tail || (tail > head && next > tail && wrapped < 0); tail = atomic.LoadInt64(&b.tail) {
		//fmt.Println("\t", wrapped, ">", tail, wrapped > tail, "||", tail, ">", head, "&&", next, ">", tail, "&&", wrapped, "<", 0, (tail > head && next > tail && wrapped < 0))
		b.rcond.Wait()
		if atomic.LoadInt64(&b.done) == 1 {
			fmt.Println("ending")
			return 0, io.EOF
		}
	}
	b.rcond.L.Unlock()

	return head, nil
}
