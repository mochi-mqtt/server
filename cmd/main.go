package main

import (
	"flag"
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
	tcpAddr := flag.String("tcp", ":1883", "network address for TCP listener")
	wsAddr := flag.String("ws", ":1882", "network address for Websocket listener")
	infoAddr := flag.String("info", ":8080", "network address for web info dashboard listener")
	flag.Parse()

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		done <- true
	}()

	fmt.Println(aurora.Magenta("Mochi MQTT Broker initializing..."))
	fmt.Println(aurora.Cyan("TCP"), *tcpAddr)
	fmt.Println(aurora.Cyan("Websocket"), *wsAddr)
	fmt.Println(aurora.Cyan("$SYS Dashboard"), *infoAddr)

	server := mqtt.New()
	tcp := listeners.NewTCP("t1", *tcpAddr)
	err := server.AddListener(tcp, nil)
	if err != nil {
		log.Fatal(err)
	}

	ws := listeners.NewWebsocket("ws1", *wsAddr)
	err = server.AddListener(ws, nil)
	if err != nil {
		log.Fatal(err)
	}

	stats := listeners.NewHTTPStats("stats", *infoAddr)
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
