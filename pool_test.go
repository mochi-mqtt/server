// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 mochi-co
// SPDX-FileContributor: mochi-co

package mqtt

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewPool(t *testing.T) {
	p := NewPool(2)
	require.NotNil(t, p)
	require.Equal(t, 2, cap(p.queue))

	o := make(chan bool)
	go func() {
		p.Enqueue(func() {
			o <- true
		})
	}()

	require.True(t, <-o)
	p.Close()
	p.Wait()
}

func TestPoolEnqueue(t *testing.T) {
	p := &Pool{
		capacity: 2,
		queue:    make(PoolTaskChan, 2),
	}

	go func() {
		p.Enqueue(func() {})
	}()
	require.NotNil(t, <-p.queue)
}

func TestSize(t *testing.T) {
	p := &Pool{
		capacity: 10,
	}

	require.Equal(t, uint64(10), p.Size())
}

func TestClose(t *testing.T) {
	p := &Pool{
		capacity: 3,
		queue:    make(PoolTaskChan, 3),
	}

	p.Close()
	require.Equal(t, uint64(0), p.Size())
	require.Nil(t, p.queue)
}
