package circ

import (
	"errors"
	"io"
	"sync/atomic"
)

var (
	ErrInsufficientBytes = errors.New("Insufficient bytes to return")
)

// Reader is a circular buffer for reading data from an io.Reader.
type Reader struct {
	buffer
}

// NewReader returns a pointer to a new Circular Reader.
func NewReader(size int64) *Reader {
	return &Reader{
		newBuffer(size),
	}
}

// ReadFrom reads bytes from an io.Reader and commits them to the buffer when
// there is sufficient capacity to do so.
func (b *Reader) ReadFrom(r io.Reader) (total int64, err error) {
DONE:
	for {
		if atomic.LoadInt64(&b.done) == 1 {
			err = io.EOF
			break DONE
		}

		// Wait until there's enough capacity in the buffer.
		st, err := b.awaitCapacity(blockSize)
		if err != nil {
			break DONE
		}

		// Find the start and end indexes within the buffer.
		start := st & (b.size - 1)
		end := start + blockSize

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

		// Move the head forward.
		atomic.StoreInt64(&b.head, end)
		b.wcond.L.Lock()
		b.wcond.Broadcast()
		b.wcond.L.Unlock()
	}

	return
}

// Peek returns the next n bytes without advancing the reader.
func (b *Reader) Peek(n int64) ([]byte, error) {
	tail := atomic.LoadInt64(&b.tail)
	head := atomic.LoadInt64(&b.head)

	// Wait until there's at least 1 byte of data.
	/*
		b.wcond.L.Lock()
		for ; head == tail; head = atomic.LoadInt64(&b.head) {
			if atomic.LoadInt64(&b.done) == 1 {
				return nil, io.EOF
			}
			b.wcond.Wait()
		}
		b.wcond.L.Unlock()
	*/

	// Figure out if we can get all n bytes.
	if head == tail || head > tail && tail+n > head || head < tail && b.size-tail+head < n {
		return nil, ErrInsufficientBytes
	}

	// If we've wrapped, get the bytes from the next and start.
	if head < tail {
		b.tmp = b.buf[tail:b.size]
		b.tmp = append(b.tmp, b.buf[:(tail+n)%b.size]...)
		return b.tmp, nil
	} else {
		return b.buf[tail : tail+n], nil
	}

	return nil, nil
}

// PeekWait waits until their are n bytes available to return.
func (b *Reader) PeekWait(n int64) ([]byte, error) {
	tail := atomic.LoadInt64(&b.tail)
	head := atomic.LoadInt64(&b.head)

	// Wait until there's at least 1 byte of data.
	b.wcond.L.Lock()
	for ; head == tail; head = atomic.LoadInt64(&b.head) {
		if atomic.LoadInt64(&b.done) == 1 {
			return nil, io.EOF
		}
		b.wcond.Wait()
	}
	b.wcond.L.Unlock()

	// Figure out if we can get all n bytes.
	if head > tail && tail+n > head || head < tail && b.size-tail+head < n {
		return nil, ErrInsufficientBytes
	}

	// If we've wrapped, get the bytes from the next and start.
	if head < tail {
		b.tmp = b.buf[tail:b.size]
		b.tmp = append(b.tmp, b.buf[:(tail+n)%b.size]...)
		return b.tmp, nil
	} else {
		return b.buf[tail : tail+n], nil
	}

	return nil, nil
}

// Read reads the next n bytes from the buffer. If n bytes are not
// available, read will wait until there is enough.
func (b *Reader) Read(n int64) (p []byte, err error) {

	// Wait until there's at least len(p) bytes to read.
	tail, err := b.awaitFilled(n)
	if err != nil {
		return
	}

	// Once we have capacity, determine if the capacity wraps, and write it into
	// the buffer p.
	if atomic.LoadInt64(&b.head) < tail {
		b.tmp = b.buf[tail:b.size]
		b.tmp = append(b.tmp, b.buf[:(tail+n)%b.size]...)
		return b.tmp, nil
	} else {
		return b.buf[tail : tail+n], nil
	}

	return
}
