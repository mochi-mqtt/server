// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co, gsagula

package badger

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"

	badgerdb "github.com/dgraph-io/badger/v4"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/storage"
	"github.com/mochi-mqtt/server/v2/packets"
	"github.com/mochi-mqtt/server/v2/system"
)

const (
	// defaultDbFile is the default file path for the badger db file.
	defaultDbFile         = ".badger"
	defaultGcInterval     = 5 * 60 // gc interval in seconds
	defaultGcDiscardRatio = 0.5
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

// Serializable is an interface for objects that can be serialized and deserialized.
type Serializable interface {
	UnmarshalBinary([]byte) error
	MarshalBinary() (data []byte, err error)
}

// setKv stores a key-value pair in the database.
func setKv[T Serializable](db *badgerdb.DB, key string, obj T) error {
	return db.Update(func(txn *badgerdb.Txn) error {
		data, err := obj.MarshalBinary()
		if err != nil {
			return err
		}
		err = txn.Set([]byte(key), data)
		return err
	})
}

// delKv deletes a key-value pair from the database.
func delKv(db *badgerdb.DB, key string) error {
	return db.Update(func(txn *badgerdb.Txn) error {
		return txn.Delete([]byte(key))
	})
}

// getKv retrieves the value associated with a key from the database.
func getKv[T Serializable](db *badgerdb.DB, key string, obj T) error {
	return db.View(func(txn *badgerdb.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		value, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		if err := obj.UnmarshalBinary(value); err != nil {
			return err
		}
		return nil
	})
}

// Options contains configuration settings for the BadgerDB instance.
type Options struct {
	Options *badgerdb.Options
	Path    string `yaml:"path" json:"path"`
	// GcDiscardRatio specifies the ratio of log discard compared to the maximum possible log discard.
	// Setting it to a higher value would result in fewer space reclaims, while setting it to a lower value
	// would result in more space reclaims at the cost of increased activity on the LSM tree.
	// discardRatio must be in the range (0.0, 1.0), both endpoints excluded, otherwise, it will be set to the default value of 0.5.
	GcDiscardRatio float64 `yaml:"gc_discard_ratio" json:"gc_discard_ratio"`
	GcInterval     int64   `yaml:"gc_interval" json:"gc_interval"`
}

// Hook is a persistent storage hook based using BadgerDB file store as a backend.
type Hook struct {
	mqtt.HookBase
	config   *Options     // options for configuring the BadgerDB instance.
	gcTicker *time.Ticker // Ticker for BadgerDB garbage collection.
	db       *badgerdb.DB // the BadgerDB instance.
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

// GcLoop periodically runs the garbage collection process to reclaim space in the value log files.
// It uses a ticker to trigger the garbage collection at regular intervals specified by the configuration.
// Refer to: https://dgraph.io/docs/badger/get-started/#garbage-collection
func (h *Hook) gcLoop() {
	for range h.gcTicker.C {
	again:
		// Run the garbage collection process with a threshold.
		// If the process returns nil (success), repeat the process.
		err := h.db.RunValueLogGC(h.config.GcDiscardRatio)
		if err == nil {
			goto again // Retry garbage collection if successful.
		}
	}
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

	if h.config.GcInterval == 0 {
		h.config.GcInterval = defaultGcInterval
	}

	if h.config.GcDiscardRatio <= 0.0 || h.config.GcDiscardRatio >= 1.0 {
		h.config.GcDiscardRatio = defaultGcDiscardRatio
	}

	if h.config.Options == nil {
		defaultOpts := badgerdb.DefaultOptions(h.config.Path)
		h.config.Options = &defaultOpts
	}
	h.config.Options.Logger = h

	var err error
	h.db, err = badgerdb.Open(*h.config.Options)
	if err != nil {
		return err
	}

	h.gcTicker = time.NewTicker(time.Duration(h.config.GcInterval) * time.Second)
	go h.gcLoop()

	return nil
}

// Stop closes the badger instance.
func (h *Hook) Stop() error {
	if h.gcTicker != nil {
		h.gcTicker.Stop()
	}
	return h.db.Close()
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

	err := setKv(h.db, in.ID, in)
	if err != nil {
		h.Log.Error("failed to upsert client data", "error", err, "data", in)
	}
}

// OnDisconnect removes a client from the store if their session has expired.
func (h *Hook) OnDisconnect(cl *mqtt.Client, _ error, expire bool) {
	if h.db == nil {
		h.Log.Error("", "error", storage.ErrDBFileNotOpen)
		return
	}

	h.updateClient(cl)

	if !expire {
		return
	}

	if errors.Is(cl.StopCause(), packets.ErrSessionTakenOver) {
		return
	}

	err := delKv(h.db, clientKey(cl))
	if err != nil {
		h.Log.Error("failed to delete client data", "error", err, "data", clientKey(cl))
	}
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

		err := setKv(h.db, in.ID, in)
		if err != nil {
			h.Log.Error("failed to upsert subscription data", "error", err, "data", in)
		}
	}
}

// OnUnsubscribed removes one or more client subscriptions from the store.
func (h *Hook) OnUnsubscribed(cl *mqtt.Client, pk packets.Packet) {
	if h.db == nil {
		h.Log.Error("", "error", storage.ErrDBFileNotOpen)
		return
	}

	for i := 0; i < len(pk.Filters); i++ {
		err := delKv(h.db, subscriptionKey(cl, pk.Filters[i].Filter))
		if err != nil {
			h.Log.Error("failed to delete subscription data", "error", err, "data", subscriptionKey(cl, pk.Filters[i].Filter))
		}
	}
}

// OnRetainMessage adds a retained message for a topic to the store.
func (h *Hook) OnRetainMessage(cl *mqtt.Client, pk packets.Packet, r int64) {
	if h.db == nil {
		h.Log.Error("", "error", storage.ErrDBFileNotOpen)
		return
	}

	if r == -1 {
		err := delKv(h.db, retainedKey(pk.TopicName))
		if err != nil {
			h.Log.Error("failed to delete retained message data", "error", err, "data", retainedKey(pk.TopicName))
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

	err := setKv(h.db, in.ID, in)
	if err != nil {
		h.Log.Error("failed to upsert retained message data", "error", err, "data", in)
	}
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

	err := setKv(h.db, in.ID, in)
	if err != nil {
		h.Log.Error("failed to upsert qos inflight data", "error", err, "data", in)
	}
}

// OnQosComplete removes a resolved inflight message from the store.
func (h *Hook) OnQosComplete(cl *mqtt.Client, pk packets.Packet) {
	if h.db == nil {
		h.Log.Error("", "error", storage.ErrDBFileNotOpen)
		return
	}

	err := delKv(h.db, inflightKey(cl, pk))
	if err != nil {
		h.Log.Error("failed to delete inflight message data", "error", err, "data", inflightKey(cl, pk))
	}
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
		Info: *sys.Clone(),
	}

	err := setKv(h.db, in.ID, in)
	if err != nil {
		h.Log.Error("failed to upsert $SYS data", "error", err, "data", in)
	}
}

// OnRetainedExpired deletes expired retained messages from the store.
func (h *Hook) OnRetainedExpired(filter string) {
	if h.db == nil {
		h.Log.Error("", "error", storage.ErrDBFileNotOpen)
		return
	}

	err := delKv(h.db, retainedKey(filter))
	if err != nil {
		h.Log.Error("failed to delete expired retained message data", "error", err, "id", retainedKey(filter))
	}
}

// OnClientExpired deleted expired clients from the store.
func (h *Hook) OnClientExpired(cl *mqtt.Client) {
	if h.db == nil {
		h.Log.Error("", "error", storage.ErrDBFileNotOpen)
		return
	}

	err := delKv(h.db, clientKey(cl))
	if err != nil {
		h.Log.Error("failed to delete expired client data", "error", err, "id", clientKey(cl))
	}
}

// StoredClients returns all stored clients from the store.
func (h *Hook) StoredClients() (v []storage.Client, err error) {
	if h.db == nil {
		h.Log.Error("", "error", storage.ErrDBFileNotOpen)
		return
	}

	err = h.db.View(func(txn *badgerdb.Txn) error {
		iterator := txn.NewIterator(badgerdb.DefaultIteratorOptions)
		defer iterator.Close()

		prefix := []byte(storage.ClientKey)
		for iterator.Seek(prefix); iterator.ValidForPrefix(prefix); iterator.Next() {
			item := iterator.Item()
			value, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			obj := storage.Client{}
			if err := obj.UnmarshalBinary(value); err != nil {
				return err
			}
			v = append(v, obj)
		}
		return nil
	})

	if err != nil && !errors.Is(err, badgerdb.ErrKeyNotFound) {
		return
	}

	return v, nil
}

// StoredSubscriptions returns all stored subscriptions from the store.
func (h *Hook) StoredSubscriptions() (v []storage.Subscription, err error) {
	if h.db == nil {
		h.Log.Error("", "error", storage.ErrDBFileNotOpen)
		return
	}

	v = make([]storage.Subscription, 0)
	err = h.db.View(func(txn *badgerdb.Txn) error {
		iterator := txn.NewIterator(badgerdb.DefaultIteratorOptions)
		defer iterator.Close()

		prefix := []byte(storage.SubscriptionKey)
		for iterator.Seek(prefix); iterator.ValidForPrefix(prefix); iterator.Next() {
			item := iterator.Item()
			value, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			obj := storage.Subscription{}
			if err := obj.UnmarshalBinary(value); err != nil {
				return err
			}
			v = append(v, obj)
		}
		return nil
	})

	if err != nil && !errors.Is(err, badgerdb.ErrKeyNotFound) {
		return
	}

	return v, nil
}

// StoredRetainedMessages returns all stored retained messages from the store.
func (h *Hook) StoredRetainedMessages() (v []storage.Message, err error) {
	if h.db == nil {
		h.Log.Error("", "error", storage.ErrDBFileNotOpen)
		return
	}

	v = make([]storage.Message, 0)
	err = h.db.View(func(txn *badgerdb.Txn) error {
		iterator := txn.NewIterator(badgerdb.DefaultIteratorOptions)
		defer iterator.Close()

		prefix := []byte(storage.RetainedKey)
		for iterator.Seek(prefix); iterator.ValidForPrefix(prefix); iterator.Next() {
			item := iterator.Item()
			value, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			obj := storage.Message{}
			if err := obj.UnmarshalBinary(value); err != nil {
				return err
			}
			v = append(v, obj)
		}
		return nil
	})

	if err != nil && !errors.Is(err, badgerdb.ErrKeyNotFound) {
		return
	}

	return v, nil
}

// StoredInflightMessages returns all stored inflight messages from the store.
func (h *Hook) StoredInflightMessages() (v []storage.Message, err error) {
	if h.db == nil {
		h.Log.Error("", "error", storage.ErrDBFileNotOpen)
		return
	}

	v = make([]storage.Message, 0)
	err = h.db.View(func(txn *badgerdb.Txn) error {
		iterator := txn.NewIterator(badgerdb.DefaultIteratorOptions)
		defer iterator.Close()

		prefix := []byte(storage.InflightKey)
		for iterator.Seek(prefix); iterator.ValidForPrefix(prefix); iterator.Next() {
			item := iterator.Item()
			value, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			obj := storage.Message{}
			if err := obj.UnmarshalBinary(value); err != nil {
				return err
			}
			v = append(v, obj)
		}
		return nil
	})

	if err != nil && !errors.Is(err, badgerdb.ErrKeyNotFound) {
		return
	}

	return v, nil
}

// StoredSysInfo returns the system info from the store.
func (h *Hook) StoredSysInfo() (v storage.SystemInfo, err error) {
	if h.db == nil {
		h.Log.Error("", "error", storage.ErrDBFileNotOpen)
		return
	}

	err = getKv(h.db, storage.SysInfoKey, &v)
	if err != nil && !errors.Is(err, badgerdb.ErrKeyNotFound) {
		return
	}

	return v, nil
}

// Errorf satisfies the badger interface for an error logger.
func (h *Hook) Errorf(m string, v ...interface{}) {
	h.Log.Error(fmt.Sprintf(strings.ToLower(strings.Trim(m, "\n")), v...), "v", v)

}

// Warningf satisfies the badger interface for a warning logger.
func (h *Hook) Warningf(m string, v ...interface{}) {
	h.Log.Warn(fmt.Sprintf(strings.ToLower(strings.Trim(m, "\n")), v...), "v", v)
}

// Infof satisfies the badger interface for an info logger.
func (h *Hook) Infof(m string, v ...interface{}) {
	h.Log.Info(fmt.Sprintf(strings.ToLower(strings.Trim(m, "\n")), v...), "v", v)
}

// Debugf satisfies the badger interface for a debug logger.
func (h *Hook) Debugf(m string, v ...interface{}) {
	h.Log.Debug(fmt.Sprintf(strings.ToLower(strings.Trim(m, "\n")), v...), "v", v)
}
