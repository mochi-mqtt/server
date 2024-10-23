// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: werbenhu

package pebble

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	pebbledb "github.com/cockroachdb/pebble"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/storage"
	"github.com/mochi-mqtt/server/v2/packets"
	"github.com/mochi-mqtt/server/v2/system"
)

const (
	// defaultDbFile is the default file path for the pebble db file.
	defaultDbFile = ".pebble"
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

// keyUpperBound returns the upper bound for a given byte slice by incrementing the last byte.
// It returns nil if all bytes are incremented and equal to 0.
func keyUpperBound(b []byte) []byte {
	end := make([]byte, len(b))
	copy(end, b)
	for i := len(end) - 1; i >= 0; i-- {
		end[i] = end[i] + 1
		if end[i] != 0 {
			return end[:i+1]
		}
	}
	return nil
}

const (
	NoSync = "NoSync" // NoSync specifies the default write options for writes which do not synchronize to disk.
	Sync   = "Sync"   // Sync specifies the default write options for writes which synchronize to disk.
)

// Options contains configuration settings for the pebble DB instance.
type Options struct {
	Options *pebbledb.Options
	Mode    string `yaml:"mode" json:"mode"`
	Path    string `yaml:"path" json:"path"`
}

// Hook is a persistent storage hook based using pebble DB file store as a backend.
type Hook struct {
	mqtt.HookBase
	config *Options               // options for configuring the pebble DB instance.
	db     *pebbledb.DB           // the pebble DB instance
	mode   *pebbledb.WriteOptions // mode holds the optional per-query parameters for Set and Delete operations
}

// ID returns the id of the hook.
func (h *Hook) ID() string {
	return "pebble-db"
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

// Init initializes and connects to the pebble instance.
func (h *Hook) Init(config any) error {
	if _, ok := config.(*Options); !ok && config != nil {
		return mqtt.ErrInvalidConfigType
	}

	if config == nil {
		h.config = new(Options)
	} else {
		h.config = config.(*Options)
	}

	if len(h.config.Path) == 0 {
		h.config.Path = defaultDbFile
	}

	if h.config.Options == nil {
		h.config.Options = &pebbledb.Options{}
	}

	h.mode = pebbledb.NoSync
	if strings.EqualFold(h.config.Mode, "Sync") {
		h.mode = pebbledb.Sync
	}

	var err error
	h.db, err = pebbledb.Open(h.config.Path, h.config.Options)
	if err != nil {
		return err
	}

	return nil
}

// Stop closes the pebble instance.
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
	h.setKv(clientKey(cl), in)
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

	h.delKv(clientKey(cl))
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
		h.setKv(in.ID, in)
	}
}

// OnUnsubscribed removes one or more client subscriptions from the store.
func (h *Hook) OnUnsubscribed(cl *mqtt.Client, pk packets.Packet) {
	if h.db == nil {
		h.Log.Error("", "error", storage.ErrDBFileNotOpen)
		return
	}

	for i := 0; i < len(pk.Filters); i++ {
		h.delKv(subscriptionKey(cl, pk.Filters[i].Filter))
	}
}

// OnRetainMessage adds a retained message for a topic to the store.
func (h *Hook) OnRetainMessage(cl *mqtt.Client, pk packets.Packet, r int64) {
	if h.db == nil {
		h.Log.Error("", "error", storage.ErrDBFileNotOpen)
		return
	}

	if r == -1 {
		h.delKv(retainedKey(pk.TopicName))
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

	h.setKv(in.ID, in)
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
	h.setKv(in.ID, in)
}

// OnQosComplete removes a resolved inflight message from the store.
func (h *Hook) OnQosComplete(cl *mqtt.Client, pk packets.Packet) {
	if h.db == nil {
		h.Log.Error("", "error", storage.ErrDBFileNotOpen)
		return
	}
	h.delKv(inflightKey(cl, pk))
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
	h.setKv(in.ID, in)
}

// OnRetainedExpired deletes expired retained messages from the store.
func (h *Hook) OnRetainedExpired(filter string) {
	if h.db == nil {
		h.Log.Error("", "error", storage.ErrDBFileNotOpen)
		return
	}
	h.delKv(retainedKey(filter))
}

// OnClientExpired deleted expired clients from the store.
func (h *Hook) OnClientExpired(cl *mqtt.Client) {
	if h.db == nil {
		h.Log.Error("", "error", storage.ErrDBFileNotOpen)
		return
	}
	h.delKv(clientKey(cl))
}

// StoredClients returns all stored clients from the store.
func (h *Hook) StoredClients() (v []storage.Client, err error) {
	if h.db == nil {
		h.Log.Error("", "error", storage.ErrDBFileNotOpen)
		return
	}

	iter, _ := h.db.NewIter(&pebbledb.IterOptions{
		LowerBound: []byte(storage.ClientKey),
		UpperBound: keyUpperBound([]byte(storage.ClientKey)),
	})

	for iter.First(); iter.Valid(); iter.Next() {
		item := storage.Client{}
		if err := item.UnmarshalBinary(iter.Value()); err == nil {
			v = append(v, item)
		}
	}
	return v, nil
}

// StoredSubscriptions returns all stored subscriptions from the store.
func (h *Hook) StoredSubscriptions() (v []storage.Subscription, err error) {
	if h.db == nil {
		h.Log.Error("", "error", storage.ErrDBFileNotOpen)
		return
	}

	iter, _ := h.db.NewIter(&pebbledb.IterOptions{
		LowerBound: []byte(storage.SubscriptionKey),
		UpperBound: keyUpperBound([]byte(storage.SubscriptionKey)),
	})

	for iter.First(); iter.Valid(); iter.Next() {
		item := storage.Subscription{}
		if err := item.UnmarshalBinary(iter.Value()); err == nil {
			v = append(v, item)
		}
	}
	return v, nil
}

// StoredRetainedMessages returns all stored retained messages from the store.
func (h *Hook) StoredRetainedMessages() (v []storage.Message, err error) {
	if h.db == nil {
		h.Log.Error("", "error", storage.ErrDBFileNotOpen)
		return
	}

	iter, _ := h.db.NewIter(&pebbledb.IterOptions{
		LowerBound: []byte(storage.RetainedKey),
		UpperBound: keyUpperBound([]byte(storage.RetainedKey)),
	})

	for iter.First(); iter.Valid(); iter.Next() {
		item := storage.Message{}
		if err := item.UnmarshalBinary(iter.Value()); err == nil {
			v = append(v, item)
		}
	}
	return v, nil
}

// StoredInflightMessages returns all stored inflight messages from the store.
func (h *Hook) StoredInflightMessages() (v []storage.Message, err error) {
	if h.db == nil {
		h.Log.Error("", "error", storage.ErrDBFileNotOpen)
		return
	}

	iter, _ := h.db.NewIter(&pebbledb.IterOptions{
		LowerBound: []byte(storage.InflightKey),
		UpperBound: keyUpperBound([]byte(storage.InflightKey)),
	})

	for iter.First(); iter.Valid(); iter.Next() {
		item := storage.Message{}
		if err := item.UnmarshalBinary(iter.Value()); err == nil {
			v = append(v, item)
		}
	}
	return v, nil
}

// StoredSysInfo returns the system info from the store.
func (h *Hook) StoredSysInfo() (v storage.SystemInfo, err error) {
	if h.db == nil {
		h.Log.Error("", "error", storage.ErrDBFileNotOpen)
		return
	}

	err = h.getKv(sysInfoKey(), &v)
	if errors.Is(err, pebbledb.ErrNotFound) {
		return v, nil
	}

	return
}

// Errorf satisfies the pebble interface for an error logger.
func (h *Hook) Errorf(m string, v ...any) {
	h.Log.Error(fmt.Sprintf(strings.ToLower(strings.Trim(m, "\n")), v...), "v", v)

}

// Warningf satisfies the pebble interface for a warning logger.
func (h *Hook) Warningf(m string, v ...any) {
	h.Log.Warn(fmt.Sprintf(strings.ToLower(strings.Trim(m, "\n")), v...), "v", v)
}

// Infof satisfies the pebble interface for an info logger.
func (h *Hook) Infof(m string, v ...any) {
	h.Log.Info(fmt.Sprintf(strings.ToLower(strings.Trim(m, "\n")), v...), "v", v)
}

// Debugf satisfies the pebble interface for a debug logger.
func (h *Hook) Debugf(m string, v ...any) {
	h.Log.Debug(fmt.Sprintf(strings.ToLower(strings.Trim(m, "\n")), v...), "v", v)
}

// delKv deletes a key-value pair from the database.
func (h *Hook) delKv(k string) error {
	err := h.db.Delete([]byte(k), h.mode)
	if err != nil {
		h.Log.Error("failed to delete data", "error", err, "key", k)
		return err
	}
	return nil
}

// setKv stores a key-value pair in the database.
func (h *Hook) setKv(k string, v storage.Serializable) error {
	bs, _ := v.MarshalBinary()
	err := h.db.Set([]byte(k), bs, h.mode)
	if err != nil {
		h.Log.Error("failed to update data", "error", err, "key", k)
		return err
	}
	return nil
}

// getKv retrieves the value associated with a key from the database.
func (h *Hook) getKv(k string, v storage.Serializable) error {
	value, closer, err := h.db.Get([]byte(k))
	if err != nil {
		return err
	}

	defer func() {
		if closer != nil {
			closer.Close()
		}
	}()
	return v.UnmarshalBinary(value)
}
