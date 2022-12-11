// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-co
// SPDX-FileContributor: mochi-co

package mqtt

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFanPool(t *testing.T) {
	f := NewFanPool(1, 2)
	require.NotNil(t, f)
	require.Equal(t, uint64(1), f.capacity)
	require.Equal(t, 2, cap(f.queue[0]))

	o := make(chan bool)
	go func() {
		f.Enqueue("test", func() {
			o <- true
		})
	}()

	require.True(t, <-o)
	f.Close()
	f.Wait()
}

func TestFillWorkers(t *testing.T) {
	f := &FanPool{
		perChan: 3,
		queue:   make([]taskChan, 2),
	}
	f.fillWorkers(2)
	require.Len(t, f.queue, 2)
	require.Equal(t, 3, cap(f.queue[0]))
}

func TestEnqueue(t *testing.T) {
	f := &FanPool{
		capacity: 2,
		queue: []taskChan{
			make(taskChan, 2),
			make(taskChan, 2),
		},
	}

	go func() {
		f.Enqueue("a", func() {})
	}()
	require.NotNil(t, <-f.queue[1])
}

func TestEnqueueOnEmpty(t *testing.T) {
	f := &FanPool{
		queue: []taskChan{},
	}

	go func() {
		f.Enqueue("a", func() {})
	}()

	require.Len(t, f.queue, 0)
}

func TestSize(t *testing.T) {
	f := &FanPool{
		capacity: 10,
	}

	require.Equal(t, uint64(10), f.Size())
}

func TestClose(t *testing.T) {
	f := &FanPool{
		capacity: 3,
		queue: []taskChan{
			make(taskChan, 2),
			make(taskChan, 2),
			make(taskChan, 2),
		},
	}

	f.Close()
	require.Equal(t, uint64(0), f.Size())
	require.Nil(t, f.queue)
}
