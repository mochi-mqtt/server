// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package listeners

import (
	"fmt"
	"net"
	"sync"

	"log/slog"
)

// MockEstablisher is a function signature which can be used in testing.
func MockEstablisher(id string, c net.Conn) error {
	return nil
}

// MockCloser is a function signature which can be used in testing.
func MockCloser(id string) {}

// MockListener is a mock listener for establishing client connections.
type MockListener struct {
	sync.RWMutex
	id        string    // the id of the listener
	address   string    // the network address the listener binds to
	Config    *Config   // configuration for the listener
	done      chan bool // indicate the listener is done
	Serving   bool      // indicate the listener is serving
	Listening bool      // indiciate the listener is listening
	ErrListen bool      // throw an error on listen
}

// NewMockListener returns a new instance of MockListener.
func NewMockListener(id, address string) *MockListener {
	return &MockListener{
		id:      id,
		address: address,
		done:    make(chan bool),
	}
}

// Serve serves the mock listener.
func (l *MockListener) Serve(establisher EstablishFn) {
	l.Lock()
	l.Serving = true
	l.Unlock()

	for range l.done {
		return
	}
}

// Init initializes the listener.
func (l *MockListener) Init(log *slog.Logger) error {
	if l.ErrListen {
		return fmt.Errorf("listen failure")
	}

	l.Lock()
	defer l.Unlock()
	l.Listening = true
	return nil
}

// ID returns the id of the mock listener.
func (l *MockListener) ID() string {
	return l.id
}

// Address returns the address of the listener.
func (l *MockListener) Address() string {
	return l.address
}

// Protocol returns the address of the listener.
func (l *MockListener) Protocol() string {
	return "mock"
}

// Close closes the mock listener.
func (l *MockListener) Close(closer CloseFn) {
	l.Lock()
	defer l.Unlock()
	l.Serving = false
	closer(l.id)
	close(l.done)
}

// IsServing indicates whether the mock listener is serving.
func (l *MockListener) IsServing() bool {
	l.Lock()
	defer l.Unlock()
	return l.Serving
}

// IsListening indicates whether the mock listener is listening.
func (l *MockListener) IsListening() bool {
	l.Lock()
	defer l.Unlock()
	return l.Listening
}
