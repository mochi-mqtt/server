package circ

import (
	"io"
	"sync/atomic"
)

// Reader is a circular buffer for reading data from an io.Reader.
type Reader struct {
	*Buffer
}

// NewReader returns a new Circular Reader.
func NewReader(size, block int) *Reader {
	b := NewBuffer(size, block)
	b.ID = "\treader"
	return &Reader{
		b,
	}
}

// NewReaderFromSlice returns a new Circular Reader using a pre-existing
// byte slice.
func NewReaderFromSlice(block int, p []byte) *Reader {
	b := NewBufferFromSlice(block, p)
	b.ID = "\treader"
	return &Reader{
		b,
	}
}

// ReadFrom reads bytes from an io.Reader and commits them to the buffer when
// there is sufficient capacity to do so.
func (b *Reader) ReadFrom(r io.Reader) (total int64, err error) {
	atomic.StoreUint32(&b.State, 1)
	defer atomic.StoreUint32(&b.State, 0)
	for {
		if atomic.LoadUint32(&b.done) == 1 {
			return total, nil
		}

		// Wait until there's enough capacity in the buffer before
		// trying to read more bytes from the io.Reader.
		err := b.awaitEmpty(b.block)
		if err != nil {
			// b.done is the only error condition for awaitCapacity
			// so loop around and return properly.
			continue
		}

		// If the block will overrun the circle end, just fill up
		// and collect the rest on the next pass.
		start := b.Index(atomic.LoadInt64(&b.head))
		end := start + b.block
		if end > b.size {
			end = b.size
		}

		// Read into the buffer between the start and end indexes only.
		n, err := r.Read(b.buf[start:end])
		total += int64(n) // incr total bytes read.
		if err != nil {
			return total, err
		}

		// Move the head forward however many bytes were read.
		atomic.AddInt64(&b.head, int64(n))

		b.wcond.L.Lock()
		b.wcond.Broadcast()
		b.wcond.L.Unlock()
	}
}

// Read reads n bytes from the buffer, and will block until at n bytes
// exist in the buffer to read.
func (b *Buffer) Read(n int) (p []byte, err error) {
	err = b.awaitFilled(n)
	if err != nil {
		return
	}

	tail := atomic.LoadInt64(&b.tail)
	next := tail + int64(n)

	// If the read overruns the buffer, get everything until the end
	// and then whatever is left from the start.
	if b.Index(tail) > b.Index(next) {
		b.tmp = b.buf[b.Index(tail):]
		b.tmp = append(b.tmp, b.buf[:b.Index(next)]...)
	} else {
		b.tmp = b.buf[b.Index(tail):b.Index(next)] // Otherwise, simple tail:next read.
	}

	return b.tmp, nil
}
