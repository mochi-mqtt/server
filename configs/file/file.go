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
	CONFIG_FILE_NAME = "mochi_config.yml"
)

// Note: struct fields must be public in order for unmarshal to
// correctly populate the data.
type Config struct {
	Server struct {
		Hooks *struct {
			AllowAll bool `yaml:"allow_all"`
		}
		Listeners struct {
			Healthcheck *struct {
				Port int `yaml:"port"`
			} `yaml:"healthcheck"`
			Stats *struct {
				Port int       `yaml:"port"`
				TLS  *struct { // TODO : Add TLS configuration
				} `yaml:"tls"`
			} `yaml:"stats"`
			TCP *struct {
				Port int       `yaml:"port"`
				TLS  *struct { // TODO : Add TLS configuration
				} `yaml:"tls"`
			} `yaml:"tcp"`
			Websocket *struct {
				Port int       `yaml:"port"`
				TLS  *struct { // TODO : Add TLS configuration
				} `yaml:"tls"`
			} `yaml:"websocket"`
		} `yaml:"listeners"`
		Logging struct {
			Level string `yaml:"level"`
		}
		// Options contains configurable options for the server.
		mqtt.Options `yaml:"options"`
	} `yaml:"server"`
}

// Configure attempts to open the configuration file defined by CONFIG_FILE_NAME.
// If no file is found, a default mqtt.Server instance is created.
func Configure() (*mqtt.Server, error) {

	data, err := os.ReadFile(CONFIG_FILE_NAME)
	if err != nil {
		return nil, err
	}

	config := new(Config)
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, err
	}

	server := mqtt.New(&config.Server.Options)

	// hooks configuration
	if config.Server.Hooks != nil {
		if config.Server.Hooks.AllowAll {
			_ = server.AddHook(new(auth.AllowHook), nil)
		}
	}

	// listeners configuration
	if config.Server.Listeners.Healthcheck != nil {
		port := fmt.Sprintf(":%s", strconv.Itoa(config.Server.Listeners.Healthcheck.Port))

		hc := listeners.NewHTTPHealthCheck("hc", port, nil)
		err = server.AddListener(hc)
		if err != nil {
			slog.Default().Error(err.Error())
			return nil, err
		}
	}

	if config.Server.Listeners.Stats != nil {
		port := fmt.Sprintf(":%s", strconv.Itoa(config.Server.Listeners.Stats.Port))

		statl := listeners.NewHTTPStats("stat", port, nil, server.Info)
		err = server.AddListener(statl)
		if err != nil {
			slog.Default().Error(err.Error())
			return nil, err
		}

	}

	if config.Server.Listeners.TCP != nil {
		port := fmt.Sprintf(":%s", strconv.Itoa(config.Server.Listeners.TCP.Port))

		tcpl := listeners.NewTCP("tcp", port, nil)
		err = server.AddListener(tcpl)
		if err != nil {
			slog.Default().Error(err.Error())
			return nil, err
		}
	}

	if config.Server.Listeners.Websocket != nil {
		port := fmt.Sprintf(":%s", strconv.Itoa(config.Server.Listeners.Websocket.Port))

		wsl := listeners.NewWebsocket("ws", port, nil)
		err = server.AddListener(wsl)
		if err != nil {
			slog.Default().Error(err.Error())
			return nil, err
		}
	}

	return server, nil
}
