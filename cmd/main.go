// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package main

import (
	"flag"
	"fmt"
	"log"
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
	tcpAddr := flag.String("tcp", ":1883", "network address for TCP listener")
	wsAddr := flag.String("ws", ":1882", "network address for Websocket listener")
	infoAddr := flag.String("info", ":8080", "network address for web info dashboard listener")
	filePath := flag.String("file", "", "the relative path to the config file")
	flag.Parse()

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		done <- true
	}()

	server := new(mqtt.Server)
	if *filePath == "" {
		fmt.Println("configured from default")
		s, err := configureServerWithDefault(tcpAddr, wsAddr, infoAddr)
		if err != nil {
			slog.Error(err.Error())
			return
		}
		server = s
	} else {
		fmt.Println("configured from file")
		s, err := configureServerFromFile(filePath)
		if err != nil {
			slog.Error(err.Error())
			return
		}
		server = s
	}

	go func() {
		err := server.Serve()
		if err != nil {
			log.Fatal(err)
		}
	}()

	<-done
	server.Log.Warn().Msg("caught signal, stopping...")
	server.Close()
	server.Log.Info().Msg("main.go finished")
}

func configureServerFromFile(path *string) (*mqtt.Server, error) {
	config, err := file.Configure(*path)
	if err != nil {
		return nil, err
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
		err := server.AddListener(hc)
		if err != nil {
			return nil, err
		}

	}

	if config.Server.Listeners.TCP != nil {
		port := fmt.Sprintf(":%s", strconv.Itoa(config.Server.Listeners.TCP.Port))

		// TODO : Add TLS
		hc := listeners.NewTCP("tcp", port, nil)
		err := server.AddListener(hc)
		if err != nil {
			return nil, err
		}
	}

	return server, nil

}

func configureServerWithDefault(tcpAddr, wsAddr, infoAddr *string) (*mqtt.Server, error) {
	server := mqtt.New(nil)
	_ = server.AddHook(new(auth.AllowHook), nil)

	tcp := listeners.NewTCP("t1", *tcpAddr, nil)
	err := server.AddListener(tcp)
	if err != nil {
		return nil, err
	}

	ws := listeners.NewWebsocket("ws1", *wsAddr, nil)
	err = server.AddListener(ws)
	if err != nil {
		return nil, err
	}

	stats := listeners.NewHTTPStats("stats", *infoAddr, nil, server.Info)
	err = server.AddListener(stats)
	if err != nil {
		return nil, err
	}

	return server, nil
}
