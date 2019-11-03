package circ

import (
	"io"
	"sync"
	"sync/atomic"

	"github.com/mochi-co/mqtt/debug"
)

var (
	// bufferSize is the default size of the buffer.
	bufferSize int64 = 1024 * 256

	// blockSize is the default size of bytes per R/W block.
	blockSize int64 = 2048
)

// buffer contains core values and methods to be included in a reader or writer.
type buffer struct {
	sync.RWMutex

	// id is the identifier of the buffer. This is used in debug output.
	id string

	// size is the size of the buffer.
	size int64

	// block is the size of the R/W block.
	block int64

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
func newBuffer(size, block int64) buffer {

	if size == 0 {
		size = bufferSize
	}

	if block == 0 {
		block = blockSize
	}
	if size < 2*block {
		size = 2 * block
	}

	return buffer{
		size:  size,
		block: block,
		buf:   make([]byte, size),
		tmp:   make([]byte, size),
		rcond: sync.NewCond(new(sync.Mutex)),
		wcond: sync.NewCond(new(sync.Mutex)),
	}
}

// Close notifies the buffer event loops to stop.
func (b *buffer) Close() {
	atomic.StoreInt64(&b.done, 1)
	debug.Println(b.id, "[B]  STORE done=1 buffer closing")
	b.Bump()
}

// Bump will broadcast all sync conditions.
func (b *buffer) Bump() {
	b.wcond.L.Lock()
	b.wcond.Broadcast()
	b.wcond.L.Unlock()
	b.rcond.L.Lock()
	b.rcond.Broadcast()
	b.rcond.L.Unlock()
}

// Get will return the tail and head positions of the buffer.
// This method is for use with testing.
func (b *buffer) GetPos() (int64, int64) {
	return atomic.LoadInt64(&b.tail), atomic.LoadInt64(&b.head)
}

// Get returns the internal buffer. This method is for use with testing.
// This method is for use with testing.
func (b *buffer) Get() []byte {
	return b.buf
}

// SetPos sets the head and tail of the buffer.
// This method is for use with testing.
func (b *buffer) SetPos(tail, head int64) {
	b.tail, b.head = tail, head
}

// Set writes bytes to a byte buffer.
// This method is for use with testing and will panic if out of range.
func (b *buffer) Set(p []byte, start, end int) {
	o := 0
	for i := start; i < end; i++ {
		b.buf[i] = p[o]
		o++
	}
}

// SetID sets id of the buffer.
func (b *buffer) SetID(id string) {
	b.id = id
}

// awaitCapacity will hold until there are n bytes free in the buffer, blocking
// until there are at least n bytes to write without overrunning the tail.
func (b *buffer) awaitCapacity(n int64) (head int64, err error) {
	head = atomic.LoadInt64(&b.head)
	next := head + n
	wrapped := next - b.size
	tail := atomic.LoadInt64(&b.tail)
	//var tail int64

	for tail = atomic.LoadInt64(&b.tail); wrapped > tail || (tail > head && next > tail && wrapped < 0); tail = atomic.LoadInt64(&b.tail) {
		//fmt.Println("\t", wrapped, ">", tail, wrapped > tail, "||", tail, ">", head, "&&", next, ">", tail, "&&", wrapped, "<", 0, (tail > head && next > tail && wrapped < 0))
		if atomic.LoadInt64(&b.done) == 1 {
			//	b.rcond.L.Unlock() // Make sure we unlock
			return 0, io.EOF
		}

		b.rcond.L.Lock()
		b.rcond.Wait()
		b.rcond.L.Unlock()
	}

	return
}

// awaitFilled will hold until there are at least n bytes waiting between the
// tail and head.
func (b *buffer) awaitFilled(n int64) (tail int64, err error) {
	//head := atomic.LoadInt64(&b.head)
	var head int64
	tail = atomic.LoadInt64(&b.tail)

	for head = atomic.LoadInt64(&b.head); head > tail && tail+n > head || head < tail && b.size-tail+head < n; head = atomic.LoadInt64(&b.head) {
		if atomic.LoadInt64(&b.done) == 1 {
			//	b.wcond.L.Unlock() // Make sure we unlock
			return 0, io.EOF
		}

		b.wcond.L.Lock()
		b.wcond.Wait()
		b.wcond.L.Unlock()
	}

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

// CapDelta returns the difference between the head and tail.
func (b *buffer) CapDelta() int64 {
	tail, head := atomic.LoadInt64(&b.tail), atomic.LoadInt64(&b.head)
	if head < tail {
		return b.size - tail + head
	}

	return head - tail
}
