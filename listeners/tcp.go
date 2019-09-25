package listeners

import (
	"net"
	"sync"
)

// TCP is a listener for establishing client connections on basic TCP protocol.
type TCP struct {

	// id is the internal id of the listener.
	id string

	// protocol is the TCP protocol to use.
	protocol string

	// address is the address to bind to.
	address string

	// listen is a net.Listener which will listen for new clients.
	listen net.Listener

	// done is a channel which indicates the process is done and should end.
	done chan bool

	// end can be called to ensure the close methods are only called once.
	end *sync.Once
}

// NewTCP initialises and returns a new TCP listener, listening on an address.
func NewTCP(id, address string) (l *TCP, err error) {
	l = &TCP{
		id:       id,
		protocol: "tcp",
		address:  address,
		done:     make(chan bool),
		end:      new(sync.Once),
	}

	l.listen, err = net.Listen(l.protocol, l.address)
	if err != nil {
		return
	}

	return
}

// ID returns the id of the listener.
func (l *TCP) ID() string {
	return l.id
}

// Serve starts listening for new TCP connections, and calls the connection
// establishment callback for any received.
func (l *TCP) Serve(establish EstablishFunc) {
DONE:
	for {
		select {
		case <-l.done:
			break DONE

		default:

			// Block until a new connection is available.
			conn, err := l.listen.Accept()
			if err != nil {
				break // Not interested in broken connections.
			}

			// Establish connection in a new goroutine.
			go establish(conn)
		}
	}
}

// Close closes the listener and any client connections.
func (l *TCP) Close(closeClients CloseFunc) {
	l.end.Do(func() {
		closeClients()
		close(l.done)
	})

	if l.listen != nil {
		err := l.listen.Close()
		if err != nil {
			// Log the error
			return
		}

		// Shunt listener off blocking listen.Accept() loop (self-pipe trick).
		net.Dial(l.protocol, l.listen.Addr().String())
	}
}
