package file

import (
	"log/slog"
	"os"

	mqtt "github.com/mochi-mqtt/server/v2"
	"gopkg.in/yaml.v3"
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
			TCP *struct {
				Port int       `yaml:"port"`
				TLS  *struct { // TODO : Add TLS configuration
				} `yaml:"tls"`
			} `yaml:"tcp"`
		} `yaml:"listeners"`
		// Options contains configurable options for the server.
		mqtt.Options `yaml:"options"`
	} `yaml:"server"`
}

func Configure(p string) (*Config, error) {
	if p == "" {
		slog.Default().Debug("no file path provided")
		return nil, nil
	}

	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}

	config := new(Config)
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, err
	}

	// if !validate(config) {

	// }

	return config, nil
}

// func validate(config *Config) bool {
// 	return true
// }

// func Configure(p string) (*mqtt.Options, error) {
// 	if p == "" {
// 		slog.Default().Debug("no file path provided")
// 		return nil, nil
// 	}

// 	data, err := os.ReadFile(p)
// 	if err != nil {
// 		return nil, err
// 	}

// 	config := new(Config)
// 	if err := yaml.Unmarshal(data, config); err != nil {
// 		return nil, err
// 	}

// 	fmt.Println(config.Server.Listeners)
// 	fmt.Println(config.Server.Listeners.TCP.TLS == nil)

// 	return &config.Server.Options, nil
// }
