// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	badgerdb "github.com/dgraph-io/badger/v4"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/hooks/storage/badger"
	"github.com/mochi-mqtt/server/v2/listeners"
)

func main() {
	badgerPath := ".badger"
	defer os.RemoveAll(badgerPath) // remove the example badger files at the end

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		done <- true
	}()

	server := mqtt.New(nil)
	_ = server.AddHook(new(auth.AllowHook), nil)

	badgerOpts := badgerdb.DefaultOptions(badgerPath) // BadgerDB options. Adjust according to your actual scenario.
	badgerOpts.ValueLogFileSize = 100 * (1 << 20)     // Set the default size of the log file to 100 MB.

	// AddHook adds a BadgerDB hook to the server with the specified options.
	// GcInterval specifies the interval at which BadgerDB garbage collection process runs.
	// Refer to https://dgraph.io/docs/badger/get-started/#garbage-collection for more information.
	err := server.AddHook(new(badger.Hook), &badger.Options{
		Path: badgerPath,

		// Set the interval for garbage collection. Adjust according to your actual scenario.
		GcInterval: 5 * 60,

		// GcDiscardRatio specifies the ratio of log discard compared to the maximum possible log discard.
		// Setting it to a higher value would result in fewer space reclaims, while setting it to a lower value
		// would result in more space reclaims at the cost of increased activity on the LSM tree.
		// discardRatio must be in the range (0.0, 1.0), both endpoints excluded, otherwise, it will be set to the default value of 0.5.
		// Adjust according to your actual scenario.
		GcDiscardRatio: 0.5,

		Options: &badgerOpts,
	})
	if err != nil {
		log.Fatal(err)
	}

	tcp := listeners.NewTCP(listeners.Config{
		ID:      "t1",
		Address: ":1883",
	})
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
