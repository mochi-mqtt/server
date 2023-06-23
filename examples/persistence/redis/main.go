// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-co
// SPDX-FileContributor: mochi-co

package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/mochi-co/mqtt/v2"
	"github.com/mochi-co/mqtt/v2/hooks/auth"
	"github.com/mochi-co/mqtt/v2/hooks/storage/redis"
	"github.com/mochi-co/mqtt/v2/listeners"
	"github.com/rs/zerolog"
	"golang.org/x/exp/slog"

	rv8 "github.com/go-redis/redis/v8"
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
	l := server.Log.Level(zerolog.DebugLevel)
	server.Log = &l

	level := new(slog.LevelVar)
	server.Slog = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))
	level.Set(slog.LevelDebug)

	err := server.AddHook(new(redis.Hook), &redis.Options{
		Options: &rv8.Options{
			Addr:     "localhost:6379", // default redis address
			Password: "",               // your password
			DB:       0,                // your redis db
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	tcp := listeners.NewTCP("t1", ":1883", nil)
	err = server.AddListener(tcp)
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
	server.Log.Warn().Msg("caught signal, stopping...")
	server.Slog.LogAttrs(context.TODO(), slog.LevelWarn, "caught signal, stopping...")
	server.Log.Info().Msg("main.go finished")
	server.Slog.LogAttrs(context.TODO(), slog.LevelInfo, "main.go finished")
	server.Close()
}
