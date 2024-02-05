// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: ishan-daga

package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func main() {
	shut := initTracer()
	defer shut(context.Background())

	tcpAddr := flag.String("tcp", ":1883", "network address for TCP listener")
	flag.Parse()

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		done <- true
	}()

	server := mqtt.New(&mqtt.Options{
		OTELTracing:    true,
		OTELTracerName: "mochi-mqtt",
	})
	server.Options.Capabilities.MaximumClientWritesPending = 16 * 1024
	_ = server.AddHook(new(auth.AllowHook), nil)
	_ = server.AddHook(new(ExampleHook), nil)

	tcp := listeners.NewTCP("t1", *tcpAddr, nil)
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
	h.Log.Info("initialised")
	return nil
}

func (h *ExampleHook) OnConnect(cl *mqtt.Client, pk packets.Packet) error {
	h.Log.InfoContext(pk.Ctx, "client connected", "client", cl.ID)
	return nil
}

func (h *ExampleHook) OnDisconnect(cl *mqtt.Client, err error, expire bool) {
	if err != nil {
		h.Log.Info("client disconnected", "client", cl.ID, "expire", expire, "error", err)
	} else {
		h.Log.Info("client disconnected", "client", cl.ID, "expire", expire)
	}

}

func (h *ExampleHook) OnSubscribed(cl *mqtt.Client, pk packets.Packet, reasonCodes []byte) {
	h.Log.InfoContext(pk.Ctx, fmt.Sprintf("subscribed qos=%v", reasonCodes), "client", cl.ID, "filters", pk.Filters)
}

func (h *ExampleHook) OnUnsubscribed(cl *mqtt.Client, pk packets.Packet) {
	h.Log.InfoContext(pk.Ctx, "unsubscribed", "client", cl.ID, "filters", pk.Filters)
}

func (h *ExampleHook) OnPublish(cl *mqtt.Client, pk packets.Packet) (packets.Packet, error) {
	h.Log.InfoContext(pk.Ctx, "received from client", "client", cl.ID, "payload", string(pk.Payload))

	pkx := pk
	if string(pk.Payload) == "hello" {
		pkx.Payload = []byte("hello world")
		h.Log.InfoContext(pk.Ctx, "received modified packet from client", "client", cl.ID, "payload", string(pkx.Payload))
	}

	if pk.Ctx == nil {
		h.Log.Warn("nil context")
		return pkx, nil
	}

	span := trace.SpanFromContext(pk.Ctx)
	span.AddEvent("published", trace.WithAttributes(
		attribute.String("client", cl.ID),
		attribute.String("payload", string(pk.Payload)),
	))

	h.Log.DebugContext(pk.Ctx, "published to client", "traceid", span.SpanContext().TraceID().String())

	return pkx, nil
}

func (h *ExampleHook) OnPublished(cl *mqtt.Client, pk packets.Packet) {
	h.Log.InfoContext(pk.Ctx, "published to client", "client", cl.ID, "payload", string(pk.Payload))
}
