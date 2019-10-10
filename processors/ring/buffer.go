package ring

import (
	"fmt"
	"io"
	"sync/atomic"
	"time"
)

// (b.Beg + b.Readable) % b.N == relative position in buffer

const (
	blockSize = 4
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
}

// NewBuffer returns a pointer to a new instance of Buffer.
func NewBuffer(size int64) *Buffer {
	return &Buffer{
		size:     size,
		sizeMask: size - 1,
		buffer:   make([]byte, size),
	}
}

// awaitCapacity will hold until there are n bytes free in the buffer.
func (b *Buffer) awaitCapacity(n int64) {
	tail := atomic.LoadInt64(&b.tail)
	head := atomic.LoadInt64(&b.head)
	next := head + n
	wrapped := next - b.size

	fmt.Printf("\ntail: %d, head: %d, next: %d, wrapped: %d\n", tail, head, next, wrapped)

DONE:
	for {
		tail = atomic.LoadInt64(&b.tail)

		if wrapped > tail || (tail > head && next > tail && wrapped < 0) {

			atomic.AddInt64(&b.tail, 1)
			fmt.Println(".",
				wrapped, ">", tail,
				wrapped > tail,
				"||", next, ">", tail, "&&", wrapped, "<", 0,
				(next > tail && wrapped < 0))
			time.Sleep(time.Second)
		} else {
			break DONE
		}
	}

	fmt.Println("\n\tcapacity ok")
	fmt.Println(b.head, b.tail)

}
