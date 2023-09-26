// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package listeners

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"log/slog"

	"github.com/gorilla/websocket"
)

var (
	// ErrInvalidMessage indicates that a message payload was not valid.
	ErrInvalidMessage = errors.New("message type not binary")
)

// Websocket is a listener for establishing websocket connections.
type Websocket struct { // [MQTT-4.2.0-1]
	sync.RWMutex
	id        string              // the internal id of the listener
	address   string              // the network address to bind to
	config    *Config             // configuration values for the listener
	listen    *http.Server        // a http server for serving websocket connections
	log       *slog.Logger        // server logger
	establish EstablishFn         // the server's establish connection handler
	upgrader  *websocket.Upgrader //  upgrade the incoming http/tcp connection to a websocket compliant connection.
	end       uint32              // ensure the close methods are only called once
}

// NewWebsocket initialises and returns a new Websocket listener, listening on an address.
func NewWebsocket(id, address string, config *Config) *Websocket {
	if config == nil {
		config = new(Config)
	}

	return &Websocket{
		id:      id,
		address: address,
		config:  config,
		upgrader: &websocket.Upgrader{
			Subprotocols: []string{"mqtt"},
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

// ID returns the id of the listener.
func (l *Websocket) ID() string {
	return l.id
}

// Address returns the address of the listener.
func (l *Websocket) Address() string {
	return l.address
}

// Protocol returns the address of the listener.
func (l *Websocket) Protocol() string {
	if l.config.TLSConfig != nil {
		return "wss"
	}

	return "ws"
}

// Init initializes the listener.
func (l *Websocket) Init(log *slog.Logger) error {
	l.log = log

	mux := http.NewServeMux()
	mux.HandleFunc("/", l.handler)
	l.listen = &http.Server{
		Addr:         l.address,
		Handler:      mux,
		TLSConfig:    l.config.TLSConfig,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	return nil
}

// handler upgrades and handles an incoming websocket connection.
func (l *Websocket) handler(w http.ResponseWriter, r *http.Request) {
	c, err := l.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer c.Close()

	err = l.establish(l.id, &wsConn{Conn: c.UnderlyingConn(), c: c})
	if err != nil {
		l.log.Warn("", "error", err)
	}
}

// Serve starts waiting for new Websocket connections, and calls the connection
// establishment callback for any received.
func (l *Websocket) Serve(establish EstablishFn) {
	var err error
	l.establish = establish

	if l.listen.TLSConfig != nil {
		err = l.listen.ListenAndServeTLS("", "")
	} else {
		err = l.listen.ListenAndServe()
	}

	// After the listener has been shutdown, no need to print the http.ErrServerClosed error.
	if err != nil && atomic.LoadUint32(&l.end) == 0 {
		l.log.Error("failed to serve.", "error", err, "listener", l.id)
	}
}

// Close closes the listener and any client connections.
func (l *Websocket) Close(closeClients CloseFn) {
	l.Lock()
	defer l.Unlock()

	if atomic.CompareAndSwapUint32(&l.end, 0, 1) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = l.listen.Shutdown(ctx)
	}

	closeClients(l.id)
}

// wsConn is a websocket connection which satisfies the net.Conn interface.
type wsConn struct {
	net.Conn
	c *websocket.Conn

	// reader for the current message (can be nil)
	r io.Reader
}

// Read reads the next span of bytes from the websocket connection and returns the number of bytes read.
func (ws *wsConn) Read(p []byte) (int, error) {
	if ws.r == nil {
		op, r, err := ws.c.NextReader()
		if err != nil {
			return 0, err
		}

		if op != websocket.BinaryMessage {
			err = ErrInvalidMessage
			return 0, err
		}

		ws.r = r
	}

	var n int
	for {
		// buffer is full, return what we've read so far
		if n == len(p) {
			return n, nil
		}

		br, err := ws.r.Read(p[n:])
		n += br
		if err != nil {
			// when ANY error occurs, we consider this the end of the current message (either because it really is, via
			// io.EOF, or because something bad happened, in which case we want to drop the remainder)
			ws.r = nil

			if errors.Is(err, io.EOF) {
				err = nil
			}
			return n, err
		}
	}
}

// Write writes bytes to the websocket connection.
func (ws *wsConn) Write(p []byte) (int, error) {
	err := ws.c.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return 0, err
	}

	return len(p), nil
}

// Close signals the underlying websocket conn to close.
func (ws *wsConn) Close() error {
	return ws.Conn.Close()
}
