package listeners

import (
	"crypto/tls"
	"net"
	"sync"
	"sync/atomic"

	"github.com/vodamiro/mqtt/server/listeners/auth"
	"github.com/vodamiro/mqtt/server/system"
)

// TCP is a listener for establishing client connections on basic TCP protocol.
type TCP struct {
	sync.RWMutex
	id       string       // the internal id of the listener.
	config   *Config      // configuration values for the listener.
	protocol string       // the TCP protocol to use.
	address  string       // the network address to bind to.
	listen   net.Listener // a net.Listener which will listen for new clients.
	end      int64        // ensure the close methods are only called once.
}

// NewTCP initialises and returns a new TCP listener, listening on an address.
func NewTCP(id, address string) *TCP {
	return &TCP{
		id:       id,
		protocol: "tcp",
		address:  address,
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
func (l *TCP) Listen(s *system.Info) error {
	var err error

	if l.config.TLS != nil && len(l.config.TLS.Certificate) > 0 && len(l.config.TLS.PrivateKey) > 0 {
		cert, err := tls.X509KeyPair(l.config.TLS.Certificate, l.config.TLS.PrivateKey)
		if err != nil {
			return err
		}
		l.listen, err = tls.Listen(l.protocol, l.address, &tls.Config{
			Certificates: []tls.Certificate{cert},
		})
	} else {
		l.listen, err = net.Listen(l.protocol, l.address)
	}
	if err != nil {
		return err
	}

	return nil
}

// Serve starts waiting for new TCP connections, and calls the connection
// establishment callback for any received.
func (l *TCP) Serve(establish EstablishFunc) {
	for {
		if atomic.LoadInt64(&l.end) == 1 {
			return
		}

		conn, err := l.listen.Accept()
		if err != nil {
			return
		}

		if atomic.LoadInt64(&l.end) == 0 {
			go establish(l.id, conn, l.config.Auth)
		}
	}
}

// Close closes the listener and any client connections.
func (l *TCP) Close(closeClients CloseFunc) {
	l.Lock()
	defer l.Unlock()

	if atomic.LoadInt64(&l.end) == 0 {
		atomic.StoreInt64(&l.end, 1)
		closeClients(l.id)
	}

	if l.listen != nil {
		err := l.listen.Close()
		if err != nil {
			return
		}
	}
}
