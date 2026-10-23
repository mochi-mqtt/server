// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package cluster

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLocalRoutesRefCount(t *testing.T) {
	l := localRoutes{}
	require.True(t, l.add("a/#", "cl1"))  // first subscriber announces the route
	require.False(t, l.add("a/#", "cl2")) // later subscribers don't
	require.False(t, l.add("a/#", "cl1")) // resubscribing doesn't
	require.Equal(t, []string{"a/#"}, l.filters())

	require.False(t, l.remove("a/#", "cl1"))
	require.False(t, l.remove("a/#", "cl1")) // already removed
	require.False(t, l.remove("b", "cl1"))   // never added
	require.True(t, l.remove("a/#", "cl2"))  // last subscriber withdraws the route
	require.Empty(t, l.filters())
}

func TestRemoteRoutesMatch(t *testing.T) {
	r := newRemoteRoutes()
	g2 := r.attach("n2")
	g3 := r.attach("n3")

	require.True(t, r.add("n2", g2, "a/+/c"))
	require.True(t, r.add("n3", g3, "a/#"))
	require.True(t, r.add("n3", g3, "$share/g/a/b/c"))
	require.False(t, r.add("n3", g3, "a/#/c")) // invalid filters are ignored

	subs := r.match("a/b/c")
	require.Contains(t, subs.Subscriptions, "n2")
	require.Contains(t, subs.Subscriptions, "n3")
	require.Contains(t, subs.Shared, "$share/g/a/b/c")
	require.Contains(t, subs.Shared["$share/g/a/b/c"], "n3")

	subs = r.match("x/y")
	require.Empty(t, subs.Subscriptions)
	require.Empty(t, subs.Shared)
}

func TestRemoteRoutesSetReplaces(t *testing.T) {
	r := newRemoteRoutes()
	g := r.attach("n2")
	require.True(t, r.add("n2", g, "old/topic"))
	require.True(t, r.set("n2", g, []string{"new/topic", "x/#"}))
	require.Equal(t, []string{"new/topic", "x/#"}, r.filters("n2"))
	require.Empty(t, r.match("old/topic").Subscriptions)
	require.Contains(t, r.match("new/topic").Subscriptions, "n2")
}

func TestRemoteRoutesStaleGeneration(t *testing.T) {
	r := newRemoteRoutes()
	old := r.attach("n2")
	cur := r.attach("n2") // the node reconnected
	require.True(t, r.set("n2", cur, []string{"a"}))

	require.False(t, r.add("n2", old, "b"))
	require.False(t, r.delete("n2", old, "a"))
	require.False(t, r.set("n2", old, nil))
	require.Equal(t, []string{"a"}, r.filters("n2"))
}

func TestRemoteRoutesDelete(t *testing.T) {
	r := newRemoteRoutes()
	g := r.attach("n2")
	require.True(t, r.add("n2", g, "a/b"))
	require.True(t, r.delete("n2", g, "a/b"))
	require.False(t, r.delete("n2", g, "a/b"))
	require.Empty(t, r.match("a/b").Subscriptions)
}

func TestRemoteRoutesPurge(t *testing.T) {
	r := newRemoteRoutes()
	g := r.attach("n2")
	require.True(t, r.add("n2", g, "a/b"))
	r.purge("n2")

	require.Empty(t, r.filters("n2"))
	require.Empty(t, r.match("a/b").Subscriptions)
	require.False(t, r.add("n2", g, "a/b")) // the purged link's updates are ignored
}
