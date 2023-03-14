// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-co
// SPDX-FileContributor: mochi-co

package listeners

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strconv"
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

	metricsInt64 []metric // metric exposed on /metrics
}

type metric struct {
	Tag, Name string
}

// NewHTTPStats initialises and returns a new HTTP listener, listening on an address.
func NewHTTPStats(id, address string, config *Config, sysInfo *system.Info) *HTTPStats {
	if config == nil {
		config = new(Config)
	}

	v := reflect.TypeOf(*sysInfo)
	metricsInt64 := make([]metric, 0, v.NumField())
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if f.Type.Kind() != reflect.Int64 {
			continue
		}
		metricsInt64 = append(metricsInt64, metric{
			Tag:  f.Tag.Get("json"),
			Name: f.Name,
		})
	}

	return &HTTPStats{
		id:           id,
		address:      address,
		sysInfo:      sysInfo,
		config:       config,
		metricsInt64: metricsInt64,
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
	mux.HandleFunc("/metrics", l.metricsHandler)
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
	info := *l.sysInfo.Clone()

	out, err := json.MarshalIndent(info, "", "\t")
	if err != nil {
		io.WriteString(w, err.Error())
	}

	w.Write(out)
}

func (l *HTTPStats) metricsHandler(w http.ResponseWriter, req *http.Request) {
	l.WriteMetrics(w)
}

// WriteMetrics writes the metrics in a prometheus format to the writer.
func (l *HTTPStats) WriteMetrics(w io.Writer) error {
	info := *l.sysInfo.Clone()

	v := reflect.ValueOf(info)
	for _, m := range l.metricsInt64 {
		f := v.FieldByName(m.Name)
		_, err := w.Write([]byte(m.Tag + " " + strconv.FormatInt(f.Int(), 10) + "\n"))
		if err != nil {
			return err
		}
	}
	return nil
}
