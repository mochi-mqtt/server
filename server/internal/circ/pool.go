package circ

import (
	"sync"
)

// BytesPool is a pool of []byte.
type BytesPool struct {
	pool sync.Pool
}

// NewBytesPool returns a sync.pool of []byte.
func NewBytesPool(n int) BytesPool {
	return BytesPool{
		pool: sync.Pool{
			New: func() interface{} {
				return make([]byte, n)
			},
		},
	}
}

// Get returns a pooled bytes.Buffer.
func (b *BytesPool) Get() []byte {
	return b.pool.Get().([]byte)
}

// Put puts the byte slice back into the pool.
func (b *BytesPool) Put(x []byte) {
	for i := range x {
		x[i] = 0
	}
	b.pool.Put(x)
}
