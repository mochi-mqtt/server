// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

// Command cluster runs one node of a mochi mqtt cluster, configured by flags or
// environment variables (see -help). It is built into the cluster container image, and
// deployed by the helm chart in deploy/helm/mochi-cluster.
//
// The admin address serves:
//
//	GET  /metrics  cluster metrics in the Prometheus text format
//	GET  /stats    cluster stats as json
//	GET  /healthz  200 if the node is in contact with the session registry and not draining
//	GET  /livez    200 while the process is serving
//	POST /drain    drain the node (optional ?rate=clients-per-second)
//
// On SIGINT or SIGTERM the node is drained before it stops, unless MQTT_DRAIN_ON_STOP=false.
package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/hashicorp/memberlist"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/cluster"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
)

const seedRetryInterval = 2 * time.Second

var version = "dev" // set at build time with -ldflags "-X main.version=..."

func main() {
	cfg, err := parseConfig(os.Args[1:], os.Getenv)
	if errors.Is(err, flag.ErrHelp) {
		os.Exit(0)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, cfg); err != nil {
		slog.Error("node failed", "error", err)
		os.Exit(1)
	}
}

// run runs the node until ctx is done, then drains and stops it.
func run(ctx context.Context, cfg *config) error {
	logger := newLogger(cfg)
	slog.SetDefault(logger)
	logger.Info("starting mochi cluster node", "version", version, "node", cfg.NodeName)

	server := mqtt.New(&mqtt.Options{InlineClient: true, Logger: logger})
	if err := addAuth(server, cfg); err != nil {
		return err
	}

	if err := addListeners(server, cfg); err != nil {
		return err
	}

	opts, err := clusterOptions(ctx, cfg, logger)
	if err != nil {
		return err
	}

	node, err := cluster.New(server, opts)
	if err != nil {
		return err
	}

	if err := server.AddHook(node, nil); err != nil {
		return err
	}

	if err := node.Start(); err != nil {
		return err
	}

	if err := server.Serve(); err != nil { // starts the listeners, and returns
		_ = server.Close()
		return err
	}

	serveErr := make(chan error, 1)
	var admin *http.Server
	if cfg.AdminAddr != "" {
		admin = &http.Server{Addr: cfg.AdminAddr, Handler: adminHandler(server, node), ReadHeaderTimeout: 10 * time.Second}
		go func() {
			if err := admin.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				serveErr <- fmt.Errorf("admin http: %w", err)
			}
		}()
		logger.Info("admin http listening", "addr", cfg.AdminAddr)
	}

	select {
	case <-ctx.Done():
	case err := <-serveErr:
		_ = server.Close()
		return err
	}

	if cfg.DrainOnStop {
		logger.Warn("caught signal, draining")
		dctx, cancel := context.WithTimeout(context.Background(), cfg.DrainTimeout)
		res, err := node.Drain(dctx, cluster.DrainOptions{Rate: cfg.DrainRate})
		cancel()
		logger.Info("drained", "disconnected", res.Disconnected, "migrated", res.Migrated, "failed", res.Failed, "error", err)
	}

	logger.Warn("stopping")
	_ = server.Close() // also leaves the cluster
	if admin != nil {
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = admin.Shutdown(sctx)
		cancel()
	}
	logger.Info("stopped")
	return nil
}

func newLogger(cfg *config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: cfg.LogLevel}
	if cfg.LogFormat == "text" {
		return slog.New(slog.NewTextHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, opts))
}

func addAuth(server *mqtt.Server, cfg *config) error {
	if cfg.AuthFile == "" {
		server.Log.Warn("MQTT_ALLOW_ALL is set: every client is allowed")
		return server.AddHook(new(auth.AllowHook), nil)
	}

	data, err := os.ReadFile(cfg.AuthFile)
	if err != nil {
		return fmt.Errorf("auth file: %w", err)
	}
	return server.AddHook(new(auth.Hook), &auth.Options{Data: data})
}

func addListeners(server *mqtt.Server, cfg *config) error {
	if cfg.MQTTAddr != "" {
		if err := server.AddListener(listeners.NewTCP(listeners.Config{ID: "tcp", Address: cfg.MQTTAddr})); err != nil {
			return err
		}
	}

	if cfg.TLSCertFile != "" {
		certs, err := newCertReloader(cfg.TLSCertFile, cfg.TLSKeyFile)
		if err != nil {
			return err
		}

		tc := &tls.Config{MinVersion: tls.VersionTLS12, GetCertificate: certs.get}
		if err := server.AddListener(listeners.NewTCP(listeners.Config{ID: "tls", Address: cfg.TLSAddr, TLSConfig: tc})); err != nil {
			return err
		}
	}

	if cfg.WSAddr != "" {
		if err := server.AddListener(listeners.NewWebsocket(listeners.Config{ID: "ws", Address: cfg.WSAddr})); err != nil {
			return err
		}
	}

	return nil
}

// clusterOptions builds the cluster options, resolving the seeds. A node which may
// bootstrap tries the seeds once, and bootstraps a new cluster if none resolve to another
// node. Any other node waits until a seed resolves, so it can never start a second cluster.
func clusterOptions(ctx context.Context, cfg *config, logger *slog.Logger) (cluster.Options, error) {
	ml := memberlist.DefaultLANConfig()
	ml.SecretKey = cfg.GossipKey

	opts := cluster.Options{
		NodeName:      cfg.NodeName,
		BindAddr:      cfg.BindAddr,
		BindPort:      cfg.GossipPort,
		AdvertiseAddr: cfg.Advertise,
		ForwardAddr:   cfg.ForwardAddr,
		RaftAddr:      cfg.RaftAddr,
		Bootstrap:     cfg.bootstrap(),
		LeaseTimeout:  cfg.LeaseTimeout,
		ClaimTimeout:  cfg.ClaimTimeout,
		Memberlist:    ml,
	}

	if len(cfg.Seeds) == 0 {
		if !opts.Bootstrap {
			return opts, errors.New("no seeds (MQTT_SEEDS) are set: set MQTT_BOOTSTRAP=true to run a new cluster")
		}
		logger.Info("no seeds set, bootstrapping a new cluster")
		return opts, nil
	}

	self := selfAddrs(cfg.Advertise)
	deadline := time.Now().Add(cfg.JoinTimeout)
	for {
		opts.Seeds = resolveSeeds(ctx, cfg.Seeds, self, net.DefaultResolver.LookupHost)
		if len(opts.Seeds) > 0 {
			logger.Info("joining cluster", "seeds", opts.Seeds)
			return opts, nil
		}

		if opts.Bootstrap {
			logger.Info("no other nodes found, bootstrapping a new cluster", "seeds", cfg.Seeds)
			return opts, nil
		}

		if time.Now().After(deadline) {
			return opts, fmt.Errorf("no seed resolved to another node within %s: %v", cfg.JoinTimeout, cfg.Seeds)
		}

		logger.Info("waiting for seeds to resolve", "seeds", cfg.Seeds)
		select {
		case <-ctx.Done():
			return opts, ctx.Err()
		case <-time.After(seedRetryInterval):
		}
	}
}

func adminHandler(server *mqtt.Server, node *cluster.Cluster) http.Handler {
	writeJSON := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", node.MetricsHandler())
	mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, node.Stats())
	})
	mux.HandleFunc("/livez", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if s := node.Stats(); !s.InContact || s.Draining {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("/drain", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "use POST", http.StatusMethodNotAllowed)
			return
		}
		rate, _ := strconv.Atoi(r.URL.Query().Get("rate"))
		res, err := node.Drain(r.Context(), cluster.DrainOptions{Rate: rate})
		resp := map[string]any{"result": res}
		if err != nil {
			resp["error"] = err.Error()
		}
		writeJSON(w, resp)
	})
	return mux
}

// certReloader serves a tls certificate from files, reloading it when the certificate
// file changes, so that renewed certificates (from cert-manager, say) apply without a
// restart.
type certReloader struct {
	certFile, keyFile string

	mu      sync.Mutex
	cert    *tls.Certificate
	modTime time.Time
}

func newCertReloader(certFile, keyFile string) (*certReloader, error) {
	r := &certReloader{certFile: certFile, keyFile: keyFile}
	if _, err := r.get(nil); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *certReloader) get(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	info, err := os.Stat(r.certFile)
	if err != nil {
		if r.cert != nil {
			return r.cert, nil // keep serving the last certificate
		}
		return nil, fmt.Errorf("tls certificate: %w", err)
	}

	if r.cert != nil && info.ModTime().Equal(r.modTime) {
		return r.cert, nil
	}

	cert, err := tls.LoadX509KeyPair(r.certFile, r.keyFile)
	if err != nil {
		if r.cert != nil {
			return r.cert, nil // the files may be mid-update
		}
		return nil, fmt.Errorf("tls certificate: %w", err)
	}

	r.cert, r.modTime = &cert, info.ModTime()
	return r.cert, nil
}
