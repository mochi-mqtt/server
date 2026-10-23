// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"runtime/pprof"
	"strings"
	"syscall"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/cluster"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
)

func main() {
	tcpAddr := flag.String("tcp", ":1883", "network address for TCP listener")
	maxInflight := flag.Int("max-inflight", 8192, "qos 1 and 2 messages which may be stored per client")
	cpuProfile := flag.String("cpuprofile", "", "write a cpu profile to this file until the server stops")
	nodeName := flag.String("cluster-name", "", "run as a cluster node with this name (standalone if empty)")
	bindAddr := flag.String("cluster-bind", "127.0.0.1", "cluster gossip bind address")
	gossipPort := flag.Int("cluster-gossip", 7946, "cluster gossip port")
	forwardAddr := flag.String("cluster-forward", ":7947", "cluster peer link address")
	raftAddr := flag.String("cluster-raft", ":7948", "cluster session registry address")
	seeds := flag.String("cluster-seeds", "", "comma-separated gossip addresses of existing cluster nodes")
	flag.Parse()

	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			log.Fatal(err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal(err)
		}
		defer pprof.StopCPUProfile()
	}

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		done <- true
	}()

	server := mqtt.New(nil)
	server.Options.Capabilities.MaximumClientWritesPending = 16 * 1024
	server.Options.Capabilities.MaximumInflight = uint16(*maxInflight)
	_ = server.AddHook(new(auth.AllowHook), nil)

	if *nodeName != "" {
		opts := cluster.Options{
			NodeName:      *nodeName,
			BindAddr:      *bindAddr,
			BindPort:      *gossipPort,
			ForwardAddr:   *forwardAddr,
			RaftAddr:      *raftAddr,
			PeerQueueSize: 64 * 1024,
		}
		if *seeds != "" {
			opts.Seeds = strings.Split(*seeds, ",")
		}

		node, err := cluster.New(server, opts)
		if err != nil {
			log.Fatal(err)
		}
		if err := server.AddHook(node, nil); err != nil {
			log.Fatal(err)
		}
		if err := node.Start(); err != nil {
			log.Fatal(err)
		}
	}

	tcp := listeners.NewTCP(listeners.Config{
		ID:      "t1",
		Address: *tcpAddr,
	})
	err := server.AddListener(tcp)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		err := server.Serve()
		if err != nil {
			log.Fatal(err)
		}
	}()

	<-done
	server.Log.Warn("caught signal, stopping...")
	_ = server.Close()
	server.Log.Info("main.go finished")
}
