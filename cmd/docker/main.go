// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 mochi-mqtt
// SPDX-FileContributor: dgduncan, mochi-co, kevinta893

package main

import (
	"crypto/tls"
	"flag"
	"github.com/mochi-mqtt/server/v2/config"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	mqtt "github.com/mochi-mqtt/server/v2"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil))) // set basic logger to ensure logs before configuration are in a consistent format

	configFile := flag.String("config", "config.yaml", "path to mochi config yaml or json file")
	flag.Parse()

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		done <- true
	}()

	configBytes, err := os.ReadFile(*configFile)
	if err != nil {
		log.Fatal(err)
	}

	options, err := config.FromBytes(configBytes)
	if err != nil {
		log.Fatal(err)
	}

	for i, listenerConfig := range options.Listeners {
		if listenerConfig.TLSCert != nil && listenerConfig.TLSCert.CertFile != nil && listenerConfig.TLSCert.KeyFile != nil && *listenerConfig.TLSCert.CertFile != "" && *listenerConfig.TLSCert.KeyFile != "" {
			cert, err := tls.LoadX509KeyPair(*listenerConfig.TLSCert.CertFile, *listenerConfig.TLSCert.KeyFile)
			if err != nil {
				log.Fatal(err)
			}

			options.Listeners[i].TLSConfig = &tls.Config{
				Certificates: []tls.Certificate{cert},
			}

			slog.Info("loaded listener tls cert", "id", listenerConfig.ID, "protocol", listenerConfig.Type, "address", listenerConfig.Address)
		}
	}

	server := mqtt.New(options)

	go func() {
		err := server.Serve()
		if err != nil {
			log.Fatal(err)
		}
	}()

	<-done
	server.Log.Warn("caught signal, stopping...")
	_ = server.Close()
	server.Log.Info("mochi mqtt shutdown complete")
}
