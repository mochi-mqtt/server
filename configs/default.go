package configs

import (
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
)

func ConfigureServerWithDefault() (*mqtt.Server, error) {

	server := mqtt.New(nil)
	_ = server.AddHook(new(auth.AllowHook), nil)

	tcp := listeners.NewTCP("t1", ":1883", nil)
	err := server.AddListener(tcp)
	if err != nil {
		return nil, err
	}

	ws := listeners.NewWebsocket("ws1", ":1882", nil)
	err = server.AddListener(ws)
	if err != nil {
		return nil, err
	}

	stats := listeners.NewHTTPStats("stats", ":8080", nil, server.Info)
	err = server.AddListener(stats)
	if err != nil {
		return nil, err
	}

	return server, nil
}
