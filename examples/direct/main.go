// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mochi-mqtt/server/v2/hooks/auth"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

func main() {
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		done <- true
	}()

	server := mqtt.New(&mqtt.Options{
		InlineClient: true, // you must enable inline client to use direct publishing and subscribing.
	})
	_ = server.AddHook(new(auth.AllowHook), nil)

	// Start the server
	go func() {
		err := server.Serve()
		if err != nil {
			log.Fatal(err)
		}
	}()

	// Demonstration of using an inline client to directly subscribe to a topic and receive a message when
	// that subscription is activated. The inline subscription method uses the same internal subscription logic
	// as used for external (normal) clients.
	go func() {
		// Inline subscriptions can also receive retained messages on subscription.
		_ = server.Publish("direct/retained", []byte("retained message"), true, 0)
		_ = server.Publish("direct/alternate/retained", []byte("some other retained message"), true, 0)

		// Subscribe to a filter and handle any received messages via a callback function.
		callbackFn := func(cl *mqtt.Client, sub packets.Subscription, pk packets.Packet) {
			server.Log.Info("inline client received message from subscription", "client", cl.ID, "subscriptionId", sub.Identifier, "topic", pk.TopicName, "payload", string(pk.Payload))
		}
		server.Log.Info("inline client subscribing")
		_ = server.Subscribe("direct/#", 1, callbackFn)
		_ = server.Subscribe("direct/#", 2, callbackFn)
	}()

	// There is a shorthand convenience function, Publish, for easily sending  publish packets if you are not
	// concerned with creating your own packets.  If you want to have more control over your packets, you can
	//directly inject a packet of any kind into the broker. See examples/hooks/main.go for usage.
	go func() {
		for range time.Tick(time.Second * 3) {
			err := server.Publish("direct/publish", []byte("scheduled message"), false, 0)
			if err != nil {
				server.Log.Error("server.Publish", "error", err)
			}
			server.Log.Info("main.go issued direct message to direct/publish")
		}
	}()

	go func() {
		time.Sleep(time.Second * 10)
		// Unsubscribe from the same filter to stop receiving messages.
		server.Log.Info("inline client unsubscribing")
		_ = server.Unsubscribe("direct/#", 1)
	}()

	<-done
	server.Log.Warn("caught signal, stopping...")
	_ = server.Close()
	server.Log.Info("main.go finished")
}
