// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package cluster

import (
	"sort"
	"sync"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

// localRoutes counts the local subscribers of each filter, so that a node announces a
// route when a filter gains its first subscriber, and withdraws it when the last one leaves.
// It is not safe for concurrent use; the cluster guards it with localMu.
type localRoutes map[string]map[string]struct{} // filter -> subscriber keys

// add adds a subscriber to a filter, returning true if the filter is a new route.
func (l localRoutes) add(filter, key string) bool {
	subs, ok := l[filter]
	if !ok {
		subs = map[string]struct{}{}
		l[filter] = subs
	}

	if _, ok := subs[key]; ok {
		return false
	}

	subs[key] = struct{}{}
	return len(subs) == 1
}

// remove removes a subscriber from a filter, returning true if the route was withdrawn.
func (l localRoutes) remove(filter, key string) bool {
	subs, ok := l[filter]
	if !ok {
		return false
	}

	if _, ok := subs[key]; !ok {
		return false
	}

	delete(subs, key)
	if len(subs) > 0 {
		return false
	}

	delete(l, filter)
	return true
}

// filters returns all local route filters, sorted.
func (l localRoutes) filters() []string {
	fs := make([]string, 0, len(l))
	for f := range l {
		fs = append(fs, f)
	}
	sort.Strings(fs)
	return fs
}

// remoteRoutes indexes the route filters announced by other nodes. Updates arrive over
// a node's inbound link, and are only applied if they come from that node's current link
// generation, so a stale connection can't overwrite a newer snapshot.
type remoteRoutes struct {
	mu    sync.Mutex
	next  uint64
	gen   map[string]uint64              // node -> current link generation
	nodes map[string]map[string]struct{} // node -> filters
	index *mqtt.TopicsIndex              // filters, with node names as subscriber ids
}

func newRemoteRoutes() *remoteRoutes {
	return &remoteRoutes{
		gen:   map[string]uint64{},
		nodes: map[string]map[string]struct{}{},
		index: mqtt.NewTopicsIndex(),
	}
}

// attach starts a new link generation for a node, returning its token.
func (r *remoteRoutes) attach(node string) uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.next++
	r.gen[node] = r.next
	return r.next
}

// current returns true if gen is the node's current link generation. r.mu must be held.
func (r *remoteRoutes) current(node string, gen uint64) bool {
	g, ok := r.gen[node]
	return ok && g == gen
}

// set replaces all of a node's routes.
func (r *remoteRoutes) set(node string, gen uint64, filters []string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.current(node, gen) {
		return false
	}

	want := make(map[string]struct{}, len(filters))
	for _, f := range filters {
		if mqtt.IsValidFilter(f, false) {
			want[f] = struct{}{}
		}
	}

	for f := range r.nodes[node] {
		if _, ok := want[f]; !ok {
			r.index.Unsubscribe(f, node)
		}
	}

	for f := range want {
		r.index.Subscribe(node, packets.Subscription{Filter: f})
	}

	r.nodes[node] = want
	return true
}

// add adds a route for a node.
func (r *remoteRoutes) add(node string, gen uint64, filter string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.current(node, gen) || !mqtt.IsValidFilter(filter, false) {
		return false
	}

	fs, ok := r.nodes[node]
	if !ok {
		fs = map[string]struct{}{}
		r.nodes[node] = fs
	}

	fs[filter] = struct{}{}
	r.index.Subscribe(node, packets.Subscription{Filter: filter})
	return true
}

// delete removes a route for a node.
func (r *remoteRoutes) delete(node string, gen uint64, filter string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.current(node, gen) {
		return false
	}

	if _, ok := r.nodes[node][filter]; !ok {
		return false
	}

	delete(r.nodes[node], filter)
	r.index.Unsubscribe(filter, node)
	return true
}

// purge removes all of a node's routes, and invalidates its current link generation.
func (r *remoteRoutes) purge(node string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for f := range r.nodes[node] {
		r.index.Unsubscribe(f, node)
	}
	delete(r.nodes, node)
	delete(r.gen, node)
}

// filters returns a node's route filters, sorted.
func (r *remoteRoutes) filters(node string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	fs := make([]string, 0, len(r.nodes[node]))
	for f := range r.nodes[node] {
		fs = append(fs, f)
	}
	sort.Strings(fs)
	return fs
}

// match returns the nodes with routes matching a topic. Subscriptions contains nodes with
// matching non-shared filters, and Shared contains the nodes with members of each matching
// shared subscription group.
func (r *remoteRoutes) match(topic string) *mqtt.Subscribers {
	return r.index.Subscribers(topic)
}
