// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-co
// SPDX-FileContributor: mochi-co

package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mochi-co/mqtt/v2"
	"github.com/mochi-co/mqtt/v2/hooks/auth"
	"github.com/mochi-co/mqtt/v2/listeners"
	"github.com/mochi-co/mqtt/v2/packets"
	"golang.org/x/exp/slog"
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
	tcp := listeners.NewTCP("t1", ":1883", nil)
	err := server.AddListener(tcp)
	if err != nil {
		log.Fatal(err)
	}

	err = server.AddHook(new(ExampleHook), map[string]any{})
	if err != nil {
		log.Fatal(err)
	}

	// Start the server
	go func() {
		err := server.Serve()
		if err != nil {
			log.Fatal(err)
		}
	}()

	// Demonstration of directly publishing messages to a topic via the
	// `server.Publish` method. Subscribe to `direct/publish` using your
	// MQTT client to see the messages.
	go func() {
		cl := server.NewClient(nil, "local", "inline", true)
		for range time.Tick(time.Second * 1) {
			err := server.InjectPacket(cl, packets.Packet{
				FixedHeader: packets.FixedHeader{
					Type: packets.Publish,
				},
				TopicName: "direct/publish",
				Payload:   []byte("injected scheduled message"),
			})
			if err != nil {
				server.Log.Error().Err(err).Msg("server.InjectPacket")
				server.Slog.LogAttrs(context.TODO(), slog.LevelError,
					"server.InjectPacket",
					slog.String("error", err.Error()))
			}
			server.Log.Info().Msgf("main.go injected packet to direct/publish")
			server.Slog.LogAttrs(context.TODO(), slog.LevelInfo,
				"main.go injected packet to direct/publish")
		}
	}()

	// There is also a shorthand convenience function, Publish, for easily sending
	// publish packets if you are not concerned with creating your own packets.
	go func() {
		for range time.Tick(time.Second * 5) {
			err := server.Publish("direct/publish", []byte("packet scheduled message"), false, 0)
			if err != nil {
				server.Log.Error().Err(err).Msg("server.Publish")
				server.Slog.LogAttrs(context.TODO(), slog.LevelError,
					"server.Publish",
					slog.String("error", err.Error()))
			}
			server.Log.Info().Msgf("main.go issued direct message to direct/publish")
			server.Slog.LogAttrs(context.TODO(), slog.LevelInfo,
				"main.go issued direct message to direct/publish")
		}
	}()

	<-done
	server.Log.Warn().Msg("caught signal, stopping...")
	server.Slog.LogAttrs(context.TODO(), slog.LevelWarn, "caught signal, stopping...")
	server.Log.Info().Msg("main.go finished")
	server.Slog.LogAttrs(context.TODO(), slog.LevelInfo, "main.go finished")
	server.Close()
}

type ExampleHook struct {
	mqtt.HookBase
}

func (h *ExampleHook) ID() string {
	return "events-example"
}

func (h *ExampleHook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqtt.OnConnect,
		mqtt.OnDisconnect,
		mqtt.OnSubscribed,
		mqtt.OnUnsubscribed,
		mqtt.OnPublished,
		mqtt.OnPublish,
	}, []byte{b})
}

func (h *ExampleHook) Init(config any) error {
	h.Log.Info().Msg("initialised")
	h.Slog.LogAttrs(context.TODO(), slog.LevelInfo,
		"initialised")
	return nil
}

func (h *ExampleHook) OnConnect(cl *mqtt.Client, pk packets.Packet) error {
	h.Log.Info().Str("client", cl.ID).Msgf("client connected")
	h.Slog.LogAttrs(context.TODO(), slog.LevelInfo,
		"initialised")
	return nil
}

func (h *ExampleHook) OnDisconnect(cl *mqtt.Client, err error, expire bool) {
	h.Log.Info().Str("client", cl.ID).Bool("expire", expire).Err(err).Msg("client disconnected")
	if err != nil {
		h.Slog.LogAttrs(context.TODO(), slog.LevelInfo,
			"client disconnected",
			slog.String("client", cl.ID),
			slog.Bool("expire", expire),
			slog.String("error", err.Error()))
	} else {
		h.Slog.LogAttrs(context.TODO(), slog.LevelInfo,
			"client disconnected",
			slog.String("client", cl.ID),
			slog.Bool("expire", expire))
	}

}

func (h *ExampleHook) OnSubscribed(cl *mqtt.Client, pk packets.Packet, reasonCodes []byte) {
	h.Log.Info().Str("client", cl.ID).Interface("filters", pk.Filters).Msgf("subscribed qos=%v", reasonCodes)
	h.Slog.LogAttrs(context.TODO(), slog.LevelInfo,
		fmt.Sprintf("subscribed qos=%v", reasonCodes),
		slog.String("client", cl.ID),
		slog.Any("filters", pk.Filters))
}

func (h *ExampleHook) OnUnsubscribed(cl *mqtt.Client, pk packets.Packet) {
	h.Log.Info().Str("client", cl.ID).Interface("filters", pk.Filters).Msg("unsubscribed")
	h.Slog.LogAttrs(context.TODO(), slog.LevelInfo,
		"unsubscribed",
		slog.String("client", cl.ID),
		slog.Any("filters", pk.Filters))
}

func (h *ExampleHook) OnPublish(cl *mqtt.Client, pk packets.Packet) (packets.Packet, error) {
	h.Log.Info().Str("client", cl.ID).Str("payload", string(pk.Payload)).Msg("received from client")
	h.Slog.LogAttrs(context.TODO(), slog.LevelInfo,
		"received from client",
		slog.String("client", cl.ID),
		slog.String("payload", string(pk.Payload)))

	pkx := pk
	if string(pk.Payload) == "hello" {
		pkx.Payload = []byte("hello world")
		h.Log.Info().Str("client", cl.ID).Str("payload", string(pkx.Payload)).Msg("received modified packet from client")
		h.Slog.LogAttrs(context.TODO(), slog.LevelInfo,
			"received modified packet from client",
			slog.String("client", cl.ID),
			slog.String("payload", string(pkx.Payload)))
	}

	return pkx, nil
}

func (h *ExampleHook) OnPublished(cl *mqtt.Client, pk packets.Packet) {
	h.Log.Info().Str("client", cl.ID).Str("payload", string(pk.Payload)).Msg("published to client")
	h.Slog.LogAttrs(context.TODO(), slog.LevelInfo,
		"published to client",
		slog.String("client", cl.ID),
		slog.String("payload", string(pk.Payload)))
}
