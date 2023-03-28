// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-co
// SPDX-FileContributor: mochi-co
// package bolt is provided for historical compatibility and may not be actively updated, you should use the badger hook instead.
package bolt

import (
	"bytes"
	"errors"
	"time"

	"github.com/mochi-co/mqtt/v2"
	"github.com/mochi-co/mqtt/v2/hooks/storage"
	"github.com/mochi-co/mqtt/v2/packets"
	"github.com/mochi-co/mqtt/v2/system"

	sgob "github.com/asdine/storm/codec/gob"
	"github.com/asdine/storm/v3"
	"go.etcd.io/bbolt"
)

const (
	// defaultDbFile is the default file path for the boltdb file.
	defaultDbFile = "bolt.db"

	// defaultTimeout is the default time to hold a connection to the file.
	defaultTimeout = 250 * time.Millisecond
)

// clientKey returns a primary key for a client.
func clientKey(cl *mqtt.Client) string {
	return cl.ID
}

// subscriptionKey returns a primary key for a subscription.
func subscriptionKey(cl *mqtt.Client, filter string) string {
	return storage.SubscriptionKey + "_" + cl.ID + ":" + filter
}

// retainedKey returns a primary key for a retained message.
func retainedKey(topic string) string {
	return storage.RetainedKey + "_" + topic
}

// inflightKey returns a primary key for an inflight message.
func inflightKey(cl *mqtt.Client, pk packets.Packet) string {
	return storage.InflightKey + "_" + cl.ID + ":" + pk.FormatID()
}

// sysInfoKey returns a primary key for system info.
func sysInfoKey() string {
	return storage.SysInfoKey
}

// Options contains configuration settings for the bolt instance.
type Options struct {
	Options *bbolt.Options
	Path    string
}

// Hook is a persistent storage hook based using boltdb file store as a backend.
type Hook struct {
	mqtt.HookBase
	config *Options  // options for configuring the boltdb instance.
	db     *storm.DB // the boltdb instance.
}

// ID returns the id of the hook.
func (h *Hook) ID() string {
	return "bolt-db"
}

// Provides indicates which hook methods this hook provides.
func (h *Hook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqtt.OnSessionEstablished,
		mqtt.OnDisconnect,
		mqtt.OnSubscribed,
		mqtt.OnUnsubscribed,
		mqtt.OnRetainMessage,
		mqtt.OnWillSent,
		mqtt.OnQosPublish,
		mqtt.OnQosComplete,
		mqtt.OnQosDropped,
		mqtt.OnSysInfoTick,
		mqtt.OnClientExpired,
		mqtt.OnRetainedExpired,
		mqtt.StoredClients,
		mqtt.StoredInflightMessages,
		mqtt.StoredRetainedMessages,
		mqtt.StoredSubscriptions,
		mqtt.StoredSysInfo,
	}, []byte{b})
}

// Init initializes and connects to the boltdb instance.
func (h *Hook) Init(config any) error {
	if _, ok := config.(*Options); !ok && config != nil {
		return mqtt.ErrInvalidConfigType
	}

	if config == nil {
		config = new(Options)
	}

	h.config = config.(*Options)
	if h.config.Options == nil {
		h.config.Options = &bbolt.Options{
			Timeout: defaultTimeout,
		}
	}
	if h.config.Path == "" {
		h.config.Path = defaultDbFile
	}

	var err error
	h.db, err = storm.Open(h.config.Path, storm.BoltOptions(0600, h.config.Options), storm.Codec(sgob.Codec))
	if err != nil {
		return err
	}

	return nil
}

// Stop closes the boltdb instance.
func (h *Hook) Stop() error {
	return h.db.Close()
}

// OnSessionEstablished adds a client to the store when their session is established.
func (h *Hook) OnSessionEstablished(cl *mqtt.Client, pk packets.Packet) {
	h.updateClient(cl)
}

// OnWillSent is called when a client sends a will message and the will message is removed
// from the client record.
func (h *Hook) OnWillSent(cl *mqtt.Client, pk packets.Packet) {
	h.updateClient(cl)
}

// updateClient writes the client data to the store.
func (h *Hook) updateClient(cl *mqtt.Client) {
	if h.db == nil {
		h.Log.Error().Err(storage.ErrDBFileNotOpen)
		return
	}

	props := cl.Properties.Props.Copy(false)
	in := &storage.Client{
		ID:              clientKey(cl),
		T:               storage.ClientKey,
		Remote:          cl.Net.Remote,
		Listener:        cl.Net.Listener,
		Username:        cl.Properties.Username,
		Clean:           cl.Properties.Clean,
		ProtocolVersion: cl.Properties.ProtocolVersion,
		Properties: storage.ClientProperties{
			SessionExpiryInterval: props.SessionExpiryInterval,
			AuthenticationMethod:  props.AuthenticationMethod,
			AuthenticationData:    props.AuthenticationData,
			RequestProblemInfo:    props.RequestProblemInfo,
			RequestResponseInfo:   props.RequestResponseInfo,
			ReceiveMaximum:        props.ReceiveMaximum,
			TopicAliasMaximum:     props.TopicAliasMaximum,
			User:                  props.User,
			MaximumPacketSize:     props.MaximumPacketSize,
		},
		Will: storage.ClientWill(cl.Properties.Will),
	}
	err := h.db.Save(in)
	if err != nil {
		h.Log.Error().Err(err).Interface("data", in).Msg("failed to save client data")
	}
}

// OnDisconnect removes a client from the store if they were using a clean session.
func (h *Hook) OnDisconnect(cl *mqtt.Client, _ error, expire bool) {
	if h.db == nil {
		h.Log.Error().Err(storage.ErrDBFileNotOpen)
		return
	}

	if !expire {
		return
	}

	if cl.StopCause() == packets.ErrSessionTakenOver {
		return
	}

	err := h.db.DeleteStruct(&storage.Client{ID: clientKey(cl)})
	if err != nil && !errors.Is(err, storm.ErrNotFound) {
		h.Log.Error().Err(err).Str("id", clientKey(cl)).Msg("failed to delete client")
	}
}

// OnSubscribed adds one or more client subscriptions to the store.
func (h *Hook) OnSubscribed(cl *mqtt.Client, pk packets.Packet, reasonCodes []byte) {
	if h.db == nil {
		h.Log.Error().Err(storage.ErrDBFileNotOpen)
		return
	}

	var in *storage.Subscription
	for i := 0; i < len(pk.Filters); i++ {
		in = &storage.Subscription{
			ID:                subscriptionKey(cl, pk.Filters[i].Filter),
			T:                 storage.SubscriptionKey,
			Client:            cl.ID,
			Qos:               reasonCodes[i],
			Filter:            pk.Filters[i].Filter,
			Identifier:        pk.Filters[i].Identifier,
			NoLocal:           pk.Filters[i].NoLocal,
			RetainHandling:    pk.Filters[i].RetainHandling,
			RetainAsPublished: pk.Filters[i].RetainAsPublished,
		}

		err := h.db.Save(in)
		if err != nil {
			h.Log.Error().Err(err).
				Str("client", cl.ID).
				Interface("data", in).
				Msg("failed to save subscription data")
		}
	}
}

// OnUnsubscribed removes one or more client subscriptions from the store.
func (h *Hook) OnUnsubscribed(cl *mqtt.Client, pk packets.Packet) {
	if h.db == nil {
		h.Log.Error().Err(storage.ErrDBFileNotOpen)
		return
	}

	for i := 0; i < len(pk.Filters); i++ {
		err := h.db.DeleteStruct(&storage.Subscription{
			ID: subscriptionKey(cl, pk.Filters[i].Filter),
		})
		if err != nil {
			h.Log.Error().Err(err).
				Str("id", subscriptionKey(cl, pk.Filters[i].Filter)).
				Msg("failed to delete client")
		}
	}
}

// OnRetainMessage adds a retained message for a topic to the store.
func (h *Hook) OnRetainMessage(cl *mqtt.Client, pk packets.Packet, r int64) {
	if h.db == nil {
		h.Log.Error().Err(storage.ErrDBFileNotOpen)
		return
	}

	if r == -1 {
		err := h.db.DeleteStruct(&storage.Message{
			ID: retainedKey(pk.TopicName),
		})
		if err != nil {
			h.Log.Error().Err(err).
				Str("id", retainedKey(pk.TopicName)).
				Msg("failed to delete retained publish")
		}
		return
	}

	props := pk.Properties.Copy(false)
	in := &storage.Message{
		ID:          retainedKey(pk.TopicName),
		T:           storage.RetainedKey,
		FixedHeader: pk.FixedHeader,
		TopicName:   pk.TopicName,
		Payload:     pk.Payload,
		Created:     pk.Created,
		Origin:      pk.Origin,
		Properties: storage.MessageProperties{
			PayloadFormat:          props.PayloadFormat,
			MessageExpiryInterval:  props.MessageExpiryInterval,
			ContentType:            props.ContentType,
			ResponseTopic:          props.ResponseTopic,
			CorrelationData:        props.CorrelationData,
			SubscriptionIdentifier: props.SubscriptionIdentifier,
			TopicAlias:             props.TopicAlias,
			User:                   props.User,
		},
	}
	err := h.db.Save(in)
	if err != nil {
		h.Log.Error().Err(err).
			Str("client", cl.ID).
			Interface("data", in).
			Msg("failed to save retained publish data")
	}
}

// OnQosPublish adds or updates an inflight message in the store.
func (h *Hook) OnQosPublish(cl *mqtt.Client, pk packets.Packet, sent int64, resends int) {
	if h.db == nil {
		h.Log.Error().Err(storage.ErrDBFileNotOpen)
		return
	}

	props := pk.Properties.Copy(false)
	in := &storage.Message{
		ID:          inflightKey(cl, pk),
		T:           storage.InflightKey,
		Origin:      pk.Origin,
		FixedHeader: pk.FixedHeader,
		TopicName:   pk.TopicName,
		Payload:     pk.Payload,
		Sent:        sent,
		Created:     pk.Created,
		Properties: storage.MessageProperties{
			PayloadFormat:          props.PayloadFormat,
			MessageExpiryInterval:  props.MessageExpiryInterval,
			ContentType:            props.ContentType,
			ResponseTopic:          props.ResponseTopic,
			CorrelationData:        props.CorrelationData,
			SubscriptionIdentifier: props.SubscriptionIdentifier,
			TopicAlias:             props.TopicAlias,
			User:                   props.User,
		},
	}

	err := h.db.Save(in)
	if err != nil {
		h.Log.Error().Err(err).
			Str("client", cl.ID).
			Interface("data", in).
			Msg("failed to save qos inflight data")
	}
}

// OnQosComplete removes a resolved inflight message from the store.
func (h *Hook) OnQosComplete(cl *mqtt.Client, pk packets.Packet) {
	if h.db == nil {
		h.Log.Error().Err(storage.ErrDBFileNotOpen)
		return
	}

	err := h.db.DeleteStruct(&storage.Message{
		ID: inflightKey(cl, pk),
	})
	if err != nil {
		h.Log.Error().Err(err).
			Str("id", inflightKey(cl, pk)).
			Msg("failed to delete inflight data")
	}
}

// OnQosDropped removes a dropped inflight message from the store.
func (h *Hook) OnQosDropped(cl *mqtt.Client, pk packets.Packet) {
	if h.db == nil {
		h.Log.Error().Err(storage.ErrDBFileNotOpen)
	}

	h.OnQosComplete(cl, pk)
}

// OnSysInfoTick stores the latest system info in the store.
func (h *Hook) OnSysInfoTick(sys *system.Info) {
	if h.db == nil {
		h.Log.Error().Err(storage.ErrDBFileNotOpen)
		return
	}

	in := &storage.SystemInfo{
		ID:   sysInfoKey(),
		T:    storage.SysInfoKey,
		Info: *sys,
	}

	err := h.db.Save(in)
	if err != nil {
		h.Log.Error().Err(err).
			Interface("data", in).
			Msg("failed to save $SYS data")
	}
}

// OnRetainedExpired deletes expired retained messages from the store.
func (h *Hook) OnRetainedExpired(filter string) {
	if h.db == nil {
		h.Log.Error().Err(storage.ErrDBFileNotOpen)
		return
	}

	if err := h.db.DeleteStruct(&storage.Message{ID: retainedKey(filter)}); err != nil {
		h.Log.Error().Err(err).Str("id", retainedKey(filter)).Msg("failed to delete retained publish")
	}
}

// OnClientExpired deleted expired clients from the store.
func (h *Hook) OnClientExpired(cl *mqtt.Client) {
	if h.db == nil {
		h.Log.Error().Err(storage.ErrDBFileNotOpen)
		return
	}

	err := h.db.DeleteStruct(&storage.Client{ID: clientKey(cl)})
	if err != nil && !errors.Is(err, storm.ErrNotFound) {
		h.Log.Error().Err(err).Str("id", clientKey(cl)).Msg("failed to delete expired client")
	}
}

// StoredClients returns all stored clients from the store.
func (h *Hook) StoredClients() (v []storage.Client, err error) {
	if h.db == nil {
		h.Log.Error().Err(storage.ErrDBFileNotOpen)
		return
	}

	err = h.db.Find("T", storage.ClientKey, &v)
	if err != nil && !errors.Is(err, storm.ErrNotFound) {
		return
	}

	return v, nil
}

// StoredSubscriptions returns all stored subscriptions from the store.
func (h *Hook) StoredSubscriptions() (v []storage.Subscription, err error) {
	if h.db == nil {
		h.Log.Error().Err(storage.ErrDBFileNotOpen)
		return
	}

	err = h.db.Find("T", storage.SubscriptionKey, &v)
	if err != nil && !errors.Is(err, storm.ErrNotFound) {
		return
	}

	return v, nil
}

// StoredRetainedMessages returns all stored retained messages from the store.
func (h *Hook) StoredRetainedMessages() (v []storage.Message, err error) {
	if h.db == nil {
		h.Log.Error().Err(storage.ErrDBFileNotOpen)
		return
	}

	err = h.db.Find("T", storage.RetainedKey, &v)
	if err != nil && !errors.Is(err, storm.ErrNotFound) {
		return
	}

	return v, nil
}

// StoredInflightMessages returns all stored inflight messages from the store.
func (h *Hook) StoredInflightMessages() (v []storage.Message, err error) {
	if h.db == nil {
		h.Log.Error().Err(storage.ErrDBFileNotOpen)
		return
	}

	err = h.db.Find("T", storage.InflightKey, &v)
	if err != nil && !errors.Is(err, storm.ErrNotFound) {
		return
	}

	return v, nil
}

// StoredSysInfo returns the system info from the store.
func (h *Hook) StoredSysInfo() (v storage.SystemInfo, err error) {
	if h.db == nil {
		h.Log.Error().Err(storage.ErrDBFileNotOpen)
		return
	}

	err = h.db.One("ID", storage.SysInfoKey, &v)
	if err != nil && !errors.Is(err, storm.ErrNotFound) {
		return
	}

	return v, nil
}
