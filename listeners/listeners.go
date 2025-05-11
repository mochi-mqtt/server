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
	TLSCert *TlsCertConfig `yaml:"tls_cert" json:"tls_cert"`
	// TLSConfig is a tls.Config configuration to be used with the listener. See examples folder for basic and mutual-tls use.
	TLSConfig *tls.Config
}

// TlsCertConfig contains file paths for TLS certificate and key files.
type TlsCertConfig struct {
	CertFile *string `yaml:"cert_file" json:"cert_file"`
	KeyFile  *string `yaml:"key_file" json:"key_file"`
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
	sync.RWMutex
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
