package ring

import (
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

var (
	blockSize int64 = 4

	ErrInsufficientBytes = errors.New("Insufficient bytes to return")
)

//   _______________________________________________________________________
//   | 0 | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10 | 11 | 12 | 13 | 14 | 15 |
//   -----------------------------------------------------------------------

// Buffer is a ring-style byte buffer.
type Buffer struct {

	// size is the size of the buffer.
	size int64

	// buffer is the circular byte buffer.
	buffer []byte

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

// NewBuffer returns a pointer to a new instance of Buffer.
func NewBuffer(size int64) *Buffer {
	return &Buffer{
		size:   size,
		buffer: make([]byte, size),
		tmp:    make([]byte, size),
		rcond:  sync.NewCond(new(sync.Mutex)),
		wcond:  sync.NewCond(new(sync.Mutex)),
	}
}

// awaitCapacity will hold until there are n bytes free in the buffer.
// awaitCapacity will block until there are at least n bytes for the head to
// write into without overrunning the tail.
func (b *Buffer) awaitCapacity(n int64) (int64, error) {
	tail := atomic.LoadInt64(&b.tail)
	head := atomic.LoadInt64(&b.head)
	next := head + n
	wrapped := next - b.size

	b.rcond.L.Lock()
	for ; wrapped > tail || (tail > head && next > tail && wrapped < 0); tail = atomic.LoadInt64(&b.tail) {
		//fmt.Println("\t", wrapped, ">", tail, wrapped > tail, "||", tail, ">", head, "&&", next, ">", tail, "&&", wrapped, "<", 0, (tail > head && next > tail && wrapped < 0))
		b.rcond.Wait()
		if atomic.LoadInt64(&b.done) == 1 {
			fmt.Println("ending")
			return 0, io.EOF
		}
	}
	b.rcond.L.Unlock()

	return head, nil
}

// ReadFrom reads bytes from an io.Reader and commits them to the buffer when
// there is sufficient capacity to do so.
func (b *Buffer) ReadFrom(r io.Reader) (total int64, err error) {
	for {
		if atomic.LoadInt64(&b.done) == 1 {
			return 0, io.EOF
		}

		// Wait until there's enough capacity in the buffer.
		st, err := b.awaitCapacity(blockSize)
		if err != nil {
			return 0, err
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
		n, err := r.Read(b.buffer[start:end])
		if err != nil {
			return int64(n), err
		}

		// Move the head forward.
		//fmt.Println(start, end, b.buffer[start:end])
		// fmt.Println(">>", b.head, start+int64(n), end)
		atomic.StoreInt64(&b.head, end) //start+int64(n))
		b.wcond.L.Lock()
		b.wcond.Broadcast()
		b.wcond.L.Unlock()

		total += int64(n) // incr total bytes read.
	}

	return
}

// WriteTo writes the contents of the buffer to an io.Writer.
func (b *Buffer) WriteTo(w io.Writer) (total int64, err error) {
	var p []byte
	var n int
DONE:
	for {
		if atomic.LoadInt64(&b.done) == 1 {
			err = io.EOF
			break DONE
		}

		p, err = b.Peek(blockSize)
		if err != nil {
			break DONE
		}

		n, err = w.Write(p)
		total += int64(n)
		if err != nil {
			break DONE
		}

		end := (atomic.LoadInt64(&b.tail) + int64(n)) % b.size
		atomic.StoreInt64(&b.tail, end)
		b.rcond.L.Lock()
		b.rcond.Broadcast()
		b.rcond.L.Unlock()
	}

	return
}

// Peek returns the next n bytes without advancing the reader.
func (b *Buffer) Peek(n int64) ([]byte, error) {
	tail := atomic.LoadInt64(&b.tail)
	head := atomic.LoadInt64(&b.head)

	// Wait until there's at least 1 byte of data.
	b.wcond.L.Lock()
	for ; head == tail; head = atomic.LoadInt64(&b.head) {
		if atomic.LoadInt64(&b.done) == 1 {
			fmt.Println("ending")
			return nil, io.EOF
		}
		b.wcond.Wait()
	}
	b.wcond.L.Unlock()

	// Figure out if we can get all n bytes.
	start := tail
	end := tail + n
	if head > tail && tail+n > head || head < tail && b.size-tail+head < n {
		//fmt.Println("row wanted overran capacity")
		return nil, ErrInsufficientBytes
	}

	if head < tail {
		//fmt.Println("alt logic, wrapped", start, ":", b.size, "---", 0, ":", n-(end-start), "//", end%b.size)
		//tmp = append(tmp, b.buffer[start:b.size]...)
		b.tmp = b.buffer[start:b.size]
		b.tmp = append(b.tmp, b.buffer[:(end%b.size)]...) //tmp = append(tmp, b.buffer[:n-(end-start)]...)
		fmt.Println("#", string(b.tmp))
		return b.tmp, nil
	} else {
		//fmt.Println("*", string(b.buffer[start:end]))
		return b.buffer[start:end], nil
	}

	return nil, nil
}
