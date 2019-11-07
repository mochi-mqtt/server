package circ

import (
	"errors"
	"io"
	"sync/atomic"
	//"time"
	//"fmt"

	"github.com/mochi-co/mqtt/debug"
)

//	debug.Printf("%s \033[1;35mREAD PACKET %d:%d n%d %+v \033[0m\n", b.id, start, end, n, string(b.buf[start:end]))

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

// Peek returns the next n bytes without advancing the reader.
func (b *Reader) Peek(n int64) ([]byte, error) {
	tail := atomic.LoadInt64(&b.tail)
	head := atomic.LoadInt64(&b.head)

	// Wait until there's at least 1 byte of data.
	b.wcond.L.Lock()
	for head = atomic.LoadInt64(&b.head); head == atomic.LoadInt64(&b.tail); head = atomic.LoadInt64(&b.head) {
		if atomic.LoadInt64(&b.done) == 1 {
			b.wcond.L.Unlock() // make sure we unlock
			return nil, io.EOF
		}
		b.wcond.Wait()
	}
	b.wcond.L.Unlock()

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

	// If we've wrapped, get the bytes from the next and start.
	if head < tail {
		b.tmp = b.buf[tail:b.size]
		b.tmp = append(b.tmp, b.buf[:(tail+d)%b.size]...)
		//fmt.Println(b.id, "PEEKBUFWRP", b.tmp)
		return b.tmp, nil
	}
	//fmt.Println(b.id, "PEEKBUFFER", b.buf[tail:tail+d])
	return b.buf[tail : tail+d], nil
}

// Read reads the next n bytes from the buffer. If n bytes are not
// available, read will wait until there is enough.
func (b *Reader) Read(n int64) (p []byte, err error) {
	if n > b.size {
		err = ErrInsufficientBytes
		return
	}

	// Wait until there's at least n bytes to read.
	tail, err := b.awaitFilled(n)
	if err != nil {
		return
	}

	// Once we have capacity, determine if the capacity wraps,
	// and write it into the buffer p.
	next := tail + n
	if next > b.size {
		b.tmp = b.buf[tail:b.size]
		b.tmp = append(b.tmp, b.buf[:next%b.size]...)
		atomic.StoreInt64(&b.tail, next%b.size)
		return b.tmp, nil
	} else {
		atomic.StoreInt64(&b.tail, next)
		return b.buf[tail:next], nil
	}

	return
}
