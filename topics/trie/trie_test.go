package trie

import (
	"testing"

	//"github.com/davecgh/go-spew/spew"

	"github.com/stretchr/testify/require"

	"github.com/mochi-co/mqtt/packets"
)

func TestNew(t *testing.T) {
	index := New()
	require.NotNil(t, index)
	require.NotNil(t, index.Root)
}

func BenchmarkNew(b *testing.B) {
	for n := 0; n < b.N; n++ {
		New()
	}
}

func TestPoperate(t *testing.T) {
	index := New()
	child := index.poperate("path/to/my/mqtt")
	require.Equal(t, "mqtt", child.Key)
	require.NotNil(t, index.Root.Leaves["path"].Leaves["to"].Leaves["my"].Leaves["mqtt"])

	child = index.poperate("a/b/c/d/e")
	require.Equal(t, "e", child.Key)
	child = index.poperate("a/b/c/c/a")
	require.Equal(t, "a", child.Key)
}

func BenchmarkPoperate(b *testing.B) {
	index := New()
	for n := 0; n < b.N; n++ {
		index.poperate("path/to/my/mqtt")
	}
}

func TestSubscribeOK(t *testing.T) {
	index := New()
	index.Subscribe("path/to/my/mqtt", "client-1", 0)
	index.Subscribe("path/to/my/mqtt", "client-2", 0)
	index.Subscribe("path/to/another/mqtt", "client-1", 0)
	index.Subscribe("path/+", "client-2", 0)
	index.Subscribe("#", "client-3", 0)

	require.Contains(t, index.Root.Leaves["path"].Leaves["to"].Leaves["my"].Leaves["mqtt"].Clients, "client-1")
	require.Equal(t, "path/to/my/mqtt", index.Root.Leaves["path"].Leaves["to"].Leaves["my"].Leaves["mqtt"].Filter)
	require.Equal(t, "mqtt", index.Root.Leaves["path"].Leaves["to"].Leaves["my"].Leaves["mqtt"].Key)
	require.Equal(t, index.Root.Leaves["path"], index.Root.Leaves["path"].Leaves["to"].Parent)
	require.NotNil(t, index.Root.Leaves["path"].Leaves["to"].Leaves["my"].Leaves["mqtt"].Clients, "client-2")

	require.Contains(t, index.Root.Leaves["path"].Leaves["to"].Leaves["another"].Leaves["mqtt"].Clients, "client-1")
	require.Contains(t, index.Root.Leaves["path"].Leaves["+"].Clients, "client-2")
	require.Contains(t, index.Root.Leaves["#"].Clients, "client-3")
}

func BenchmarkSubscribe(b *testing.B) {
	index := New()
	for n := 0; n < b.N; n++ {
		index.Subscribe("path/to/mqtt/basic", "client-1", 0)
	}
}

func TestUnsubscribeA(t *testing.T) {
	index := New()
	index.Subscribe("path/to/my/mqtt", "client-1", 0)
	index.Subscribe("path/to/+/mqtt", "client-1", 0)
	index.Subscribe("path/to/stuff", "client-1", 0)
	index.Subscribe("path/to/stuff", "client-2", 0)
	index.Subscribe("#", "client-3", 0)
	require.Contains(t, index.Root.Leaves["path"].Leaves["to"].Leaves["my"].Leaves["mqtt"].Clients, "client-1")
	require.Contains(t, index.Root.Leaves["path"].Leaves["to"].Leaves["+"].Leaves["mqtt"].Clients, "client-1")
	require.Contains(t, index.Root.Leaves["path"].Leaves["to"].Leaves["stuff"].Clients, "client-1")
	require.Contains(t, index.Root.Leaves["path"].Leaves["to"].Leaves["stuff"].Clients, "client-2")
	require.Contains(t, index.Root.Leaves["#"].Clients, "client-3")

	ok := index.Unsubscribe("path/to/my/mqtt", "client-1")

	require.Equal(t, true, ok)
	require.Nil(t, index.Root.Leaves["path"].Leaves["to"].Leaves["my"])
	require.Contains(t, index.Root.Leaves["path"].Leaves["to"].Leaves["+"].Leaves["mqtt"].Clients, "client-1")

	ok = index.Unsubscribe("path/to/stuff", "client-1")
	require.Equal(t, true, ok)
	require.NotContains(t, index.Root.Leaves["path"].Leaves["to"].Leaves["stuff"].Clients, "client-1")
	require.Contains(t, index.Root.Leaves["path"].Leaves["to"].Leaves["stuff"].Clients, "client-2")
	require.Contains(t, index.Root.Leaves["#"].Clients, "client-3")

	ok = index.Unsubscribe("fdasfdas/dfsfads/sa", "client-1")
	require.Equal(t, false, ok)

}

func TestUnsubscribeCascade(t *testing.T) {
	index := New()
	index.Subscribe("a/b/c", "client-1", 0)
	index.Subscribe("a/b/c/e/e", "client-1", 0)

	ok := index.Unsubscribe("a/b/c/e/e", "client-1")
	require.Equal(t, true, ok)
	require.NotEmpty(t, index.Root.Leaves)
	require.Contains(t, index.Root.Leaves["a"].Leaves["b"].Leaves["c"].Clients, "client-1")
}

// This benchmark is Unsubscribe-Subscribe
func BenchmarkUnsubscribe(b *testing.B) {
	index := New()

	for n := 0; n < b.N; n++ {
		index.Subscribe("path/to/my/mqtt", "client-1", 0)
		index.Unsubscribe("path/to/mqtt/basic", "client-1")
	}
}

func TestSubscribersFind(t *testing.T) {
	tt := []struct {
		filter string
		topic  string
		len    int
	}{
		{
			filter: "a",
			topic:  "a",
			len:    1,
		},
		{
			filter: "a/",
			topic:  "a",
			len:    0,
		},
		{
			filter: "a/",
			topic:  "a/",
			len:    1,
		},
		{
			filter: "path/to/my/mqtt",
			topic:  "path/to/my/mqtt",
			len:    1,
		},
		{
			filter: "path/to/+/mqtt",
			topic:  "path/to/my/mqtt",
			len:    1,
		},
		{
			filter: "+/to/+/mqtt",
			topic:  "path/to/my/mqtt",
			len:    1,
		},
		{
			filter: "#",
			topic:  "path/to/my/mqtt",
			len:    1,
		},
		{
			filter: "+/+/+/+",
			topic:  "path/to/my/mqtt",
			len:    1,
		},
		{
			filter: "+/+/+/#",
			topic:  "path/to/my/mqtt",
			len:    1,
		},
		{
			filter: "zen/#",
			topic:  "zen",
			len:    1,
		},
		{
			filter: "+/+/#",
			topic:  "path/to/my/mqtt",
			len:    1,
		},
		{
			filter: "path/to/",
			topic:  "path/to/my/mqtt",
			len:    0,
		},
		{
			filter: "#/stuff",
			topic:  "path/to/my/mqtt",
			len:    0,
		},
	}

	for i, check := range tt {
		index := New()
		index.Subscribe(check.filter, "client-1", 0)
		clients := index.Subscribers(check.topic)
		//spew.Dump(clients)
		require.Equal(t, check.len, len(clients), "Unexpected clients len at %d %s %s", i, check.filter, check.topic)
	}

}

func BenchmarkSubscribers(b *testing.B) {
	index := New()
	index.Subscribe("path/to/my/mqtt", "client-1", 0)
	index.Subscribe("path/to/+/mqtt", "client-1", 0)
	index.Subscribe("something/things/stuff/+", "client-1", 0)
	index.Subscribe("path/to/stuff", "client-2", 0)
	index.Subscribe("#", "client-3", 0)

	for n := 0; n < b.N; n++ {
		index.Subscribers("path/to/testing/mqtt")
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

func TestRetainMessage(t *testing.T) {
	pk := &packets.PublishPacket{TopicName: "path/to/my/mqtt"}
	pk2 := &packets.PublishPacket{TopicName: "path/to/another/mqtt"}

	index := New()
	index.RetainMessage(pk)
	require.NotNil(t, index.Root.Leaves["path"].Leaves["to"].Leaves["my"].Leaves["mqtt"])
	require.Equal(t, pk, index.Root.Leaves["path"].Leaves["to"].Leaves["my"].Leaves["mqtt"].Message)

	index.Subscribe("path/to/another/mqtt", "client-1", 0)
	require.NotNil(t, index.Root.Leaves["path"].Leaves["to"].Leaves["another"].Leaves["mqtt"].Clients["client-1"])
	require.NotNil(t, index.Root.Leaves["path"].Leaves["to"].Leaves["another"].Leaves["mqtt"])

	index.RetainMessage(pk2)
	require.NotNil(t, index.Root.Leaves["path"].Leaves["to"].Leaves["another"].Leaves["mqtt"])
	require.Equal(t, pk2, index.Root.Leaves["path"].Leaves["to"].Leaves["another"].Leaves["mqtt"].Message)
	require.Contains(t, index.Root.Leaves["path"].Leaves["to"].Leaves["another"].Leaves["mqtt"].Clients, "client-1")
}

func BenchmarkRetainMessage(b *testing.B) {
	index := New()
	pk := &packets.PublishPacket{TopicName: "path/to/another/mqtt"}
	for n := 0; n < b.N; n++ {
		index.RetainMessage(pk)
	}
}

func TestMessagesPattern(t *testing.T) {

	tt := []struct {
		packet *packets.PublishPacket
		filter string
		len    int
	}{
		{
			&packets.PublishPacket{TopicName: "a/b/c/d"},
			"a/b/c/d",
			1,
		},
		{
			&packets.PublishPacket{TopicName: "a/b/c/e"},
			"a/+/c/+",
			2,
		},
		{
			&packets.PublishPacket{TopicName: "a/b/d/f"},
			"+/+/+/+",
			3,
		},
		{
			&packets.PublishPacket{TopicName: "q/w/e/r/t/y"},
			"q/w/e/#",
			1,
		},
		{
			&packets.PublishPacket{TopicName: "q/w/x/r/t/x"},
			"q/#",
			2,
		},
		{
			&packets.PublishPacket{TopicName: "asd"},
			"asd",
			1,
		},
		{
			&packets.PublishPacket{TopicName: "asd/fgh/jkl"},
			"#",
			8,
		},
		{
			&packets.PublishPacket{TopicName: "stuff/asdadsa/dsfdsafdsadfsa/dsfdsf/sdsadas"},
			"stuff/#/things", // indexer will ignore trailing /things
			1,
		},
	}
	index := New()

	for _, check := range tt {
		index.RetainMessage(check.packet)
	}

	for i, check := range tt {
		messages := index.Messages(check.filter)
		require.Equal(t, check.len, len(messages), "Unexpected messages len at %d %s %s", i, check.filter, check.packet.TopicName)
	}
}

func TestMessagesFind(t *testing.T) {
	index := New()
	index.RetainMessage(&packets.PublishPacket{TopicName: "a/a", Payload: []byte{'a'}})
	index.RetainMessage(&packets.PublishPacket{TopicName: "a/b", Payload: []byte{'b'}})
	messages := index.Messages("a/a")
	require.Equal(t, 1, len(messages))

	messages = index.Messages("a/+")
	require.Equal(t, 2, len(messages))
}

func BenchmarkMessages(b *testing.B) {
	index := New()
	index.RetainMessage(&packets.PublishPacket{TopicName: "path/to/my/mqtt"})
	index.RetainMessage(&packets.PublishPacket{TopicName: "path/to/another/mqtt"})
	index.RetainMessage(&packets.PublishPacket{TopicName: "path/a/some/mqtt"})
	index.RetainMessage(&packets.PublishPacket{TopicName: "what/is"})
	index.RetainMessage(&packets.PublishPacket{TopicName: "q/w/e/r/t/y"})

	for n := 0; n < b.N; n++ {
		index.Messages("path/to/+/mqtt")
	}
}
