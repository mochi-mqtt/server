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

// Get will return the tail and head positions of the buffer.
func (b *buffer) GetPos() (int64, int64) {
	return atomic.LoadInt64(&b.tail), atomic.LoadInt64(&b.head)
}

// Set writes bytes to a byte buffer. This method should only be used for testing
// and will panic if out of range.
func (b *buffer) Set(p []byte, start, end int) {
	o := 0
	for i := start; i < end; i++ {
		b.buf[i] = p[o]
		o++
	}
}

// SetPos sets the head and tail of the buffer. This method should only be
// used for testing.
func (b *buffer) SetPos(tail, head int64) {
	b.tail, b.head = tail, head
}

// awaitCapacity will hold until there are n bytes free in the buffer, blocking
// until there are at least n bytes to write without overrunning the tail.
func (b *buffer) awaitCapacity(n int64) (head int64, err error) {
	head = atomic.LoadInt64(&b.head)
	next := head + n
	wrapped := next - b.size
	tail := atomic.LoadInt64(&b.tail)

	b.rcond.L.Lock()
	for ; wrapped > tail || (tail > head && next > tail && wrapped < 0); tail = atomic.LoadInt64(&b.tail) {
		//fmt.Println("\t", wrapped, ">", tail, wrapped > tail, "||", tail, ">", head, "&&", next, ">", tail, "&&", wrapped, "<", 0, (tail > head && next > tail && wrapped < 0))
		if atomic.LoadInt64(&b.done) == 1 {
			fmt.Println("ending")
			return 0, io.EOF
		}
		b.rcond.Wait()
	}
	b.rcond.L.Unlock()

	return
}

// awaitFilled will hold until there are at least n bytes waiting between the
// tail and head.
func (b *buffer) awaitFilled(n int64) (tail int64, err error) {
	head := atomic.LoadInt64(&b.head)
	tail = atomic.LoadInt64(&b.tail)

	b.wcond.L.Lock()
	for ; head > tail && tail+n > head || head < tail && b.size-tail+head < n; head = atomic.LoadInt64(&b.head) {
		if atomic.LoadInt64(&b.done) == 1 {
			fmt.Println("ending")
			return 0, io.EOF
		}
		b.wcond.Wait()
	}
	b.wcond.L.Unlock()

	return
}

// CommitHead moves the head position of the buffer n bytes. If there is not enough
// capacity, the method will wait until there is.
//func (b *buffer) CommitHead(n int64) error {
//	return nil
//}

// CommitTail moves the tail position of the buffer n bytes, and will wait until
// there is enough capacity for at least n bytes.
func (b *buffer) CommitTail(n int64) error {
	_, err := b.awaitFilled(n)
	if err != nil {
		return err
	}

	tail := atomic.LoadInt64(&b.tail)
	if tail+n < b.size {
		atomic.StoreInt64(&b.tail, tail+n)
	} else {
		atomic.StoreInt64(&b.tail, (tail+n)%b.size)
	}

	b.rcond.L.Lock()
	b.rcond.Broadcast()
	b.rcond.L.Unlock()

	return nil
}
