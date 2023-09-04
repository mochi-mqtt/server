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

	server := mqtt.New(nil)
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
			server.Log.Info().Str("client", cl.ID).Int("subid", sub.Identifier).Str("topic", pk.TopicName).Str("payload", string(pk.Payload)).Msgf("inline client received message from subscription")
		}
		server.Log.Info().Msgf("inline client subscribing")
		server.Subscribe("direct/#", 1, callbackFn)
		server.Subscribe("direct/#", 2, callbackFn)
	}()

	// There is a shorthand convenience function, Publish, for easily sending
	// publish packets if you are not concerned with creating your own packets.
	go func() {
		for range time.Tick(time.Second * 3) {
			err := server.Publish("direct/publish", []byte("scheduled message"), false, 0)
			if err != nil {
				server.Log.Error().Err(err).Msg("server.Publish")
			}
			server.Log.Info().Msgf("main.go issued direct message to direct/publish")
		}
	}()

	go func() {
		time.Sleep(time.Second * 10)
		// Unsubscribe from the same filter to stop receiving messages.
		server.Log.Info().Msgf("inline client unsubscribing")
		server.Unsubscribe("direct/#", 1)
	}()
	// If you want to have more control over your packets, you can directly inject a packet of any kind into the broker.
	//go func() {
	//	for range time.Tick(time.Second * 5) {
	//		err := server.InjectPacket(cl, packets.Packet{
	//			FixedHeader: packets.FixedHeader{
	//				Type: packets.Publish,
	//			},
	//			TopicName: "direct/publish",
	//			Payload:   []byte("injected scheduled message"),
	//		})
	//		if err != nil {
	//			log.Fatal(err)
	//		}
	//	}
	//}()

	<-done
	server.Log.Warn().Msg("caught signal, stopping...")
	_ = server.Close()
	server.Log.Info().Msg("main.go finished")
}
