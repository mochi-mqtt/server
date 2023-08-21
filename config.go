package mqtt

import (
	"log/slog"
	"os"

	"gopkg.in/yaml.v3"
)

// Note: struct fields must be public in order for unmarshal to
// correctly populate the data.
type Config struct {
	Server struct {
		// Options contains configurable options for the server.
		Options `yaml:"options"`
	} `yaml:"server"`
}

func OpenConfigFile(p string) (*Options, error) {
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

	return &config.Server.Options, nil
}
