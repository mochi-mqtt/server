package circ

import (
	"sync"
	"sync/atomic"
)

// BytesPool is a pool of []byte.
type BytesPool struct {
	// int64/uint64 has to the first words in order
	// to be 64-aligned on 32-bit architectures.
	used int64 // access atomically
	pool *sync.Pool
}

// NewBytesPool returns a sync.pool of []byte.
func NewBytesPool(n int) *BytesPool {
	if n == 0 {
		n = DefaultBufferSize
	}

	return &BytesPool{
		pool: &sync.Pool{
			New: func() interface{} {
				return make([]byte, n)
			},
		},
	}
}

// Get returns a pooled bytes.Buffer.
func (b *BytesPool) Get() []byte {
	atomic.AddInt64(&b.used, 1)
	return b.pool.Get().([]byte)
}

// Put puts the byte slice back into the pool.
func (b *BytesPool) Put(x []byte) {
	for i := range x {
		x[i] = 0
	}
	b.pool.Put(x)
	atomic.AddInt64(&b.used, -1)
}

// InUse returns the number of pool blocks in use.
func (b *BytesPool) InUse() int64 {
	return atomic.LoadInt64(&b.used)
}
