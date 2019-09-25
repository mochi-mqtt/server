package listeners

import (
	"errors"
	"net"
	"sync"
)

// EstablishFunc is a callback function for establishing new clients.
type EstablishFunc func(net.Conn)

// CloseFunc is a callback function for closing all listener clients.
type CloseFunc func()

// Listener is an interface for network listeners. A network listener listens
// for incoming client connections and adds them to the server.
type Listener interface {

	// Serve starts the listener listening for new connections. The method
	// takes a callbacks for establishing new clients.
	Serve(EstablishFunc) error

	// ID returns the ID of the listener.
	ID() string

	// Close closes all connections for a listener and stops listening. The method
	// takes a callback for closing all client connections before ending.
	Close(CloseFunc) error
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
		err := listener.Serve(establisher)
		if err != nil {
			l.Errors <- errors.New(err.Error() + "; " + id)
		}
	}()
}

// Close stops a listener from the internal map.
func (l *Listeners) Close(id string, closer CloseFunc) {
	l.RLock()
	listener := l.internal[id]
	l.RUnlock()

	err := listener.Close(closer)
	if err != nil {
		l.Errors <- errors.New(err.Error() + "; " + id)
	}
}

/*


// ServeAll iterates and serves all registered listeners.
func (l *Listeners) ServeAll(establisher EstablishFunc) {
	l.RLock()
	ids := make([]string, 0, len(l.internal))
	for id := range l.internal {
		ids = append(ids, id)
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

	err := listener.Close(closer)
	if err != nil {
	}
	log.Println("listener ended", id, err)
}

// CloseAll iterates and closes all registered listeners.
func (l *Listeners) CloseAll(closer CloseFunc) error {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	l.RLock()
	ids := make([]string, 0, len(l.internal))
	for id := range l.internal {
		ids = append(ids, id)
	}
	l.RUnlock()

	for _, id := range ids {
		l.Close(id, closer)
	}

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		log.Println("timed out closing all")
	}

	return nil
}
*/

// MockCloseFunc is a function signature which can be used in testing.
func MockCloser() {}

// MockCloseFunc is a function signature which can be used in testing.
func MockEstablisher(c net.Conn) {}

// TCP is a listener for establishing client connections on basic TCP protocol.
type MockListener struct {

	// id is the internal id of the listener.
	id string

	// address is the address to bind to.
	address string

	// serving indicates that the listener is serving.
	serving bool

	// done is sent when the mock listener should close.
	done chan error

	// accept is sent to an establisher.
	accept chan error

	// errors is a channel of errors from the listener.
	errors chan error
}

// Serve serves the mock listener.
func (l *MockListener) Serve(establisher EstablishFunc) error {
	l.serving = true
DONE:
	for {
		select {
		case <-l.done:
			break DONE

		case o := <-l.accept:
			return o
		}
	}

	return nil
}

// ID returns the id of the mock listener.
func (l *MockListener) ID() string {
	return l.id
}

// Close closes the mock listener.
func (l *MockListener) Close(closer CloseFunc) error {
	l.serving = false
	closer()
	close(l.done)
	return nil
}

// NewMockListener returns a new instance of MockListener
func NewMockListener(id, address string) *MockListener {
	return &MockListener{
		id:      id,
		address: address,
		done:    make(chan error),
		accept:  make(chan error),
		errors:  make(chan error),
	}
}
