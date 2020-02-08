package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/logrusorgru/aurora"

	mqtt "github.com/mochi-co/mqtt/server"
	"github.com/mochi-co/mqtt/server/listeners"
)

func main() {
	var err error

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		done <- true
	}()

	fmt.Println(aurora.Magenta("Mochi MQTT Broker initializing..."))

	server := mqtt.New()
	tcp := listeners.NewTCP("t1", ":1883")
	err = server.AddListener(tcp, nil)
	if err != nil {
		log.Fatal(err)
	}

	ws := listeners.NewWebsocket("ws1", ":1882")
	err = server.AddListener(ws, nil)
	if err != nil {
		log.Fatal(err)
	}

	stats := listeners.NewHTTPStats("stats", ":8080")
	err = server.AddListener(stats, nil)
	if err != nil {
		log.Fatal(err)
	}

	go server.Serve()
	fmt.Println(aurora.BgMagenta("  Started!  "))

	<-done
	fmt.Println(aurora.BgRed("  Caught Signal  "))

	server.Close()
	fmt.Println(aurora.BgGreen("  Finished  "))

}
