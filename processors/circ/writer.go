package circ

import (
	"fmt"
	"io"
	"sync/atomic"
)

// Writer is a circular buffer for writing data to an io.Writer.
type Writer struct {
	buffer
}

// NewWriter returns a pointer to a new Circular Writer.
func NewWriter(size int64) *Writer {
	return &Writer{
		newBuffer(size),
	}
}

// WriteTo writes the contents of the buffer to an io.Writer.
func (b *Writer) WriteTo(w io.Writer) (total int64, err error) {
	var p []byte
	var n int
DONE:
	for {
		if atomic.LoadInt64(&b.done) == 1 {
			err = io.EOF
			break DONE
		}

		// Peek until there's bytes to write using the Peek method
		// of the Reader type.
		p, err = (*Reader)(b).Peek(blockSize)
		if err != nil {
			break DONE
		}

		// Write the peeked bytes to the io.Writer.
		n, err = w.Write(p)
		total += int64(n)
		if err != nil {
			break DONE
		}

		// Move the tail forward the bytes written.
		end := (atomic.LoadInt64(&b.tail) + int64(n)) % b.size
		atomic.StoreInt64(&b.tail, end)
		b.rcond.L.Lock()
		b.rcond.Broadcast()
		b.rcond.L.Unlock()
	}

	return
}

// Write writes the buffer to the buffer p, returning the number of bytes written.
func (b *Writer) Write(p []byte) (nn int, err error) {
	if atomic.LoadInt64(&b.done) == 1 {
		return 0, io.EOF
	}

	// Wait until there's enough capacity to write to the buffer.
	_, err = b.awaitCapacity(int64(len(p)))
	if err != nil {
		return
	}

	// Write the outgoing bytes to the buffer, wrapping if necessary.
	nn = b.writeBytes(p)

	fmt.Println(atomic.LoadInt64(&b.tail), nn)

	return
}

// writeBytes writes bytes to the buffer from the start position, and returns
// the new head position. This function does not wait for capacity and will
// overwrite any existing bytes.
func (b *Writer) writeBytes(p []byte) int {
	tail := atomic.LoadInt64(&b.tail)

	var o int64
	for i := 0; i < len(p); i++ {
		o = (tail + int64(i)) % b.size
		b.buf[o] = p[i]
	}

	return len(p)
}
