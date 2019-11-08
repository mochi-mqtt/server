package circ

import (
	"io"
	"sync/atomic"
)

// Reader is a circular buffer for reading data from an io.Reader.
type Reader struct {
	Buffer
}

// NewReader returns a pointer to a new Circular Reader.
func NewReader(size, block int) *Reader {
	b := NewBuffer(size, block)
	b.ID = "\treader"
	return &Reader{
		b,
	}
}

// ReadFrom reads bytes from an io.Reader and commits them to the buffer when
// there is sufficient capacity to do so.
func (b *Reader) ReadFrom(r io.Reader) (total int64, err error) {
	for {
		if atomic.LoadInt64(&b.done) == 1 {
			return total, nil
		}

		// Wait until there's enough capacity in the buffer before
		// trying to read more bytes from the io.Reader.
		err := b.awaitCapacity(b.block)
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
			return total, nil
		}

		// Move the head forward however many bytes were read.
		atomic.AddInt64(&b.head, int64(n))

		b.wcond.L.Lock()
		b.wcond.Broadcast()
		b.wcond.L.Unlock()
	}
}

/*
func (b *Reader) ReadFrom(r io.Reader) (total int64, err error) {
DONE:
	for {
		if atomic.LoadInt64(&b.done) == 1 {
			err = io.EOF
			break DONE
		}

		// Wait until there's enough capacity in the buffer.
		st, err := b.awaitCapacity(b.block)
		if err != nil {
			continue
		}

		// Find the start and end indexes within the buffer.
		start := st & (b.size - 1)
		end := start + b.block

		// If we've overrun, just fill up to the end and then collect
		// the rest on the next pass.
		if end > b.size {
			end = b.size
		}

		// Read into the buffer between the start and end indexes only.
		n, err := r.Read(b.buf[start:end])
		total += int64(n) // incr total bytes read.
		if err != nil {
			break DONE
		}

		next := start + int64(n)

		//time.Sleep(time.Millisecond * 100)

		// Move the head forward.
		atomic.StoreInt64(&b.head, next)

		b.wcond.L.Lock()
		b.wcond.Broadcast()
		b.wcond.L.Unlock()
	}

	return
}
*/
