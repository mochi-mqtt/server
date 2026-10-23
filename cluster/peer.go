// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package cluster

import (
	"bufio"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const (
	minRedialDelay = time.Millisecond * 100
	maxRedialDelay = time.Second * 5
	maxWriteBatch  = 256 // frames written per flush
)

// peer is an outbound link from this node to another node. It carries this node's
// route snapshot and deltas, and publishes forwarded to the other node, in order.
// The link is redialled until the peer is stopped, and each new connection begins
// with a full route snapshot.
type peer struct {
	c        *Cluster
	name     string
	queue    chan []byte // encoded frames waiting to be written
	done     chan struct{}
	draining atomic.Bool // the peer is draining, so mustn't be sent sessions
	stopOnce sync.Once

	mu   sync.Mutex
	addr string
	conn net.Conn
}

func newPeer(c *Cluster, name, addr string) *peer {
	return &peer{
		c:     c,
		name:  name,
		addr:  addr,
		queue: make(chan []byte, c.opts.PeerQueueSize),
		done:  make(chan struct{}),
	}
}

// send queues a frame for the peer, returning false if the queue is full.
func (p *peer) send(frame []byte) bool {
	select {
	case p.queue <- frame:
		return true
	default:
		return false
	}
}

// sendWait queues a frame for the peer, waiting up to timeout for room in the queue if
// it's full. It only waits while the link is connected, as a queue which isn't being
// written won't drain. queued is false if the frame couldn't be queued, and waited is
// true if the queue was full.
func (p *peer) sendWait(frame []byte, timeout time.Duration) (queued, waited bool) {
	if p.send(frame) {
		return true, false
	}

	if !p.connected() {
		return false, true
	}

	t := time.NewTimer(timeout)
	defer t.Stop()
	select {
	case p.queue <- frame:
		return true, true
	case <-p.done:
		return false, true
	case <-t.C:
		return false, true
	}
}

// connected returns true if the link is currently connected.
func (p *peer) connected() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.conn != nil
}

// address returns the peer's forward address.
func (p *peer) address() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.addr
}

// setAddr updates the peer's address, reconnecting if it has changed.
func (p *peer) setAddr(addr string) {
	p.mu.Lock()
	changed := p.addr != addr
	p.addr = addr
	p.mu.Unlock()

	if changed {
		p.resync()
	}
}

// resync drops the current connection, so that the link reconnects and resends a
// full route snapshot.
func (p *peer) resync() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conn != nil {
		_ = p.conn.Close()
	}
}

// stop closes the link permanently.
func (p *peer) stop() {
	p.stopOnce.Do(func() {
		close(p.done)
		p.resync()
	})
}

func (p *peer) stopped() bool {
	select {
	case <-p.done:
		return true
	default:
		return false
	}
}

// run maintains the link until the peer is stopped.
func (p *peer) run() {
	delay := minRedialDelay
	for !p.stopped() {
		connected, err := p.serve()
		if p.stopped() {
			return
		}

		if connected {
			delay = minRedialDelay
		}

		p.c.log.Debug("cluster peer link closed", "peer", p.name, "error", err)

		select {
		case <-p.done:
			return
		case <-time.After(delay):
		}

		delay = min(delay*2, maxRedialDelay)
	}
}

// serve dials the peer and writes frames until the connection fails or the peer is
// stopped. connected is true if the connection was established.
func (p *peer) serve() (connected bool, err error) {
	p.mu.Lock()
	addr := p.addr
	p.mu.Unlock()

	conn, err := net.DialTimeout("tcp", addr, p.c.opts.DialTimeout)
	if err != nil {
		return false, err
	}

	p.mu.Lock()
	if p.stopped() {
		p.mu.Unlock()
		_ = conn.Close()
		return false, nil
	}
	p.conn = conn
	p.mu.Unlock()
	p.c.metrics.linkConnects.Add(1)

	defer func() {
		p.mu.Lock()
		p.conn = nil
		p.mu.Unlock()
		_ = conn.Close()
	}()

	// The link is write-only, so a read only returns when the connection is closed.
	closed := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, conn)
		close(closed)
	}()

	w := bufio.NewWriterSize(conn, 64*1024)
	write := func(frames ...[]byte) error {
		_ = conn.SetWriteDeadline(time.Now().Add(p.c.opts.WriteTimeout))
		for _, f := range frames {
			if _, err := w.Write(f); err != nil {
				return err
			}
		}
		return w.Flush()
	}

	// Any route deltas still queued from before the snapshot are replayed after it.
	// That's safe, as the last queued change to each filter matches the snapshot.
	hello := encodeFrame(frameHello, []byte(p.c.opts.NodeName))
	if err := write(hello, p.c.routeSnapshot()); err != nil {
		return true, err
	}

	// Retained message changes are ordered by version rather than by the link, so it
	// doesn't matter whether queued changes are older or newer than the snapshot.
	for frames := p.c.retainedSnapshot(); len(frames) > 0; {
		n := min(len(frames), maxWriteBatch)
		if err := write(frames[:n]...); err != nil {
			return true, err
		}
		frames = frames[n:]
	}

	for {
		select {
		case <-p.done:
			return true, nil
		case <-closed:
			return true, io.EOF
		case f := <-p.queue:
			batch := [][]byte{f}
		drain:
			for len(batch) < maxWriteBatch {
				select {
				case f := <-p.queue:
					batch = append(batch, f)
				default:
					break drain
				}
			}

			if err := write(batch...); err != nil {
				return true, err
			}
		}
	}
}
