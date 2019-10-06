package pools

/*
import (
	"bufio"
	"io"
	"sync"
)

// BufioReadersPool is a pool of bufio.Reader.
type BufioReadersPool struct {
	pool sync.Pool
}

// NewBufioReadersPool returns a sync.pool of bufio.Reader.
func NewBufioReadersPool(size int) BufioReadersPool {
	return BufioReadersPool{
		pool: sync.Pool{
			New: func() interface{} {
				return bufio.NewReaderSize(nil, size)
			},
		},
	}
}

// Get returns a pooled bufio.Reader.
func (b BufioReadersPool) Get(r io.Reader) *bufio.Reader {
	buf := b.pool.Get().(*bufio.Reader)
	buf.Reset(r)
	return buf
}

// Put puts the bufio.Reader back into the pool.
func (b BufioReadersPool) Put(x *bufio.Reader) {
	x.Reset(nil)
	b.pool.Put(x)
}

// BufioWritersPool is a pool of bufio.Writer.
type BufioWritersPool struct {
	pool sync.Pool
}

// NewBufioWritersPool returns a sync.pool of bufio.Writer.
func NewBufioWritersPool(size int) BufioWritersPool {
	return BufioWritersPool{
		pool: sync.Pool{
			New: func() interface{} {
				return bufio.NewWriterSize(nil, size)
			},
		},
	}
}

// Get returns a pooled bufio.Writer.
func (b BufioWritersPool) Get(r io.Writer) *bufio.Writer {
	buf := b.pool.Get().(*bufio.Writer)
	buf.Reset(r)
	return buf
}

// Put puts the bufio.Writer back into the pool.
func (b BufioWritersPool) Put(x *bufio.Writer) {
	x.Reset(nil)
	b.pool.Put(x)
}
*/
