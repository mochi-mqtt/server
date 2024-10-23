// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co, werbenhu

// Package bolt is provided for historical compatibility and may not be actively updated, you should use the badger hook instead.
package bolt

import (
	"bytes"
	"errors"
	"time"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/storage"
	"github.com/mochi-mqtt/server/v2/packets"
	"github.com/mochi-mqtt/server/v2/system"
	"go.etcd.io/bbolt"
)

var (
	ErrBucketNotFound = errors.New("bucket not found")
	ErrKeyNotFound    = errors.New("key not found")
)

const (
	// defaultDbFile is the default file path for the boltdb file.
	defaultDbFile = ".bolt"

	// defaultTimeout is the default time to hold a connection to the file.
	defaultTimeout = 250 * time.Millisecond

	defaultBucket = "mochi"
)

// clientKey returns a primary key for a client.
func clientKey(cl *mqtt.Client) string {
	return storage.ClientKey + "_" + cl.ID
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
	Bucket  string `yaml:"bucket" json:"bucket"`
	Path    string `yaml:"path" json:"path"`
}

// Hook is a persistent storage hook based using boltdb file store as a backend.
type Hook struct {
	mqtt.HookBase
	config *Options  // options for configuring the boltdb instance.
	db     *bbolt.DB // the boltdb instance.
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
	if len(h.config.Path) == 0 {
		h.config.Path = defaultDbFile
	}

	if len(h.config.Bucket) == 0 {
		h.config.Bucket = defaultBucket
	}

	var err error
	h.db, err = bbolt.Open(h.config.Path, 0600, h.config.Options)
	if err != nil {
		return err
	}

	err = h.db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(h.config.Bucket))
		return err
	})
	return err
}

// Stop closes the boltdb instance.
func (h *Hook) Stop() error {
	err := h.db.Close()
	h.db = nil
	return err
}

// OnSessionEstablished adds a client to the store when their session is established.
func (h *Hook) OnSessionEstablished(cl *mqtt.Client, pk packets.Packet) {
	h.updateClient(cl)
}

// OnWillSent is called when a client sends a Will Message and the Will Message is removed from the client record.
func (h *Hook) OnWillSent(cl *mqtt.Client, pk packets.Packet) {
	h.updateClient(cl)
}

// updateClient writes the client data to the store.
func (h *Hook) updateClient(cl *mqtt.Client) {
	if h.db == nil {
		h.Log.Error("", "error", storage.ErrDBFileNotOpen)
		return
	}

	props := cl.Properties.Props.Copy(false)
	in := &storage.Client{
		ID:              cl.ID,
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

	_ = h.setKv(clientKey(cl), in)
}

// OnDisconnect removes a client from the store if they were using a clean session.
func (h *Hook) OnDisconnect(cl *mqtt.Client, _ error, expire bool) {
	if h.db == nil {
		h.Log.Error("", "error", storage.ErrDBFileNotOpen)
		return
	}

	if !expire {
		return
	}

	if cl.StopCause() == packets.ErrSessionTakenOver {
		return
	}

	_ = h.delKv(clientKey(cl))
}

// OnSubscribed adds one or more client subscriptions to the store.
func (h *Hook) OnSubscribed(cl *mqtt.Client, pk packets.Packet, reasonCodes []byte) {
	if h.db == nil {
		h.Log.Error("", "error", storage.ErrDBFileNotOpen)
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
		_ = h.setKv(in.ID, in)
	}
}

// OnUnsubscribed removes one or more client subscriptions from the store.
func (h *Hook) OnUnsubscribed(cl *mqtt.Client, pk packets.Packet) {
	if h.db == nil {
		h.Log.Error("", "error", storage.ErrDBFileNotOpen)
		return
	}

	for i := 0; i < len(pk.Filters); i++ {
		_ = h.delKv(subscriptionKey(cl, pk.Filters[i].Filter))
	}
}

// OnRetainMessage adds a retained message for a topic to the store.
func (h *Hook) OnRetainMessage(cl *mqtt.Client, pk packets.Packet, r int64) {
	if h.db == nil {
		h.Log.Error("", "error", storage.ErrDBFileNotOpen)
		return
	}

	if r == -1 {
		_ = h.delKv(retainedKey(pk.TopicName))
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
		Client:      cl.ID,
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

	_ = h.setKv(in.ID, in)
}

// OnQosPublish adds or updates an inflight message in the store.
func (h *Hook) OnQosPublish(cl *mqtt.Client, pk packets.Packet, sent int64, resends int) {
	if h.db == nil {
		h.Log.Error("", "error", storage.ErrDBFileNotOpen)
		return
	}

	props := pk.Properties.Copy(false)
	in := &storage.Message{
		ID:          inflightKey(cl, pk),
		T:           storage.InflightKey,
		Client:      cl.ID,
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

	_ = h.setKv(in.ID, in)
}

// OnQosComplete removes a resolved inflight message from the store.
func (h *Hook) OnQosComplete(cl *mqtt.Client, pk packets.Packet) {
	if h.db == nil {
		h.Log.Error("", "error", storage.ErrDBFileNotOpen)
		return
	}

	_ = h.delKv(inflightKey(cl, pk))
}

// OnQosDropped removes a dropped inflight message from the store.
func (h *Hook) OnQosDropped(cl *mqtt.Client, pk packets.Packet) {
	if h.db == nil {
		h.Log.Error("", "error", storage.ErrDBFileNotOpen)
	}

	h.OnQosComplete(cl, pk)
}

// OnSysInfoTick stores the latest system info in the store.
func (h *Hook) OnSysInfoTick(sys *system.Info) {
	if h.db == nil {
		h.Log.Error("", "error", storage.ErrDBFileNotOpen)
		return
	}

	in := &storage.SystemInfo{
		ID:   sysInfoKey(),
		T:    storage.SysInfoKey,
		Info: *sys,
	}

	_ = h.setKv(in.ID, in)
}

// OnRetainedExpired deletes expired retained messages from the store.
func (h *Hook) OnRetainedExpired(filter string) {
	if h.db == nil {
		h.Log.Error("", "error", storage.ErrDBFileNotOpen)
		return
	}
	_ = h.delKv(retainedKey(filter))
}

// OnClientExpired deleted expired clients from the store.
func (h *Hook) OnClientExpired(cl *mqtt.Client) {
	if h.db == nil {
		h.Log.Error("", "error", storage.ErrDBFileNotOpen)
		return
	}
	_ = h.delKv(clientKey(cl))
}

// StoredClients returns all stored clients from the store.
func (h *Hook) StoredClients() (v []storage.Client, err error) {
	if h.db == nil {
		h.Log.Error("", "error", storage.ErrDBFileNotOpen)
		return v, storage.ErrDBFileNotOpen
	}

	err = h.iterKv(storage.ClientKey, func(value []byte) error {
		obj := storage.Client{}
		err = obj.UnmarshalBinary(value)
		if err == nil {
			v = append(v, obj)
		}
		return err
	})
	return v, nil
}

// StoredSubscriptions returns all stored subscriptions from the store.
func (h *Hook) StoredSubscriptions() (v []storage.Subscription, err error) {
	if h.db == nil {
		h.Log.Error("", "error", storage.ErrDBFileNotOpen)
		return v, storage.ErrDBFileNotOpen
	}

	v = make([]storage.Subscription, 0)
	err = h.iterKv(storage.SubscriptionKey, func(value []byte) error {
		obj := storage.Subscription{}
		err = obj.UnmarshalBinary(value)
		if err == nil {
			v = append(v, obj)
		}
		return err
	})
	return
}

// StoredRetainedMessages returns all stored retained messages from the store.
func (h *Hook) StoredRetainedMessages() (v []storage.Message, err error) {
	if h.db == nil {
		h.Log.Error("", "error", storage.ErrDBFileNotOpen)
		return v, storage.ErrDBFileNotOpen
	}

	v = make([]storage.Message, 0)
	err = h.iterKv(storage.RetainedKey, func(value []byte) error {
		obj := storage.Message{}
		err = obj.UnmarshalBinary(value)
		if err == nil {
			v = append(v, obj)
		}
		return err
	})
	return
}

// StoredInflightMessages returns all stored inflight messages from the store.
func (h *Hook) StoredInflightMessages() (v []storage.Message, err error) {
	if h.db == nil {
		h.Log.Error("", "error", storage.ErrDBFileNotOpen)
		return v, storage.ErrDBFileNotOpen
	}

	v = make([]storage.Message, 0)
	err = h.iterKv(storage.InflightKey, func(value []byte) error {
		obj := storage.Message{}
		err = obj.UnmarshalBinary(value)
		if err == nil {
			v = append(v, obj)
		}
		return err
	})
	return
}

// StoredSysInfo returns the system info from the store.
func (h *Hook) StoredSysInfo() (v storage.SystemInfo, err error) {
	if h.db == nil {
		h.Log.Error("", "error", storage.ErrDBFileNotOpen)
		return v, storage.ErrDBFileNotOpen
	}

	err = h.getKv(storage.SysInfoKey, &v)
	if err != nil && !errors.Is(err, ErrKeyNotFound) {
		return
	}

	return v, nil
}

// setKv stores a key-value pair in the database.
func (h *Hook) setKv(k string, v storage.Serializable) error {
	err := h.db.Update(func(tx *bbolt.Tx) error {

		bucket := tx.Bucket([]byte(h.config.Bucket))
		data, _ := v.MarshalBinary()
		err := bucket.Put([]byte(k), data)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		h.Log.Error("failed to upsert data", "error", err, "key", k)
	}
	return err
}

// delKv deletes a key-value pair from the database.
func (h *Hook) delKv(k string) error {
	err := h.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(h.config.Bucket))
		err := bucket.Delete([]byte(k))
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		h.Log.Error("failed to delete data", "error", err, "key", k)
	}
	return err
}

// getKv retrieves the value associated with a key from the database.
func (h *Hook) getKv(k string, v storage.Serializable) error {
	err := h.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(h.config.Bucket))

		value := bucket.Get([]byte(k))
		if value == nil {
			return ErrKeyNotFound
		}

		return v.UnmarshalBinary(value)
	})
	if err != nil {
		h.Log.Error("failed to get data", "error", err, "key", k)
	}
	return err
}

// iterKv iterates over key-value pairs with keys having the specified prefix in the database.
func (h *Hook) iterKv(prefix string, visit func([]byte) error) error {
	err := h.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(h.config.Bucket))

		c := bucket.Cursor()
		for k, v := c.Seek([]byte(prefix)); k != nil && string(k[:len(prefix)]) == prefix; k, v = c.Next() {
			if err := visit(v); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		h.Log.Error("failed to iter data", "error", err, "prefix", prefix)
	}
	return err
}
