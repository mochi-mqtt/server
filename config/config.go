// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package config

import (
	"encoding/json"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/hooks/debug"
	"github.com/mochi-mqtt/server/v2/hooks/storage/badger"
	"github.com/mochi-mqtt/server/v2/hooks/storage/bolt"
	"github.com/mochi-mqtt/server/v2/hooks/storage/redis"
	"github.com/mochi-mqtt/server/v2/listeners"
	"gopkg.in/yaml.v3"

	mqtt "github.com/mochi-mqtt/server/v2"
)

type config struct {
	Options     mqtt.Options
	Listeners   []listeners.Config `yaml:"listeners" json:"listeners"`
	HookConfigs HookConfigs        `yaml:"hooks" json:"hooks"`
}

type HookConfigs struct {
	Auth    *HookAuthConfig    `yaml:"auth" json:"auth"`
	Storage *HookStorageConfig `yaml:"storage" json:"storage"`
	Debug   *debug.Options     `yaml:"debug" json:"debug"`
}

type HookAuthConfig struct {
	Ledger   auth.Ledger `yaml:"ledger" json:"ledger"`
	AllowAll bool        `yaml:"allow_all" json:"allow_all"`
}

type HookStorageConfig struct {
	Badger *badger.Options `yaml:"badger" json:"badger"`
	Bolt   *bolt.Options   `yaml:"bolt" json:"bolt"`
	Redis  *redis.Options  `yaml:"redis" json:"redis"`
}

func (hc HookConfigs) ToHooks() []mqtt.HookLoadConfig {
	var hlc []mqtt.HookLoadConfig

	if hc.Auth != nil {
		hlc = append(hlc, hc.toHooksAuth()...)
	}

	if hc.Storage != nil {
		hlc = append(hlc, hc.toHooksAuth()...)
	}

	if hc.Debug != nil {
		hlc = append(hlc, mqtt.HookLoadConfig{
			Hook:   new(debug.Hook),
			Config: hc.Debug,
		})
	}

	return hlc
}

func (hc HookConfigs) toHooksAuth() []mqtt.HookLoadConfig {
	var hlc []mqtt.HookLoadConfig
	if hc.Auth.AllowAll {
		hlc = append(hlc, mqtt.HookLoadConfig{
			Hook: new(auth.AllowHook),
		})
	} else {
		hlc = append(hlc, mqtt.HookLoadConfig{
			Hook: new(auth.Hook),
			Config: &auth.Options{
				Ledger: &auth.Ledger{ // avoid copying sync.Locker
					Users: hc.Auth.Ledger.Users,
					Auth:  hc.Auth.Ledger.Auth,
					ACL:   hc.Auth.Ledger.ACL,
				},
			},
		})
	}
	return hlc
}

func (hc HookConfigs) toHooksStorage() []mqtt.HookLoadConfig {
	var hlc []mqtt.HookLoadConfig
	if hc.Storage.Badger != nil {
		hlc = append(hlc, mqtt.HookLoadConfig{
			Hook:   new(badger.Hook),
			Config: hc.Storage.Badger,
		})
	}

	if hc.Storage.Bolt != nil {
		hlc = append(hlc, mqtt.HookLoadConfig{
			Hook:   new(bolt.Hook),
			Config: hc.Storage.Bolt,
		})
	}

	if hc.Storage.Redis != nil {
		hlc = append(hlc, mqtt.HookLoadConfig{
			Hook:   new(redis.Hook),
			Config: hc.Storage.Redis,
		})
	}
	return hlc
}

func FromBytes(b []byte) (*mqtt.Options, error) {
	c := new(config)
	o := mqtt.Options{}

	if len(b) == 0 {
		return nil, nil
	}

	if b[0] == '{' {
		err := json.Unmarshal(b, c)
		if err != nil {
			return nil, err
		}
	} else {
		err := yaml.Unmarshal(b, c)
		if err != nil {
			return nil, err
		}
	}

	o = c.Options
	o.Hooks = c.HookConfigs.ToHooks()
	o.Listeners = c.Listeners

	return &o, nil
}
