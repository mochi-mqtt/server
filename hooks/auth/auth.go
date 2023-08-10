// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package auth

import (
	"bytes"
	"context"

	"log/slog"

	"github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

// Options contains the configuration/rules data for the auth ledger.
type Options struct {
	Data   []byte
	Ledger *Ledger
}

// Hook is an authentication hook which implements an auth ledger.
type Hook struct {
	mqtt.HookBase
	config *Options
	ledger *Ledger
}

// ID returns the ID of the hook.
func (h *Hook) ID() string {
	return "auth-ledger"
}

// Provides indicates which hook methods this hook provides.
func (h *Hook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqtt.OnConnectAuthenticate,
		mqtt.OnACLCheck,
	}, []byte{b})
}

// Init configures the hook with the auth ledger to be used for checking.
func (h *Hook) Init(config any) error {
	if _, ok := config.(*Options); !ok && config != nil {
		return mqtt.ErrInvalidConfigType
	}

	if config == nil {
		config = new(Options)
	}

	h.config = config.(*Options)

	var err error
	if h.config.Ledger != nil {
		h.ledger = h.config.Ledger
	} else if len(h.config.Data) > 0 {
		h.ledger = new(Ledger)
		err = h.ledger.Unmarshal(h.config.Data)
	}
	if err != nil {
		return err
	}

	if h.ledger == nil {
		h.ledger = &Ledger{
			Auth: AuthRules{},
			ACL:  ACLRules{},
		}
	}

	h.Log.LogAttrs(context.TODO(), slog.LevelInfo,
		"loaded auth rules",
		slog.Int("authentication", len(h.ledger.Auth)),
		slog.Int("acl", len(h.ledger.ACL)))

	return nil
}

// OnConnectAuthenticate returns true if the connecting client has rules which provide access
// in the auth ledger.
func (h *Hook) OnConnectAuthenticate(cl *mqtt.Client, pk packets.Packet) bool {
	if _, ok := h.ledger.AuthOk(cl, pk); ok {
		return true
	}

	h.Log.LogAttrs(context.TODO(), slog.LevelInfo,
		"client failed authentication check",
		slog.String("username", string(pk.Connect.Username)),
		slog.String("remote", cl.Net.Remote))
	return false
}

// OnACLCheck returns true if the connecting client has matching read or write access to subscribe
// or publish to a given topic.
func (h *Hook) OnACLCheck(cl *mqtt.Client, topic string, write bool) bool {
	if _, ok := h.ledger.ACLOk(cl, topic, write); ok {
		return true
	}

	h.Log.LogAttrs(context.TODO(), slog.LevelDebug,
		"client failed allowed ACL check",
		slog.String("client", cl.ID),
		slog.String("username", string(cl.Properties.Username)),
		slog.String("topic", topic))

	return false
}
