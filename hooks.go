// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co, thedevop, dgduncan

package mqtt

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/mochi-mqtt/server/v2/hooks/storage"
	"github.com/mochi-mqtt/server/v2/packets"
	"github.com/mochi-mqtt/server/v2/system"
)

const (
	SetOptions byte = iota
	OnSysInfoTick
	OnStarted
	OnStopped
	OnConnectAuthenticate
	OnACLCheck
	OnConnect
	OnSessionEstablish
	OnSessionEstablished
	OnDisconnect
	OnAuthPacket
	OnPacketRead
	OnPacketEncode
	OnPacketSent
	OnPacketProcessed
	OnSubscribe
	OnSubscribed
	OnSelectSubscribers
	OnUnsubscribe
	OnUnsubscribed
	OnPublish
	OnPublished
	OnPublishDropped
	OnRetainMessage
	OnRetainPublished
	OnQosPublish
	OnQosComplete
	OnQosDropped
	OnPacketIDExhausted
	OnWill
	OnWillSent
	OnClientExpired
	OnRetainedExpired
	StoredClients
	StoredSubscriptions
	StoredInflightMessages
	StoredRetainedMessages
	StoredSysInfo
)

var (
	// ErrInvalidConfigType indicates a different Type of config value was expected to what was received.
	ErrInvalidConfigType = errors.New("invalid config type provided")
)

// Hook provides an interface of handlers for different events which occur
// during the lifecycle of the broker.
type Hook interface {
	ID() string
	Provides(b byte) bool
	Init(config any) error
	Stop() error
	SetOpts(l *slog.Logger, o *HookOptions)
	OnStarted()
	OnStopped()
	OnConnectAuthenticate(cl *Client, pk packets.Packet) bool
	OnACLCheck(cl *Client, topic string, write bool) bool
	OnSysInfoTick(*system.Info)
	OnConnect(cl *Client, pk packets.Packet) error
	OnSessionEstablish(cl *Client, pk packets.Packet)
	OnSessionEstablished(cl *Client, pk packets.Packet)
	OnDisconnect(cl *Client, err error, expire bool)
	OnAuthPacket(cl *Client, pk packets.Packet) (packets.Packet, error)
	OnPacketRead(cl *Client, pk packets.Packet) (packets.Packet, error) // triggers when a new packet is received by a client, but before packet validation
	OnPacketEncode(cl *Client, pk packets.Packet) packets.Packet        // modify a packet before it is byte-encoded and written to the client
	OnPacketSent(cl *Client, pk packets.Packet, b []byte)               // triggers when packet bytes have been written to the client
	OnPacketProcessed(cl *Client, pk packets.Packet, err error)         // triggers after a packet from the client been processed (handled)
	OnSubscribe(cl *Client, pk packets.Packet) packets.Packet
	OnSubscribed(cl *Client, pk packets.Packet, reasonCodes []byte)
	OnSelectSubscribers(subs *Subscribers, pk packets.Packet) *Subscribers
	OnUnsubscribe(cl *Client, pk packets.Packet) packets.Packet
	OnUnsubscribed(cl *Client, pk packets.Packet)
	OnPublish(cl *Client, pk packets.Packet) (packets.Packet, error)
	OnPublished(cl *Client, pk packets.Packet)
	OnPublishDropped(cl *Client, pk packets.Packet)
	OnRetainMessage(cl *Client, pk packets.Packet, r int64)
	OnRetainPublished(cl *Client, pk packets.Packet)
	OnQosPublish(cl *Client, pk packets.Packet, sent int64, resends int)
	OnQosComplete(cl *Client, pk packets.Packet)
	OnQosDropped(cl *Client, pk packets.Packet)
	OnPacketIDExhausted(cl *Client, pk packets.Packet)
	OnWill(cl *Client, will Will) (Will, error)
	OnWillSent(cl *Client, pk packets.Packet)
	OnClientExpired(cl *Client)
	OnRetainedExpired(filter string)
	StoredClients() ([]storage.Client, error)
	StoredSubscriptions() ([]storage.Subscription, error)
	StoredInflightMessages() ([]storage.Message, error)
	StoredRetainedMessages() ([]storage.Message, error)
	StoredSysInfo() (storage.SystemInfo, error)
}

// HookOptions contains values which are inherited from the server on initialisation.
type HookOptions struct {
	Capabilities *Capabilities
}

// Hooks is a slice of Hook interfaces to be called in sequence.
type Hooks struct {
	Log        *slog.Logger   // a logger for the hook (from the server)
	internal   atomic.Value   // a slice of []Hook
	wg         sync.WaitGroup // a waitgroup for syncing hook shutdown
	qty        int64          // the number of hooks in use
	sync.Mutex                // a mutex for locking when adding hooks
}

// Len returns the number of hooks added.
func (h *Hooks) Len() int64 {
	return atomic.LoadInt64(&h.qty)
}

// Provides returns true if any one hook provides any of the requested hook methods.
func (h *Hooks) Provides(b ...byte) bool {
	for _, hook := range h.GetAll() {
		for _, hb := range b {
			if hook.Provides(hb) {
				return true
			}
		}
	}

	return false
}

// Add adds and initializes a new hook.
func (h *Hooks) Add(hook Hook, config any) error {
	h.Lock()
	defer h.Unlock()

	err := hook.Init(config)
	if err != nil {
		return fmt.Errorf("failed initialising %s hook: %w", hook.ID(), err)
	}

	i, ok := h.internal.Load().([]Hook)
	if !ok {
		i = []Hook{}
	}

	i = append(i, hook)
	h.internal.Store(i)
	atomic.AddInt64(&h.qty, 1)
	h.wg.Add(1)

	return nil
}

// GetAll returns a slice of all the hooks.
func (h *Hooks) GetAll() []Hook {
	i, ok := h.internal.Load().([]Hook)
	if !ok {
		return []Hook{}
	}

	return i
}

// Stop indicates all attached hooks to gracefully end.
func (h *Hooks) Stop() {
	go func() {
		for _, hook := range h.GetAll() {
			h.Log.Info("stopping hook", "hook", hook.ID())
			if err := hook.Stop(); err != nil {
				h.Log.Debug("problem stopping hook", "error", err, "hook", hook.ID())
			}

			h.wg.Done()
		}
	}()

	h.wg.Wait()
}

// OnSysInfoTick is called when the $SYS topic values are published out.
func (h *Hooks) OnSysInfoTick(sys *system.Info) {
	for _, hook := range h.GetAll() {
		if hook.Provides(OnSysInfoTick) {
			hook.OnSysInfoTick(sys)
		}
	}
}

// OnStarted is called when the server has successfully started.
func (h *Hooks) OnStarted() {
	for _, hook := range h.GetAll() {
		if hook.Provides(OnStarted) {
			hook.OnStarted()
		}
	}
}

// OnStopped is called when the server has successfully stopped.
func (h *Hooks) OnStopped() {
	for _, hook := range h.GetAll() {
		if hook.Provides(OnStopped) {
			hook.OnStopped()
		}
	}
}

// OnConnect is called when a new client connects, and may return a packets.Code as an error to halt the connection.
func (h *Hooks) OnConnect(cl *Client, pk packets.Packet) error {
	for _, hook := range h.GetAll() {
		if hook.Provides(OnConnect) {
			err := hook.OnConnect(cl, pk)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// OnSessionEstablish is called right after a new client connects and authenticates and right before
// the session is established and CONNACK is sent.
func (h *Hooks) OnSessionEstablish(cl *Client, pk packets.Packet) {
	for _, hook := range h.GetAll() {
		if hook.Provides(OnSessionEstablish) {
			hook.OnSessionEstablish(cl, pk)
		}
	}
}

// OnSessionEstablished is called when a new client establishes a session (after OnConnect).
func (h *Hooks) OnSessionEstablished(cl *Client, pk packets.Packet) {
	for _, hook := range h.GetAll() {
		if hook.Provides(OnSessionEstablished) {
			hook.OnSessionEstablished(cl, pk)
		}
	}
}

// OnDisconnect is called when a client is disconnected for any reason.
func (h *Hooks) OnDisconnect(cl *Client, err error, expire bool) {
	for _, hook := range h.GetAll() {
		if hook.Provides(OnDisconnect) {
			hook.OnDisconnect(cl, err, expire)
		}
	}
}

// OnPacketRead is called when a packet is received from a client.
func (h *Hooks) OnPacketRead(cl *Client, pk packets.Packet) (pkx packets.Packet, err error) {
	pkx = pk
	for _, hook := range h.GetAll() {
		if hook.Provides(OnPacketRead) {
			npk, err := hook.OnPacketRead(cl, pkx)
			if err != nil && errors.Is(err, packets.ErrRejectPacket) {
				h.Log.Debug("packet rejected", "hook", hook.ID(), "packet", pkx)
				return pk, err
			} else if err != nil {
				continue
			}

			pkx = npk
		}
	}

	return
}

// OnAuthPacket is called when an auth packet is received. It is intended to allow developers
// to create their own auth packet handling mechanisms.
func (h *Hooks) OnAuthPacket(cl *Client, pk packets.Packet) (pkx packets.Packet, err error) {
	pkx = pk
	for _, hook := range h.GetAll() {
		if hook.Provides(OnAuthPacket) {
			npk, err := hook.OnAuthPacket(cl, pkx)
			if err != nil {
				return pk, err
			}

			pkx = npk
		}
	}

	return
}

// OnPacketEncode is called immediately before a packet is encoded to be sent to a client.
func (h *Hooks) OnPacketEncode(cl *Client, pk packets.Packet) packets.Packet {
	for _, hook := range h.GetAll() {
		if hook.Provides(OnPacketEncode) {
			pk = hook.OnPacketEncode(cl, pk)
		}
	}

	return pk
}

// OnPacketProcessed is called when a packet has been received and successfully handled by the broker.
func (h *Hooks) OnPacketProcessed(cl *Client, pk packets.Packet, err error) {
	for _, hook := range h.GetAll() {
		if hook.Provides(OnPacketProcessed) {
			hook.OnPacketProcessed(cl, pk, err)
		}
	}
}

// OnPacketSent is called when a packet has been sent to a client. It takes a bytes parameter
// containing the bytes sent.
func (h *Hooks) OnPacketSent(cl *Client, pk packets.Packet, b []byte) {
	for _, hook := range h.GetAll() {
		if hook.Provides(OnPacketSent) {
			hook.OnPacketSent(cl, pk, b)
		}
	}
}

// OnSubscribe is called when a client subscribes to one or more filters. This method
// differs from OnSubscribed in that it allows you to modify the subscription values
// before the packet is processed. The return values of the hook methods are passed-through
// in the order the hooks were attached.
func (h *Hooks) OnSubscribe(cl *Client, pk packets.Packet) packets.Packet {
	for _, hook := range h.GetAll() {
		if hook.Provides(OnSubscribe) {
			pk = hook.OnSubscribe(cl, pk)
		}
	}
	return pk
}

// OnSubscribed is called when a client subscribes to one or more filters.
func (h *Hooks) OnSubscribed(cl *Client, pk packets.Packet, reasonCodes []byte) {
	for _, hook := range h.GetAll() {
		if hook.Provides(OnSubscribed) {
			hook.OnSubscribed(cl, pk, reasonCodes)
		}
	}
}

// OnSelectSubscribers is called when subscribers have been collected for a topic, but before
// shared subscription subscribers have been selected. This hook can be used to programmatically
// remove or add clients to a publish to subscribers process, or to select the subscriber for a shared
// group in a custom manner (such as based on client id, ip, etc).
func (h *Hooks) OnSelectSubscribers(subs *Subscribers, pk packets.Packet) *Subscribers {
	for _, hook := range h.GetAll() {
		if hook.Provides(OnSelectSubscribers) {
			subs = hook.OnSelectSubscribers(subs, pk)
		}
	}
	return subs
}

// OnUnsubscribe is called when a client unsubscribes from one or more filters. This method
// differs from OnUnsubscribed in that it allows you to modify the unsubscription values
// before the packet is processed. The return values of the hook methods are passed-through
// in the order the hooks were attached.
func (h *Hooks) OnUnsubscribe(cl *Client, pk packets.Packet) packets.Packet {
	for _, hook := range h.GetAll() {
		if hook.Provides(OnUnsubscribe) {
			pk = hook.OnUnsubscribe(cl, pk)
		}
	}
	return pk
}

// OnUnsubscribed is called when a client unsubscribes from one or more filters.
func (h *Hooks) OnUnsubscribed(cl *Client, pk packets.Packet) {
	for _, hook := range h.GetAll() {
		if hook.Provides(OnUnsubscribed) {
			hook.OnUnsubscribed(cl, pk)
		}
	}
}

// OnPublish is called when a client publishes a message. This method differs from OnPublished
// in that it allows you to modify you to modify the incoming packet before it is processed.
// The return values of the hook methods are passed-through in the order the hooks were attached.
func (h *Hooks) OnPublish(cl *Client, pk packets.Packet) (pkx packets.Packet, err error) {
	pkx = pk
	for _, hook := range h.GetAll() {
		if hook.Provides(OnPublish) {
			npk, err := hook.OnPublish(cl, pkx)
			if err != nil {
				if errors.Is(err, packets.ErrRejectPacket) {
					h.Log.Debug("publish packet rejected",
						"error", err,
						"hook", hook.ID(),
						"packet", pkx)
					return pk, err
				}
				h.Log.Error("publish packet error",
					"error", err,
					"hook", hook.ID(),
					"packet", pkx)
				return pk, err
			}
			pkx = npk
		}
	}

	return
}

// OnPublished is called when a client has published a message to subscribers.
func (h *Hooks) OnPublished(cl *Client, pk packets.Packet) {
	for _, hook := range h.GetAll() {
		if hook.Provides(OnPublished) {
			hook.OnPublished(cl, pk)
		}
	}
}

// OnPublishDropped is called when a message to a client was dropped instead of delivered
// such as when a client is too slow to respond.
func (h *Hooks) OnPublishDropped(cl *Client, pk packets.Packet) {
	for _, hook := range h.GetAll() {
		if hook.Provides(OnPublishDropped) {
			hook.OnPublishDropped(cl, pk)
		}
	}
}

// OnRetainMessage is called then a published message is retained.
func (h *Hooks) OnRetainMessage(cl *Client, pk packets.Packet, r int64) {
	for _, hook := range h.GetAll() {
		if hook.Provides(OnRetainMessage) {
			hook.OnRetainMessage(cl, pk, r)
		}
	}
}

// OnRetainPublished is called when a retained message is published.
func (h *Hooks) OnRetainPublished(cl *Client, pk packets.Packet) {
	for _, hook := range h.GetAll() {
		if hook.Provides(OnRetainPublished) {
			hook.OnRetainPublished(cl, pk)
		}
	}
}

// OnQosPublish is called when a publish packet with Qos >= 1 is issued to a subscriber.
// In other words, this method is called when a new inflight message is created or resent.
// It is typically used to store a new inflight message.
func (h *Hooks) OnQosPublish(cl *Client, pk packets.Packet, sent int64, resends int) {
	for _, hook := range h.GetAll() {
		if hook.Provides(OnQosPublish) {
			hook.OnQosPublish(cl, pk, sent, resends)
		}
	}
}

// OnQosComplete is called when the Qos flow for a message has been completed.
// In other words, when an inflight message is resolved.
// It is typically used to delete an inflight message from a store.
func (h *Hooks) OnQosComplete(cl *Client, pk packets.Packet) {
	for _, hook := range h.GetAll() {
		if hook.Provides(OnQosComplete) {
			hook.OnQosComplete(cl, pk)
		}
	}
}

// OnQosDropped is called the Qos flow for a message expires. In other words, when
// an inflight message expires or is abandoned. It is typically used to delete an
// inflight message from a store.
func (h *Hooks) OnQosDropped(cl *Client, pk packets.Packet) {
	for _, hook := range h.GetAll() {
		if hook.Provides(OnQosDropped) {
			hook.OnQosDropped(cl, pk)
		}
	}
}

// OnPacketIDExhausted is called when the client runs out of unused packet ids to
// assign to a packet.
func (h *Hooks) OnPacketIDExhausted(cl *Client, pk packets.Packet) {
	for _, hook := range h.GetAll() {
		if hook.Provides(OnPacketIDExhausted) {
			hook.OnPacketIDExhausted(cl, pk)
		}
	}
}

// OnWill is called when a client disconnects and publishes an LWT message. This method
// differs from OnWillSent in that it allows you to modify the LWT message before it is
// published. The return values of the hook methods are passed-through in the order
// the hooks were attached.
func (h *Hooks) OnWill(cl *Client, will Will) Will {
	for _, hook := range h.GetAll() {
		if hook.Provides(OnWill) {
			mlwt, err := hook.OnWill(cl, will)
			if err != nil {
				h.Log.Error("parse will error",
					"error", err,
					"hook", hook.ID(),
					"will", will)
				continue
			}
			will = mlwt
		}
	}

	return will
}

// OnWillSent is called when an LWT message has been issued from a disconnecting client.
func (h *Hooks) OnWillSent(cl *Client, pk packets.Packet) {
	for _, hook := range h.GetAll() {
		if hook.Provides(OnWillSent) {
			hook.OnWillSent(cl, pk)
		}
	}
}

// OnClientExpired is called when a client session has expired and should be deleted.
func (h *Hooks) OnClientExpired(cl *Client) {
	for _, hook := range h.GetAll() {
		if hook.Provides(OnClientExpired) {
			hook.OnClientExpired(cl)
		}
	}
}

// OnRetainedExpired is called when a retained message has expired and should be deleted.
func (h *Hooks) OnRetainedExpired(filter string) {
	for _, hook := range h.GetAll() {
		if hook.Provides(OnRetainedExpired) {
			hook.OnRetainedExpired(filter)
		}
	}
}

// StoredClients returns all clients, e.g. from a persistent store, is used to
// populate the server clients list before start.
func (h *Hooks) StoredClients() (v []storage.Client, err error) {
	for _, hook := range h.GetAll() {
		if hook.Provides(StoredClients) {
			v, err := hook.StoredClients()
			if err != nil {
				h.Log.Error("failed to load clients", "error", err, "hook", hook.ID())
				return v, err
			}

			if len(v) > 0 {
				return v, nil
			}
		}
	}

	return
}

// StoredSubscriptions returns all subcriptions, e.g. from a persistent store, and is
// used to populate the server subscriptions list before start.
func (h *Hooks) StoredSubscriptions() (v []storage.Subscription, err error) {
	for _, hook := range h.GetAll() {
		if hook.Provides(StoredSubscriptions) {
			v, err := hook.StoredSubscriptions()
			if err != nil {
				h.Log.Error("failed to load subscriptions", "error", err, "hook", hook.ID())
				return v, err
			}

			if len(v) > 0 {
				return v, nil
			}
		}
	}

	return
}

// StoredInflightMessages returns all inflight messages, e.g. from a persistent store,
// and is used to populate the restored clients with inflight messages before start.
func (h *Hooks) StoredInflightMessages() (v []storage.Message, err error) {
	for _, hook := range h.GetAll() {
		if hook.Provides(StoredInflightMessages) {
			v, err := hook.StoredInflightMessages()
			if err != nil {
				h.Log.Error("failed to load inflight messages", "error", err, "hook", hook.ID())
				return v, err
			}

			if len(v) > 0 {
				return v, nil
			}
		}
	}

	return
}

// StoredRetainedMessages returns all retained messages, e.g. from a persistent store,
// and is used to populate the server topics with retained messages before start.
func (h *Hooks) StoredRetainedMessages() (v []storage.Message, err error) {
	for _, hook := range h.GetAll() {
		if hook.Provides(StoredRetainedMessages) {
			v, err := hook.StoredRetainedMessages()
			if err != nil {
				h.Log.Error("failed to load retained messages", "error", err, "hook", hook.ID())
				return v, err
			}

			if len(v) > 0 {
				return v, nil
			}
		}
	}

	return
}

// StoredSysInfo returns a set of system info values.
func (h *Hooks) StoredSysInfo() (v storage.SystemInfo, err error) {
	for _, hook := range h.GetAll() {
		if hook.Provides(StoredSysInfo) {
			v, err := hook.StoredSysInfo()
			if err != nil {
				h.Log.Error("failed to load $SYS info", "error", err, "hook", hook.ID())
				return v, err
			}

			if v.Version != "" {
				return v, nil
			}
		}
	}

	return
}

// OnConnectAuthenticate is called when a user attempts to authenticate with the server.
// An implementation of this method MUST be used to allow or deny access to the
// server (see hooks/auth/allow_all or basic). It can be used in custom hooks to
// check connecting users against an existing user database.
func (h *Hooks) OnConnectAuthenticate(cl *Client, pk packets.Packet) bool {
	for _, hook := range h.GetAll() {
		if hook.Provides(OnConnectAuthenticate) {
			if ok := hook.OnConnectAuthenticate(cl, pk); ok {
				return true
			}
		}
	}

	return false
}

// OnACLCheck is called when a user attempts to publish or subscribe to a topic filter.
// An implementation of this method MUST be used to allow or deny access to the
// (see hooks/auth/allow_all or basic). It can be used in custom hooks to
// check publishing and subscribing users against an existing permissions or roles database.
func (h *Hooks) OnACLCheck(cl *Client, topic string, write bool) bool {
	for _, hook := range h.GetAll() {
		if hook.Provides(OnACLCheck) {
			if ok := hook.OnACLCheck(cl, topic, write); ok {
				return true
			}
		}
	}

	return false
}

// HookBase provides a set of default methods for each hook. It should be embedded in
// all hooks.
type HookBase struct {
	Hook
	Log  *slog.Logger
	Opts *HookOptions
}

// ID returns the ID of the hook.
func (h *HookBase) ID() string {
	return "base"
}

// Provides indicates which methods a hook provides. The default is none - this method
// should be overridden by the embedding hook.
func (h *HookBase) Provides(b byte) bool {
	return false
}

// Init performs any pre-start initializations for the hook, such as connecting to databases
// or opening files.
func (h *HookBase) Init(config any) error {
	return nil
}

// SetOpts is called by the server to propagate internal values and generally should
// not be called manually.
func (h *HookBase) SetOpts(l *slog.Logger, opts *HookOptions) {
	h.Log = l
	h.Opts = opts
}

// Stop is called to gracefully shut down the hook.
func (h *HookBase) Stop() error {
	return nil
}

// OnStarted is called when the server starts.
func (h *HookBase) OnStarted() {}

// OnStopped is called when the server stops.
func (h *HookBase) OnStopped() {}

// OnSysInfoTick is called when the server publishes system info.
func (h *HookBase) OnSysInfoTick(*system.Info) {}

// OnConnectAuthenticate is called when a user attempts to authenticate with the server.
func (h *HookBase) OnConnectAuthenticate(cl *Client, pk packets.Packet) bool {
	return false
}

// OnACLCheck is called when a user attempts to subscribe or publish to a topic.
func (h *HookBase) OnACLCheck(cl *Client, topic string, write bool) bool {
	return false
}

// OnConnect is called when a new client connects.
func (h *HookBase) OnConnect(cl *Client, pk packets.Packet) error {
	return nil
}

// OnSessionEstablish is called right after a new client connects and authenticates and right before
// the session is established and CONNACK is sent.
func (h *HookBase) OnSessionEstablish(cl *Client, pk packets.Packet) {}

// OnSessionEstablished is called when a new client establishes a session (after OnConnect).
func (h *HookBase) OnSessionEstablished(cl *Client, pk packets.Packet) {}

// OnDisconnect is called when a client is disconnected for any reason.
func (h *HookBase) OnDisconnect(cl *Client, err error, expire bool) {}

// OnAuthPacket is called when an auth packet is received from the client.
func (h *HookBase) OnAuthPacket(cl *Client, pk packets.Packet) (packets.Packet, error) {
	return pk, nil
}

// OnPacketRead is called when a packet is received.
func (h *HookBase) OnPacketRead(cl *Client, pk packets.Packet) (packets.Packet, error) {
	return pk, nil
}

// OnPacketEncode is called before a packet is byte-encoded and written to the client.
func (h *HookBase) OnPacketEncode(cl *Client, pk packets.Packet) packets.Packet {
	return pk
}

// OnPacketSent is called immediately after a packet is written to a client.
func (h *HookBase) OnPacketSent(cl *Client, pk packets.Packet, b []byte) {}

// OnPacketProcessed is called immediately after a packet from a client is processed.
func (h *HookBase) OnPacketProcessed(cl *Client, pk packets.Packet, err error) {}

// OnSubscribe is called when a client subscribes to one or more filters.
func (h *HookBase) OnSubscribe(cl *Client, pk packets.Packet) packets.Packet {
	return pk
}

// OnSubscribed is called when a client subscribes to one or more filters.
func (h *HookBase) OnSubscribed(cl *Client, pk packets.Packet, reasonCodes []byte) {}

// OnSelectSubscribers is called when selecting subscribers to receive a message.
func (h *HookBase) OnSelectSubscribers(subs *Subscribers, pk packets.Packet) *Subscribers {
	return subs
}

// OnUnsubscribe is called when a client unsubscribes from one or more filters.
func (h *HookBase) OnUnsubscribe(cl *Client, pk packets.Packet) packets.Packet {
	return pk
}

// OnUnsubscribed is called when a client unsubscribes from one or more filters.
func (h *HookBase) OnUnsubscribed(cl *Client, pk packets.Packet) {}

// OnPublish is called when a client publishes a message.
func (h *HookBase) OnPublish(cl *Client, pk packets.Packet) (packets.Packet, error) {
	return pk, nil
}

// OnPublished is called when a client has published a message to subscribers.
func (h *HookBase) OnPublished(cl *Client, pk packets.Packet) {}

// OnPublishDropped is called when a message to a client is dropped instead of being delivered.
func (h *HookBase) OnPublishDropped(cl *Client, pk packets.Packet) {}

// OnRetainMessage is called then a published message is retained.
func (h *HookBase) OnRetainMessage(cl *Client, pk packets.Packet, r int64) {}

// OnRetainPublished is called when a retained message is published.
func (h *HookBase) OnRetainPublished(cl *Client, pk packets.Packet) {}

// OnQosPublish is called when a publish packet with Qos > 1 is issued to a subscriber.
func (h *HookBase) OnQosPublish(cl *Client, pk packets.Packet, sent int64, resends int) {}

// OnQosComplete is called when the Qos flow for a message has been completed.
func (h *HookBase) OnQosComplete(cl *Client, pk packets.Packet) {}

// OnQosDropped is called the Qos flow for a message expires.
func (h *HookBase) OnQosDropped(cl *Client, pk packets.Packet) {}

// OnPacketIDExhausted is called when the client runs out of unused packet ids to assign to a packet.
func (h *HookBase) OnPacketIDExhausted(cl *Client, pk packets.Packet) {}

// OnWill is called when a client disconnects and publishes an LWT message.
func (h *HookBase) OnWill(cl *Client, will Will) (Will, error) {
	return will, nil
}

// OnWillSent is called when an LWT message has been issued from a disconnecting client.
func (h *HookBase) OnWillSent(cl *Client, pk packets.Packet) {}

// OnClientExpired is called when a client session has expired.
func (h *HookBase) OnClientExpired(cl *Client) {}

// OnRetainedExpired is called when a retained message for a topic has expired.
func (h *HookBase) OnRetainedExpired(topic string) {}

// StoredClients returns all clients from a store.
func (h *HookBase) StoredClients() (v []storage.Client, err error) {
	return
}

// StoredSubscriptions returns all subcriptions from a store.
func (h *HookBase) StoredSubscriptions() (v []storage.Subscription, err error) {
	return
}

// StoredInflightMessages returns all inflight messages from a store.
func (h *HookBase) StoredInflightMessages() (v []storage.Message, err error) {
	return
}

// StoredRetainedMessages returns all retained messages from a store.
func (h *HookBase) StoredRetainedMessages() (v []storage.Message, err error) {
	return
}

// StoredSysInfo returns a set of system info values.
func (h *HookBase) StoredSysInfo() (v storage.SystemInfo, err error) {
	return
}
