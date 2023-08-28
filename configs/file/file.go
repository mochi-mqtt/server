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
				Port int `yaml:"port"`
			} `yaml:"stats"`
			TCP *struct {
				Port int       `yaml:"port"`
				TLS  *struct { // TODO : Add TLS configuration
				} `yaml:"tls"`
			} `yaml:"tcp"`
		} `yaml:"listeners"`
		Logging struct {
			Level string `yaml:"level"`
		}
		// Options contains configurable options for the server.
		mqtt.Options `yaml:"options"`
	} `yaml:"server"`
}

func Configure() (*mqtt.Server, error) {

	data, err := os.ReadFile(CONFIG_FILE_NAME)
	if err != nil {
		slog.Default().Error(CONFIG_FILE_NAME + " not found!")
		return nil, err
	}

	config := new(Config)
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, err
	}

	// TODO : add validate
	// if !validate(config) {

	// }

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

		// TODO : Add TLS
		hc := listeners.NewHTTPHealthCheck("hc", port, nil)
		err = server.AddListener(hc)
		if err != nil {
			slog.Default().Error(err.Error())
			return nil, err
		}

	}

	if config.Server.Listeners.TCP != nil {
		port := fmt.Sprintf(":%s", strconv.Itoa(config.Server.Listeners.TCP.Port))

		// TODO : Add TLS
		hc := listeners.NewTCP("tcp", port, nil)
		err = server.AddListener(hc)
		if err != nil {
			slog.Default().Error(err.Error())
			return nil, err
		}

	}

	return server, nil
}

// func validate(config *Config) bool {
// 	return true
// }
