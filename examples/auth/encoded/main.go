// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
)

func main() {
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		done <- true
	}()

	// You can also run from top-level server.go folder:
	// go run examples/auth/encoded/main.go --path=examples/auth/encoded/auth.yaml
	// go run examples/auth/encoded/main.go --path=examples/auth/encoded/auth.json
	path := flag.String("path", "auth.yaml", "path to data auth file")
	flag.Parse()

	// Get ledger from yaml file
	data, err := os.ReadFile(*path)
	if err != nil {
		log.Fatal(err)
	}

	server := mqtt.New(nil)
	err = server.AddHook(new(auth.Hook), &auth.Options{
		Data: data, // build ledger from byte slice, yaml or json
	})
	if err != nil {
		log.Fatal(err)
	}

	tcp := listeners.NewTCP("t1", ":1883", nil)
	err = server.AddListener(tcp)
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
