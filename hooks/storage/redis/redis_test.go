// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-co
// SPDX-FileContributor: mochi-co

package redis

import (
	"os"
	"sort"
	"testing"
	"time"

	"github.com/mochi-co/mqtt/v2"
	"github.com/mochi-co/mqtt/v2/hooks/storage"
	"github.com/mochi-co/mqtt/v2/packets"
	"github.com/mochi-co/mqtt/v2/system"

	miniredis "github.com/alicebob/miniredis/v2"
	redis "github.com/go-redis/redis/v8"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

var (
	logger = zerolog.New(os.Stderr).With().Timestamp().Logger().Level(zerolog.Disabled)

	client = &mqtt.Client{
		ID: "test",
		Net: mqtt.ClientConnection{
			Remote:   "test.addr",
			Listener: "listener",
		},
		Properties: mqtt.ClientProperties{
			Username: []byte("username"),
			Clean:    false,
		},
	}

	pkf = packets.Packet{Filters: packets.Subscriptions{{Filter: "a/b/c"}}}
)

func newHook(t *testing.T, addr string) *Hook {
	h := new(Hook)
	h.SetOpts(&logger, nil)

	err := h.Init(&Options{
		Options: &redis.Options{
			Addr: addr,
		},
	})
	require.NoError(t, err)

	return h
}

func teardown(t *testing.T, h *Hook) {
	if h.db != nil {
		err := h.db.FlushAll(h.ctx).Err()
		require.NoError(t, err)
		h.Stop()
	}
}

func TestClientKey(t *testing.T) {
	k := clientKey(&mqtt.Client{ID: "cl1"})
	require.Equal(t, "cl1", k)
}

func TestSubscriptionKey(t *testing.T) {
	k := subscriptionKey(&mqtt.Client{ID: "cl1"}, "a/b/c")
	require.Equal(t, "cl1:a/b/c", k)
}

func TestRetainedKey(t *testing.T) {
	k := retainedKey("a/b/c")
	require.Equal(t, "a/b/c", k)
}

func TestInflightKey(t *testing.T) {
	k := inflightKey(&mqtt.Client{ID: "cl1"}, packets.Packet{PacketID: 1})
	require.Equal(t, "cl1:1", k)
}

func TestSysInfoKey(t *testing.T) {
	require.Equal(t, storage.SysInfoKey, sysInfoKey())
}

func TestID(t *testing.T) {
	h := new(Hook)
	h.SetOpts(&logger, nil)
	require.Equal(t, "redis-db", h.ID())
}

func TestProvides(t *testing.T) {
	h := new(Hook)
	h.SetOpts(&logger, nil)
	require.True(t, h.Provides(mqtt.OnSessionEstablished))
	require.True(t, h.Provides(mqtt.OnDisconnect))
	require.True(t, h.Provides(mqtt.OnSubscribed))
	require.True(t, h.Provides(mqtt.OnUnsubscribed))
	require.True(t, h.Provides(mqtt.OnRetainMessage))
	require.True(t, h.Provides(mqtt.OnQosPublish))
	require.True(t, h.Provides(mqtt.OnQosComplete))
	require.True(t, h.Provides(mqtt.OnQosDropped))
	require.True(t, h.Provides(mqtt.OnSysInfoTick))
	require.True(t, h.Provides(mqtt.StoredClients))
	require.True(t, h.Provides(mqtt.StoredInflightMessages))
	require.True(t, h.Provides(mqtt.StoredRetainedMessages))
	require.True(t, h.Provides(mqtt.StoredSubscriptions))
	require.True(t, h.Provides(mqtt.StoredSysInfo))
	require.False(t, h.Provides(mqtt.OnACLCheck))
	require.False(t, h.Provides(mqtt.OnConnectAuthenticate))
}

func TestHKey(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	h.SetOpts(&logger, nil)
	require.Equal(t, defaultHPrefix+"test", h.hKey("test"))
}

func TestInitUseDefaults(t *testing.T) {
	s := miniredis.RunT(t)
	s.StartAddr(defaultAddr)
	defer s.Close()

	h := newHook(t, defaultAddr)
	h.SetOpts(&logger, nil)
	err := h.Init(nil)
	require.NoError(t, err)
	defer teardown(t, h)

	require.Equal(t, defaultHPrefix, h.config.HPrefix)
	require.Equal(t, defaultAddr, h.config.Options.Addr)
}

func TestInitBadConfig(t *testing.T) {
	h := new(Hook)
	h.SetOpts(&logger, nil)

	err := h.Init(map[string]any{})
	require.Error(t, err)
}

func TestInitBadAddr(t *testing.T) {
	h := new(Hook)
	h.SetOpts(&logger, nil)
	err := h.Init(&Options{
		Options: &redis.Options{
			Addr: "abc:123",
		},
	})
	require.Error(t, err)
}

func TestOnSessionEstablishedThenOnDisconnect(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	defer teardown(t, h)

	h.OnSessionEstablished(client, packets.Packet{})

	r := new(storage.Client)
	row, err := h.db.HGet(h.ctx, h.hKey(storage.ClientKey), clientKey(client)).Result()
	require.NoError(t, err)
	err = r.UnmarshalBinary([]byte(row))
	require.NoError(t, err)

	require.Equal(t, client.ID, r.ID)
	require.Equal(t, client.Net.Remote, r.Remote)
	require.Equal(t, client.Net.Listener, r.Listener)
	require.Equal(t, client.Properties.Username, r.Username)
	require.Equal(t, client.Properties.Clean, r.Clean)
	require.NotSame(t, client, r)

	h.OnDisconnect(client, nil, false)
	r2 := new(storage.Client)
	row, err = h.db.HGet(h.ctx, h.hKey(storage.ClientKey), clientKey(client)).Result()
	require.NoError(t, err)
	err = r2.UnmarshalBinary([]byte(row))
	require.NoError(t, err)
	require.Equal(t, client.ID, r.ID)

	h.OnDisconnect(client, nil, true)
	r3 := new(storage.Client)
	_, err = h.db.HGet(h.ctx, h.hKey(storage.ClientKey), clientKey(client)).Result()
	require.Error(t, err)
	require.ErrorIs(t, err, redis.Nil)
	require.Empty(t, r3.ID)
}

func TestOnSessionEstablishedNoDB(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())

	h.db = nil
	h.OnSessionEstablished(client, packets.Packet{})
}

func TestOnSessionEstablishedClosedDB(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	teardown(t, h)
	h.OnSessionEstablished(client, packets.Packet{})
}

func TestOnWillSent(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	defer teardown(t, h)

	c1 := client
	c1.Properties.Will.Flag = 1
	h.OnWillSent(c1, packets.Packet{})

	r := new(storage.Client)
	row, err := h.db.HGet(h.ctx, h.hKey(storage.ClientKey), clientKey(client)).Result()
	require.NoError(t, err)
	err = r.UnmarshalBinary([]byte(row))
	require.NoError(t, err)

	require.Equal(t, uint32(1), r.Will.Flag)
	require.NotSame(t, client, r)
}

func TestOnClientExpired(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	defer teardown(t, h)

	cl := &mqtt.Client{ID: "cl1"}
	clientKey := clientKey(cl)

	err := h.db.HSet(h.ctx, h.hKey(storage.ClientKey), clientKey, &storage.Client{ID: cl.ID}).Err()
	require.NoError(t, err)

	r := new(storage.Client)
	row, err := h.db.HGet(h.ctx, h.hKey(storage.ClientKey), clientKey).Result()
	require.NoError(t, err)
	err = r.UnmarshalBinary([]byte(row))
	require.NoError(t, err)
	require.Equal(t, clientKey, r.ID)

	h.OnClientExpired(cl)
	_, err = h.db.HGet(h.ctx, h.hKey(storage.ClientKey), clientKey).Result()
	require.Error(t, err)
	require.ErrorIs(t, redis.Nil, err)
}

func TestOnClientExpiredClosedDB(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	teardown(t, h)
	h.OnClientExpired(client)
}

func TestOnClientExpiredNoDB(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	h.db = nil
	h.OnClientExpired(client)
}

func TestOnDisconnectNoDB(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	h.db = nil
	h.OnDisconnect(client, nil, false)
}

func TestOnDisconnectClosedDB(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	teardown(t, h)
	h.OnDisconnect(client, nil, false)
}

func TestOnDisconnectSessionTakenOver(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())

	testClient := &mqtt.Client{
		ID: "test",
		Net: mqtt.ClientConnection{
			Remote:   "test.addr",
			Listener: "listener",
		},
		Properties: mqtt.ClientProperties{
			Username: []byte("username"),
			Clean:    false,
		},
	}

	testClient.Stop(packets.ErrSessionTakenOver)
	teardown(t, h)
	h.OnDisconnect(testClient, nil, true)
}

func TestOnSubscribedThenOnUnsubscribed(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	defer teardown(t, h)

	h.OnSubscribed(client, pkf, []byte{0})

	r := new(storage.Subscription)
	row, err := h.db.HGet(h.ctx, h.hKey(storage.SubscriptionKey), subscriptionKey(client, pkf.Filters[0].Filter)).Result()
	require.NoError(t, err)
	err = r.UnmarshalBinary([]byte(row))
	require.NoError(t, err)
	require.Equal(t, client.ID, r.Client)
	require.Equal(t, pkf.Filters[0].Filter, r.Filter)
	require.Equal(t, byte(0), r.Qos)

	h.OnUnsubscribed(client, pkf)
	_, err = h.db.HGet(h.ctx, h.hKey(storage.SubscriptionKey), subscriptionKey(client, pkf.Filters[0].Filter)).Result()
	require.Error(t, err)
	require.ErrorIs(t, err, redis.Nil)
}

func TestOnSubscribedNoDB(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	h.db = nil
	h.OnSubscribed(client, pkf, []byte{0})
}

func TestOnSubscribedClosedDB(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	teardown(t, h)
	h.OnSubscribed(client, pkf, []byte{0})
}

func TestOnUnsubscribedNoDB(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	h.db = nil
	h.OnUnsubscribed(client, pkf)
}

func TestOnUnsubscribedClosedDB(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	teardown(t, h)
	h.OnUnsubscribed(client, pkf)
}

func TestOnRetainMessageThenUnset(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	defer teardown(t, h)

	pk := packets.Packet{
		FixedHeader: packets.FixedHeader{
			Retain: true,
		},
		Payload:   []byte("hello"),
		TopicName: "a/b/c",
	}

	h.OnRetainMessage(client, pk, 1)

	r := new(storage.Message)
	row, err := h.db.HGet(h.ctx, h.hKey(storage.RetainedKey), retainedKey(pk.TopicName)).Result()
	require.NoError(t, err)
	err = r.UnmarshalBinary([]byte(row))
	require.NoError(t, err)

	require.NoError(t, err)
	require.Equal(t, pk.TopicName, r.TopicName)
	require.Equal(t, pk.Payload, r.Payload)

	h.OnRetainMessage(client, pk, -1)
	_, err = h.db.HGet(h.ctx, h.hKey(storage.RetainedKey), retainedKey(pk.TopicName)).Result()
	require.Error(t, err)
	require.ErrorIs(t, err, redis.Nil)

	// coverage: delete deleted
	h.OnRetainMessage(client, pk, -1)
	_, err = h.db.HGet(h.ctx, h.hKey(storage.RetainedKey), retainedKey(pk.TopicName)).Result()
	require.Error(t, err)
	require.ErrorIs(t, err, redis.Nil)
}

func TestOnRetainedExpired(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	defer teardown(t, h)

	m := &storage.Message{
		ID:        retainedKey("a/b/c"),
		T:         storage.RetainedKey,
		TopicName: "a/b/c",
	}

	err := h.db.HSet(h.ctx, h.hKey(storage.RetainedKey), m.ID, m).Err()
	require.NoError(t, err)

	r := new(storage.Message)
	row, err := h.db.HGet(h.ctx, h.hKey(storage.RetainedKey), m.ID).Result()
	require.NoError(t, err)
	err = r.UnmarshalBinary([]byte(row))
	require.NoError(t, err)
	require.NoError(t, err)
	require.Equal(t, m.TopicName, r.TopicName)

	h.OnRetainedExpired(m.TopicName)

	_, err = h.db.HGet(h.ctx, h.hKey(storage.RetainedKey), m.ID).Result()
	require.Error(t, err)
	require.ErrorIs(t, err, redis.Nil)
}

func TestOnRetainedExpiredClosedDB(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	teardown(t, h)
	h.OnRetainedExpired("a/b/c")
}

func TestOnRetainedExpiredNoDB(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	h.db = nil
	h.OnRetainedExpired("a/b/c")
}

func TestOnRetainMessageNoDB(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	h.db = nil
	h.OnRetainMessage(client, packets.Packet{}, 0)
}

func TestOnRetainMessageClosedDB(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	teardown(t, h)
	h.OnRetainMessage(client, packets.Packet{}, 0)
}

func TestOnQosPublishThenQOSComplete(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	defer teardown(t, h)

	pk := packets.Packet{
		FixedHeader: packets.FixedHeader{
			Retain: true,
			Qos:    2,
		},
		Payload:   []byte("hello"),
		TopicName: "a/b/c",
	}

	h.OnQosPublish(client, pk, time.Now().Unix(), 0)

	r := new(storage.Message)
	row, err := h.db.HGet(h.ctx, h.hKey(storage.InflightKey), inflightKey(client, pk)).Result()
	require.NoError(t, err)
	err = r.UnmarshalBinary([]byte(row))
	require.NoError(t, err)
	require.Equal(t, pk.TopicName, r.TopicName)
	require.Equal(t, pk.Payload, r.Payload)

	// ensure dates are properly saved to bolt
	require.True(t, r.Sent > 0)
	require.True(t, time.Now().Unix()-1 < r.Sent)

	// OnQosDropped is a passthrough to OnQosComplete here
	h.OnQosDropped(client, pk)
	_, err = h.db.HGet(h.ctx, h.hKey(storage.InflightKey), inflightKey(client, pk)).Result()
	require.Error(t, err)
	require.ErrorIs(t, err, redis.Nil)
}

func TestOnQosPublishNoDB(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	h.db = nil
	h.OnQosPublish(client, packets.Packet{}, time.Now().Unix(), 0)
}

func TestOnQosPublishClosedDB(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	teardown(t, h)
	h.OnQosPublish(client, packets.Packet{}, time.Now().Unix(), 0)
}

func TestOnQosCompleteNoDB(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	h.db = nil
	h.OnQosComplete(client, packets.Packet{})
}

func TestOnQosCompleteClosedDB(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	teardown(t, h)
	h.OnQosComplete(client, packets.Packet{})
}

func TestOnQosDroppedNoDB(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	h.db = nil
	h.OnQosDropped(client, packets.Packet{})
}

func TestOnSysInfoTick(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	defer teardown(t, h)

	info := &system.Info{
		Version:       "2.0.0",
		BytesReceived: 100,
	}

	h.OnSysInfoTick(info)

	r := new(storage.SystemInfo)
	row, err := h.db.HGet(h.ctx, h.hKey(storage.SysInfoKey), storage.SysInfoKey).Result()
	require.NoError(t, err)
	err = r.UnmarshalBinary([]byte(row))
	require.NoError(t, err)
	require.Equal(t, info.Version, r.Version)
	require.Equal(t, info.BytesReceived, r.BytesReceived)
	require.NotSame(t, info, r)
}

func TestOnSysInfoTickClosedDB(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	teardown(t, h)
	h.OnSysInfoTick(new(system.Info))
}
func TestOnSysInfoTickNoDB(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	h.db = nil
	h.OnSysInfoTick(new(system.Info))
}

func TestStoredClients(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	defer teardown(t, h)

	// populate with clients
	err := h.db.HSet(h.ctx, h.hKey(storage.ClientKey), "cl1", &storage.Client{ID: "cl1", T: storage.ClientKey}).Err()
	require.NoError(t, err)

	err = h.db.HSet(h.ctx, h.hKey(storage.ClientKey), "cl2", &storage.Client{ID: "cl2", T: storage.ClientKey}).Err()
	require.NoError(t, err)

	err = h.db.HSet(h.ctx, h.hKey(storage.ClientKey), "cl3", &storage.Client{ID: "cl3", T: storage.ClientKey}).Err()
	require.NoError(t, err)

	r, err := h.StoredClients()
	require.NoError(t, err)
	require.Len(t, r, 3)

	sort.Slice(r[:], func(i, j int) bool { return r[i].ID < r[j].ID })
	require.Equal(t, "cl1", r[0].ID)
	require.Equal(t, "cl2", r[1].ID)
	require.Equal(t, "cl3", r[2].ID)
}

func TestStoredClientsNoDB(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	h.db = nil
	v, err := h.StoredClients()
	require.Empty(t, v)
	require.NoError(t, err)
}

func TestStoredClientsClosedDB(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	teardown(t, h)

	v, err := h.StoredClients()
	require.Empty(t, v)
	require.Error(t, err)
}

func TestStoredSubscriptions(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	defer teardown(t, h)

	// populate with subscriptions
	err := h.db.HSet(h.ctx, h.hKey(storage.SubscriptionKey), "sub1", &storage.Subscription{ID: "sub1", T: storage.SubscriptionKey}).Err()
	require.NoError(t, err)

	err = h.db.HSet(h.ctx, h.hKey(storage.SubscriptionKey), "sub2", &storage.Subscription{ID: "sub2", T: storage.SubscriptionKey}).Err()
	require.NoError(t, err)

	err = h.db.HSet(h.ctx, h.hKey(storage.SubscriptionKey), "sub3", &storage.Subscription{ID: "sub3", T: storage.SubscriptionKey}).Err()
	require.NoError(t, err)

	r, err := h.StoredSubscriptions()
	require.NoError(t, err)
	require.Len(t, r, 3)
	sort.Slice(r[:], func(i, j int) bool { return r[i].ID < r[j].ID })
	require.Equal(t, "sub1", r[0].ID)
	require.Equal(t, "sub2", r[1].ID)
	require.Equal(t, "sub3", r[2].ID)
}

func TestStoredSubscriptionsNoDB(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	h.db = nil
	v, err := h.StoredSubscriptions()
	require.Empty(t, v)
	require.NoError(t, err)
}

func TestStoredSubscriptionsClosedDB(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	teardown(t, h)

	v, err := h.StoredSubscriptions()
	require.Empty(t, v)
	require.Error(t, err)
}

func TestStoredRetainedMessages(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	defer teardown(t, h)

	// populate with messages
	err := h.db.HSet(h.ctx, h.hKey(storage.RetainedKey), "m1", &storage.Message{ID: "m1", T: storage.RetainedKey}).Err()
	require.NoError(t, err)

	err = h.db.HSet(h.ctx, h.hKey(storage.RetainedKey), "m2", &storage.Message{ID: "m2", T: storage.RetainedKey}).Err()
	require.NoError(t, err)

	err = h.db.HSet(h.ctx, h.hKey(storage.RetainedKey), "m3", &storage.Message{ID: "m3", T: storage.RetainedKey}).Err()
	require.NoError(t, err)

	err = h.db.HSet(h.ctx, h.hKey(storage.InflightKey), "i3", &storage.Message{ID: "i3", T: storage.InflightKey}).Err()
	require.NoError(t, err)

	r, err := h.StoredRetainedMessages()
	require.NoError(t, err)
	require.Len(t, r, 3)
	sort.Slice(r[:], func(i, j int) bool { return r[i].ID < r[j].ID })
	require.Equal(t, "m1", r[0].ID)
	require.Equal(t, "m2", r[1].ID)
	require.Equal(t, "m3", r[2].ID)
}

func TestStoredRetainedMessagesNoDB(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	h.db = nil
	v, err := h.StoredRetainedMessages()
	require.Empty(t, v)
	require.NoError(t, err)
}

func TestStoredRetainedMessagesClosedDB(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	teardown(t, h)

	v, err := h.StoredRetainedMessages()
	require.Empty(t, v)
	require.Error(t, err)
}

func TestStoredInflightMessages(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	defer teardown(t, h)

	// populate with messages
	err := h.db.HSet(h.ctx, h.hKey(storage.InflightKey), "i1", &storage.Message{ID: "i1", T: storage.InflightKey}).Err()
	require.NoError(t, err)

	err = h.db.HSet(h.ctx, h.hKey(storage.InflightKey), "i2", &storage.Message{ID: "i2", T: storage.InflightKey}).Err()
	require.NoError(t, err)

	err = h.db.HSet(h.ctx, h.hKey(storage.InflightKey), "i3", &storage.Message{ID: "i3", T: storage.InflightKey}).Err()
	require.NoError(t, err)

	err = h.db.HSet(h.ctx, h.hKey(storage.RetainedKey), "m3", &storage.Message{ID: "m3", T: storage.RetainedKey}).Err()
	require.NoError(t, err)

	r, err := h.StoredInflightMessages()
	require.NoError(t, err)
	require.Len(t, r, 3)
	sort.Slice(r[:], func(i, j int) bool { return r[i].ID < r[j].ID })
	require.Equal(t, "i1", r[0].ID)
	require.Equal(t, "i2", r[1].ID)
	require.Equal(t, "i3", r[2].ID)
}

func TestStoredInflightMessagesNoDB(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	h.db = nil
	v, err := h.StoredInflightMessages()
	require.Empty(t, v)
	require.NoError(t, err)
}

func TestStoredInflightMessagesClosedDB(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	teardown(t, h)

	v, err := h.StoredInflightMessages()
	require.Empty(t, v)
	require.Error(t, err)
}

func TestStoredSysInfo(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	defer teardown(t, h)

	// populate with sys info
	err := h.db.HSet(h.ctx, h.hKey(storage.SysInfoKey), storage.SysInfoKey,
		&storage.SystemInfo{
			ID: storage.SysInfoKey,
			Info: system.Info{
				Version: "2.0.0",
			},
			T: storage.SysInfoKey,
		}).Err()
	require.NoError(t, err)

	r, err := h.StoredSysInfo()
	require.NoError(t, err)
	require.Equal(t, "2.0.0", r.Info.Version)
}

func TestStoredSysInfoNoDB(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	h.db = nil
	v, err := h.StoredSysInfo()
	require.Empty(t, v)
	require.NoError(t, err)
}

func TestStoredSysInfoClosedDB(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()
	h := newHook(t, s.Addr())
	teardown(t, h)

	v, err := h.StoredSysInfo()
	require.Empty(t, v)
	require.Error(t, err)
}
