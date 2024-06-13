// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 xlango, natefinch
// SPDX-FileContributor: xiang_yuanlang@topsec.com.cn

package listeners

import (
	"github.com/xlango/npipe"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
)

type WindowsPipe struct {
	sync.RWMutex
	id      string       // the internal id of the listener
	address string       // the network address to bind to
	listen  net.Listener // a net.Listener which will listen for new clients
	config  Config       // configuration values for the listener
	log     *slog.Logger // server logger
	end     uint32       // ensure the close methods are only called once
}

// NewWindowsPipe initializes and returns a new WindowsPipe listener, listening on an address.
func NewWindowsPipe(config Config) *WindowsPipe {
	return &WindowsPipe{
		id:      config.ID,
		address: config.Address,
		config:  config,
	}
}

// ID returns the id of the listener.
func (l *WindowsPipe) ID() string {
	return l.id
}

// Address returns the address of the listener.
func (l *WindowsPipe) Address() string {
	if l.listen != nil {
		return l.listen.Addr().String()
	}
	return l.address
}

// Protocol returns the address of the listener.
func (l *WindowsPipe) Protocol() string {
	return "WindowsPipe"
}

// Init initializes the listener.
func (l *WindowsPipe) Init(log *slog.Logger) error {
	l.log = log

	var err error

	l.listen, err = npipe.Listen(l.address)

	return err
}

// Serve starts waiting for new WindowsPipe connections, and calls the establish
// connection callback for any received.
func (l *WindowsPipe) Serve(establish EstablishFn) {
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
func (l *WindowsPipe) Close(closeClients CloseFn) {
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
