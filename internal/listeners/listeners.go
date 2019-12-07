package listeners

import (
	"fmt"
	"net"
	"sync"

	"github.com/mochi-co/mqtt/internal/auth"
)

// Config contains configuration values for a listener.
type Config struct {
	Auth auth.Controller // an authentication controller containing auth and ACL logic.
	TLS  *TLS            // the TLS certficates and settings for the connection.
}

// TLS contains the TLS certificates and settings for the listener connection.
type TLS struct {
	// ...
}

// EstablishFunc is a callback function for establishing new clients.
type EstablishFunc func(id string, c net.Conn, ac auth.Controller) error

// CloseFunc is a callback function for closing all listener clients.
type CloseFunc func(id string)

// Listener is an interface for network listeners. A network listener listens
// for incoming client connections and adds them to the server.
type Listener interface {
	SetConfig(*Config)   // set the listener config.
	Listen() error       // open the network address.
	Serve(EstablishFunc) // starting actively listening for new connections.
	ID() string          // return the id of the listener.
	Close(CloseFunc)     // stop and close the listener.
}

// Listeners contains the network listeners for the broker.
type Listeners struct {
	sync.RWMutex
	wg       sync.WaitGroup      // a waitgroup that waits for all listeners to finish.
	internal map[string]Listener // a map of active listeners.
}

// New returns a new instance of Listeners.
func New() Listeners {
	return Listeners{
		internal: map[string]Listener{},
	}
}

// Add adds a new listener to the listeners map, keyed on id.
func (l *Listeners) Add(val Listener) {
	l.Lock()
	l.internal[val.ID()] = val
	l.Unlock()
}

// Get returns the value of a listener if it exists.
func (l *Listeners) Get(id string) (Listener, bool) {
	l.RLock()
	val, ok := l.internal[id]
	l.RUnlock()
	return val, ok
}

// Len returns the length of the listeners map.
func (l *Listeners) Len() int {
	l.RLock()
	val := len(l.internal)
	l.RUnlock()
	return val
}

// Delete removes a listener from the internal map.
func (l *Listeners) Delete(id string) {
	l.Lock()
	delete(l.internal, id)
	l.Unlock()
}

// Serve starts a listener serving from the internal map.
func (l *Listeners) Serve(id string, establisher EstablishFunc) error {
	l.RLock()
	listener := l.internal[id]
	l.RUnlock()

	// Start listening on the network address.
	err := listener.Listen()
	if err != nil {
		return err
	}

	go func(e EstablishFunc) {
		defer l.wg.Done()
		l.wg.Add(1)
		listener.Serve(e)

	}(establisher)

	return nil
}

// ServeAll starts all listeners serving from the internal map.
func (l *Listeners) ServeAll(establisher EstablishFunc) error {
	l.RLock()
	i := 0
	ids := make([]string, len(l.internal))
	for id := range l.internal {
		ids[i] = id
		i++
	}
	l.RUnlock()

	for _, id := range ids {
		err := l.Serve(id, establisher)
		if err != nil {
			return err
		}
	}

	return nil
}

// Close stops a listener from the internal map.
func (l *Listeners) Close(id string, closer CloseFunc) {
	l.RLock()
	listener := l.internal[id]
	l.RUnlock()
	listener.Close(closer)
}

// CloseAll iterates and closes all registered listeners.
func (l *Listeners) CloseAll(closer CloseFunc) {
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
	l.wg.Wait()
}

// MockCloser is a function signature which can be used in testing.
func MockCloser(id string) {}

// MockEstablisher is a function signature which can be used in testing.
func MockEstablisher(id string, c net.Conn, ac auth.Controller) error {
	return nil
}

// MockListener is a mock listener for establishing client connections.
type MockListener struct {
	sync.RWMutex
	id          string
	Config      *Config
	address     string
	IsListening bool
	IsServing   bool
	done        chan bool
	errListen   bool
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
	l.IsServing = true
	l.Unlock()
DONE:
	for {
		select {
		case <-l.done:
			break DONE
		}
	}
}

// SetConfig sets the configuration values of the mock listener.
func (l *MockListener) Listen() error {
	if l.errListen {
		return fmt.Errorf("listen failure")
	}

	l.Lock()
	l.IsListening = true
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
	l.IsServing = false
	closer(l.id)
	close(l.done)
}
