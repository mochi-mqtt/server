// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	mqtt "github.com/mochi-mqtt/server/v2"
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

	authRules := &auth.Ledger{
		Auth: auth.AuthRules{ // Auth disallows all by default
			{Username: "peach", Password: "password1", Allow: true},
			{Username: "melon", Password: "password2", Allow: true},
			{Remote: "127.0.0.1:*", Allow: true},
			{Remote: "localhost:*", Allow: true},
		},
		ACL: auth.ACLRules{ // ACL allows all by default
			{Remote: "127.0.0.1:*"}, // local superuser allow all
			{
				// user melon can read and write to their own topic
				Username: "melon", Filters: auth.Filters{
					"melon/#":   auth.ReadWrite,
					"updates/#": auth.WriteOnly, // can write to updates, but can't read updates from others
				},
			},
			{
				// Otherwise, no clients have publishing permissions
				Filters: auth.Filters{
					"#":         auth.ReadOnly,
					"updates/#": auth.Deny,
				},
			},
		},
	}

	// you may also find this useful...
	// d, _ := authRules.ToYAML()
	// d, _ := authRules.ToJSON()
	// fmt.Println(string(d))

	server := mqtt.New(nil)
	err := server.AddHook(new(auth.Hook), &auth.Options{
		Ledger: authRules,
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
	server.Log.Warn("caught signal, stopping...")
	_ = server.Close()
	server.Log.Info("main.go finished")
}
