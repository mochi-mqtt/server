// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

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
		if os.IsNotExist(err) {
			slog.Default().Info("mochi_config.yml not found")
			slog.Default().Info("defaulting to standard broker configuration")
			server, _ = configureServerWithDefault()
		}

		return
	}

	go func() {
		err := server.Serve()
		if err != nil {
			slog.Default().Error(err.Error())
			return
		}
	}()

	<-done
	server.Log.Warn().Msg("caught signal, stopping...")
	server.Close()
	server.Log.Info().Msg("main.go finished")
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
