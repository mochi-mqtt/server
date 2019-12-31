package server

/*
import (
	"sync"
)

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
*/
