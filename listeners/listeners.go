package listeners

import (
	"net"
	"sync"

	"github.com/mochi-co/mqtt/auth"
)

// EstablishFunc is a callback function for establishing new clients.
type EstablishFunc func(net.Conn, auth.Controller) error

// CloseFunc is a callback function for closing all listener clients.
type CloseFunc func(id string)

// Listener is an interface for network listeners. A network listener listens
// for incoming client connections and adds them to the server.
type Listener interface {

	// SetConfig sets the Config values for the listener.
	SetConfig(*Config)

	// Listen starts listening on the network address.
	Listen() error

	// Serve starts the listener waiting for new connections. The method
	// takes a callbacks for establishing new clients.
	Serve(EstablishFunc)

	// ID returns the ID of the listener.
	ID() string

	// Close closes all connections for a listener and stops listening. The method
	// takes a callback for closing all client connections before ending.
	Close(CloseFunc)
}

// Listeners contains the network listeners for the broker.
type Listeners struct {
	sync.RWMutex

	// wg is a waitgroup that waits for all listeners to finish.
	wg sync.WaitGroup

	// internal is a map of active listeners.
	internal map[string]Listener

	// Errors is a channel of errors sent by listeners.
	//Errors chan error
}

// NewListeners returns a new instance of Listeners.
func NewListeners() Listeners {
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
func MockEstablisher(c net.Conn, ac auth.Controller) error {
	return nil
}

// MockListener is a mock listener for establishing client connections.
type MockListener struct {
	sync.RWMutex

	// id is the internal id of the listener.
	id string

	// Config contains configuration values for the listener.
	Config *Config

	// address is the address to bind to.
	address string

	// IsListening indicates that the listener is listening.
	IsListening bool

	// IsServing indicates that the listener is serving.
	IsServing bool

	// done is sent when the mock listener should close.
	done chan bool
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
