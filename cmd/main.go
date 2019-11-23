package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/logrusorgru/aurora"

	"github.com/mochi-co/mqtt"
	"github.com/mochi-co/mqtt/internal/listeners"
)

func main() {

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
	log.Println(tcp)
	err := server.AddListener(tcp, nil)
	if err != nil {
		panic(err)
	}

	// Start broker...
	go server.Serve()
	fmt.Println(aurora.BgMagenta("  Started!  "))

	// Wait for signals...
	<-done
	fmt.Println(aurora.BgRed("  Caught Signal  "))

	// End gracefully.
	server.Close()
	fmt.Println(aurora.BgGreen("  Finished  "))

}
