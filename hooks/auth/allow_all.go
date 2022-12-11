// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-co
// SPDX-FileContributor: mochi-co

package auth

import (
	"bytes"

	"github.com/mochi-co/mqtt/v2"
	"github.com/mochi-co/mqtt/v2/packets"
)

// AllowHook is an authentication hook which allows connection access
// for all users and read and write access to all topics.
type AllowHook struct {
	mqtt.HookBase
}

// ID returns the ID of the hook.
func (h *AllowHook) ID() string {
	return "allow-all-auth"
}

// Provides indicates which hook methods this hook provides.
func (h *AllowHook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqtt.OnConnectAuthenticate,
		mqtt.OnACLCheck,
	}, []byte{b})
}

// OnConnectAuthenticate returns true/allowed for all requests.
func (h *AllowHook) OnConnectAuthenticate(cl *mqtt.Client, pk packets.Packet) bool {
	return true
}

// OnACLCheck returns true/allowed for all checks.
func (h *AllowHook) OnACLCheck(cl *mqtt.Client, topic string, write bool) bool {
	return true
}
