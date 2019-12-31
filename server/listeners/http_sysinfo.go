package listeners

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mochi-co/mqtt/server/listeners/auth"
	"github.com/mochi-co/mqtt/server/system"
)

// HTTPStats is a listener for presenting the server $SYS stats on a JSON http endpoint.
type HTTPStats struct {
	sync.RWMutex
	id      string       // the internal id of the listener.
	config  *Config      // configuration values for the listener.
	system  *system.Info // pointers to the server data.
	address string       // the network address to bind to.
	listen  *http.Server // the http server.
	end     int64        // ensure the close methods are only called once.}
}

// NewHTTPStats initialises and returns a new HTTP listener, listening on an address.
func NewHTTPStats(id, address string) *HTTPStats {
	return &HTTPStats{
		id:      id,
		address: address,
		config: &Config{
			Auth: new(auth.Allow),
			TLS:  new(TLS),
		},
	}
}

// SetConfig sets the configuration values for the listener config.
func (l *HTTPStats) SetConfig(config *Config) {
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
func (l *HTTPStats) ID() string {
	l.RLock()
	id := l.id
	l.RUnlock()
	return id
}

// Listen starts listening on the listener's network address.
func (l *HTTPStats) Listen(s *system.Info) error {
	l.system = s

	mux := http.NewServeMux()
	mux.HandleFunc("/", l.jsonHandler)
	l.listen = &http.Server{
		Addr:    l.address,
		Handler: mux,
	}
	return nil
}

// Serve starts listening for new connections and serving responses.
func (l *HTTPStats) Serve(establish EstablishFunc) {
	l.listen.ListenAndServe()
}

// Close closes the listener and any client connections.
func (l *HTTPStats) Close(closeClients CloseFunc) {
	l.Lock()
	defer l.Unlock()

	if atomic.LoadInt64(&l.end) == 0 {
		atomic.StoreInt64(&l.end, 1)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		l.listen.Shutdown(ctx)
	}

	closeClients(l.id)
}

// jsonHandler is an HTTP handler which outputs the $SYS stats as JSON.
func (l *HTTPStats) jsonHandler(w http.ResponseWriter, req *http.Request) {
	info, err := json.MarshalIndent(l.system, "", "\t")
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}

	w.Write(info)
}
