// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package debug

import (
	"fmt"
	"log/slog"
	"strings"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/storage"
	"github.com/mochi-mqtt/server/v2/packets"
)

// Options contains configuration settings for the debug output.
type Options struct {
	ShowPacketData bool // include decoded packet data (default false)
	ShowPings      bool // show ping requests and responses (default false)
	ShowPasswords  bool // show connecting user passwords (default false)
}

// Hook is a debugging hook which logs additional low-level information from the server.
type Hook struct {
	mqtt.HookBase
	config *Options
	Log    *slog.Logger
}

// ID returns the ID of the hook.
func (h *Hook) ID() string {
	return "debug"
}

// Provides indicates that this hook provides all methods.
func (h *Hook) Provides(b byte) bool {
	return true
}

// Init is called when the hook is initialized.
func (h *Hook) Init(config any) error {
	if _, ok := config.(*Options); !ok && config != nil {
		return mqtt.ErrInvalidConfigType
	}

	if config == nil {
		config = new(Options)
	}

	h.config = config.(*Options)

	return nil
}

// SetOpts is called when the hook receives inheritable server parameters.
func (h *Hook) SetOpts(l *slog.Logger, opts *mqtt.HookOptions) {
	h.Log = l
	h.Log.Debug("", "method", "SetOpts")
}

// Stop is called when the hook is stopped.
func (h *Hook) Stop() error {
	h.Log.Debug("", "method", "Stop")
	return nil
}

// OnStarted is called when the server starts.
func (h *Hook) OnStarted() {
	h.Log.Debug("", "method", "OnStarted")
}

// OnStopped is called when the server stops.
func (h *Hook) OnStopped() {
	h.Log.Debug("", "method", "OnStopped")
}

// OnPacketRead is called when a new packet is received from a client.
func (h *Hook) OnPacketRead(cl *mqtt.Client, pk packets.Packet) (packets.Packet, error) {
	if (pk.FixedHeader.Type == packets.Pingresp || pk.FixedHeader.Type == packets.Pingreq) && !h.config.ShowPings {
		return pk, nil
	}

	h.Log.Debug(fmt.Sprintf("%s << %s", strings.ToUpper(packets.PacketNames[pk.FixedHeader.Type]), cl.ID), "m", h.packetMeta(pk))
	return pk, nil
}

// OnPacketSent is called when a packet is sent to a client.
func (h *Hook) OnPacketSent(cl *mqtt.Client, pk packets.Packet, b []byte) {
	if (pk.FixedHeader.Type == packets.Pingresp || pk.FixedHeader.Type == packets.Pingreq) && !h.config.ShowPings {
		return
	}

	h.Log.Debug(fmt.Sprintf("%s >> %s", strings.ToUpper(packets.PacketNames[pk.FixedHeader.Type]), cl.ID), "m", h.packetMeta(pk))
}

// OnRetainMessage is called when a published message is retained (or retain deleted/modified).
func (h *Hook) OnRetainMessage(cl *mqtt.Client, pk packets.Packet, r int64) {
	h.Log.Debug("retained message on topic", "m", h.packetMeta(pk))
}

// OnQosPublish is called when a publish packet with Qos is issued to a subscriber.
func (h *Hook) OnQosPublish(cl *mqtt.Client, pk packets.Packet, sent int64, resends int) {
	h.Log.Debug("inflight out", "m", h.packetMeta(pk))
}

// OnQosComplete is called when the Qos flow for a message has been completed.
func (h *Hook) OnQosComplete(cl *mqtt.Client, pk packets.Packet) {
	h.Log.Debug("inflight complete", "m", h.packetMeta(pk))
}

// OnQosDropped is called the Qos flow for a message expires.
func (h *Hook) OnQosDropped(cl *mqtt.Client, pk packets.Packet) {
	h.Log.Debug("inflight dropped", "m", h.packetMeta(pk))
}

// OnLWTSent is called when a Will Message has been issued from a disconnecting client.
func (h *Hook) OnLWTSent(cl *mqtt.Client, pk packets.Packet) {
	h.Log.Debug("sent lwt for client", "method", "OnLWTSent", "client", cl.ID)
}

// OnRetainedExpired is called when the server clears expired retained messages.
func (h *Hook) OnRetainedExpired(filter string) {
	h.Log.Debug("retained message expired", "method", "OnRetainedExpired", "topic", filter)
}

// OnClientExpired is called when the server clears an expired client.
func (h *Hook) OnClientExpired(cl *mqtt.Client) {
	h.Log.Debug("client session expired", "method", "OnClientExpired", "client", cl.ID)
}

// StoredClients is called when the server restores clients from a store.
func (h *Hook) StoredClients() (v []storage.Client, err error) {
	h.Log.Debug("", "method", "StoredClients")

	return v, nil
}

// StoredSubscriptions is called when the server restores subscriptions from a store.
func (h *Hook) StoredSubscriptions() (v []storage.Subscription, err error) {
	h.Log.Debug("", "method", "StoredSubscriptions")
	return v, nil
}

// StoredRetainedMessages is called when the server restores retained messages from a store.
func (h *Hook) StoredRetainedMessages() (v []storage.Message, err error) {
	h.Log.Debug("", "method", "StoredRetainedMessages")
	return v, nil
}

// StoredInflightMessages is called when the server restores inflight messages from a store.
func (h *Hook) StoredInflightMessages() (v []storage.Message, err error) {
	h.Log.Debug("", "method", "StoredInflightMessages")
	return v, nil
}

// StoredSysInfo is called when the server restores system info from a store.
func (h *Hook) StoredSysInfo() (v storage.SystemInfo, err error) {
	h.Log.Debug("", "method", "StoredSysInfo")

	return v, nil
}

// packetMeta adds additional type-specific metadata to the debug logs.
func (h *Hook) packetMeta(pk packets.Packet) map[string]any {
	m := map[string]any{}
	switch pk.FixedHeader.Type {
	case packets.Connect:
		m["id"] = pk.Connect.ClientIdentifier
		m["clean"] = pk.Connect.Clean
		m["keepalive"] = pk.Connect.Keepalive
		m["version"] = pk.ProtocolVersion
		m["username"] = string(pk.Connect.Username)
		if h.config.ShowPasswords {
			m["password"] = string(pk.Connect.Password)
		}
		if pk.Connect.WillFlag {
			m["will_topic"] = pk.Connect.WillTopic
			m["will_payload"] = string(pk.Connect.WillPayload)
		}
	case packets.Publish:
		m["topic"] = pk.TopicName
		m["payload"] = string(pk.Payload)
		m["raw"] = pk.Payload
		m["qos"] = pk.FixedHeader.Qos
		m["id"] = pk.PacketID
	case packets.Connack:
		fallthrough
	case packets.Disconnect:
		fallthrough
	case packets.Puback:
		fallthrough
	case packets.Pubrec:
		fallthrough
	case packets.Pubrel:
		fallthrough
	case packets.Pubcomp:
		m["id"] = pk.PacketID
		m["reason"] = int(pk.ReasonCode)
		if pk.ReasonCode > packets.CodeSuccess.Code && pk.ProtocolVersion == 5 {
			m["reason_string"] = pk.Properties.ReasonString
		}
	case packets.Subscribe:
		f := map[string]int{}
		ids := map[string]int{}
		for _, v := range pk.Filters {
			f[v.Filter] = int(v.Qos)
			ids[v.Filter] = v.Identifier
		}
		m["filters"] = f
		m["subids"] = f

	case packets.Unsubscribe:
		f := []string{}
		for _, v := range pk.Filters {
			f = append(f, v.Filter)
		}
		m["filters"] = f
	case packets.Suback:
		fallthrough
	case packets.Unsuback:
		r := []int{}
		for _, v := range pk.ReasonCodes {
			r = append(r, int(v))
		}
		m["reasons"] = r
	case packets.Auth:
		// tbd
	}

	if h.config.ShowPacketData {
		m["packet"] = pk
	}

	return m
}
