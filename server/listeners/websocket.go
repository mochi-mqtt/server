package listeners

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/mochi-co/mqtt/server/listeners/auth"
	"github.com/mochi-co/mqtt/server/system"
)

var (
	// ErrInvalidMessage indicates that a message payload was not valid.
	ErrInvalidMessage = errors.New("message type not binary")

	// wsUpgrader is used to upgrade the incoming http/tcp connection to a
	// websocket compliant connection.
	wsUpgrader = &websocket.Upgrader{
		Subprotocols: []string{"mqtt"},
		CheckOrigin:  func(r *http.Request) bool { return true },
	}
)

// Websocket is a listener for establishing websocket connections.
type Websocket struct {
	sync.RWMutex
	id        string        // the internal id of the listener.
	address   string        // the network address to bind to.
	config    *Config       // configuration values for the listener.
	listen    *http.Server  // an http server for serving websocket connections.
	establish EstablishFunc // the server's establish connection handler.
	end       uint32        // ensure the close methods are only called once.
}

// wsConn is a websocket connection which satisfies the net.Conn interface.
// Inspired by
type wsConn struct {
	net.Conn
	c *websocket.Conn
}

// Read reads the next span of bytes from the websocket connection and returns
// the number of bytes read.
func (ws *wsConn) Read(p []byte) (n int, err error) {
	op, r, err := ws.c.NextReader()
	if err != nil {
		return
	}

	if op != websocket.BinaryMessage {
		err = ErrInvalidMessage
		return
	}

	return r.Read(p)
}

// Write writes bytes to the websocket connection.
func (ws *wsConn) Write(p []byte) (n int, err error) {
	err = ws.c.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return
	}

	return len(p), nil
}

// Close signals the underlying websocket conn to close.
func (ws *wsConn) Close() error {
	return ws.Conn.Close()
}

// NewWebsocket initialises and returns a new Websocket listener, listening on an address.
func NewWebsocket(id, address string) *Websocket {
	return &Websocket{
		id:      id,
		address: address,
		config: &Config{
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
	mux := http.NewServeMux()
	mux.HandleFunc("/", l.handler)
	l.listen = &http.Server{
		Addr:    l.address,
		Handler: mux,
	}

	// The following logic is deprecated in favour of passing through the tls.Config
	// value directly, however it remains in order to provide backwards compatibility.
	// It will be removed someday, so use the preferred method (l.config.TLSConfig).
	if l.config.TLS != nil && len(l.config.TLS.Certificate) > 0 && len(l.config.TLS.PrivateKey) > 0 {
		cert, err := tls.X509KeyPair(l.config.TLS.Certificate, l.config.TLS.PrivateKey)
		if err != nil {
			return err
		}

		l.listen.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
		}
	} else {
		l.listen.TLSConfig = l.config.TLSConfig
	}

	return nil
}

func (l *Websocket) handler(w http.ResponseWriter, r *http.Request) {
	c, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer c.Close()

	l.establish(l.id, &wsConn{c.UnderlyingConn(), c}, l.config.Auth)
}

// Serve starts waiting for new Websocket connections, and calls the connection
// establishment callback for any received.
func (l *Websocket) Serve(establish EstablishFunc) {
	l.establish = establish

	if l.listen.TLSConfig != nil {
		l.listen.ListenAndServeTLS("", "")
	} else {
		l.listen.ListenAndServe()
	}
}

// Close closes the listener and any client connections.
func (l *Websocket) Close(closeClients CloseFunc) {
	l.Lock()
	defer l.Unlock()

	if atomic.CompareAndSwapUint32(&l.end, 0, 1) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		l.listen.Shutdown(ctx)
	}

	closeClients(l.id)
}
