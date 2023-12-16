package mempool

import (
	"bytes"
	"sync"
)

var bufPool = NewBuffer(0)

// GetBuffer takes a Buffer from the default buffer pool
func GetBuffer() *bytes.Buffer { return bufPool.Get() }

// PutBuffer returns Buffer to the default buffer pool
func PutBuffer(x *bytes.Buffer) { bufPool.Put(x) }

type BufferPool interface {
	Get() *bytes.Buffer
	Put(x *bytes.Buffer)
}

// NewBuffer returns a buffer pool. The max specify the max capacity of the Buffer the pool will
// return. If the Buffer becoomes large than max, it will no longer be returned to the pool. If
// max <= 0, no limit will be enforced.
func NewBuffer(max int) BufferPool {
	if max > 0 {
		return newBufferWithCap(max)
	}

	return newBuffer()
}

// Buffer is a Buffer pool.
type Buffer struct {
	pool *sync.Pool
}

func newBuffer() *Buffer {
	return &Buffer{
		pool: &sync.Pool{
			New: func() any { return new(bytes.Buffer) },
		},
	}
}

// Get a Buffer from the pool.
func (b *Buffer) Get() *bytes.Buffer {
	return b.pool.Get().(*bytes.Buffer)
}

// Put the Buffer back into pool. It resets the Buffer for reuse.
func (b *Buffer) Put(x *bytes.Buffer) {
	x.Reset()
	b.pool.Put(x)
}

// BufferWithCap is a Buffer pool that
type BufferWithCap struct {
	bp  *Buffer
	max int
}

func newBufferWithCap(max int) *BufferWithCap {
	return &BufferWithCap{
		bp:  newBuffer(),
		max: max,
	}
}

// Get a Buffer from the pool.
func (b *BufferWithCap) Get() *bytes.Buffer {
	return b.bp.Get()
}

// Put the Buffer back into the pool if the capacity doesn't exceed the limit. It resets the Buffer
// for reuse.
func (b *BufferWithCap) Put(x *bytes.Buffer) {
	if x.Cap() > b.max {
		return
	}
	b.bp.Put(x)
}
