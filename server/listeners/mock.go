package listeners

import (
	"fmt"

	"net"
	"sync"

	"github.com/mochi-co/mqtt/server/listeners/auth"
	"github.com/mochi-co/mqtt/server/system"
)

// MockCloser is a function signature which can be used in testing.
func MockCloser(id string) {}

// MockEstablisher is a function signature which can be used in testing.
func MockEstablisher(id string, c net.Conn, ac auth.Controller) error {
	return nil
}

// MockListener is a mock listener for establishing client connections.
type MockListener struct {
	sync.RWMutex
	id        string    // the id of the listener.
	address   string    // the network address the listener binds to.
	Config    *Config   // configuration for the listener.
	done      chan bool // indicate the listener is done.
	Serving   bool      // indicate the listener is serving.
	Listening bool      // indiciate the listener is listening.
	ErrListen bool      // throw an error on listen.
}

// NewMockListener returns a new instance of MockListener
func NewMockListener(id, address string) *MockListener {
	return &MockListener{
		id:      id,
		address: address,
		done:    make(chan bool),
	}
}

// Serve serves the mock listener.
func (l *MockListener) Serve(establisher EstablishFunc) {
	l.Lock()
	l.Serving = true
	l.Unlock()
	for range l.done {
		return
	}
}

// Listen begins listening for incoming traffic.
func (l *MockListener) Listen(s *system.Info) error {
	if l.ErrListen {
		return fmt.Errorf("listen failure")
	}

	l.Lock()
	l.Listening = true
	l.Unlock()
	return nil
}

// SetConfig sets the configuration values of the mock listener.
func (l *MockListener) SetConfig(config *Config) {
	l.Lock()
	l.Config = config
	l.Unlock()
}

// ID returns the id of the mock listener.
func (l *MockListener) ID() string {
	l.RLock()
	id := l.id
	l.RUnlock()
	return id
}

// Close closes the mock listener.
func (l *MockListener) Close(closer CloseFunc) {
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
