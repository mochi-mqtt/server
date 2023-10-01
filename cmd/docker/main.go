// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 mochi-mqtt
// SPDX-FileContributor: dduncan

package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/configs/file"
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

	server, err := file.Configure()
	if err != nil {
		slog.Error("failed to use file configuration", "error", err)
		slog.Warn("defaulting to standard broker configuration")
		server, _ = configureServerWithDefault()
	}

	go func() {
		err := server.Serve()
		if err != nil {
			slog.Error("", "error", err)
			return
		}
	}()

	<-done
	server.Log.Warn("caught signal, stopping...")
	server.Close()
	server.Log.Info("main.go finished")
}

func configureServerWithDefault() (*mqtt.Server, error) {
	server := mqtt.New(nil)
	_ = server.AddHook(new(auth.AllowHook), nil)

	tcp := listeners.NewTCP("t1", ":1883", nil)
	err := server.AddListener(tcp)
	if err != nil {
		return nil, err
	}

	ws := listeners.NewWebsocket("ws1", ":1882", nil)
	err = server.AddListener(ws)
	if err != nil {
		return nil, err
	}

	stats := listeners.NewHTTPStats("stats", ":8080", nil, server.Info)
	err = server.AddListener(stats)
	if err != nil {
		return nil, err
	}

	return server, nil
}
