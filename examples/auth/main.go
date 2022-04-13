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
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		done <- true
	}()

	fmt.Println(aurora.Magenta("Mochi MQTT Server initializing..."), aurora.Cyan("TCP"))

	server := mqtt.NewServer(nil)
	tcp := listeners.NewTCP("t1", ":1883")
	err := server.AddListener(tcp, &listeners.Config{
		Auth: &Auth{
			Users: map[string]string{
				"peach": "password1",
				"melon": "password2",
				"apple": "password3",
			},
			AllowedTopics: map[string][]string{
				// Melon user only has access to melon topics.
				// If you were implementing this in the real world, you might ensure
				// that any topic prefixed with "melon" is allowed (see ACL func below).
				"melon": {"melon/info", "melon/events"},
			},
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		err := server.Serve()
		if err != nil {
			log.Fatal(err)
		}
	}()
	fmt.Println(aurora.BgMagenta("  Started!  "))

	<-done
	fmt.Println(aurora.BgRed("  Caught Signal  "))

	server.Close()
	fmt.Println(aurora.BgGreen("  Finished  "))
}

// Auth is an example auth provider for the server. In the real world
// you are more likely to replace these fields with database/cache lookups
// to check against an auth list. As the Auth Controller is an interface, it can
// be built however you want, as long as it fulfils the interface signature.
type Auth struct {
	Users         map[string]string   // A map of usernames (key) with passwords (value).
	AllowedTopics map[string][]string // A map of usernames and topics
}

// Authenticate returns true if a username and password are acceptable.
// Auth always returns true.
func (a *Auth) Authenticate(user, password []byte) bool {
	// If the user exists in the auth users map, and the password is correct,
	// then they can connect to the server.
	if pass, ok := a.Users[string(user)]; ok && pass == string(password) {
		return true
	}

	return false
}

// ACL returns true if a user has access permissions to read or write on a topic.
func (a *Auth) ACL(user []byte, topic string, write bool) bool {

	// An example ACL - if the user has an entry in the auth allow list, then they are
	// subject to ACL restrictions. Only let them use a topic if it's available for their
	// user.
	if topics, ok := a.AllowedTopics[string(user)]; ok {
		for _, t := range topics {

			// In the real world you might allow all topics prefixed with a user's username,
			// or similar multi-topic filters.
			if t == topic {
				return true
			}
		}

		return false
	}

	// Otherwise, allow all topics.
	return true
}
