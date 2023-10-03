// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 mochi-mqtt
// SPDX-FileContributor: dduncan

package file

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
	"gopkg.in/yaml.v3"
)

const (
	// CONFIG_FILE_NAME the name of the configuration file used for file based configuration
	DefaultFileName = "mochi_config.yml"

	LoggingOutputJson = "JSON"
	LoggingOutputText = "TEXT"
)

// Note: struct fields must be public in order for unmarshal to
// correctly populate the data.
type Config struct {
	Server struct {
		Hooks *struct {
			AllowAll bool `yaml:"allow_all"`
		}
		Listeners struct {
			Stats     *Stats     `yaml:"stats"`
			TCP       *TCP       `yaml:"tcp"`
			Websocket *Websocket `yaml:"websocket"`
		} `yaml:"listeners"`
		Logging      *Logging         `yaml:"logging"`
		mqtt.Options `yaml:"options"` // Options contains configurable options for the server.

	} `yaml:"server"`
}

type Stats struct {
	Port int `yaml:"port"`
}

type TCP struct {
	Port int `yaml:"port"`
}
type Websocket struct {
	Port int `yaml:"port"`
}

type Logging struct {
	Level  string `yaml:"level"`
	Output string `yaml:"output"`
}

// Configure attempts to open the configuration file defined by CONFIG_FILE_NAME.
// If no file is found, a default mqtt.Server instance is created.
func Configure(filepath string) (*mqtt.Server, error) {
	if filepath == "" {
		filepath = DefaultFileName
	}

	data, err := os.ReadFile(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Warn("config file not found", "filepath", filepath)
		}
		return nil, err
	}

	config := new(Config)
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, err
	}

	server := mqtt.New(&config.Server.Options)

	if config.Server.Hooks != nil {
		if config.Server.Hooks.AllowAll {
			_ = server.AddHook(new(auth.AllowHook), nil)
		}
	}

	configureLogging(config.Server.Logging, server)

	if err := configureStats(config.Server.Listeners.Stats, server); err != nil {
		return nil, err
	}

	if err := configureTCP(config.Server.Listeners.TCP, server); err != nil {
		return nil, err
	}

	if err := configureWebsocket(config.Server.Listeners.Websocket, server); err != nil {
		return nil, err
	}

	return server, nil
}

func configureStats(config *Stats, server *mqtt.Server) error {
	if config == nil {
		return nil
	}

	port := formatPort(config.Port)
	lc := new(listeners.Config)

	statl := listeners.NewHTTPStats("stat", port, lc, server.Info)
	return server.AddListener(statl)
}

func configureTCP(config *TCP, server *mqtt.Server) error {
	if config == nil {
		return nil
	}

	port := formatPort(config.Port)
	lc := new(listeners.Config)

	tcpl := listeners.NewTCP("tcp", port, lc)
	return server.AddListener(tcpl)
}

func configureWebsocket(config *Websocket, server *mqtt.Server) error {
	if config == nil {
		return nil
	}

	port := formatPort(config.Port)
	lc := new(listeners.Config)

	wsl := listeners.NewWebsocket("ws", port, lc)
	return server.AddListener(wsl)
}

func configureLogging(config *Logging, server *mqtt.Server) { //nolint:unparam
	if config == nil {
		return
	}

	var level slog.Level
	if err := level.UnmarshalText([]byte(config.Level)); err != nil {
		slog.Warn(err.Error())
		slog.Warn(fmt.Sprintf("logging level not recognized, defaulting to level %s", slog.LevelInfo.String()))
		level = slog.LevelInfo
	}

	var handler slog.Handler
	switch config.Output {
	case LoggingOutputJson:
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	case LoggingOutputText:
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	default:
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	}

	server.Log = slog.New(handler)
}

func formatPort(port int) string {
	return fmt.Sprintf(":%s", strconv.Itoa(port))
}
