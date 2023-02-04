// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 mochi-co
// SPDX-FileContributor: mochi-co

package mqtt

import (
	"sync"
	"sync/atomic"
)

// Pool is a fairly basic fixed sized worker pool.
type Pool struct {
	wg       sync.WaitGroup
	queue    PoolTaskChan
	capacity uint64
}

// PoolTaskChan is a channel of tasks to be run.
type PoolTaskChan chan func()

// PoolTask is the function signature for functions processed by the pool.
type PoolTask func()

// New returns a new instance of Pool with a specified number of workers.
func NewPool(size uint64) *Pool {
	p := &Pool{
		capacity: size,
		queue:    make(PoolTaskChan, size),
	}

	for i := uint64(0); i < size; i++ {
		go p.worker(p.queue)
		p.wg.Add(1)
	}

	atomic.StoreUint64(&p.capacity, size)

	return p
}

// worker is a worker goroutine which processes tasks from the queue.
func (p *Pool) worker(ch PoolTaskChan) {
	defer p.wg.Done()
	var task func()
	var ok bool
	for {
		task, ok = <-ch
		if !ok {
			return
		}
		task()
	}
}

// Enqueue adds a new task to the queue to be processed.
func (p *Pool) Enqueue(task PoolTask) {
	p.queue <- task
}

// Wait blocks until all the workers in the pool have completed.
func (p *Pool) Wait() {
	p.wg.Wait()
}

// Close issues a shutdown signal to the workers.
func (p *Pool) Close() {
	close(p.queue)
	atomic.StoreUint64(&p.capacity, 0)
	p.queue = nil
}

// Size returns the current number of workers in the pool.
func (p *Pool) Size() uint64 {
	return atomic.LoadUint64(&p.capacity)
}
