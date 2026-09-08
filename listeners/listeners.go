// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package listeners

import (
	"crypto/tls"
	"net"
	"sync"

	"log/slog"
)

// Config contains configuration values for a listener.
type Config struct {
	Type    string
	ID      string
	Address string
	// TLSConfig is a tls.Config configuration to be used with the listener. See examples folder for basic and mutual-tls use.
	TLSConfig *tls.Config
}

// EstablishFn is a callback function for establishing new clients.
type EstablishFn func(id string, c net.Conn) error

// CloseFn is a callback function for closing all listener clients.
type CloseFn func(id string)

// Listener is an interface for network listeners. A network listener listens
// for incoming client connections and adds them to the server.
type Listener interface {
	Init(*slog.Logger) error // open the network address
	Serve(EstablishFn)       // starting actively listening for new connections
	ID() string              // return the id of the listener
	Address() string         // the address of the listener
	Protocol() string        // the protocol in use by the listener
	Close(CloseFn)           // stop and close the listener
}

// Listeners contains the network listeners for the broker.
type Listeners struct {
	ClientsWg sync.WaitGroup      // a waitgroup that waits for all clients in all listeners to finish.
	internal  map[string]Listener // a map of active listeners.
	shutdown  sync.RWMutex        // guards closed; taken for writing while shutdown latches.
	closed    bool                // once set, no further client is registered.
	sync.RWMutex
}

// Establish registers a connection with ClientsWg for the duration of
// establish, and refuses it once shutdown has begun.
//
// sync.WaitGroup forbids a positive Add concurrent with Wait. Registering
// a client from the connection's own goroutine, as the server used to do,
// means a connection accepted while CloseAll is waiting races the very
// wait that is meant to cover it -- and CloseAll can return while that
// client is still being attached, which is the one thing a graceful
// shutdown promises not to do.
//
// Latching shutdown under the write lock, and registering only under the
// read lock, makes that ordering impossible rather than unlikely.
//
// Every connection the server attaches passes through here, because
// EstablishConnection is both what ServeAll hands to a listener and what a
// caller embedding the server calls with a connection of its own. Wrapping
// the listener's establisher instead covered only the first of those, and
// left a direct caller outside the latch and outside the wait -- so Close
// returned while such a client was still being attached, which is the very
// thing this exists to prevent.
func (l *Listeners) Establish(id string, c net.Conn, establish EstablishFn) error {
	l.shutdown.RLock()
	if l.closed {
		l.shutdown.RUnlock()
		// Shutdown has begun and this connection will not be served.
		// Closing it is the whole of the answer; there is nothing here
		// worth logging on every connection that arrives while a
		// server is going down.
		return c.Close()
	}
	l.ClientsWg.Add(1)
	l.shutdown.RUnlock()

	defer l.ClientsWg.Done()
	return establish(id, c)
}

// New returns a new instance of Listeners.
func New() *Listeners {
	return &Listeners{
		internal: map[string]Listener{},
	}
}

// Add adds a new listener to the listeners map, keyed on id.
func (l *Listeners) Add(val Listener) {
	l.Lock()
	defer l.Unlock()
	l.internal[val.ID()] = val
}

// Get returns the value of a listener if it exists.
func (l *Listeners) Get(id string) (Listener, bool) {
	l.RLock()
	defer l.RUnlock()
	val, ok := l.internal[id]
	return val, ok
}

// Len returns the length of the listeners map.
func (l *Listeners) Len() int {
	l.RLock()
	defer l.RUnlock()
	return len(l.internal)
}

// Delete removes a listener from the internal map.
func (l *Listeners) Delete(id string) {
	l.Lock()
	defer l.Unlock()
	delete(l.internal, id)
}

// Serve starts a listener serving from the internal map.
func (l *Listeners) Serve(id string, establisher EstablishFn) {
	l.RLock()
	defer l.RUnlock()
	listener := l.internal[id]

	go func(e EstablishFn) {
		listener.Serve(e)
	}(establisher)
}

// ServeAll starts all listeners serving from the internal map.
func (l *Listeners) ServeAll(establisher EstablishFn) {
	l.RLock()
	i := 0
	ids := make([]string, len(l.internal))
	for id := range l.internal {
		ids[i] = id
		i++
	}
	l.RUnlock()

	for _, id := range ids {
		l.Serve(id, establisher)
	}
}

// Close stops a listener from the internal map.
func (l *Listeners) Close(id string, closer CloseFn) {
	l.RLock()
	defer l.RUnlock()
	if listener, ok := l.internal[id]; ok {
		listener.Close(closer)
	}
}

// CloseAll iterates and closes all registered listeners.
func (l *Listeners) CloseAll(closer CloseFn) {
	// Latch shutdown before waiting, so that no client can be registered
	// once the wait below has begun.
	l.shutdown.Lock()
	l.closed = true
	l.shutdown.Unlock()

	l.RLock()
	i := 0
	ids := make([]string, len(l.internal))
	for id := range l.internal {
		ids[i] = id
		i++
	}
	l.RUnlock()

	for _, id := range ids {
		l.Close(id, closer)
	}
	l.ClientsWg.Wait()
}
