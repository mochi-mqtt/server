// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/mochi-mqtt/server/v2/configs/file"
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
		slog.Default().Error(err.Error())
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

// func configureServerFromFile(path *string) (*mqtt.Server, error) {
// 	config, err := file.Configure(*path)
// 	if err != nil {
// 		return nil, err
// 	}

// 	server := mqtt.New(&config.Server.Options)

// 	if config.Server.Hooks != nil {
// 		if config.Server.Hooks.AllowAll {
// 			_ = server.AddHook(new(auth.AllowHook), nil)
// 		}
// 	}

// 	if config.Server.Listeners.Healthcheck != nil {
// 		port := fmt.Sprintf(":%s", strconv.Itoa(config.Server.Listeners.Healthcheck.Port))

// 		// TODO : Add TLS
// 		hc := listeners.NewHTTPHealthCheck("hc", port, nil)
// 		err := server.AddListener(hc)
// 		if err != nil {
// 			return nil, err
// 		}

// 	}

// 	if config.Server.Listeners.TCP != nil {
// 		port := fmt.Sprintf(":%s", strconv.Itoa(config.Server.Listeners.TCP.Port))

// 		// TODO : Add TLS
// 		hc := listeners.NewTCP("tcp", port, nil)
// 		err := server.AddListener(hc)
// 		if err != nil {
// 			return nil, err
// 		}
// 	}

// 	return server, nil

// }

// func configureServerWithDefault(tcpAddr, wsAddr, infoAddr *string) (*mqtt.Server, error) {
// 	server := mqtt.New(nil)
// 	_ = server.AddHook(new(auth.AllowHook), nil)

// 	tcp := listeners.NewTCP("t1", *tcpAddr, nil)
// 	err := server.AddListener(tcp)
// 	if err != nil {
// 		return nil, err
// 	}

// 	ws := listeners.NewWebsocket("ws1", *wsAddr, nil)
// 	err = server.AddListener(ws)
// 	if err != nil {
// 		return nil, err
// 	}

// 	stats := listeners.NewHTTPStats("stats", *infoAddr, nil, server.Info)
// 	err = server.AddListener(stats)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return server, nil
// }
