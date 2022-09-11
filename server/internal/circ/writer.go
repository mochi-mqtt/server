package circ

import (
	"io"
	"log"
	"sync/atomic"
)

// Writer is a circular buffer for writing data to an io.Writer.
type Writer struct {
	*Buffer
}

// NewWriter returns a pointer to a new Circular Writer.
func NewWriter(size, block int) *Writer {
	b := NewBuffer(size, block)
	b.ID = "writer"
	return &Writer{
		b,
	}
}

// NewWriterFromSlice returns a new Circular Writer using a pre-existing
// byte slice.
func NewWriterFromSlice(block int, p []byte) *Writer {
	b := NewBufferFromSlice(block, p)
	b.ID = "writer"
	return &Writer{
		b,
	}
}

// WriteTo writes the contents of the buffer to an io.Writer.
func (b *Writer) WriteTo(w io.Writer) (total int64, err error) {
	atomic.StoreUint32(&b.State, 2)
	defer atomic.StoreUint32(&b.State, 0)
	for {
		if atomic.LoadUint32(&b.done) == 1 && b.CapDelta() == 0 {
			return total, io.EOF
		}

		// Read from the buffer until there is at least 1 byte to write.
		err = b.awaitFilled(1)
		if err != nil {
			return
		}

		// Get all the bytes between the tail and head, wrapping if necessary.
		tail := atomic.LoadInt64(&b.tail)
		rTail := b.Index(tail)
		rHead := b.Index(atomic.LoadInt64(&b.head))
		n := b.CapDelta()
		p := make([]byte, 0, n)

		if rTail > rHead {
			p = append(p, b.buf[rTail:]...)
			p = append(p, b.buf[:rHead]...)
		} else {
			p = append(p, b.buf[rTail:rHead]...)
		}

		n, err = w.Write(p)
		total += int64(n)
		if err != nil {
			log.Println("error writing to buffer io.Writer;", err)
			return
		}

		// Move the tail forward the bytes written and broadcast change.
		atomic.StoreInt64(&b.tail, tail+int64(n))
		b.rcond.L.Lock()
		b.rcond.Broadcast()
		b.rcond.L.Unlock()
	}
}

// Write writes the buffer to the buffer p, returning the number of bytes written.
// The bytes written to the buffer are picked up by WriteTo.
func (b *Writer) Write(p []byte) (total int, err error) {
	err = b.awaitEmpty(len(p))
	if err != nil {
		return
	}

	total = b.writeBytes(p)
	atomic.AddInt64(&b.head, int64(total))
	b.wcond.L.Lock()
	b.wcond.Broadcast()
	b.wcond.L.Unlock()
	return
}

// writeBytes writes bytes to the buffer from the start position, and returns
// the new head position. This function does not wait for capacity and will
// overwrite any existing bytes.
func (b *Writer) writeBytes(p []byte) int {
	var o int
	var n int
	for i := 0; i < len(p); i++ {
		o = b.Index(atomic.LoadInt64(&b.head) + int64(i))
		b.buf[o] = p[i]
		n++
	}

	return n
}
