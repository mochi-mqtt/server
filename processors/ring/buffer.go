package ring

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"
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

	//if wrapped > tail || (tail > head && next > tail && wrapped < 0) {
	for ; wrapped > tail || (tail > head && next > tail && wrapped < 0); tail = atomic.LoadInt64(&b.tail) {
		/*
			fmt.Println("\t",
				wrapped, ">", tail,
				wrapped > tail,
				"||",
				tail, ">", head, "&&", next, ">", tail, "&&", wrapped, "<", 0,
				(tail > head && next > tail && wrapped < 0))
		*/

		b.rcond.L.Lock()
		b.rcond.Wait()
		b.rcond.L.Unlock()

		if atomic.LoadInt64(&b.done) == 1 {
			fmt.Println("ending")
			return 0, io.EOF
		}
	}
	//}

	return head, nil
}
