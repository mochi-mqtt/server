package listeners

import (
	"net"
	"sync"
)

// EstablishFunc is a callback function for establishing new clients.
type EstablishFunc func(net.Conn)

// CloseFunc is a callback function for closing all listener clients.
type CloseFunc func(id string)

// Listener is an interface for network listeners. A network listener listens
// for incoming client connections and adds them to the server.
type Listener interface {

	// Serve starts the listener listening for new connections. The method
	// takes a callbacks for establishing new clients.
	Serve(EstablishFunc)

	// ID returns the ID of the listener.
	ID() string

	// Close closes all connections for a listener and stops listening. The method
	// takes a callback for closing all client connections before ending.
	Close(CloseFunc)
}

// listeners contains the network listeners for the broker.
type Listeners struct {
	sync.RWMutex

	// wg is a waitgroup that waits for all listeners to finish.
	wg sync.WaitGroup

	// internal is a map of active listeners.
	internal map[string]Listener

	// Errors is a channel of errors sent by listeners.
	Errors chan error
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

// Delete removes a listener from the internal map.
func (l *Listeners) Delete(id string) {
	l.Lock()
	delete(l.internal, id)
	l.Unlock()
}

// Serve starts a listener serving from the internal map.
func (l *Listeners) Serve(id string, establisher EstablishFunc) {
	l.RLock()
	listener := l.internal[id]
	l.RUnlock()

	go func() {
		defer l.wg.Done()
		l.wg.Add(1)
		listener.Serve(establisher)
	}()
}

// ServeAll starts all listeners serving from the internal map.
func (l *Listeners) ServeAll(establisher EstablishFunc) {
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

// MockCloseFunc is a function signature which can be used in testing.
func MockCloser(id string) {}

// MockCloseFunc is a function signature which can be used in testing.
func MockEstablisher(c net.Conn) {}

// TCP is a listener for establishing client connections on basic TCP protocol.
type MockListener struct {
	sync.RWMutex

	// id is the internal id of the listener.
	id string

	// address is the address to bind to.
	address string

	// isServing indicates that the listener is serving.
	isServing bool

	// done is sent when the mock listener should close.
	done chan error
}

// Serve serves the mock listener.
func (l *MockListener) Serve(establisher EstablishFunc) {
	l.Lock()
	l.isServing = true
	l.Unlock()
DONE:
	for {
		select {
		case <-l.done:
			break DONE
		}
	}
}

// ID returns the id of the mock listener.
func (l *MockListener) ID() string {
	l.RLock()
	id := l.id
	l.RUnlock()
	return id
}

// serving indicates if the listener is serving
func (l *MockListener) serving() bool {
	l.RLock()
	ok := l.isServing
	l.RUnlock()
	return ok
}

// Close closes the mock listener.
func (l *MockListener) Close(closer CloseFunc) {
	l.Lock()
	defer l.Unlock()
	l.isServing = false
	closer(l.id)
	close(l.done)
}

// NewMockListener returns a new instance of MockListener
func NewMockListener(id, address string) *MockListener {
	return &MockListener{
		id:      id,
		address: address,
		done:    make(chan error),
	}
}
