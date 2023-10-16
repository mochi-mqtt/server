package file

import (
	"errors"
	"fmt"
	"os"
	"testing"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/stretchr/testify/assert"
)

var goodConfig = []byte(`
server:
  hooks:
    allow_all: true
  listeners:
    healthcheck:
      - id: hc
        port: 8081
    stats:
      - id: stats1
        port: 8080
    tcp:
      - id: tcp1
        port: 1883
    websocket:
      - id: ws1
        port: 1885
  logging:
    level: INFO
    output: TEXT
  options:
    capabilities:
      maximum_message_expiry_interval: 100
      compatibilities:
        obscure_not_authorized: true
    
`,
)

var goodConfigNoOptions = []byte(`
server:
  hooks:
    allow_all: true
  listeners:
    healthcheck:
      - id: hc
        port: 8081
    stats:
      - id: stats1
        port: 8080
    tcp:
      - id: tcp1
        port: 1883
    websocket:
      - id: ws1
        port: 1885
  logging:
    level: INFO
    output: TEXT    
`,
)

var invalidConfig = []byte(`
server:
  hooks:
    allow_all: true
  		listeners:
  logging:
    level: INFO
    output: TEXT
  options:
    capabilities:
      maximum_message_expiry_interval: 100
      compatibilities:
        obscure_not_authorized: true
    
`,
)

func newServer() *mqtt.Server {
	cc := *mqtt.DefaultServerCapabilities
	cc.MaximumMessageExpiryInterval = 0
	cc.ReceiveMaximum = 0
	s := mqtt.New(&mqtt.Options{
		Capabilities: &cc,
	})
	return s
}

func TestConfigure(t *testing.T) {
	tests := []struct {
		name     string
		readfile func(name string) ([]byte, error)
		wantErr  bool
	}{
		{
			name: "sucessfull configuration",
			readfile: func(name string) ([]byte, error) {
				return goodConfig, nil
			},
			wantErr: false,
		},
		{
			name: "sucessfull configuration - check server config",
			readfile: func(name string) ([]byte, error) {
				return goodConfig, nil
			},
			wantErr: false,
		},
		{
			name: "invalid yaml",
			readfile: func(name string) ([]byte, error) {
				return invalidConfig, nil
			},
			wantErr: true,
		},
		{
			name: "no file found",
			readfile: func(name string) ([]byte, error) {
				return nil, os.ErrNotExist
			},
			wantErr: true,
		},
		{
			name: "unknown error on readfile",
			readfile: func(name string) ([]byte, error) {
				return nil, errors.New("unknown error")
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			readFile = tt.readfile
			_, err := Configure("")

			if tt.wantErr {
				assert.Error(t, err)
			}
		})
	}
}

func TestConfigureServer(t *testing.T) {
	tests := []struct {
		name     string
		readfile func(name string) ([]byte, error)
		want     *mqtt.Server
		wantErr  bool
	}{
		{
			name: "sucessfull configuration",
			readfile: func(name string) ([]byte, error) {
				return goodConfig, nil
			},
			want:    &mqtt.Server{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			readFile = tt.readfile
			_, err := Configure("")

			if tt.wantErr {
				assert.Error(t, err)
			}
		})
	}
}

func TestConfigureStats(t *testing.T) {
	tests := []struct {
		name    string
		config  []*Stats
		want    *mqtt.Server
		wantErr bool
	}{
		{
			name: "sucessfull configuration",
			config: []*Stats{
				{
					ID:   "stats1",
					Port: 8080,
				},
			},
			want:    &mqtt.Server{},
			wantErr: false,
		},
		{
			name: "invalid configuration",
			config: []*Stats{
				{
					ID:   "stats1",
					Port: 8080,
				},
				{
					ID:   "stats1",
					Port: 8080,
				},
			},
			want:    &mqtt.Server{},
			wantErr: false,
		},
		{
			name:    "invalid configuration",
			config:  nil,
			want:    &mqtt.Server{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := configureStats(tt.config, newServer())
			if err != nil {
				fmt.Println(err)
			}
		})
	}
}

func TestConfigureTCP(t *testing.T) {
	tests := []struct {
		name    string
		config  []*TCP
		want    *mqtt.Server
		wantErr bool
	}{
		{
			name: "sucessfull configuration",
			config: []*TCP{
				{
					ID:   "stats1",
					Port: 8080,
				},
			},
			want:    &mqtt.Server{},
			wantErr: false,
		},
		{
			name: "invalid configuration",
			config: []*TCP{
				{
					ID:   "stats1",
					Port: 8080,
				},
				{
					ID:   "stats1",
					Port: 8080,
				},
			},
			want:    &mqtt.Server{},
			wantErr: false,
		},
		{
			name:    "invalid configuration",
			config:  nil,
			want:    &mqtt.Server{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := configureTCP(tt.config, newServer())
			if err != nil {
				fmt.Println(err)
			}
		})
	}
}

func TestConfigureWebsocket(t *testing.T) {
	tests := []struct {
		name    string
		config  []*Websocket
		want    *mqtt.Server
		wantErr bool
	}{
		{
			name: "sucessfull configuration",
			config: []*Websocket{
				{
					ID:   "stats1",
					Port: 8080,
				},
			},
			want:    &mqtt.Server{},
			wantErr: false,
		},
		{
			name: "invalid configuration",
			config: []*Websocket{
				{
					ID:   "stats1",
					Port: 8080,
				},
				{
					ID:   "stats1",
					Port: 8080,
				},
			},
			want:    &mqtt.Server{},
			wantErr: false,
		},
		{
			name:    "invalid configuration",
			config:  nil,
			want:    &mqtt.Server{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := configureWebsocket(tt.config, newServer())
			if err != nil {
				fmt.Println(err)
			}
		})
	}
}

func TestConfigureLogging(t *testing.T) {
	tests := []struct {
		name    string
		config  *Logging
		want    *mqtt.Server
		wantErr bool
	}{
		{
			name: "sucessfull configuration",
			config: &Logging{
				Level:  "INFO",
				Output: "TEXT",
			},
			want:    &mqtt.Server{},
			wantErr: false,
		},
		{
			name:    "nil configuration",
			config:  nil,
			want:    &mqtt.Server{},
			wantErr: false,
		},
		{
			name: "json configuration",
			config: &Logging{
				Level:  "INFO",
				Output: "JSON",
			},
			want:    &mqtt.Server{},
			wantErr: false,
		},
		{
			name: "debug configuration",
			config: &Logging{
				Level:  "DEBUG",
				Output: "JSON",
			},
			want:    &mqtt.Server{},
			wantErr: false,
		},
		{
			name: "warn configuration",
			config: &Logging{
				Level:  "WARN",
				Output: "JSON",
			},
			want:    &mqtt.Server{},
			wantErr: false,
		},
		{
			name: "unknown configuration",
			config: &Logging{
				Level:  "WHAT_IS_THIS",
				Output: "JSON",
			},
			want:    &mqtt.Server{},
			wantErr: false,
		},
		{
			name: "unknown output configuration",
			config: &Logging{
				Level:  "WARN",
				Output: "WHAT_IS_THIS",
			},
			want:    &mqtt.Server{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configureLogging(tt.config, newServer())
		})
	}
}
