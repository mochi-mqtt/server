// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 mochi-mqtt, mochi-co
// SPDX-FileContributor: Derek Duncan

package listeners

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// HTTPHealthCheck is a listener for providing an HTTP healthcheck endpoint.
type HTTPHealthCheck struct {
	sync.RWMutex
	id      string       // the internal id of the listener
	address string       // the network address to bind to
	config  *Config      // configuration values for the listener
	listen  *http.Server // the http server
	end     uint32       // ensure the close methods are only called once
}

// NewHTTPHealthCheck initialises and returns a new HTTP listener, listening on an address.
func NewHTTPHealthCheck(id, address string, config *Config) *HTTPHealthCheck {
	if config == nil {
		config = new(Config)
	}
	return &HTTPHealthCheck{
		id:      id,
		address: address,
		config:  config,
	}
}

// ID returns the id of the listener.
func (l *HTTPHealthCheck) ID() string {
	return l.id
}

// Address returns the address of the listener.
func (l *HTTPHealthCheck) Address() string {
	return l.address
}

// Protocol returns the address of the listener.
func (l *HTTPHealthCheck) Protocol() string {
	if l.listen != nil && l.listen.TLSConfig != nil {
		return "https"
	}

	return "http"
}

// Init initializes the listener.
func (l *HTTPHealthCheck) Init(_ *slog.Logger) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthcheck", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	l.listen = &http.Server{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		Addr:         l.address,
		Handler:      mux,
	}

	if l.config.TLSConfig != nil {
		l.listen.TLSConfig = l.config.TLSConfig
	}

	return nil
}

// Serve starts listening for new connections and serving responses.
func (l *HTTPHealthCheck) Serve(establish EstablishFn) {
	if l.listen.TLSConfig != nil {
		_ = l.listen.ListenAndServeTLS("", "")
	} else {
		_ = l.listen.ListenAndServe()
	}
}

// Close closes the listener and any client connections.
func (l *HTTPHealthCheck) Close(closeClients CloseFn) {
	l.Lock()
	defer l.Unlock()

	if atomic.CompareAndSwapUint32(&l.end, 0, 1) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = l.listen.Shutdown(ctx)
	}

	closeClients(l.id)
}
