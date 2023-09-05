// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package mqtt

import (
	"fmt"
	"testing"

	"github.com/mochi-mqtt/server/v2/packets"
	"github.com/stretchr/testify/require"
)

const (
	testGroup  = "testgroup"
	otherGroup = "other"
)

func TestNewSharedSubscriptions(t *testing.T) {
	s := NewSharedSubscriptions()
	require.NotNil(t, s.internal)
}

func TestSharedSubscriptionsAdd(t *testing.T) {
	s := NewSharedSubscriptions()
	s.Add(testGroup, "cl1", packets.Subscription{Filter: "a/b/c"})
	require.Contains(t, s.internal, testGroup)
	require.Contains(t, s.internal[testGroup], "cl1")
}

func TestSharedSubscriptionsGet(t *testing.T) {
	s := NewSharedSubscriptions()
	s.Add(testGroup, "cl1", packets.Subscription{Qos: 2})
	s.Add(testGroup, "cl2", packets.Subscription{Qos: 2})
	require.Contains(t, s.internal, testGroup)
	require.Contains(t, s.internal[testGroup], "cl1")
	require.Contains(t, s.internal, testGroup)
	require.Contains(t, s.internal[testGroup], "cl2")

	sub, ok := s.Get(testGroup, "cl2")
	require.Equal(t, true, ok)
	require.Equal(t, byte(2), sub.Qos)
}

func TestSharedSubscriptionsGetAll(t *testing.T) {
	s := NewSharedSubscriptions()
	s.Add(testGroup, "cl1", packets.Subscription{Qos: 0})
	s.Add(testGroup, "cl2", packets.Subscription{Qos: 1})
	s.Add(otherGroup, "cl3", packets.Subscription{Qos: 2})
	require.Contains(t, s.internal, testGroup)
	require.Contains(t, s.internal[testGroup], "cl1")
	require.Contains(t, s.internal, testGroup)
	require.Contains(t, s.internal[testGroup], "cl2")
	require.Contains(t, s.internal, otherGroup)
	require.Contains(t, s.internal[otherGroup], "cl3")

	subs := s.GetAll()
	require.Len(t, subs, 2)
	require.Len(t, subs[testGroup], 2)
	require.Len(t, subs[otherGroup], 1)
}

func TestSharedSubscriptionsLen(t *testing.T) {
	s := NewSharedSubscriptions()
	s.Add(testGroup, "cl1", packets.Subscription{Qos: 0})
	s.Add(testGroup, "cl2", packets.Subscription{Qos: 1})
	s.Add(otherGroup, "cl2", packets.Subscription{Qos: 1})
	require.Contains(t, s.internal, testGroup)
	require.Contains(t, s.internal[testGroup], "cl1")
	require.Contains(t, s.internal, testGroup)
	require.Contains(t, s.internal[testGroup], "cl2")
	require.Contains(t, s.internal, otherGroup)
	require.Contains(t, s.internal[otherGroup], "cl2")
	require.Equal(t, 3, s.Len())
	require.Equal(t, 2, s.GroupLen())
}

func TestSharedSubscriptionsDelete(t *testing.T) {
	s := NewSharedSubscriptions()
	s.Add(testGroup, "cl1", packets.Subscription{Qos: 1})
	s.Add(testGroup, "cl2", packets.Subscription{Qos: 2})
	require.Contains(t, s.internal, testGroup)
	require.Contains(t, s.internal[testGroup], "cl1")
	require.Contains(t, s.internal, testGroup)
	require.Contains(t, s.internal[testGroup], "cl2")

	require.Equal(t, 2, s.Len())

	s.Delete(testGroup, "cl1")
	_, ok := s.Get(testGroup, "cl1")
	require.False(t, ok)
	require.Equal(t, 1, s.GroupLen())
	require.Equal(t, 1, s.Len())

	s.Delete(testGroup, "cl2")
	_, ok = s.Get(testGroup, "cl2")
	require.False(t, ok)
	require.Equal(t, 0, s.GroupLen())
	require.Equal(t, 0, s.Len())
}

func TestNewSubscriptions(t *testing.T) {
	s := NewSubscriptions()
	require.NotNil(t, s.internal)
}

func TestSubscriptionsAdd(t *testing.T) {
	s := NewSubscriptions()
	s.Add("cl1", packets.Subscription{})
	require.Contains(t, s.internal, "cl1")
}

func TestSubscriptionsGet(t *testing.T) {
	s := NewSubscriptions()
	s.Add("cl1", packets.Subscription{Qos: 2})
	s.Add("cl2", packets.Subscription{Qos: 2})
	require.Contains(t, s.internal, "cl1")
	require.Contains(t, s.internal, "cl2")

	sub, ok := s.Get("cl1")
	require.True(t, ok)
	require.Equal(t, byte(2), sub.Qos)
}

func TestSubscriptionsGetAll(t *testing.T) {
	s := NewSubscriptions()
	s.Add("cl1", packets.Subscription{Qos: 0})
	s.Add("cl2", packets.Subscription{Qos: 1})
	s.Add("cl3", packets.Subscription{Qos: 2})
	require.Contains(t, s.internal, "cl1")
	require.Contains(t, s.internal, "cl2")
	require.Contains(t, s.internal, "cl3")

	subs := s.GetAll()
	require.Len(t, subs, 3)
}

func TestSubscriptionsLen(t *testing.T) {
	s := NewSubscriptions()
	s.Add("cl1", packets.Subscription{Qos: 0})
	s.Add("cl2", packets.Subscription{Qos: 1})
	require.Contains(t, s.internal, "cl1")
	require.Contains(t, s.internal, "cl2")
	require.Equal(t, 2, s.Len())
}

func TestSubscriptionsDelete(t *testing.T) {
	s := NewSubscriptions()
	s.Add("cl1", packets.Subscription{Qos: 1})
	require.Contains(t, s.internal, "cl1")

	s.Delete("cl1")
	_, ok := s.Get("cl1")
	require.False(t, ok)
}

func TestNewTopicsIndex(t *testing.T) {
	index := NewTopicsIndex()
	require.NotNil(t, index)
	require.NotNil(t, index.root)
}

func BenchmarkNewTopicsIndex(b *testing.B) {
	for n := 0; n < b.N; n++ {
		NewTopicsIndex()
	}
}

func TestSubscribe(t *testing.T) {
	tt := []struct {
		desc         string
		client       string
		filter       string
		subscription packets.Subscription
		wasNew       bool
	}{
		{
			desc:   "subscribe",
			client: "cl1",

			subscription: packets.Subscription{Filter: "a/b/c", Qos: 2},
			wasNew:       true,
		},
		{
			desc:   "subscribe existed",
			client: "cl1",

			subscription: packets.Subscription{Filter: "a/b/c", Qos: 1},
			wasNew:       false,
		},
		{
			desc:   "subscribe case sensitive didnt exist",
			client: "cl1",

			subscription: packets.Subscription{Filter: "A/B/c", Qos: 1},
			wasNew:       true,
		},
		{
			desc:   "wildcard+ sub",
			client: "cl1",

			subscription: packets.Subscription{Filter: "d/+"},
			wasNew:       true,
		},
		{
			desc:         "wildcard# sub",
			client:       "cl1",
			subscription: packets.Subscription{Filter: "d/e/#"},
			wasNew:       true,
		},
	}

	index := NewTopicsIndex()
	for _, tx := range tt {
		t.Run(tx.desc, func(t *testing.T) {
			require.Equal(t, tx.wasNew, index.Subscribe(tx.client, tx.subscription))
		})
	}

	final := index.root.particles.get("a").particles.get("b").particles.get("c")
	require.NotNil(t, final)
	client, exists := final.subscriptions.Get("cl1")
	require.True(t, exists)
	require.Equal(t, byte(1), client.Qos)
}

func TestSubscribeShared(t *testing.T) {
	index := NewTopicsIndex()
	index.Subscribe("cl1", packets.Subscription{Filter: SharePrefix + "/tmp/a/b/c", Qos: 2})
	final := index.root.particles.get("a").particles.get("b").particles.get("c")
	require.NotNil(t, final)
	client, exists := final.shared.Get("tmp", "cl1")
	require.True(t, exists)
	require.Equal(t, byte(2), client.Qos)
	require.Equal(t, 0, final.subscriptions.Len())
	require.Equal(t, 1, final.shared.Len())
}

func BenchmarkSubscribe(b *testing.B) {
	index := NewTopicsIndex()
	for n := 0; n < b.N; n++ {
		index.Subscribe("client-1", packets.Subscription{Filter: "a/b/c"})
	}
}

func BenchmarkSubscribeShared(b *testing.B) {
	index := NewTopicsIndex()
	for n := 0; n < b.N; n++ {
		index.Subscribe("client-1", packets.Subscription{Filter: "$SHARE/tmp/a/b/c"})
	}
}

func TestUnsubscribe(t *testing.T) {
	index := NewTopicsIndex()
	index.Subscribe("cl1", packets.Subscription{Filter: "a/b/c/d", Qos: 1})
	client, exists := index.root.particles.get("a").particles.get("b").particles.get("c").particles.get("d").subscriptions.Get("cl1")
	require.NotNil(t, client)
	require.True(t, exists)

	index.Subscribe("cl1", packets.Subscription{Filter: "a/b/+/d", Qos: 1})
	client, exists = index.root.particles.get("a").particles.get("b").particles.get("+").particles.get("d").subscriptions.Get("cl1")
	require.NotNil(t, client)
	require.True(t, exists)

	index.Subscribe("cl1", packets.Subscription{Filter: "d/e/f", Qos: 1})
	client, exists = index.root.particles.get("d").particles.get("e").particles.get("f").subscriptions.Get("cl1")
	require.NotNil(t, client)
	require.True(t, exists)

	index.Subscribe("cl2", packets.Subscription{Filter: "d/e/f", Qos: 1})
	client, exists = index.root.particles.get("d").particles.get("e").particles.get("f").subscriptions.Get("cl2")
	require.NotNil(t, client)
	require.True(t, exists)

	index.Subscribe("cl3", packets.Subscription{Filter: "#", Qos: 2})
	client, exists = index.root.particles.get("#").subscriptions.Get("cl3")
	require.NotNil(t, client)
	require.True(t, exists)

	ok := index.Unsubscribe("a/b/c/d", "cl1")
	require.True(t, ok)
	require.Nil(t, index.root.particles.get("a").particles.get("b").particles.get("c"))
	client, exists = index.root.particles.get("a").particles.get("b").particles.get("+").particles.get("d").subscriptions.Get("cl1")
	require.NotNil(t, client)
	require.True(t, exists)

	ok = index.Unsubscribe("d/e/f", "cl1")
	require.True(t, ok)

	require.Equal(t, 1, index.root.particles.get("d").particles.get("e").particles.get("f").subscriptions.Len())
	client, exists = index.root.particles.get("d").particles.get("e").particles.get("f").subscriptions.Get("cl2")
	require.NotNil(t, client)
	require.True(t, exists)

	ok = index.Unsubscribe("fdasfdas/dfsfads/sa", "nobody")
	require.False(t, ok)
}

func TestUnsubscribeNoCascade(t *testing.T) {
	index := NewTopicsIndex()
	index.Subscribe("cl1", packets.Subscription{Filter: "a/b/c"})
	index.Subscribe("cl1", packets.Subscription{Filter: "a/b/c/e/e"})

	ok := index.Unsubscribe("a/b/c/e/e", "cl1")
	require.True(t, ok)
	require.Equal(t, 1, index.root.particles.len())

	client, exists := index.root.particles.get("a").particles.get("b").particles.get("c").subscriptions.Get("cl1")
	require.NotNil(t, client)
	require.True(t, exists)
}

func TestUnsubscribeShared(t *testing.T) {
	index := NewTopicsIndex()
	index.Subscribe("cl1", packets.Subscription{Filter: "$SHARE/tmp/a/b/c", Qos: 2})
	final := index.root.particles.get("a").particles.get("b").particles.get("c")
	require.NotNil(t, final)
	client, exists := final.shared.Get("tmp", "cl1")
	require.True(t, exists)
	require.Equal(t, byte(2), client.Qos)

	require.True(t, index.Unsubscribe("$share/tmp/a/b/c", "cl1"))
	_, exists = final.shared.Get("tmp", "cl1")
	require.False(t, exists)
}

func BenchmarkUnsubscribe(b *testing.B) {
	index := NewTopicsIndex()

	for n := 0; n < b.N; n++ {
		b.StopTimer()
		index.Subscribe("cl1", packets.Subscription{Filter: "a/b/c"})
		b.StartTimer()
		index.Unsubscribe("a/b/c", "cl1")
	}
}

func TestIndexSeek(t *testing.T) {
	filter := "a/b/c/d/e/f"
	index := NewTopicsIndex()
	k1 := index.set(filter, 0)
	require.Equal(t, "f", k1.key)
	k1.subscriptions.Add("cl1", packets.Subscription{})

	require.Equal(t, k1, index.seek(filter, 0))
	require.Nil(t, index.seek("d/e/f", 0))
}

func TestIndexTrim(t *testing.T) {
	index := NewTopicsIndex()
	k1 := index.set("a/b/c", 0)
	require.Equal(t, "c", k1.key)
	k1.subscriptions.Add("cl1", packets.Subscription{})

	k2 := index.set("a/b/c/d/e/f", 0)
	require.Equal(t, "f", k2.key)
	k2.subscriptions.Add("cl1", packets.Subscription{})

	k3 := index.set("a/b", 0)
	require.Equal(t, "b", k3.key)
	k3.subscriptions.Add("cl1", packets.Subscription{})

	index.trim(k2)
	require.NotNil(t, index.root.particles.get("a").particles.get("b").particles.get("c"))
	require.NotNil(t, index.root.particles.get("a").particles.get("b").particles.get("c").particles.get("d").particles.get("e").particles.get("f"))
	require.NotNil(t, index.root.particles.get("a").particles.get("b"))

	k2.subscriptions.Delete("cl1")
	index.trim(k2)

	require.Nil(t, index.root.particles.get("a").particles.get("b").particles.get("c").particles.get("d"))
	require.NotNil(t, index.root.particles.get("a").particles.get("b").particles.get("c"))

	k1.subscriptions.Delete("cl1")
	k3.subscriptions.Delete("cl1")
	index.trim(k2)
	require.Nil(t, index.root.particles.get("a"))
}

func TestIndexSet(t *testing.T) {
	index := NewTopicsIndex()
	child := index.set("a/b/c", 0)
	require.Equal(t, "c", child.key)
	require.NotNil(t, index.root.particles.get("a").particles.get("b").particles.get("c"))

	child = index.set("a/b/c/d/e", 0)
	require.Equal(t, "e", child.key)

	child = index.set("a/b/c/c/a", 0)
	require.Equal(t, "a", child.key)
}

func TestIndexSetPrefixed(t *testing.T) {
	index := NewTopicsIndex()
	child := index.set("/c", 0)
	require.Equal(t, "c", child.key)
	require.NotNil(t, index.root.particles.get("").particles.get("c"))
}

func BenchmarkIndexSet(b *testing.B) {
	index := NewTopicsIndex()
	for n := 0; n < b.N; n++ {
		index.set("a/b/c", 0)
	}
}

func TestRetainMessage(t *testing.T) {
	pk := packets.Packet{
		FixedHeader: packets.FixedHeader{Retain: true},
		TopicName:   "a/b/c",
		Payload:     []byte("hello"),
	}

	index := NewTopicsIndex()
	r := index.RetainMessage(pk)
	require.Equal(t, int64(1), r)
	pke, ok := index.Retained.Get(pk.TopicName)
	require.True(t, ok)
	require.Equal(t, pk, pke)

	pk2 := packets.Packet{
		FixedHeader: packets.FixedHeader{Retain: true},
		TopicName:   "a/b/d/f",
		Payload:     []byte("hello"),
	}
	r = index.RetainMessage(pk2)
	require.Equal(t, int64(1), r)
	// The same message already exists, but we're not doing a deep-copy check, so it's considered to be a new message.
	r = index.RetainMessage(pk2)
	require.Equal(t, int64(1), r)

	// Clear existing retained
	pk3 := packets.Packet{TopicName: "a/b/c", Payload: []byte{}}
	r = index.RetainMessage(pk3)
	require.Equal(t, int64(-1), r)
	_, ok = index.Retained.Get(pk.TopicName)
	require.False(t, ok)

	// Clear no retained
	r = index.RetainMessage(pk3)
	require.Equal(t, int64(0), r)
}

func BenchmarkRetainMessage(b *testing.B) {
	index := NewTopicsIndex()
	for n := 0; n < b.N; n++ {
		index.RetainMessage(packets.Packet{TopicName: "a/b/c/d"})
	}
}

func TestIsolateParticle(t *testing.T) {
	particle, hasNext := isolateParticle("path/to/my/mqtt", 0)
	require.Equal(t, "path", particle)
	require.Equal(t, true, hasNext)
	particle, hasNext = isolateParticle("path/to/my/mqtt", 1)
	require.Equal(t, "to", particle)
	require.Equal(t, true, hasNext)
	particle, hasNext = isolateParticle("path/to/my/mqtt", 2)
	require.Equal(t, "my", particle)
	require.Equal(t, true, hasNext)
	particle, hasNext = isolateParticle("path/to/my/mqtt", 3)
	require.Equal(t, "mqtt", particle)
	require.Equal(t, false, hasNext)

	particle, hasNext = isolateParticle("/path/", 0)
	require.Equal(t, "", particle)
	require.Equal(t, true, hasNext)
	particle, hasNext = isolateParticle("/path/", 1)
	require.Equal(t, "path", particle)
	require.Equal(t, true, hasNext)
	particle, hasNext = isolateParticle("/path/", 2)
	require.Equal(t, "", particle)
	require.Equal(t, false, hasNext)

	particle, hasNext = isolateParticle("a/b/c/+/+", 3)
	require.Equal(t, "+", particle)
	require.Equal(t, true, hasNext)
	particle, hasNext = isolateParticle("a/b/c/+/+", 4)
	require.Equal(t, "+", particle)
	require.Equal(t, false, hasNext)
}

func BenchmarkIsolateParticle(b *testing.B) {
	for n := 0; n < b.N; n++ {
		isolateParticle("path/to/my/mqtt", 3)
	}
}

func TestScanSubscribers(t *testing.T) {
	index := NewTopicsIndex()
	index.Subscribe("cl1", packets.Subscription{Qos: 1, Filter: "a/b/c", Identifier: 22})
	index.Subscribe("cl1", packets.Subscription{Qos: 1, Filter: "a/b/c/d/e/f"})
	index.Subscribe("cl1", packets.Subscription{Qos: 2, Filter: "a/b/c/d/+/f"})
	index.Subscribe("cl2", packets.Subscription{Qos: 0, Filter: "a/#"})
	index.Subscribe("cl2", packets.Subscription{Qos: 1, Filter: "a/b/c"})
	index.Subscribe("cl2", packets.Subscription{Qos: 2, Filter: "a/b/+", Identifier: 77})
	index.Subscribe("cl2", packets.Subscription{Qos: 2, Filter: "d/e/f", Identifier: 7237})
	index.Subscribe("cl2", packets.Subscription{Qos: 2, Filter: "$SYS/uptime", Identifier: 3})
	index.Subscribe("cl3", packets.Subscription{Qos: 1, Filter: "+/b", Identifier: 234})
	index.Subscribe("cl4", packets.Subscription{Qos: 0, Filter: "#", Identifier: 5})
	index.Subscribe("cl2", packets.Subscription{Qos: 0, Filter: "$SYS/test", Identifier: 2})

	subs := index.scanSubscribers("a/b/c", 0, nil, new(Subscribers))
	require.Equal(t, 3, len(subs.Subscriptions))
	require.Contains(t, subs.Subscriptions, "cl1")
	require.Contains(t, subs.Subscriptions, "cl2")
	require.Contains(t, subs.Subscriptions, "cl4")

	require.Equal(t, byte(1), subs.Subscriptions["cl1"].Qos)
	require.Equal(t, byte(2), subs.Subscriptions["cl2"].Qos)
	require.Equal(t, byte(0), subs.Subscriptions["cl4"].Qos)

	require.Equal(t, 22, subs.Subscriptions["cl1"].Identifiers["a/b/c"])
	require.Equal(t, 0, subs.Subscriptions["cl2"].Identifiers["a/#"])
	require.Equal(t, 77, subs.Subscriptions["cl2"].Identifiers["a/b/+"])
	require.Equal(t, 0, subs.Subscriptions["cl2"].Identifiers["a/b/c"])
	require.Equal(t, 5, subs.Subscriptions["cl4"].Identifiers["#"])

	subs = index.scanSubscribers("d/e/f/g", 0, nil, new(Subscribers))
	require.Equal(t, 1, len(subs.Subscriptions))
	require.Contains(t, subs.Subscriptions, "cl4")
	require.Equal(t, byte(0), subs.Subscriptions["cl4"].Qos)
	require.Equal(t, 5, subs.Subscriptions["cl4"].Identifiers["#"])

	subs = index.scanSubscribers("", 0, nil, new(Subscribers))
	require.Equal(t, 0, len(subs.Subscriptions))
}

func TestScanSubscribersTopicInheritanceBug(t *testing.T) {
	index := NewTopicsIndex()
	index.Subscribe("cl1", packets.Subscription{Qos: 0, Filter: "a/b/c"})
	index.Subscribe("cl2", packets.Subscription{Qos: 0, Filter: "a/b"})

	subs := index.scanSubscribers("a/b/c", 0, nil, new(Subscribers))
	require.Equal(t, 1, len(subs.Subscriptions))
}

func TestScanSubscribersShared(t *testing.T) {
	index := NewTopicsIndex()
	index.Subscribe("cl1", packets.Subscription{Qos: 1, Filter: SharePrefix + "/tmp/a/b/c", Identifier: 111})
	index.Subscribe("cl2", packets.Subscription{Qos: 0, Filter: SharePrefix + "/tmp/a/b/c", Identifier: 112})
	index.Subscribe("cl3", packets.Subscription{Qos: 0, Filter: SharePrefix + "/tmp2/a/b/c", Identifier: 113})
	index.Subscribe("cl2", packets.Subscription{Qos: 0, Filter: SharePrefix + "/tmp/a/b/+", Identifier: 10})
	index.Subscribe("cl3", packets.Subscription{Qos: 1, Filter: SharePrefix + "/tmp/a/b/+", Identifier: 200})
	index.Subscribe("cl4", packets.Subscription{Qos: 0, Filter: SharePrefix + "/tmp/a/b/+", Identifier: 201})
	index.Subscribe("cl5", packets.Subscription{Qos: 0, Filter: SharePrefix + "/tmp/a/b/c/#"})
	subs := index.scanSubscribers("a/b/c", 0, nil, new(Subscribers))
	require.Equal(t, 4, len(subs.Shared))
}

func TestSelectSharedSubscriber(t *testing.T) {
	index := NewTopicsIndex()
	index.Subscribe("cl1", packets.Subscription{Qos: 1, Filter: SharePrefix + "/tmp/a/b/c", Identifier: 110})
	index.Subscribe("cl1b", packets.Subscription{Qos: 0, Filter: SharePrefix + "/tmp/a/b/c", Identifier: 111})
	index.Subscribe("cl2", packets.Subscription{Qos: 0, Filter: SharePrefix + "/tmp/a/b/c", Identifier: 112})
	index.Subscribe("cl3", packets.Subscription{Qos: 0, Filter: SharePrefix + "/tmp2/a/b/c", Identifier: 113})
	subs := index.scanSubscribers("a/b/c", 0, nil, new(Subscribers))
	require.Equal(t, 2, len(subs.Shared))
	require.Contains(t, subs.Shared, SharePrefix+"/tmp/a/b/c")
	require.Contains(t, subs.Shared, SharePrefix+"/tmp2/a/b/c")
	require.Len(t, subs.Shared[SharePrefix+"/tmp/a/b/c"], 3)
	require.Len(t, subs.Shared[SharePrefix+"/tmp2/a/b/c"], 1)
	subs.SelectShared()
	require.Len(t, subs.SharedSelected, 2)
}

func TestMergeSharedSelected(t *testing.T) {
	s := &Subscribers{
		SharedSelected: map[string]packets.Subscription{
			"cl1": {Qos: 1, Filter: SharePrefix + "/tmp/a/b/c", Identifier: 110},
			"cl2": {Qos: 1, Filter: SharePrefix + "/tmp2/a/b/c", Identifier: 111},
		},
		Subscriptions: map[string]packets.Subscription{
			"cl2": {Qos: 1, Filter: "a/b/c", Identifier: 112},
		},
	}

	s.MergeSharedSelected()

	require.Equal(t, 2, len(s.Subscriptions))
	require.Contains(t, s.Subscriptions, "cl1")
	require.Contains(t, s.Subscriptions, "cl2")
	require.EqualValues(t, map[string]int{
		SharePrefix + "/tmp2/a/b/c": 111,
		"a/b/c":                     112,
	}, s.Subscriptions["cl2"].Identifiers)
}

func TestSubscribersFind(t *testing.T) {
	tt := []struct {
		filter  string
		topic   string
		matched bool
	}{
		{filter: "a", topic: "a", matched: true},
		{filter: "a/", topic: "a", matched: false},
		{filter: "a/", topic: "a/", matched: true},
		{filter: "/a", topic: "/a", matched: true},
		{filter: "path/to/my/mqtt", topic: "path/to/my/mqtt", matched: true},
		{filter: "path/to/+/mqtt", topic: "path/to/my/mqtt", matched: true},
		{filter: "+/to/+/mqtt", topic: "path/to/my/mqtt", matched: true},
		{filter: "#", topic: "path/to/my/mqtt", matched: true},
		{filter: "+/+/+/+", topic: "path/to/my/mqtt", matched: true},
		{filter: "+/+/+/#", topic: "path/to/my/mqtt", matched: true},
		{filter: "zen/#", topic: "zen", matched: true}, // as per 4.7.1.2
		{filter: "trailing-end/#", topic: "trailing-end/", matched: true},
		{filter: "+/prefixed", topic: "/prefixed", matched: true},
		{filter: "+/+/#", topic: "path/to/my/mqtt", matched: true},
		{filter: "path/to/", topic: "path/to/my/mqtt", matched: false},
		{filter: "#/stuff", topic: "path/to/my/mqtt", matched: false},
		{filter: "#", topic: "$SYS/info", matched: false},
		{filter: "$SYS/#", topic: "$SYS/info", matched: true},
		{filter: "+/info", topic: "$SYS/info", matched: false},
	}

	for _, tx := range tt {
		t.Run("filter:'"+tx.filter+"' vs topic:'"+tx.topic+"'", func(t *testing.T) {
			index := NewTopicsIndex()
			index.Subscribe("cl1", packets.Subscription{Filter: tx.filter})
			subs := index.Subscribers(tx.topic)
			require.Equal(t, tx.matched, len(subs.Subscriptions) == 1)
		})
	}
}

func BenchmarkSubscribers(b *testing.B) {
	index := NewTopicsIndex()
	index.Subscribe("cl1", packets.Subscription{Filter: "a/b/c"})
	index.Subscribe("cl1", packets.Subscription{Filter: "a/+/c"})
	index.Subscribe("cl1", packets.Subscription{Filter: "a/b/c/+"})
	index.Subscribe("cl2", packets.Subscription{Filter: "a/b/c/d"})
	index.Subscribe("cl3", packets.Subscription{Filter: "#"})

	for n := 0; n < b.N; n++ {
		index.Subscribers("a/b/c")
	}
}

func TestMessagesPattern(t *testing.T) {
	payload := []byte("hello")
	fh := packets.FixedHeader{Type: packets.Publish, Retain: true}

	pks := []packets.Packet{
		{TopicName: "$SYS/uptime", Payload: payload, FixedHeader: fh},
		{TopicName: "$SYS/info", Payload: payload, FixedHeader: fh},
		{TopicName: "a/b/c/d", Payload: payload, FixedHeader: fh},
		{TopicName: "a/b/c/e", Payload: payload, FixedHeader: fh},
		{TopicName: "a/b/d/f", Payload: payload, FixedHeader: fh},
		{TopicName: "q/w/e/r/t/y", Payload: payload, FixedHeader: fh},
		{TopicName: "q/x/e/r/t/o", Payload: payload, FixedHeader: fh},
		{TopicName: "asdf", Payload: payload, FixedHeader: fh},
	}

	tt := []struct {
		filter string
		len    int
	}{
		{"a/b/c/d", 1},
		{"$SYS/+", 2},
		{"$SYS/#", 2},
		{"#", len(pks) - 2},
		{"a/b/c/+", 2},
		{"a/+/c/+", 2},
		{"+/+/+/d", 1},
		{"q/w/e/#", 1},
		{"+/+/+/+", 3},
		{"q/#", 2},
		{"asdf", 1},
		{"", 0},
		{"#", 6},
	}

	index := NewTopicsIndex()
	for _, pk := range pks {
		index.RetainMessage(pk)
	}

	for _, tx := range tt {
		t.Run("filter:'"+tx.filter, func(t *testing.T) {
			messages := index.Messages(tx.filter)
			require.Equal(t, tx.len, len(messages))
		})
	}
}

func BenchmarkMessages(b *testing.B) {
	index := NewTopicsIndex()
	index.RetainMessage(packets.Packet{TopicName: "a/b/c/d"})
	index.RetainMessage(packets.Packet{TopicName: "a/b/d/e/f"})
	index.RetainMessage(packets.Packet{TopicName: "d/e/f/g"})
	index.RetainMessage(packets.Packet{TopicName: "$SYS/info"})
	index.RetainMessage(packets.Packet{TopicName: "q/w/e/r/t/y"})

	for n := 0; n < b.N; n++ {
		index.Messages("+/b/c/+")
	}
}

func TestNewParticles(t *testing.T) {
	cl := newParticles()
	require.NotNil(t, cl.internal)
}

func TestParticlesAdd(t *testing.T) {
	p := newParticles()
	p.add(&particle{key: "a"})
	require.Contains(t, p.internal, "a")
}

func TestParticlesGet(t *testing.T) {
	p := newParticles()
	p.add(&particle{key: "a"})
	p.add(&particle{key: "b"})
	require.Contains(t, p.internal, "a")
	require.Contains(t, p.internal, "b")

	particle := p.get("a")
	require.NotNil(t, particle)
	require.Equal(t, "a", particle.key)
}

func TestParticlesGetAll(t *testing.T) {
	p := newParticles()
	p.add(&particle{key: "a"})
	p.add(&particle{key: "b"})
	p.add(&particle{key: "c"})
	require.Contains(t, p.internal, "a")
	require.Contains(t, p.internal, "b")
	require.Contains(t, p.internal, "c")

	particles := p.getAll()
	require.Len(t, particles, 3)
}

func TestParticlesLen(t *testing.T) {
	p := newParticles()
	p.add(&particle{key: "a"})
	p.add(&particle{key: "b"})
	require.Contains(t, p.internal, "a")
	require.Contains(t, p.internal, "b")
	require.Equal(t, 2, p.len())
}

func TestParticlesDelete(t *testing.T) {
	p := newParticles()
	p.add(&particle{key: "a"})
	require.Contains(t, p.internal, "a")

	p.delete("a")
	particle := p.get("a")
	require.Nil(t, particle)
}

func TestIsValid(t *testing.T) {
	require.True(t, IsValidFilter("a/b/c", false))
	require.True(t, IsValidFilter("a/b//c", false))
	require.True(t, IsValidFilter("$SYS", false))
	require.True(t, IsValidFilter("$SYS/info", false))
	require.True(t, IsValidFilter("$sys/info", false))
	require.True(t, IsValidFilter("abc/#", false))
	require.False(t, IsValidFilter("", false))
	require.False(t, IsValidFilter(SharePrefix, false))
	require.False(t, IsValidFilter(SharePrefix+"/", false))
	require.False(t, IsValidFilter(SharePrefix+"/b+/", false))
	require.False(t, IsValidFilter(SharePrefix+"/+", false))
	require.False(t, IsValidFilter(SharePrefix+"/#", false))
	require.False(t, IsValidFilter(SharePrefix+"/#/", false))
	require.False(t, IsValidFilter("a/#/c", false))
}

func TestIsValidForPublish(t *testing.T) {
	require.True(t, IsValidFilter("", true))
	require.True(t, IsValidFilter("a/b/c", true))
	require.False(t, IsValidFilter("a/b/+/d", true))
	require.False(t, IsValidFilter("a/b/#", true))
	require.False(t, IsValidFilter("$SYS/info", true))
}

func TestIsSharedFilter(t *testing.T) {
	require.True(t, IsSharedFilter(SharePrefix+"/tmp/a/b/c"))
	require.False(t, IsSharedFilter("a/b/c"))
}

func TestNewInboundAliases(t *testing.T) {
	a := NewInboundTopicAliases(5)
	require.NotNil(t, a)
	require.NotNil(t, a.internal)
	require.Equal(t, uint16(5), a.maximum)
}

func TestInboundAliasesSet(t *testing.T) {
	topic := "test"
	id := uint16(1)
	a := NewInboundTopicAliases(5)
	require.Equal(t, topic, a.Set(id, topic))
	require.Contains(t, a.internal, id)
	require.Equal(t, a.internal[id], topic)

	require.Equal(t, topic, a.Set(id, ""))
}

func TestInboundAliasesSetMaxZero(t *testing.T) {
	topic := "test"
	id := uint16(1)
	a := NewInboundTopicAliases(0)
	require.Equal(t, topic, a.Set(id, topic))
	require.NotContains(t, a.internal, id)
}

func TestNewOutboundAliases(t *testing.T) {
	a := NewOutboundTopicAliases(5)
	require.NotNil(t, a)
	require.NotNil(t, a.internal)
	require.Equal(t, uint16(5), a.maximum)
	require.Equal(t, uint32(0), a.cursor)
}

func TestOutboundAliasesSet(t *testing.T) {
	a := NewOutboundTopicAliases(3)
	n, ok := a.Set("t1")
	require.False(t, ok)
	require.Equal(t, uint16(1), n)

	n, ok = a.Set("t2")
	require.False(t, ok)
	require.Equal(t, uint16(2), n)

	n, ok = a.Set("t3")
	require.False(t, ok)
	require.Equal(t, uint16(3), n)

	n, ok = a.Set("t4")
	require.False(t, ok)
	require.Equal(t, uint16(0), n)

	n, ok = a.Set("t2")
	require.True(t, ok)
	require.Equal(t, uint16(2), n)
}

func TestOutboundAliasesSetMaxZero(t *testing.T) {
	topic := "test"
	a := NewOutboundTopicAliases(0)
	n, ok := a.Set(topic)
	require.False(t, ok)
	require.Equal(t, uint16(0), n)
}

func TestNewTopicAliases(t *testing.T) {
	a := NewTopicAliases(5)
	require.NotNil(t, a.Inbound)
	require.Equal(t, uint16(5), a.Inbound.maximum)
	require.NotNil(t, a.Outbound)
	require.Equal(t, uint16(5), a.Outbound.maximum)
}

func TestNewInlineSubscriptions(t *testing.T) {
	subscriptions := NewInlineSubscriptions()
	require.NotNil(t, subscriptions)
	require.NotNil(t, subscriptions.internal)
	require.Equal(t, 0, subscriptions.Len())
}

func TestInlineSubscriptionAdd(t *testing.T) {
	subscriptions := NewInlineSubscriptions()
	handler := func(cl *Client, sub packets.Subscription, pk packets.Packet) {
		// handler logic
	}

	subscription := InlineSubscription{
		Subscription: packets.Subscription{Filter: "a/b/c", Identifier: 1},
		Handler:      handler,
	}
	subscriptions.Add(subscription)

	sub, ok := subscriptions.Get(1)
	require.True(t, ok)
	require.Equal(t, "a/b/c", sub.Filter)
	require.Equal(t, fmt.Sprintf("%p", handler), fmt.Sprintf("%p", sub.Handler))
}

func TestInlineSubscriptionGet(t *testing.T) {
	subscriptions := NewInlineSubscriptions()
	handler := func(cl *Client, sub packets.Subscription, pk packets.Packet) {
		// handler logic
	}

	subscription := InlineSubscription{
		Subscription: packets.Subscription{Filter: "a/b/c", Identifier: 1},
		Handler:      handler,
	}
	subscriptions.Add(subscription)

	sub, ok := subscriptions.Get(1)
	require.True(t, ok)
	require.Equal(t, "a/b/c", sub.Filter)
	require.Equal(t, fmt.Sprintf("%p", handler), fmt.Sprintf("%p", sub.Handler))

	_, ok = subscriptions.Get(999)
	require.False(t, ok)
}

func TestInlineSubscriptionsGetAll(t *testing.T) {
	subscriptions := NewInlineSubscriptions()

	subscriptions.Add(InlineSubscription{
		Subscription: packets.Subscription{Filter: "a/b/c", Identifier: 1},
	})
	subscriptions.Add(InlineSubscription{
		Subscription: packets.Subscription{Filter: "a/b/c", Identifier: 1},
	})
	subscriptions.Add(InlineSubscription{
		Subscription: packets.Subscription{Filter: "a/b/c", Identifier: 2},
	})
	subscriptions.Add(InlineSubscription{
		Subscription: packets.Subscription{Filter: "d/e/f", Identifier: 3},
	})

	allSubs := subscriptions.GetAll()
	require.Len(t, allSubs, 3)
	require.Contains(t, allSubs, 1)
	require.Contains(t, allSubs, 2)
	require.Contains(t, allSubs, 3)
}

func TestInlineSubscriptionDelete(t *testing.T) {
	subscriptions := NewInlineSubscriptions()
	handler := func(cl *Client, sub packets.Subscription, pk packets.Packet) {
		// handler logic
	}

	subscription := InlineSubscription{
		Subscription: packets.Subscription{Filter: "a/b/c", Identifier: 1},
		Handler:      handler,
	}
	subscriptions.Add(subscription)

	subscriptions.Delete(1)
	_, ok := subscriptions.Get(1)
	require.False(t, ok)
	require.Empty(t, subscriptions.GetAll())
	require.Zero(t, subscriptions.Len())
}

func TestInlineSubscribe(t *testing.T) {

	handler := func(cl *Client, sub packets.Subscription, pk packets.Packet) {
		// handler logic
	}

	tt := []struct {
		desc         string
		filter       string
		subscription InlineSubscription
		wasNew       bool
	}{
		{
			desc:         "subscribe",
			filter:       "a/b/c",
			subscription: InlineSubscription{Handler: handler, Subscription: packets.Subscription{Filter: "a/b/c", Identifier: 1}},
			wasNew:       true,
		},
		{
			desc:         "subscribe existed",
			filter:       "a/b/c",
			subscription: InlineSubscription{Handler: handler, Subscription: packets.Subscription{Filter: "a/b/c", Identifier: 1}},
			wasNew:       false,
		},
		{
			desc:         "subscribe different identifier",
			filter:       "a/b/c",
			subscription: InlineSubscription{Handler: handler, Subscription: packets.Subscription{Filter: "a/b/c", Identifier: 2}},
			wasNew:       true,
		},
		{
			desc:         "subscribe case sensitive didnt exist",
			filter:       "A/B/c",
			subscription: InlineSubscription{Handler: handler, Subscription: packets.Subscription{Filter: "A/B/c", Identifier: 1}},
			wasNew:       true,
		},
		{
			desc:         "wildcard+ sub",
			filter:       "d/+",
			subscription: InlineSubscription{Handler: handler, Subscription: packets.Subscription{Filter: "d/+", Identifier: 1}},
			wasNew:       true,
		},
		{
			desc:         "wildcard# sub",
			filter:       "d/e/#",
			subscription: InlineSubscription{Handler: handler, Subscription: packets.Subscription{Filter: "d/e/#", Identifier: 1}},
			wasNew:       true,
		},
	}

	index := NewTopicsIndex()
	for _, tx := range tt {
		t.Run(tx.desc, func(t *testing.T) {
			require.Equal(t, tx.wasNew, index.InlineSubscribe(tx.subscription))
		})
	}

	final := index.root.particles.get("a").particles.get("b").particles.get("c")
	require.NotNil(t, final)
}

func TestInlineUnsubscribe(t *testing.T) {
	handler := func(cl *Client, sub packets.Subscription, pk packets.Packet) {
		// handler logic
	}

	index := NewTopicsIndex()
	index.InlineSubscribe(InlineSubscription{Handler: handler, Subscription: packets.Subscription{Filter: "a/b/c/d", Identifier: 1}})
	sub, exists := index.root.particles.get("a").particles.get("b").particles.get("c").particles.get("d").inlineSubscriptions.Get(1)
	require.NotNil(t, sub)
	require.True(t, exists)

	index = NewTopicsIndex()
	index.InlineSubscribe(InlineSubscription{Handler: handler, Subscription: packets.Subscription{Filter: "a/b/c/d", Identifier: 1}})
	sub, exists = index.root.particles.get("a").particles.get("b").particles.get("c").particles.get("d").inlineSubscriptions.Get(1)
	require.NotNil(t, sub)
	require.True(t, exists)

	index.InlineSubscribe(InlineSubscription{Handler: handler, Subscription: packets.Subscription{Filter: "d/e/f", Identifier: 1}})
	sub, exists = index.root.particles.get("d").particles.get("e").particles.get("f").inlineSubscriptions.Get(1)
	require.NotNil(t, sub)
	require.True(t, exists)

	index.InlineSubscribe(InlineSubscription{Handler: handler, Subscription: packets.Subscription{Filter: "d/e/f", Identifier: 2}})
	sub, exists = index.root.particles.get("d").particles.get("e").particles.get("f").inlineSubscriptions.Get(2)
	require.NotNil(t, sub)
	require.True(t, exists)

	index.InlineSubscribe(InlineSubscription{Handler: handler, Subscription: packets.Subscription{Filter: "a/b/+/d", Identifier: 1}})
	sub, exists = index.root.particles.get("a").particles.get("b").particles.get("+").particles.get("d").inlineSubscriptions.Get(1)
	require.NotNil(t, sub)
	require.True(t, exists)

	index.InlineSubscribe(InlineSubscription{Handler: handler, Subscription: packets.Subscription{Filter: "d/e/f", Identifier: 1}})
	sub, exists = index.root.particles.get("d").particles.get("e").particles.get("f").inlineSubscriptions.Get(1)
	require.NotNil(t, sub)
	require.True(t, exists)

	index.InlineSubscribe(InlineSubscription{Handler: handler, Subscription: packets.Subscription{Filter: "d/e/f", Identifier: 1}})
	sub, exists = index.root.particles.get("d").particles.get("e").particles.get("f").inlineSubscriptions.Get(1)
	require.NotNil(t, sub)
	require.True(t, exists)

	index.InlineSubscribe(InlineSubscription{Handler: handler, Subscription: packets.Subscription{Filter: "#", Identifier: 1}})
	sub, exists = index.root.particles.get("#").inlineSubscriptions.Get(1)
	require.NotNil(t, sub)
	require.True(t, exists)

	ok := index.InlineUnsubscribe(1, "a/b/c/d")
	require.True(t, ok)
	require.Nil(t, index.root.particles.get("a").particles.get("b").particles.get("c"))

	sub, exists = index.root.particles.get("a").particles.get("b").particles.get("+").particles.get("d").inlineSubscriptions.Get(1)
	require.NotNil(t, sub)
	require.True(t, exists)

	ok = index.InlineUnsubscribe(1, "d/e/f")
	require.True(t, ok)
	require.NotNil(t, index.root.particles.get("d").particles.get("e").particles.get("f"))

	ok = index.InlineUnsubscribe(1, "not/exist")
	require.False(t, ok)
}
