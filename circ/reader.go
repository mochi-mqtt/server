package circ

import (
	"errors"
	"fmt"
	"io"
	"sync/atomic"

	"github.com/mochi-co/mqtt/debug"
)

var (
	ErrInsufficientBytes = errors.New("Insufficient bytes to return")
)

// Reader is a circular buffer for reading data from an io.Reader.
type Reader struct {
	buffer
}

// NewReader returns a pointer to a new Circular Reader.
func NewReader(size, block int64) *Reader {
	b := newBuffer(size, block)
	b.id = "\treader"
	return &Reader{
		b,
	}
}

// ReadFrom reads bytes from an io.Reader and commits them to the buffer when
// there is sufficient capacity to do so.
func (b *Reader) ReadFrom(r io.Reader) (total int64, err error) {
DONE:
	for {
		if atomic.LoadInt64(&b.done) == 1 {
			err = io.EOF
			debug.Println(b.id, "*[R] DONE BREAK")
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
			debug.Println(b.id, "*[R] READ ERR", err)
			break DONE
		}

		debug.Println(b.id, "RECV", n, start, end)
		debug.Println(b.id, "RECVBUF", b.buf[:end])

		// Move the head forward.
		atomic.StoreInt64(&b.head, start+int64(n))

		debug.Println(b.id, "RECVHEAD", atomic.LoadInt64(&b.head))

		b.wcond.L.Lock()
		b.wcond.Broadcast()
		b.wcond.L.Unlock()
	}

	return
}

// Peek returns the next n bytes without advancing the reader.
func (b *Reader) Peek(n int64) ([]byte, error) {
	debug.Println(b.id, "\033[1;35m[R] n", n, "\033[0m")

	tail := atomic.LoadInt64(&b.tail)
	head := atomic.LoadInt64(&b.head)

	// Wait until there's at least 1 byte of data.
	debug.Println(b.id, "\033[1;34m[R] BEGIN PEEKING ", tail, head, "\033[0m")
	for head = atomic.LoadInt64(&b.head); head == atomic.LoadInt64(&b.tail); head = atomic.LoadInt64(&b.head) {
		if atomic.LoadInt64(&b.done) == 1 {
			//b.wcond.L.Unlock() // make sure we unlock
			return nil, io.EOF
		}
		b.wcond.L.Lock()
		b.wcond.Wait()
		b.wcond.L.Unlock()
	}
	debug.Println(b.id, "\033[1;34m[R] Peek unlocked", tail, head, "\033[0m")

	// Figure out if we can get all n bytes.
	//if head == tail || head > tail && tail+n > head || head < tail && b.size-tail+head < n {
	if head == tail || head < tail && b.size-tail+head < n {
		return nil, ErrInsufficientBytes
	}

	var d int64
	if head < tail {
		d = b.size - tail + head
	} else {
		d = head - tail
	}

	// Peek maximum of n bytes.
	if d > n {
		d = n
	}

	debug.Println(b.id, "\033[1;35m[R] d", d, "\033[0m")

	// If we've wrapped, get the bytes from the next and start.
	if head < tail {
		b.tmp = b.buf[tail:b.size]
		b.tmp = append(b.tmp, b.buf[:(tail+d)%b.size]...)
		debug.Println(b.id, "\033[1;35m[R] PEEKED WRAP", tail, (tail+d)%b.size, "\033[0m")

		return b.tmp, nil
	}

	debug.Println(b.id, "\033[1;35m[R] PEEKED", tail, tail+d, "\033[0m")
	debug.Println(b.id, "\033[1;35m[R] BUFFER", b.buf[tail:tail+d], "\033[0m")
	debug.Println(b.id, "\033[1;35m[R] SBUF", string(b.buf[tail:tail+d]), "\033[0m")

	return b.buf[tail : tail+d], nil

}

// Read reads the next n bytes from the buffer. If n bytes are not
// available, read will wait until there is enough.
func (b *Reader) Read(n int64) (p []byte, err error) {
	if n > b.size {
		err = ErrInsufficientBytes
		return
	}

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
		atomic.StoreInt64(&b.tail, (tail+n)%b.size)
		return b.tmp, nil
	} else if tail+n < b.size {
		atomic.StoreInt64(&b.tail, tail+n)
		return b.buf[tail : tail+n], nil
	} else {
		return p, fmt.Errorf("next byte extended size")
	}

	return
}
