// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package config

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/hooks/storage/badger"
	"github.com/mochi-mqtt/server/v2/hooks/storage/bolt"
	"github.com/mochi-mqtt/server/v2/hooks/storage/redis"
	"github.com/mochi-mqtt/server/v2/listeners"

	mqtt "github.com/mochi-mqtt/server/v2"
)

var (
	yamlBytes = []byte(`
listeners:
  - type: "tcp"
    id: "file-tcp1"
    address: ":1883"
hooks:
  auth:
    allow_all: true
options:
  client_net_write_buffer_size: 2048
  capabilities:
    minimum_protocol_version: 3
    compatibilities:
      restore_sys_info_on_restart: true
`)

	jsonBytes = []byte(`{
   "listeners": [
      {
         "type": "tcp",
         "id": "file-tcp1",
         "address": ":1883"
      }
   ],
   "hooks": {
      "auth": {
         "allow_all": true
      }
   },
   "options": {
      "client_net_write_buffer_size": 2048,
      "capabilities": {
         "minimum_protocol_version": 3,
         "compatibilities": {
            "restore_sys_info_on_restart": true
         }
      }
   }
}
`)

	parsedOptions = mqtt.Options{
		Listeners: []listeners.Config{
			{
				Type:    listeners.TypeTCP,
				ID:      "file-tcp1",
				Address: ":1883",
			},
		},
		Hooks: []mqtt.HookLoadConfig{
			{
				Hook: new(auth.AllowHook),
			},
		},
		ClientNetWriteBufferSize: 2048,
		Capabilities: &mqtt.Capabilities{
			MinimumProtocolVersion: 3,
			Compatibilities: mqtt.Compatibilities{
				RestoreSysInfoOnRestart: true,
			},
		},
	}
)

func TestFromBytesEmptyL(t *testing.T) {
	_, err := FromBytes([]byte{})
	require.NoError(t, err)
}

func TestFromBytesYAML(t *testing.T) {
	o, err := FromBytes(yamlBytes)
	require.NoError(t, err)
	require.Equal(t, parsedOptions, *o)
}

func TestFromBytesYAMLError(t *testing.T) {
	_, err := FromBytes(append(yamlBytes, 'a'))
	require.Error(t, err)
}

func TestFromBytesJSON(t *testing.T) {
	o, err := FromBytes(jsonBytes)
	require.NoError(t, err)
	require.Equal(t, parsedOptions, *o)
}

func TestFromBytesJSONError(t *testing.T) {
	_, err := FromBytes(append(jsonBytes, 'a'))
	require.Error(t, err)
}

func TestToHooksAuthAllowAll(t *testing.T) {
	hc := HookConfigs{
		Auth: &HookAuthConfig{
			AllowAll: true,
		},
	}

	th := hc.toHooksAuth()
	expect := []mqtt.HookLoadConfig{
		{Hook: new(auth.AllowHook)},
	}
	require.Equal(t, expect, th)
}

func TestToHooksAuthAllowLedger(t *testing.T) {
	hc := HookConfigs{
		Auth: &HookAuthConfig{
			Ledger: auth.Ledger{
				Auth: auth.AuthRules{
					{Username: "peach", Password: "password1", Allow: true},
				},
			},
		},
	}

	th := hc.toHooksAuth()
	expect := []mqtt.HookLoadConfig{
		{
			Hook: new(auth.Hook),
			Config: &auth.Options{
				Ledger: &auth.Ledger{ // avoid copying sync.Locker
					Auth: auth.AuthRules{
						{Username: "peach", Password: "password1", Allow: true},
					},
				},
			},
		},
	}
	require.Equal(t, expect, th)
}

func TestToHooksStorageBadger(t *testing.T) {
	hc := HookConfigs{
		Storage: &HookStorageConfig{
			Badger: &badger.Options{
				Path: "badger",
			},
		},
	}

	th := hc.toHooksStorage()
	expect := []mqtt.HookLoadConfig{
		{
			Hook:   new(badger.Hook),
			Config: hc.Storage.Badger,
		},
	}

	require.Equal(t, expect, th)
}

func TestToHooksStorageBolt(t *testing.T) {
	hc := HookConfigs{
		Storage: &HookStorageConfig{
			Bolt: &bolt.Options{
				Path: "bolt",
			},
		},
	}

	th := hc.toHooksStorage()
	expect := []mqtt.HookLoadConfig{
		{
			Hook:   new(bolt.Hook),
			Config: hc.Storage.Bolt,
		},
	}

	require.Equal(t, expect, th)
}

func TestToHooksStorageRedis(t *testing.T) {
	hc := HookConfigs{
		Storage: &HookStorageConfig{
			Redis: &redis.Options{
				Username: "test",
			},
		},
	}

	th := hc.toHooksStorage()
	expect := []mqtt.HookLoadConfig{
		{
			Hook:   new(redis.Hook),
			Config: hc.Storage.Redis,
		},
	}

	require.Equal(t, expect, th)
}
