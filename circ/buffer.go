package circ

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	//"github.com/mochi-co/mqtt/debug"
)

var (
	// DefaultBufferSize is the default size of the buffer.
	DefaultBufferSize int = 2048

	// DefaultBlockSize is the default size of bytes per R/W block.
	DefaultBlockSize int = 128

	ErrOutOfRange        = fmt.Errorf("Indexes out of range")
	ErrInsufficientBytes = fmt.Errorf("Insufficient bytes to return")
)

// buffer contains core values and methods to be included in a reader or writer.
type Buffer struct {
	mu sync.RWMutex

	// ID is the identifier of the buffer. This is used in debug output.
	ID string

	// size is the size of the buffer.
	size int

	// mask is a bitmask of the buffer size (size-1).
	mask int

	// block is the size of the R/W block.
	block int

	// buffer is the circular byte buffer.
	buf []byte

	// tmp is a temporary buffer.
	tmp []byte

	// head is the current position in the sequence.
	// head is a forever increasing index.
	head int64

	// tail is the committed position in the sequence, typically where we
	// have successfully consumed and processed to.
	// tail is a forever increasing index.
	tail int64

	// rcond is the sync condition for the reader.
	rcond *sync.Cond

	// wcond is the sync condition for the writer.
	wcond *sync.Cond

	// done indicates that the buffer is closed.
	done int64
}

// NewBuffer returns a new instance of buffer. You should call NewReader or
// NewWriter instead of this function.
func NewBuffer(size, block int) Buffer {

	if size == 0 {
		size = DefaultBufferSize
	}

	if block == 0 {
		block = DefaultBlockSize
	}
	if size < 2*block {
		size = 2 * block
	}

	return Buffer{
		size:  size,
		mask:  size - 1,
		block: block,
		buf:   make([]byte, size),
		tmp:   make([]byte, size),
		rcond: sync.NewCond(new(sync.Mutex)),
		wcond: sync.NewCond(new(sync.Mutex)),
	}
}

// Get will return the tail and head positions of the buffer.
// This method is for use with testing.
func (b *Buffer) GetPos() (int64, int64) {
	return atomic.LoadInt64(&b.tail), atomic.LoadInt64(&b.head)
}

// SetPos sets the head and tail of the buffer.
func (b *Buffer) SetPos(tail, head int64) {
	atomic.StoreInt64(&b.tail, tail)
	atomic.StoreInt64(&b.head, head)
}

// Get returns the internal buffer.
func (b *Buffer) Get() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf
}

// Set writes bytes to a range of indexes in the byte buffer.
func (b *Buffer) Set(p []byte, start, end int) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if end > b.size || start > b.size {
		return ErrOutOfRange
	}

	o := 0
	for i := start; i < end; i++ {
		b.buf[i] = p[o]
		o++
	}

	return nil
}

// Index returns the buffer-relative index of an integer.
func (b *Buffer) Index(i int64) int {
	return b.mask & int(i)
}

// awaitCapacity will block until there is at least n bytes between
// the head and the tail (looking forward).
func (b *Buffer) awaitCapacity(n int) error {
	head := atomic.LoadInt64(&b.head)
	tail := atomic.LoadInt64(&b.tail)
	next := head + int64(n)

	// If the head has wrapped behind the tail, and next will overrun tail,
	// then wait until tail has moved.
	b.rcond.L.Lock()
	for tail = atomic.LoadInt64(&b.tail); b.Index(head) < b.Index(tail) && b.Index(next) > b.Index(tail); tail = atomic.LoadInt64(&b.tail) {
		if atomic.LoadInt64(&b.done) == 1 {
			b.rcond.L.Unlock()
			return io.EOF
		}
		b.rcond.Wait()
	}
	b.rcond.L.Unlock()

	return nil
}

// awaitFilled will block until there are at least n bytes between the
// tail and the head (looking forward).
func (b *Buffer) awaitFilled(n int) error {
	head := atomic.LoadInt64(&b.head)
	tail := atomic.LoadInt64(&b.tail)

	// Because awaitCapacity prevents the head from overrunning the t
	// able on write, we can simply ensure there is enough space
	// the forever-incrementing tail and head integers.
	b.wcond.L.Lock()
	for head = atomic.LoadInt64(&b.head); tail+int64(n) > head; head = atomic.LoadInt64(&b.head) {
		if atomic.LoadInt64(&b.done) == 1 {
			b.wcond.L.Unlock()
			return io.EOF
		}

		b.wcond.Wait()
	}
	b.wcond.L.Unlock()

	return nil
}

// CommitTail moves the tail position of the buffer n bytes, waiting
// until there is enough capacity for at least n bytes.
func (b *Buffer) CommitTail(n int) error {
	err := b.awaitFilled(n)
	if err != nil {
		return err
	}

	atomic.AddInt64(&b.tail, int64(n))

	b.rcond.L.Lock()
	b.rcond.Broadcast()
	b.rcond.L.Unlock()

	return nil
}

// CapDelta returns the difference between the head and tail.
func (b *Buffer) CapDelta() int {
	return int(atomic.LoadInt64(&b.head) - atomic.LoadInt64(&b.tail))
}
