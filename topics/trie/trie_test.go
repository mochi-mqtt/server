package trie

import (
	//"log"
	//"strings"
	"testing"

	//"github.com/davecgh/go-spew/spew"

	"github.com/stretchr/testify/require"
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

func TestSubscribe(t *testing.T) {
	index := New()
	index.Subscribe("path/to/my/mqtt", "client-1", 0)
	index.Subscribe("path/to/my/mqtt", "client-2", 0)
	index.Subscribe("path/to/another/mqtt", "client-1", 0)
	index.Subscribe("path/+", "client-2", 0)
	index.Subscribe("#", "client-3", 0)

	require.NotNil(t, index.Root.Leaves["path"].Leaves["to"].Leaves["my"].Leaves["mqtt"].Clients["client-1"])
	require.Equal(t, "path/to/my/mqtt", index.Root.Leaves["path"].Leaves["to"].Leaves["my"].Leaves["mqtt"].Filter)
	require.Equal(t, "mqtt", index.Root.Leaves["path"].Leaves["to"].Leaves["my"].Leaves["mqtt"].Key)
	require.Equal(t, index.Root.Leaves["path"], index.Root.Leaves["path"].Leaves["to"].Parent)
	require.NotNil(t, index.Root.Leaves["path"].Leaves["to"].Leaves["my"].Leaves["mqtt"].Clients["client-2"])
	require.NotNil(t, index.Root.Leaves["path"].Leaves["to"].Leaves["another"].Leaves["mqtt"].Clients["client-2"])
	require.NotNil(t, index.Root.Leaves["path"].Leaves["+"].Clients["client-2"])
	require.NotNil(t, index.Root.Leaves["#"].Clients["client-3"])

	//	spew.Dump(index.Root)
}

func BenchmarkSubscribe(b *testing.B) {
	index := New()
	for n := 0; n < b.N; n++ {
		index.Subscribe("path/to/mqtt/basic", "client-1", 0)
	}
}

func TestUnsubscribe(t *testing.T) {
	index := New()
	index.Subscribe("path/to/my/mqtt", "client-1", 0)
	index.Subscribe("path/to/+/mqtt", "client-1", 0)
	index.Subscribe("path/to/stuff", "client-1", 0)
	index.Subscribe("path/to/stuff", "client-2", 0)
	index.Subscribe("#", "client-3", 0)
	require.NotNil(t, index.Root.Leaves["path"].Leaves["to"].Leaves["my"].Leaves["mqtt"].Clients["client-1"])
	require.NotNil(t, index.Root.Leaves["path"].Leaves["to"].Leaves["+"].Leaves["mqtt"].Clients["client-1"])
	require.NotNil(t, index.Root.Leaves["path"].Leaves["to"].Leaves["stuff"].Clients["client-1"])
	require.NotNil(t, index.Root.Leaves["path"].Leaves["to"].Leaves["stuff"].Clients["client-2"])
	require.NotNil(t, index.Root.Leaves["#"].Clients["client-3"])

	ok := index.Unsubscribe("path/to/my/mqtt", "client-1")
	require.Equal(t, true, ok)
	require.Nil(t, index.Root.Leaves["path"].Leaves["to"].Leaves["my"])
	require.NotNil(t, index.Root.Leaves["path"].Leaves["to"].Leaves["+"].Leaves["mqtt"].Clients["client-1"])

	ok = index.Unsubscribe("path/to/stuff", "client-1")
	require.Equal(t, true, ok)
	require.NotNil(t, index.Root.Leaves["path"].Leaves["to"].Leaves["stuff"].Clients["client-1"])
	require.NotNil(t, index.Root.Leaves["path"].Leaves["to"].Leaves["stuff"].Clients["client-2"])
	require.NotNil(t, index.Root.Leaves["#"].Clients["client-3"])

	ok = index.Unsubscribe("fdasfdas/dfsfads/sa", "client-1")
	require.Equal(t, false, ok)

	//	spew.Dump(index.Root)
}

// This benchmark is Unsubscribe-Subscribe
func BenchmarkUnsubscribe(b *testing.B) {
	index := New()
	for n := 0; n < b.N; n++ {
		index.Subscribe("path/to/my/mqtt", "client-1", 0)
		index.Unsubscribe("path/to/mqtt/basic", "client-1")
	}
}

func TestSubscribers(t *testing.T) {
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
}

func BenchmarkIsolateParticle(b *testing.B) {
	for n := 0; n < b.N; n++ {
		isolateParticle("path/to/my/mqtt", 3)
	}
}
