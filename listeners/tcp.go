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

// TCP is a listener for establishing client connections on basic TCP protocol.
type TCP struct { // [MQTT-4.2.0-1]
	sync.RWMutex
	id      string       // the internal id of the listener
	address string       // the network address to bind to
	listen  net.Listener // a net.Listener which will listen for new clients
	config  *Config      // configuration values for the listener
	log     *slog.Logger // server logger
	end     uint32       // ensure the close methods are only called once
}

// NewTCP initialises and returns a new TCP listener, listening on an address.
func NewTCP(id, address string, config *Config) *TCP {
	if config == nil {
		config = new(Config)
	}

	return &TCP{
		id:      id,
		address: address,
		config:  config,
	}
}

// ID returns the id of the listener.
func (l *TCP) ID() string {
	return l.id
}

// Address returns the address of the listener.
func (l *TCP) Address() string {
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
			go func() {
				err = establish(l.id, conn)
				if err != nil {
					l.log.Warn("", "error", err)
				}
			}()
		}
	}
}

// Close closes the listener and any client connections.
func (l *TCP) Close(closeClients CloseFn) {
	l.Lock()
	defer l.Unlock()

	if atomic.CompareAndSwapUint32(&l.end, 0, 1) {
		closeClients(l.id)
	}

	if l.listen != nil {
		err := l.listen.Close()
		if err != nil {
			return
		}
	}
}
