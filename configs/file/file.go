// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 mochi-mqtt
// SPDX-FileContributor: dduncan

package file

import (
	"crypto/tls"
	"crypto/x509"
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
			HealthCheck *HealthCheck `yaml:"healthcheck"`
			Stats       *Stats       `yaml:"stats"`
			TCP         *TCP         `yaml:"tcp"`
			Websocket   *Websocket   `yaml:"websocket"`
			TLS         *TLS         `yaml:"tls"`
		} `yaml:"listeners"`
		Logging *Logging `yaml:"logging"`
		// Options contains configurable options for the server.
		mqtt.Options `yaml:"options"`
	} `yaml:"server"`
}

type TLS struct {
	Cert   string `yaml:"cert_file"`
	Key    string `yaml:"priv_key"`
	CACert string `yaml:"cacert_file"`
}

type HealthCheck struct {
	Port       int  `yaml:"port"`
	TLSEnabled bool `yaml:"tls_enabled"`
}

type Stats struct {
	Port       int  `yaml:"port"`
	TLSEnabled bool `yaml:"tls_enabled"`
}

type TCP struct {
	Port       int  `yaml:"port"`
	TLSEnabled bool `yaml:"tls_enabled"`
}
type Websocket struct {
	Port       int  `yaml:"port"`
	TLSEnabled bool `yaml:"tls_enabled"`
}

type Logging struct {
	Level string `yaml:"level"`
}

// Configure attempts to open the configuration file defined by CONFIG_FILE_NAME.
// If no file is found, a default mqtt.Server instance is created.
func Configure() (*mqtt.Server, error) {

	data, err := os.ReadFile(CONFIG_FILE_NAME)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Default().Info("mochi_config.yml not found")
		}
		return nil, err
	}

	config := new(Config)
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, err
	}

	// TODO : Wait for slog change to make level var for level config
	server := mqtt.New(&config.Server.Options)

	// hooks configuration
	if config.Server.Hooks != nil {
		if config.Server.Hooks.AllowAll {
			_ = server.AddHook(new(auth.AllowHook), nil)
		}
	}

	// if err := configureLogging(config.Server.Logging, server); err != nil {
	// 	return nil, err
	// }

	// listeners configuration
	tlsc, err := configureTLS(config.Server.Listeners.TLS)
	if err != nil {
		return nil, err
	}

	if err := configureHealthCheck(config.Server.Listeners.HealthCheck, server, tlsc); err != nil {
		return nil, err
	}

	if err := configureStats(config.Server.Listeners.Stats, server, tlsc); err != nil {
		return nil, err
	}

	if err := configureTCP(config.Server.Listeners.TCP, server, tlsc); err != nil {
		return nil, err
	}

	if err := configureWebsocket(config.Server.Listeners.Websocket, server, tlsc); err != nil {
		return nil, err
	}

	return server, nil
}

func configureTLS(config *TLS) (*tls.Config, error) {
	if config == nil {
		return nil, nil
	}

	tlsc := new(tls.Config)
	if config.Cert != "" && config.Key != "" { // Skip if cert and key are missing
		slog.Info("Certificate and Key information found in config", "step", "config")
		cert, err := tls.LoadX509KeyPair(config.Cert, config.Key)
		if err != nil {
			return nil, err
		}
		tlsc.Certificates = []tls.Certificate{cert}
	}

	if config.CACert != "" { // Skip if CA certificate is missing
		caCert, err := os.ReadFile(config.CACert)
		if err != nil {
			return nil, err
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		tlsc.ClientCAs = caCertPool
	}

	return tlsc, nil
}

func configureHealthCheck(config *HealthCheck, server *mqtt.Server, tlsc *tls.Config) error {
	if config == nil {
		return nil
	}

	port := formatPort(config.Port)
	lc := new(listeners.Config)

	if config.TLSEnabled {
		lc.TLSConfig = tlsc
	}

	hc := listeners.NewHTTPHealthCheck("hc", port, lc)
	err := server.AddListener(hc)
	if err != nil {
		slog.Error(err.Error())
		return err
	}

	return nil
}

func configureStats(config *Stats, server *mqtt.Server, tlsc *tls.Config) error {
	if config == nil {
		return nil
	}

	port := formatPort(config.Port)
	lc := new(listeners.Config)

	if config.TLSEnabled {
		lc.TLSConfig = tlsc
	}

	statl := listeners.NewHTTPStats("stat", port, lc, server.Info)
	return server.AddListener(statl)
}

func configureTCP(config *TCP, server *mqtt.Server, tlsc *tls.Config) error {
	if config == nil {
		return nil
	}

	port := formatPort(config.Port)
	lc := new(listeners.Config)

	if config.TLSEnabled {
		lc.TLSConfig = tlsc
	}

	tcpl := listeners.NewTCP("tcp", port, lc)
	return server.AddListener(tcpl)
}

func configureWebsocket(config *Websocket, server *mqtt.Server, tlsc *tls.Config) error {
	if config == nil {
		return nil
	}

	port := formatPort(config.Port)
	lc := new(listeners.Config)

	if config.TLSEnabled {
		lc.TLSConfig = tlsc
	}

	wsl := listeners.NewWebsocket("ws", port, lc)
	return server.AddListener(wsl)
}

// TODO : wait for slog change
// func configureLogging(config *Logging, server *mqtt.Server) error {
// 	if config == nil {
// 		return nil
// 	}

// 	var level slog.Level
// 	switch config.Level {
// 	case slog.LevelDebug.String():
// 		level = slog.LevelDebug
// 	case slog.LevelInfo.String():
// 		level = slog.LevelInfo
// 	case slog.LevelWarn.String():
// 		level = slog.LevelWarn
// 	case slog.LevelError.String():
// 		level = slog.LevelError
// 	default:
// 		slog.Warn(fmt.Sprintf("logging level not recognized, defaulting to level %s", slog.LevelInfo.String()))
// 		level = slog.LevelInfo
// 	}

// 	return nil
// }

func formatPort(port int) string {
	return fmt.Sprintf(":%s", strconv.Itoa(port))
}
