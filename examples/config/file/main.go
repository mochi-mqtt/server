// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: dgduncan

package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
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

	config, err := file.Configure("examples/config/file/mochi_config.yml")
	if err != nil {
		slog.Default().Error(err.Error())
		return
	}

	server := mqtt.New(&config.Server.Options)

	if config.Server.Hooks != nil {
		if config.Server.Hooks.AllowAll {
			_ = server.AddHook(new(auth.AllowHook), nil)
		}
	}

	if config.Server.Listeners.Healthcheck != nil {
		port := fmt.Sprintf(":%s", strconv.Itoa(config.Server.Listeners.Healthcheck.Port))

		// TODO : Add TLS
		hc := listeners.NewHTTPHealthCheck("hc", port, nil)
		err = server.AddListener(hc)
		if err != nil {
			slog.Default().Error(err.Error())
			return
		}

	}

	if config.Server.Listeners.TCP != nil {
		port := fmt.Sprintf(":%s", strconv.Itoa(config.Server.Listeners.TCP.Port))

		// TODO : Add TLS
		hc := listeners.NewTCP("tcp", port, nil)
		err = server.AddListener(hc)
		if err != nil {
			slog.Default().Error(err.Error())
			return
		}

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
