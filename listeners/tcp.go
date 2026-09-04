// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package listeners

import (
	"crypto/tls"
	"net"
	"sync"
	"sync/atomic"

	"log/slog"
)

const TypeTCP = "tcp"

// TCP is a listener for establishing client connections on basic TCP protocol.
type TCP struct { // [MQTT-4.2.0-1]
	sync.RWMutex
	id      string       // the internal id of the listener
	address string       // the network address to bind to
	listen  net.Listener // a net.Listener which will listen for new clients
	config  Config       // configuration values for the listener
	log     *slog.Logger // server logger
	end     uint32       // ensure the close methods are only called once
	// connWg tracks establish() goroutines this listener has spawned but not
	// yet finished. Close waits on it before returning, so every establish()
	// call -- including its Server.attachClient's own internal
	// Listeners.ClientsWg.Add/Done pairing -- is guaranteed complete before
	// Listeners.CloseAll's later ClientsWg.Wait() runs. Without this, Serve's
	// "go func(){ establish(...) }()" is a fire-and-forget spawn: Add(1)
	// only happens deep inside attachClient, on a goroutine with no ordering
	// guarantee relative to a concurrent Close/Wait -- see #424.
	connWg sync.WaitGroup
}

// NewTCP initializes and returns a new TCP listener, listening on an address.
func NewTCP(config Config) *TCP {
	return &TCP{
		id:      config.ID,
		address: config.Address,
		config:  config,
	}
}

// ID returns the id of the listener.
func (l *TCP) ID() string {
	return l.id
}

// Address returns the address of the listener.
func (l *TCP) Address() string {
	if l.listen != nil {
		return l.listen.Addr().String()
	}
	return l.address
}

// Protocol returns the address of the listener.
func (l *TCP) Protocol() string {
	return "tcp"
}

// Init initializes the listener.
func (l *TCP) Init(log *slog.Logger) error {
	l.log = log

	var err error
	if l.config.TLSConfig != nil {
		l.listen, err = tls.Listen("tcp", l.address, l.config.TLSConfig)
	} else {
		l.listen, err = net.Listen("tcp", l.address)
	}

	return err
}

// Serve starts waiting for new TCP connections, and calls the establish
// connection callback for any received.
func (l *TCP) Serve(establish EstablishFn) {
	for {
		if atomic.LoadUint32(&l.end) == 1 {
			return
		}

		conn, err := l.listen.Accept()
		if err != nil {
			return
		}

		if atomic.LoadUint32(&l.end) == 0 {
			// Add happens here, synchronously on the accepting goroutine,
			// before the handler goroutine is even spawned -- not inside
			// it. That ordering is what Close (below) relies on: by the
			// time l.connWg.Wait() can observe a zero count, every
			// establish() this listener ever started really has finished.
			l.connWg.Add(1)
			go func() {
				defer l.connWg.Done()
				err := establish(l.id, conn)
				if err != nil {
					l.log.Warn("", "error", err)
				}
			}()
		} else {
			// Accepted right as Close was closing this listener: nothing
			// will ever call establish for it, so nothing will ever close
			// the connection either unless we do it here.
			conn.Close()
		}
	}
}

// Close closes the listener and any client connections.
func (l *TCP) Close(closeClients CloseFn) {
	l.Lock()
	if atomic.CompareAndSwapUint32(&l.end, 0, 1) {
		closeClients(l.id)
	}
	if l.listen != nil {
		_ = l.listen.Close()
	}
	l.Unlock()

	// Outside the lock: nothing establish() reaches touches l's own mutex,
	// so there's no reason to hold it while waiting. By the time this
	// returns, every establish() this listener ever spawned -- and so every
	// Listeners.ClientsWg.Add/Done pairing it made -- has fully completed;
	// see connWg's doc comment for why that guarantee matters to the caller.
	l.connWg.Wait()
}
