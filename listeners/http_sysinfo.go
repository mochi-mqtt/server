// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package listeners

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mochi-mqtt/server/v2/system"
)

// HTTPStats is a listener for presenting the server $SYS stats on a JSON http endpoint.
type HTTPStats struct {
	sync.RWMutex
	id      string       // the internal id of the listener
	address string       // the network address to bind to
	config  *Config      // configuration values for the listener
	listen  *http.Server // the http server
	sysInfo *system.Info // pointers to the server data
	log     *slog.Logger // server logger
	end     uint32       // ensure the close methods are only called once
}

// NewHTTPStats initialises and returns a new HTTP listener, listening on an address.
func NewHTTPStats(id, address string, config *Config, sysInfo *system.Info) *HTTPStats {
	if config == nil {
		config = new(Config)
	}
	return &HTTPStats{
		id:      id,
		address: address,
		sysInfo: sysInfo,
		config:  config,
	}
}

// ID returns the id of the listener.
func (l *HTTPStats) ID() string {
	return l.id
}

// Address returns the address of the listener.
func (l *HTTPStats) Address() string {
	return l.address
}

// Protocol returns the address of the listener.
func (l *HTTPStats) Protocol() string {
	if l.listen != nil && l.listen.TLSConfig != nil {
		return "https"
	}

	return "http"
}

// Init initializes the listener.
func (l *HTTPStats) Init(log *slog.Logger) error {
	l.log = log
	mux := http.NewServeMux()
	mux.HandleFunc("/", l.jsonHandler)
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
func (l *HTTPStats) Serve(establish EstablishFn) {

	var err error
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
func (l *HTTPStats) Close(closeClients CloseFn) {
	l.Lock()
	defer l.Unlock()

	if atomic.CompareAndSwapUint32(&l.end, 0, 1) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = l.listen.Shutdown(ctx)
	}

	closeClients(l.id)
}

// jsonHandler is an HTTP handler which outputs the $SYS stats as JSON.
func (l *HTTPStats) jsonHandler(w http.ResponseWriter, req *http.Request) {
	info := *l.sysInfo.Clone()

	out, err := json.MarshalIndent(info, "", "\t")
	if err != nil {
		_, _ = io.WriteString(w, err.Error())
	}

	_, _ = w.Write(out)
}
