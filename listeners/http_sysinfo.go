// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-co
// SPDX-FileContributor: mochi-co

package listeners

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mochi-co/mqtt/v2/system"

	"github.com/rs/zerolog"
)

// HTTPStats is a listener for presenting the server $SYS stats on a JSON http endpoint.
type HTTPStats struct {
	sync.RWMutex
	id      string          // the internal id of the listener
	address string          // the network address to bind to
	config  *Config         // configuration values for the listener
	listen  *http.Server    // the http server
	log     *zerolog.Logger // server logger
	sysInfo *system.Info    // pointers to the server data
	end     uint32          // ensure the close methods are only called once
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
func (l *HTTPStats) Init(log *zerolog.Logger) error {
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
	if l.listen.TLSConfig != nil {
		l.listen.ListenAndServeTLS("", "")
	} else {
		l.listen.ListenAndServe()
	}
}

// Close closes the listener and any client connections.
func (l *HTTPStats) Close(closeClients CloseFn) {
	l.Lock()
	defer l.Unlock()

	if atomic.CompareAndSwapUint32(&l.end, 0, 1) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		l.listen.Shutdown(ctx)
	}

	closeClients(l.id)
}

// jsonHandler is an HTTP handler which outputs the $SYS stats as JSON.
func (l *HTTPStats) jsonHandler(w http.ResponseWriter, req *http.Request) {
	info := &system.Info{
		Version:             l.sysInfo.Version,
		Started:             atomic.LoadInt64(&l.sysInfo.Started),
		Time:                atomic.LoadInt64(&l.sysInfo.Time),
		Uptime:              atomic.LoadInt64(&l.sysInfo.Uptime),
		BytesReceived:       atomic.LoadInt64(&l.sysInfo.BytesReceived),
		BytesSent:           atomic.LoadInt64(&l.sysInfo.BytesSent),
		ClientsConnected:    atomic.LoadInt64(&l.sysInfo.ClientsConnected),
		ClientsMaximum:      atomic.LoadInt64(&l.sysInfo.ClientsMaximum),
		ClientsTotal:        atomic.LoadInt64(&l.sysInfo.ClientsTotal),
		ClientsDisconnected: atomic.LoadInt64(&l.sysInfo.ClientsDisconnected),
		MessagesReceived:    atomic.LoadInt64(&l.sysInfo.MessagesReceived),
		MessagesSent:        atomic.LoadInt64(&l.sysInfo.MessagesSent),
		InflightDropped:     atomic.LoadInt64(&l.sysInfo.InflightDropped),
		Subscriptions:       atomic.LoadInt64(&l.sysInfo.Subscriptions),
		PacketsReceived:     atomic.LoadInt64(&l.sysInfo.PacketsReceived),
		PacketsSent:         atomic.LoadInt64(&l.sysInfo.PacketsSent),
		Retained:            atomic.LoadInt64(&l.sysInfo.Retained),
		Inflight:            atomic.LoadInt64(&l.sysInfo.Inflight),
		MemoryAlloc:         atomic.LoadInt64(&l.sysInfo.MemoryAlloc),
		Threads:             atomic.LoadInt64(&l.sysInfo.Threads),
	}

	out, err := json.MarshalIndent(info, "", "\t")
	if err != nil {
		io.WriteString(w, err.Error())
	}

	w.Write(out)
}
