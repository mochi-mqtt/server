package circ

import (
	"io"
	"sync/atomic"

	"github.com/mochi-co/mqtt/debug"
)

// Writer is a circular buffer for writing data to an io.Writer.
type Writer struct {
	buffer
}

// NewWriter returns a pointer to a new Circular Writer.
func NewWriter(size, block int64) *Writer {
	b := newBuffer(size, block)
	b.id = "writer"
	return &Writer{
		b,
	}
}

// WriteTo writes the contents of the buffer to an io.Writer.
func (b *Writer) WriteTo(w io.Writer) (total int64, err error) {
	var p []byte
	var n int
DONE:
	for {
		debug.Println(b.id, "\033[1;36m*[W]----------- SPIN --------- ", "\033[0m")
		debug.Println(b.id, "\033[1;36m", atomic.LoadInt64(&b.tail), atomic.LoadInt64(&b.head), b.buf[atomic.LoadInt64(&b.tail):atomic.LoadInt64(&b.head)], "\033[0m")
		if atomic.LoadInt64(&b.done) == 1 {
			if b.CapDelta() == 0 {
				err = io.EOF
				debug.Println(b.id, "\033[1;30m*[W] WriteTo caught b.done=1", err, "\033[0m")
				break DONE
			} else {
				debug.Println(b.id, "\033[1;30m*[W] ---- capDelta not met", err, "\033[0m")
			}
		}

		// Peek until there's bytes to write using the Peek method
		// of the Reader type.
		p, err = (*Reader)(b).Peek(b.block)
		if err != nil {
			continue
			//break DONE
		}
		debug.Println(b.id, "\033[1;32m*[W] PEEK OK", n, string(p), "\033[0m")

		// Write the peeked bytes to the io.Writer.
		n, err = w.Write(p)
		total += int64(n)
		if err != nil {
			debug.Println(b.id, "\033[1;35m*[W] w.Write err", err, "\033[0m")
			break DONE
		}

		debug.Println(b.id, "\033[1;31m*[W] SENT (n, p)", n, string(p), "\033[0m")
		debug.Println(b.id, "\033[0;35m*[W]", p, "\033[0m")
		debug.Println(b.id, "\033[0;35m*[W]", b.buf[:total], "\033[0m")

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
		if b.CapDelta() == 0 {
			debug.Println(b.id, "\033[1;31m[W] Write caught b.done=1", err, "\033[0m")
			return 0, io.EOF
		} else {
			debug.Println(b.id, "\033[1;31m[W] capDelta not met", err, "\033[0m")
		}
	}

	// Wait until there's enough capacity to write to the buffer.
	_, err = b.awaitCapacity(int64(len(p)))
	if err != nil {
		return
	}

	// Write the outgoing bytes to the buffer, wrapping if necessary.
	nn = b.writeBytes(p)

	// Move the head forward.
	next := (atomic.LoadInt64(&b.head) + int64(nn)) % b.size
	atomic.StoreInt64(&b.head, next)

	b.wcond.L.Lock()
	b.wcond.Broadcast()
	b.wcond.L.Unlock()

	return
}

// writeBytes writes bytes to the buffer from the start position, and returns
// the new head position. This function does not wait for capacity and will
// overwrite any existing bytes.
func (b *Writer) writeBytes(p []byte) int {
	//tail := atomic.LoadInt64(&b.tail)
	head := atomic.LoadInt64(&b.head)

	debug.Println(b.id, "\033[1;31m[W] writing to byte buffer", "\033[0m")

	var o int64
	for i := 0; i < len(p); i++ {
		o = (head + int64(i)) % b.size
		debug.Println(b.id, "\033[1;31m[W]", o, p[i], "\033[0m")
		b.buf[o] = p[i]
	}

	return len(p)
}
