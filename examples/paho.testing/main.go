// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package main

import (
	"bytes"
	"log"
	"os"
	"os/signal"
	"syscall"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/listeners"
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
	server.Options.Capabilities.Compatibilities.ObscureNotAuthorized = true
	server.Options.Capabilities.Compatibilities.PassiveClientDisconnect = true
	server.Options.Capabilities.Compatibilities.NoInheritedPropertiesOnAck = true

	_ = server.AddHook(new(pahoAuthHook), nil)
	tcp := listeners.NewTCP("t1", ":1883", nil)
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

type pahoAuthHook struct {
	mqtt.HookBase
}

func (h *pahoAuthHook) ID() string {
	return "allow-all-auth"
}

func (h *pahoAuthHook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqtt.OnConnectAuthenticate,
		mqtt.OnConnect,
		mqtt.OnACLCheck,
	}, []byte{b})
}

func (h *pahoAuthHook) OnConnectAuthenticate(cl *mqtt.Client, pk packets.Packet) bool {
	return true
}

func (h *pahoAuthHook) OnACLCheck(cl *mqtt.Client, topic string, write bool) bool {
	return topic != "test/nosubscribe"
}

func (h *pahoAuthHook) OnConnect(cl *mqtt.Client, pk packets.Packet) error {
	// Handle paho test_server_keep_alive
	if pk.Connect.Keepalive == 120 && pk.Connect.Clean {
		cl.State.Keepalive = 60
		cl.State.ServerKeepalive = true
	}
	return nil
}
