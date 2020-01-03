package listeners

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/net/websocket"

	"github.com/mochi-co/mqtt/server/listeners/auth"
	"github.com/mochi-co/mqtt/server/system"
)

// Websocket is a listener for establishing websocket connections.
type Websocket struct {
	sync.RWMutex
	id       string       // the internal id of the listener.
	protocol string       // the protocol of the listener.
	config   *Config      // configuration values for the listener.
	address  string       // the network address to bind to.
	listen   net.Listener // a net.Listener which will listen for new clients.
	end      int64        // ensure the close methods are only called once.}
}

// NewWebsocket initialises and returns a new Websocket listener, listening on an address.
func NewWebsocket(id, address string) *Websocket {
	return &Websocket{
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
func (l *Websocket) SetConfig(config *Config) {
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
func (l *Websocket) ID() string {
	l.RLock()
	id := l.id
	l.RUnlock()
	return id
}

// Listen starts listening on the listener's network address.
func (l *Websocket) Listen(s *system.Info) error {
	var err error
	l.listen, err = net.Listen(l.protocol, l.address)
	if err != nil {
		return err
	}

	return nil
}

// Serve starts waiting for new Websocket connections, and calls the connection
// establishment callback for any received.
func (l *Websocket) Serve(establish EstablishFunc) {
	server := &websocket.Server{
		Handshake: func(c *websocket.Config, req *http.Request) error {

			c.Protocol = []string{"mqtt"}

			// If the remote address is an IP, prepend a protocol string so it can
			// be parsed without errors.
			if !strings.Contains(req.RemoteAddr, "://") {
				req.RemoteAddr = "ws://" + req.RemoteAddr
			}

			// Websocket struggles to get a request origin address, so the remote
			// address from the request is parsed into the origin struct instead.
			var err error
			c.Origin, err = url.Parse(req.RemoteAddr)
			if err != nil {
				fmt.Println(err)
			}

			return nil
		},
		Handler: func(c *websocket.Conn) {
			c.PayloadType = websocket.BinaryFrame
			err := establish(l.id, c, l.config.Auth)
			if err != nil {
				fmt.Println(err)
			}
		},
	}

	err := http.Serve(l.listen, server)
	if err != nil {
		return
	}
}

// Close closes the listener and any client connections.
func (l *Websocket) Close(closeClients CloseFunc) {
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
