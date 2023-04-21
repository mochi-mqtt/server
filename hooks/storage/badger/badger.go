// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-co
// SPDX-FileContributor: mochi-co

package badger

import (
	"bytes"
	"errors"
	"strings"

	"github.com/mochi-co/mqtt/v2"
	"github.com/mochi-co/mqtt/v2/hooks/storage"
	"github.com/mochi-co/mqtt/v2/packets"
	"github.com/mochi-co/mqtt/v2/system"

	"github.com/timshannon/badgerhold"
)

const (
	// defaultDbFile is the default file path for the badger db file.
	defaultDbFile = ".badger"
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

// Options contains configuration settings for the BadgerDB instance.
type Options struct {
	Options *badgerhold.Options
	Path    string
}

// Hook is a persistent storage hook based using BadgerDB file store as a backend.
type Hook struct {
	mqtt.HookBase
	config *Options          // options for configuring the BadgerDB instance.
	db     *badgerhold.Store // the BadgerDB instance.
}

// ID returns the id of the hook.
func (h *Hook) ID() string {
	return "badger-db"
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

// Init initializes and connects to the badger instance.
func (h *Hook) Init(config any) error {
	if _, ok := config.(*Options); !ok && config != nil {
		return mqtt.ErrInvalidConfigType
	}

	if config == nil {
		config = new(Options)
	}

	h.config = config.(*Options)
	if h.config.Path == "" {
		h.config.Path = defaultDbFile
	}

	options := badgerhold.DefaultOptions
	options.Dir = h.config.Path
	options.ValueDir = h.config.Path
	options.Logger = h

	var err error
	h.db, err = badgerhold.Open(options)
	if err != nil {
		return err
	}

	return nil
}

// Stop closes the badger instance.
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

	err := h.db.Upsert(in.ID, in)
	if err != nil {
		h.Log.Error().Err(err).Interface("data", in).Msg("failed to upsert client data")
	}
}

// OnDisconnect removes a client from the store if their session has expired.
func (h *Hook) OnDisconnect(cl *mqtt.Client, _ error, expire bool) {
	if h.db == nil {
		h.Log.Error().Err(storage.ErrDBFileNotOpen)
		return
	}

	h.updateClient(cl)

	if !expire {
		return
	}

	if cl.StopCause() == packets.ErrSessionTakenOver {
		return
	}

	err := h.db.Delete(clientKey(cl), new(storage.Client))
	if err != nil {
		h.Log.Error().Err(err).Interface("data", clientKey(cl)).Msg("failed to delete client data")
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

		err := h.db.Upsert(in.ID, in)
		if err != nil {
			h.Log.Error().Err(err).Interface("data", in).Msg("failed to upsert subscription data")
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
		err := h.db.Delete(subscriptionKey(cl, pk.Filters[i].Filter), new(storage.Subscription))
		if err != nil {
			h.Log.Error().Err(err).Interface("data", subscriptionKey(cl, pk.Filters[i].Filter)).Msg("failed to delete subscription data")
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
		err := h.db.Delete(retainedKey(pk.TopicName), new(storage.Message))
		if err != nil {
			h.Log.Error().Err(err).Interface("data", retainedKey(pk.TopicName)).Msg("failed to delete retained message data")
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

	err := h.db.Upsert(in.ID, in)
	if err != nil {
		h.Log.Error().Err(err).Interface("data", in).Msg("failed to upsert retained message data")
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
		PacketID:    pk.PacketID,
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

	err := h.db.Upsert(in.ID, in)
	if err != nil {
		h.Log.Error().Err(err).Interface("data", in).Msg("failed to upsert qos inflight data")
	}
}

// OnQosComplete removes a resolved inflight message from the store.
func (h *Hook) OnQosComplete(cl *mqtt.Client, pk packets.Packet) {
	if h.db == nil {
		h.Log.Error().Err(storage.ErrDBFileNotOpen)
		return
	}

	err := h.db.Delete(inflightKey(cl, pk), new(storage.Message))
	if err != nil {
		h.Log.Error().Err(err).Interface("data", inflightKey(cl, pk)).Msg("failed to delete inflight message data")
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

	err := h.db.Upsert(in.ID, in)
	if err != nil {
		h.Log.Error().Err(err).Interface("data", in).Msg("failed to upsert $SYS data")
	}
}

// OnRetainedExpired deletes expired retained messages from the store.
func (h *Hook) OnRetainedExpired(filter string) {
	if h.db == nil {
		h.Log.Error().Err(storage.ErrDBFileNotOpen)
		return
	}

	err := h.db.Delete(retainedKey(filter), new(storage.Message))
	if err != nil {
		h.Log.Error().Err(err).Str("id", retainedKey(filter)).Msg("failed to delete expired retained message data")
	}
}

// OnClientExpired deleted expired clients from the store.
func (h *Hook) OnClientExpired(cl *mqtt.Client) {
	if h.db == nil {
		h.Log.Error().Err(storage.ErrDBFileNotOpen)
		return
	}

	err := h.db.Delete(clientKey(cl), new(storage.Client))
	if err != nil {
		h.Log.Error().Err(err).Str("id", clientKey(cl)).Msg("failed to delete expired client data")
	}
}

// StoredClients returns all stored clients from the store.
func (h *Hook) StoredClients() (v []storage.Client, err error) {
	if h.db == nil {
		h.Log.Error().Err(storage.ErrDBFileNotOpen)
		return
	}

	err = h.db.Find(&v, badgerhold.Where("T").Eq(storage.ClientKey))
	if err != nil && !errors.Is(err, badgerhold.ErrNotFound) {
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

	err = h.db.Find(&v, badgerhold.Where("T").Eq(storage.SubscriptionKey))
	if err != nil && !errors.Is(err, badgerhold.ErrNotFound) {
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

	err = h.db.Find(&v, badgerhold.Where("T").Eq(storage.RetainedKey))
	if err != nil && !errors.Is(err, badgerhold.ErrNotFound) {
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

	err = h.db.Find(&v, badgerhold.Where("T").Eq(storage.InflightKey))
	if err != nil && !errors.Is(err, badgerhold.ErrNotFound) {
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

	err = h.db.Get(storage.SysInfoKey, &v)
	if err != nil && !errors.Is(err, badgerhold.ErrNotFound) {
		return
	}

	return v, nil
}

// Errorf satisfies the badger interface for an error logger.
func (h *Hook) Errorf(m string, v ...interface{}) {
	h.Log.Error().Interface("v", v).Msgf(strings.ToLower(strings.Trim(m, "\n")), v...)
}

// Warningf satisfies the badger interface for a warning logger.
func (h *Hook) Warningf(m string, v ...interface{}) {
	h.Log.Warn().Interface("v", v).Msgf(strings.ToLower(strings.Trim(m, "\n")), v...)
}

// Infof satisfies the badger interface for an info logger.
func (h *Hook) Infof(m string, v ...interface{}) {
	h.Log.Info().Interface("v", v).Msgf(strings.ToLower(strings.Trim(m, "\n")), v...)
}

// Debugf satisfies the badger interface for a debug logger.
func (h *Hook) Debugf(m string, v ...interface{}) {
	h.Log.Debug().Interface("v", v).Msgf(strings.ToLower(strings.Trim(m, "\n")), v...)
}
