package listeners

import (
	"net"
	"sync"

	"github.com/mochi-co/mqtt/auth"
)

// TCP is a listener for establishing client connections on basic TCP protocol.
type TCP struct {
	sync.RWMutex

	// id is the internal id of the listener.
	id string

	// config contains configuration values for the listener.
	config *Config

	// protocol is the TCP protocol to use.
	protocol string

	// address is the address to bind to.
	address string

	// listen is a net.Listener which will listen for new clients.
	listen net.Listener

	// done is a channel which indicates the process is done and should end.
	done chan bool

	// start can be called to ensure the serve methods are only called once.
	start *sync.Once

	// end can be called to ensure the close methods are only called once.
	end *sync.Once
}

// NewTCP initialises and returns a new TCP listener, listening on an address.
func NewTCP(id, address string) *TCP {
	return &TCP{
		id:       id,
		protocol: "tcp",
		address:  address,
		done:     make(chan bool),
		start:    new(sync.Once),
		end:      new(sync.Once),
		config: &Config{ // default configuration.
			Auth: new(auth.Allow),
			TLS:  new(TLS),
		},
	}
}

// SetConfig sets the configuration values for the listener config.
func (l *TCP) SetConfig(config *Config) {
	l.Lock()
	if config != nil {
		l.config = config

		// If a config has been passed without an auth controller,
		// it may be a mistake, so disallow all traffic.
		if l.config.Auth == nil {
			l.config.Auth = new(auth.Disallow)
		}
	}

	l.Unlock()
}

// ID returns the id of the listener.
func (l *TCP) ID() string {
	l.RLock()
	id := l.id
	l.RUnlock()
	return id
}

// Listen starts listening on the listener's network address.
func (l *TCP) Listen() error {
	var err error
	l.listen, err = net.Listen(l.protocol, l.address)
	if err != nil {
		return err
	}

	return nil
}

// Serve starts waiting for new TCP connections, and calls the connection
// establishment callback for any received.
func (l *TCP) Serve(establish EstablishFunc) {
	l.start.Do(func() {
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
				go func(c net.Conn) {
					err := establish(c, l.config.Auth)
					if err != nil {
						return
					}
				}(conn)
			}
		}
	})
}

// Close closes the listener and any client connections.
func (l *TCP) Close(closeClients CloseFunc) {
	l.Lock()
	defer l.Unlock()

	l.end.Do(func() {
		closeClients(l.id)
		close(l.done)
	})

	if l.listen != nil {
		err := l.listen.Close()
		if err != nil {
			return
		}

		// Shunt listener off blocking listen.Accept() loop (self-pipe trick).
		net.Dial(l.protocol, l.listen.Addr().String())
	}
}
