package ring

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

var (
	blockSize int64 = 4
)

// Buffer is a ring-style byte buffer.
type Buffer struct {

	// size is the size of the buffer.
	size int64

	// sizeMask is size-1, and is used as a cheaper alternative to modulo.
	// instead of calculating `(start+next) % size`, we can apply the mask
	// and use `(start+next) & sizeMask`.
	//sizeMask int64

	// buffer is the circular byte buffer.
	buffer []byte

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
	//wcond *sync.Cond
}

// NewBuffer returns a pointer to a new instance of Buffer.
func NewBuffer(size int64) *Buffer {
	return &Buffer{
		size: size,
		//	sizeMask: size - 1,
		buffer: make([]byte, size),
		rcond:  sync.NewCond(new(sync.Mutex)),
		//wcond:  sync.NewCond(new(sync.Mutex)),
	}
}

// awaitCapacity will hold until there are n bytes free in the buffer.
func (b *Buffer) awaitCapacity(n int64) (int64, error) {
	tail := atomic.LoadInt64(&b.tail)
	head := atomic.LoadInt64(&b.head)
	next := head + n
	wrapped := next - b.size
	//fmt.Println(tail, head, next, wrapped)

	for ; wrapped > tail || (tail > head && next > tail && wrapped < 0); tail = atomic.LoadInt64(&b.tail) {
		/*
			fmt.Println("\t",
				wrapped, ">", tail, wrapped > tail,
				"||",
				tail, ">", head, "&&", next, ">", tail, "&&", wrapped, "<", 0, (tail > head && next > tail && wrapped < 0))
		*/
		b.rcond.L.Lock()
		b.rcond.Wait()
		b.rcond.L.Unlock()

		if atomic.LoadInt64(&b.done) == 1 {
			fmt.Println("ending")
			return 0, io.EOF
		}
	}

	return head, nil
}

// ReadFrom reads bytes from an io.Reader and commits them to the buffer when
// there is sufficient capacity to do so.
func (b *Buffer) ReadFrom(r io.Reader) (totalBytes int64, err error) {
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

		totalBytes += int64(n)

		// st, err := b.awaitCapacity(n)

		// Move the head forward.
		atomic.StoreInt64(&b.head, start+int64(n))

		// Broadcast write condition
		// ...
	}
}
