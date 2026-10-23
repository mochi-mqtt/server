// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

// This example runs one node of a cluster. To run three nodes on one machine:
//
//	go run ./examples/cluster -name n1 -mqtt :1883 -gossip 7946 -forward :7947 -raft :7948 -http :8081
//	go run ./examples/cluster -name n2 -mqtt :1884 -gossip 7956 -forward :7957 -raft :7958 -http :8082 -seeds 127.0.0.1:7946
//	go run ./examples/cluster -name n3 -mqtt :1885 -gossip 7966 -forward :7967 -raft :7968 -http :8083 -seeds 127.0.0.1:7946
//
// A client subscribed on one node receives messages published on any other node, and a
// client can reconnect to any node and resume its session.
//
// The http address serves:
//
//	GET  /metrics  cluster metrics in the Prometheus text format
//	GET  /stats    cluster stats as json
//	GET  /clients  the node's clients as json
//	GET  /healthz  200 if the node is in contact with the session registry
//	POST /drain    drain the node (optional ?rate=clients-per-second), returning the result
//
// On SIGINT or SIGTERM the node is drained before it stops, unless -drain-on-stop=false.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/cluster"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
)

func main() {
	name := flag.String("name", "", "unique node name (required)")
	mqttAddr := flag.String("mqtt", ":1883", "mqtt tcp listener address")
	bindAddr := flag.String("bind", "127.0.0.1", "gossip bind address")
	advertise := flag.String("advertise", "", "address announced to other nodes, if different from the bind address")
	gossipPort := flag.Int("gossip", 7946, "gossip port")
	forwardAddr := flag.String("forward", ":7947", "peer link listener address")
	raftAddr := flag.String("raft", ":7948", "session registry (raft) listener address")
	httpAddr := flag.String("http", "", "admin http listener address for metrics, stats and draining")
	seeds := flag.String("seeds", "", "comma-separated gossip addresses of existing nodes")
	bootstrap := flag.Bool("bootstrap", false, "create a new cluster if no seed can be joined (first node only)")
	lease := flag.Duration("lease", 0, "registry lease timeout (default 10s)")
	claim := flag.Duration("claim", 0, "session claim timeout (default 5s)")
	drainOnStop := flag.Bool("drain-on-stop", true, "drain the node before stopping")
	drainRate := flag.Int("drain-rate", 0, "clients disconnected per second when draining (0 for all at once)")
	flag.Parse()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	server := mqtt.New(&mqtt.Options{InlineClient: true})
	err := server.AddHook(new(auth.AllowHook), nil)
	if err != nil {
		log.Fatal(err)
	}

	opts := cluster.Options{
		NodeName:      *name,
		BindAddr:      *bindAddr,
		BindPort:      *gossipPort,
		AdvertiseAddr: *advertise,
		ForwardAddr:   *forwardAddr,
		RaftAddr:      *raftAddr,
		Bootstrap:     *bootstrap,
		LeaseTimeout:  *lease,
		ClaimTimeout:  *claim,
	}
	if *seeds != "" {
		opts.Seeds = strings.Split(*seeds, ",")
	}

	node, err := cluster.New(server, opts)
	if err != nil {
		log.Fatal(err)
	}

	err = server.AddHook(node, nil)
	if err != nil {
		log.Fatal(err)
	}

	err = server.AddListener(listeners.NewTCP(listeners.Config{ID: "t1", Address: *mqttAddr}))
	if err != nil {
		log.Fatal(err)
	}

	err = node.Start()
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		err := server.Serve()
		if err != nil {
			log.Fatal(err)
		}
	}()

	if *httpAddr != "" {
		go serveAdmin(*httpAddr, server, node)
	}

	<-sigs
	if *drainOnStop {
		server.Log.Warn("caught signal, draining...")
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		res, err := node.Drain(ctx, cluster.DrainOptions{Rate: *drainRate})
		cancel()
		server.Log.Info("drained", "disconnected", res.Disconnected, "migrated", res.Migrated, "failed", res.Failed, "error", err)
	}

	server.Log.Warn("stopping...")
	_ = server.Close() // also leaves the cluster
	server.Log.Info("main.go finished")
}

// serveAdmin serves metrics, stats and draining over http.
func serveAdmin(addr string, server *mqtt.Server, node *cluster.Cluster) {
	writeJSON := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", node.MetricsHandler())
	mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, node.Stats())
	})
	mux.HandleFunc("/clients", func(w http.ResponseWriter, r *http.Request) {
		type client struct {
			ID        string `json:"id"`
			Connected bool   `json:"connected"`
		}
		clients := []client{}
		for _, cl := range server.Clients.GetAll() {
			if !cl.Net.Inline {
				clients = append(clients, client{ID: cl.ID, Connected: !cl.Closed()})
			}
		}
		writeJSON(w, clients)
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

	server.Log.Info("admin http listening", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
