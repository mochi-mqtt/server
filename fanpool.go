// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-co
// SPDX-FileContributor: mochi-co, chowyu08, muXxer

package mqtt

import (
	"sync"
	"sync/atomic"

	xh "github.com/cespare/xxhash/v2"
)

// taskChan is a channel for incoming task functions.
type taskChan chan func()

// FanPool is a fixed-sized fan-style worker pool with multiple
// working 'columns'. Instead of a single queue channel processed by
// many goroutines, this fan pool uses many queue channels each
// processed by a single goroutine.
// Very special thanks are given to the authors of HMQ in particular
// @chowyu08 and @muXxer for their work on the fixpool worker pool
// https://github.com/fhmq/hmq/blob/master/pool/fixpool.go
// from which this fan-pool is heavily inspired.
type FanPool struct {
	queue    []taskChan
	wg       sync.WaitGroup
	capacity uint64
	perChan  uint64
	Mutex    sync.Mutex
}

// New returns a new instance of FanPool. fanSize controls the number of 'columns'
// of the fan, whereas queueSize controls the size of each column's queue.
func NewFanPool(fanSize, queueSize uint64) *FanPool {
	pool := &FanPool{
		capacity: fanSize,
		perChan:  queueSize,
		queue:    make([]taskChan, fanSize),
	}

	pool.fillWorkers(fanSize)

	return pool
}

// fillWorkers adds columns to the fan pool with an associated worker goroutine.
func (p *FanPool) fillWorkers(n uint64) {
	for i := uint64(0); i < n; i++ {
		p.queue[i] = make(taskChan, p.perChan)
		go p.worker(p.queue[i])
		p.wg.Add(1)
	}
}

// worker is a worker goroutine which processes tasks from a single queue.
func (p *FanPool) worker(ch taskChan) {
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
func (p *FanPool) Enqueue(id string, task func()) {
	if p.Size() == 0 {
		return
	}

	// We can use xh.Sum64 to get a specific queue index
	// which remains the same for a client id, giving each
	// client their own queue.
	p.queue[xh.Sum64([]byte(id))%p.Size()] <- task
}

// Wait blocks until all the workers in the pool have completed.
func (p *FanPool) Wait() {
	p.wg.Wait()
}

// Close issues a shutdown signal to the workers.
func (p *FanPool) Close() {
	for i := 0; i < int(p.Size()); i++ {
		if p.queue[i] != nil {
			close(p.queue[i])
		}
	}
	p.queue = nil
	atomic.StoreUint64(&p.capacity, 0)
}

// Size returns the current number of workers in the pool.
func (p *FanPool) Size() uint64 {
	return atomic.LoadUint64(&p.capacity)
}
