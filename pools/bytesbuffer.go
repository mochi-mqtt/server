package pools

import (
	"bytes"
	"sync"
)

// BytesBuffersPool is a pool of bytes.Buffer.
type BytesBuffersPool struct {
	pool sync.Pool
}

// NewBytesBuffersPool returns a sync.pool of bytes.Buffer.
func NewBytesBuffersPool() BytesBuffersPool {
	return BytesBuffersPool{
		pool: sync.Pool{
			New: func() interface{} {
				return new(bytes.Buffer)
			},
		},
	}
}

// Get returns a pooled bytes.Buffer.
func (b BytesBuffersPool) Get() *bytes.Buffer {
	return b.pool.Get().(*bytes.Buffer)
}

// Put puts the bytes.Buffer back into the pool.
func (b BytesBuffersPool) Put(x *bytes.Buffer) {
	x.Reset()
	b.pool.Put(x)
}
