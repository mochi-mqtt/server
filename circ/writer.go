package circ

import (
	"io"
	"sync/atomic"
)

// Writer is a circular buffer for writing data to an io.Writer.
type Writer struct {
	Buffer
}

// NewWriter returns a pointer to a new Circular Writer.
func NewWriter(size, block int) *Writer {
	b := NewBuffer(size, block)
	b.ID = "writer"
	return &Writer{
		b,
	}
}

// WriteTo writes the contents of the buffer to an io.Writer.
func (b *Writer) WriteTo(w io.Writer) (total int, err error) {
	for {
		if atomic.LoadInt64(&b.done) == 1 && b.CapDelta() == 0 {
			return total, io.EOF
		}

		// Read from the buffer until there is at least 1 byte to write.
		err = b.awaitFilled(1)
		if err != nil {
			return
		}

		// Get all the bytes between the tail and head, wrapping if necessary.
		tail := atomic.LoadInt64(&b.tail)
		head := atomic.LoadInt64(&b.head)
		n := b.CapDelta()
		p := make([]byte, 0, n)

		if b.Index(tail) > b.Index(head) {
			p = append(p, b.buf[b.Index(tail):]...)
			p = append(p, b.buf[:b.Index(head)]...)
		} else {
			p = append(p, b.buf[b.Index(tail):b.Index(head)]...)
		}

		// Write capDelta n bytes to the io.Writer.
		n, err = w.Write(p)
		total += n
		if err != nil {
			return
		}

		// Move the tail forward the bytes written and broadcast change.
		atomic.StoreInt64(&b.tail, tail+int64(n))
		b.rcond.L.Lock()
		b.rcond.Broadcast()
		b.rcond.L.Unlock()
	}
}
