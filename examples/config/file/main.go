// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: dgduncan

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
		slog.Error(err.Error())
		return
	}

	go func() {
		err := server.Serve()
		if err != nil {
			slog.Error(err.Error())
			return
		}
	}()

	<-done
	server.Log.Warn("caught signal, stopping...")
	server.Close()
	server.Log.Info("main.go finished")
}
