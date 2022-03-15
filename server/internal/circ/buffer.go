package circ

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"
)

var (
	// DefaultBufferSize is the default size of the buffer in bytes.
	DefaultBufferSize int = 1024 * 256

	// DefaultBlockSize is the default size per R/W block in bytes.
	DefaultBlockSize int = 1024 * 8

	// ErrOutOfRange indicates that the index was out of range.
	ErrOutOfRange = errors.New("Indexes out of range")

	// ErrInsufficientBytes indicates that there were not enough bytes to return.
	ErrInsufficientBytes = errors.New("Insufficient bytes to return")
)

// Buffer is a circular buffer for reading and writing messages.
type Buffer struct {
	buf   []byte       // the bytes buffer.
	tmp   []byte       // a temporary buffer.
	Mu    sync.RWMutex // the buffer needs its own mutex to work properly.
	ID    string       // the identifier of the buffer. This is used in debug output.
	head  int64        // the current position in the sequence - a forever increasing index.
	tail  int64        // the committed position in the sequence - a forever increasing index.
	rcond *sync.Cond   // the sync condition for the buffer reader.
	wcond *sync.Cond   // the sync condition for the buffer writer.
	size  int          // the size of the buffer.
	mask  int          // a bitmask of the buffer size (size-1).
	block int          // the size of the R/W block.
	done  uint32       // indicates that the buffer is closed.
	State uint32       // indicates whether the buffer is reading from (1) or writing to (2).
}

// NewBuffer returns a new instance of buffer. You should call NewReader or
// NewWriter instead of this function.
func NewBuffer(size, block int) *Buffer {
	if size == 0 {
		size = DefaultBufferSize
	}

	if block == 0 {
		block = DefaultBlockSize
	}

	if size < 2*block {
		size = 2 * block
	}

	return &Buffer{
		size:  size,
		mask:  size - 1,
		block: block,
		buf:   make([]byte, size),
		rcond: sync.NewCond(new(sync.Mutex)),
		wcond: sync.NewCond(new(sync.Mutex)),
	}
}

// NewBufferFromSlice returns a new instance of buffer using a
// pre-existing byte slice.
func NewBufferFromSlice(block int, buf []byte) *Buffer {
	l := len(buf)

	if block == 0 {
		block = DefaultBlockSize
	}

	b := &Buffer{
		size:  l,
		mask:  l - 1,
		block: block,
		buf:   buf,
		rcond: sync.NewCond(new(sync.Mutex)),
		wcond: sync.NewCond(new(sync.Mutex)),
	}

	return b
}

// GetPos will return the tail and head positions of the buffer.
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
	b.Mu.Lock()
	defer b.Mu.Unlock()
	return b.buf
}

// Set writes bytes to a range of indexes in the byte buffer.
func (b *Buffer) Set(p []byte, start, end int) error {
	b.Mu.Lock()
	defer b.Mu.Unlock()

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

// awaitEmpty will block until there is at least n bytes between
// the head and the tail (looking forward).
func (b *Buffer) awaitEmpty(n int) error {
	// If the head has wrapped behind the tail, and next will overrun tail,
	// then wait until tail has moved.
	b.rcond.L.Lock()
	for !b.checkEmpty(n) {
		if atomic.LoadUint32(&b.done) == 1 {
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
	// Because awaitCapacity prevents the head from overrunning the t
	// able on write, we can simply ensure there is enough space
	// the forever-incrementing tail and head integers.
	b.wcond.L.Lock()
	for !b.checkFilled(n) {
		if atomic.LoadUint32(&b.done) == 1 {
			b.wcond.L.Unlock()
			return io.EOF
		}

		b.wcond.Wait()
	}
	b.wcond.L.Unlock()

	return nil
}

// checkEmpty returns true if there are at least n bytes between the head and
// the tail.
func (b *Buffer) checkEmpty(n int) bool {
	head := atomic.LoadInt64(&b.head)
	next := head + int64(n)
	tail := atomic.LoadInt64(&b.tail)
	if next-tail > int64(b.size) {
		return false
	}

	return true
}

// checkFilled returns true if there are at least n bytes between the tail and
// the head.
func (b *Buffer) checkFilled(n int) bool {
	if atomic.LoadInt64(&b.tail)+int64(n) <= atomic.LoadInt64(&b.head) {
		return true
	}

	return false
}

// CommitTail moves the tail position of the buffer n bytes.
func (b *Buffer) CommitTail(n int) {
	atomic.AddInt64(&b.tail, int64(n))
	b.rcond.L.Lock()
	b.rcond.Broadcast()
	b.rcond.L.Unlock()
}

// CapDelta returns the difference between the head and tail.
func (b *Buffer) CapDelta() int {
	return int(atomic.LoadInt64(&b.head) - atomic.LoadInt64(&b.tail))
}

// Stop signals the buffer to stop processing.
func (b *Buffer) Stop() {
	atomic.StoreUint32(&b.done, 1)
	b.rcond.L.Lock()
	b.rcond.Broadcast()
	b.rcond.L.Unlock()
	b.wcond.L.Lock()
	b.wcond.Broadcast()
	b.wcond.L.Unlock()
}
