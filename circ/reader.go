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
	debug.Println(b.id, "*[R] STARTING SPIN")
DONE:
	for {
		debug.Println(b.id, "*[R] SPIN")
		if atomic.LoadInt64(&b.done) == 1 {
			err = io.EOF
			debug.Println(b.id, "*[R] readFrom DONE")
			break DONE
		}

		// Wait until there's enough capacity in the buffer.
		st, err := b.awaitCapacity(b.block)
		if err != nil {
			debug.Println(b.id, "*[R] readFrom await capacity error, ", err)
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
		debug.Println(b.id, "*[R] readFrom mapping (start, end)", start, end)
		n, err := r.Read(b.buf[start:end])
		total += int64(n) // incr total bytes read.
		if err != nil {
			debug.Println("*[R] r.READ error", err)
			break DONE
		}
		debug.Println(b.id, "*[R] READ (start, end, n)", start, n, b.buf[start:start+int64(n)])

		// Move the head forward.
		debug.Println(b.id, "*[R] STORE HEAD", start+int64(n))
		atomic.StoreInt64(&b.head, start+int64(n))

		debug.Println(b.id, "##### *[R] wcond.Locking")
		b.wcond.L.Lock()
		debug.Println(b.id, "##### *[R] wcond.Locked")
		debug.Println(b.id, "##### *[R] wcond.Broadcasting")
		b.wcond.Broadcast()
		debug.Println(b.id, "##### *[R] wcond.Broadcasted")
		debug.Println(b.id, "##### *[R] wcond.Unlocking")
		b.wcond.L.Unlock()
		debug.Println(b.id, "##### *[R] wcond.Unlocked")
	}

	debug.Println(b.id, "*[R] FINISHED SPIN", err)

	return
}

// Peek returns the next n bytes without advancing the reader.
func (b *Reader) Peek(n int64) ([]byte, error) {
	debug.Println(b.id, "[R] START PEEKING (n)", n)

	tail := atomic.LoadInt64(&b.tail)
	var head int64
	//head := atomic.LoadInt64(&b.head)

	// Wait until there's at least 1 byte of data.
	debug.Println(b.id, "[R] PEEKING (tail, head, n)", tail, head, n, "/", atomic.LoadInt64(&b.head))

	debug.Println(b.id, "##### [R] wcond.Locking")
	b.wcond.L.Lock()
	debug.Println(b.id, "##### [R] wcond.Locked")
	for head = atomic.LoadInt64(&b.head); head == atomic.LoadInt64(&b.tail); head = atomic.LoadInt64(&b.head) {
		debug.Println(b.id, "[R] PEEKING insufficient (tail, head)", tail, head)
		if atomic.LoadInt64(&b.done) == 1 {
			debug.Println(b.id, "************ [R] PEEK caught DONE")
			b.wcond.L.Unlock() // make sure we unlock
			return nil, io.EOF
		}
		debug.Println(b.id, "##### [R] wcond.Waiting")
		b.wcond.Wait()
		debug.Println(b.id, "##### [R] wcond.Waited")

	}
	debug.Println(b.id, "##### [R] wcond.Unlocking")
	b.wcond.L.Unlock()
	debug.Println(b.id, "##### [R] wcond.Unlocked")
	debug.Println(b.id, "[R] PEEKING available")

	// Figure out if we can get all n bytes.
	//if head == tail || head > tail && tail+n > head || head < tail && b.size-tail+head < n {
	if head == tail || head < tail && b.size-tail+head < n {
		debug.Println(b.id, "[R] insufficient bytes (tail, next, head)", tail, tail+n, head)
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

	// If we've wrapped, get the bytes from the next and start.
	if head < tail {
		b.tmp = b.buf[tail:b.size]
		b.tmp = append(b.tmp, b.buf[:(tail+d)%b.size]...)
		debug.Println(b.id, "[R] PEEKED wrapped (tail, next, buf)", tail, (tail+d)%b.size, b.tmp)
		return b.tmp, nil
	}

	debug.Println(b.id, "[R] PEEKED (tail, next, buf)", tail, tail+d, b.buf[tail:tail+d])
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
	debug.Println(b.id, "[R] awaiting read fill, (n)", n)
	tail, err := b.awaitFilled(n)
	if err != nil {
		debug.Println(b.id, "[R] filled err (tail, err)", tail, err)
		return
	}

	debug.Println(b.id, "[R] filled (tail, next)", tail, tail+n)

	// Once we have capacity, determine if the capacity wraps, and write it into
	// the buffer p.
	if atomic.LoadInt64(&b.head) < tail {
		b.tmp = b.buf[tail:b.size]
		b.tmp = append(b.tmp, b.buf[:(tail+n)%b.size]...)
		atomic.StoreInt64(&b.tail, (tail+n)%b.size)
		debug.Println(b.id, "[R] READ wrapped", b.tmp)
		debug.Println(b.id, "[R] STORE TAIL wrapped", (tail+n)%b.size)
		return b.tmp, nil
	} else if tail+n < b.size {
		atomic.StoreInt64(&b.tail, tail+n)
		debug.Println(b.id, "[R] READ", b.buf[tail:tail+n])
		debug.Println(b.id, "[R] STORE TAIL", tail+n)
		return b.buf[tail : tail+n], nil
	} else {
		return p, fmt.Errorf("next byte extended size")
	}

	return
}
