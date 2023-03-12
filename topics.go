// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 J. Blake / mochi-co
// SPDX-FileContributor: mochi-co

package mqtt

import (
	"strings"
	"sync"
	"sync/atomic"

	"github.com/mochi-co/mqtt/v2/packets"
)

var (
	SharePrefix = "$SHARE" // the prefix indicating a share topic
	SysPrefix   = "$SYS"   // the prefix indicating a system info topic
)

// TopicAliases contains inbound and outbound topic alias registrations.
type TopicAliases struct {
	Inbound  *InboundTopicAliases
	Outbound *OutboundTopicAliases
}

// NewTopicAliases returns an instance of TopicAliases.
func NewTopicAliases(topicAliasMaximum uint16) TopicAliases {
	return TopicAliases{
		Inbound:  NewInboundTopicAliases(topicAliasMaximum),
		Outbound: NewOutboundTopicAliases(topicAliasMaximum),
	}
}

// NewInboundTopicAliases returns a pointer to InboundTopicAliases.
func NewInboundTopicAliases(topicAliasMaximum uint16) *InboundTopicAliases {
	return &InboundTopicAliases{
		maximum:  topicAliasMaximum,
		internal: map[uint16]string{},
	}
}

// InboundTopicAliases contains a map of topic aliases received from the client.
type InboundTopicAliases struct {
	internal map[uint16]string
	sync.RWMutex
	maximum uint16
}

// Set sets a new alias for a specific topic.
func (a *InboundTopicAliases) Set(id uint16, topic string) string {
	a.Lock()
	defer a.Unlock()

	if a.maximum == 0 {
		return topic // ?
	}

	if existing, ok := a.internal[id]; ok && topic == "" {
		return existing
	}

	a.internal[id] = topic
	return topic
}

// OutboundTopicAliases contains a map of topic aliases sent from the broker to the client.
type OutboundTopicAliases struct {
	internal map[string]uint16
	sync.RWMutex
	cursor  uint32
	maximum uint16
}

// NewOutboundTopicAliases returns a pointer to OutboundTopicAliases.
func NewOutboundTopicAliases(topicAliasMaximum uint16) *OutboundTopicAliases {
	return &OutboundTopicAliases{
		maximum:  topicAliasMaximum,
		internal: map[string]uint16{},
	}
}

// Set sets a new topic alias for a topic and returns the alias value, and a boolean
// indicating if the alias already existed.
func (a *OutboundTopicAliases) Set(topic string) (uint16, bool) {
	a.Lock()
	defer a.Unlock()

	if a.maximum == 0 {
		return 0, false
	}

	if i, ok := a.internal[topic]; ok {
		return i, true
	}

	i := atomic.LoadUint32(&a.cursor)
	if i+1 > uint32(a.maximum) {
		// if i+1 > math.MaxUint16 {
		return 0, false
	}

	a.internal[topic] = uint16(i) + 1
	atomic.StoreUint32(&a.cursor, i+1)
	return uint16(i) + 1, false
}

// SharedSubscriptions contains a map of subscriptions to a shared filter,
// keyed on share group then client id.
type SharedSubscriptions struct {
	internal map[string]map[string]packets.Subscription
	sync.RWMutex
}

// NewSharedSubscriptions returns a new instance of Subscriptions.
func NewSharedSubscriptions() *SharedSubscriptions {
	return &SharedSubscriptions{
		internal: map[string]map[string]packets.Subscription{},
	}
}

// Add creates a new shared subscription for a group and client id pair.
func (s *SharedSubscriptions) Add(group, id string, val packets.Subscription) {
	s.Lock()
	defer s.Unlock()
	if _, ok := s.internal[group]; !ok {
		s.internal[group] = map[string]packets.Subscription{}
	}
	s.internal[group][id] = val
}

// Delete deletes a client id from a shared subscription group.
func (s *SharedSubscriptions) Delete(group, id string) {
	s.Lock()
	defer s.Unlock()
	delete(s.internal[group], id)
	if len(s.internal[group]) == 0 {
		delete(s.internal, group)
	}
}

// Get returns the subscription properties for a client id in a share group, if one exists.
func (s *SharedSubscriptions) Get(group, id string) (val packets.Subscription, ok bool) {
	s.RLock()
	defer s.RUnlock()
	if _, ok := s.internal[group]; !ok {
		return val, ok
	}

	val, ok = s.internal[group][id]
	return val, ok
}

// GroupLen returns the number of groups subscribed to the filter.
func (s *SharedSubscriptions) GroupLen() int {
	s.RLock()
	defer s.RUnlock()
	val := len(s.internal)
	return val
}

// Len returns the total number of shared subscriptions to a filter across all groups.
func (s *SharedSubscriptions) Len() int {
	s.RLock()
	defer s.RUnlock()
	n := 0
	for _, group := range s.internal {
		n += len(group)
	}
	return n
}

// GetAll returns all shared subscription groups and their subscriptions.
func (s *SharedSubscriptions) GetAll() map[string]map[string]packets.Subscription {
	s.RLock()
	defer s.RUnlock()
	m := map[string]map[string]packets.Subscription{}
	for group, subs := range s.internal {
		if _, ok := m[group]; !ok {
			m[group] = map[string]packets.Subscription{}
		}

		for id, sub := range subs {
			m[group][id] = sub
		}
	}
	return m
}

// Subscriptions is a map of subscriptions keyed on client.
type Subscriptions struct {
	internal map[string]packets.Subscription
	sync.RWMutex
}

// NewSubscriptions returns a new instance of Subscriptions.
func NewSubscriptions() *Subscriptions {
	return &Subscriptions{
		internal: map[string]packets.Subscription{},
	}
}

// Add adds a new subscription for a client. ID can be a filter in the
// case this map is client state, or a client id if particle state.
func (s *Subscriptions) Add(id string, val packets.Subscription) {
	s.Lock()
	defer s.Unlock()
	s.internal[id] = val
}

// GetAll returns all subscriptions.
func (s *Subscriptions) GetAll() map[string]packets.Subscription {
	s.RLock()
	defer s.RUnlock()
	m := map[string]packets.Subscription{}
	for k, v := range s.internal {
		m[k] = v
	}
	return m
}

// Get returns a subscriptions for a specific client or filter id.
func (s *Subscriptions) Get(id string) (val packets.Subscription, ok bool) {
	s.RLock()
	defer s.RUnlock()
	val, ok = s.internal[id]
	return val, ok
}

// Len returns the number of subscriptions.
func (s *Subscriptions) Len() int {
	s.RLock()
	defer s.RUnlock()
	val := len(s.internal)
	return val
}

// Delete removes a subscription by client or filter id.
func (s *Subscriptions) Delete(id string) {
	s.Lock()
	defer s.Unlock()
	delete(s.internal, id)
}

// ClientSubscriptions is a map of aggregated subscriptions for a client.
type ClientSubscriptions map[string]packets.Subscription

// Subscribers contains the shared and non-shared subscribers matching a topic.
type Subscribers struct {
	Shared         map[string]map[string]packets.Subscription
	SharedSelected map[string]packets.Subscription
	Subscriptions  map[string]packets.Subscription
}

// SelectShared returns one subscriber for each shared subscription group.
func (s *Subscribers) SelectShared() {
	s.SharedSelected = map[string]packets.Subscription{}
	for _, subs := range s.Shared {
		for client, sub := range subs {
			cls, ok := s.SharedSelected[client]
			if !ok {
				cls = sub
			}

			s.SharedSelected[client] = cls.Merge(sub)
			break
		}
	}
}

// MergeSharedSelected merges the selected subscribers for a shared subscription group
// and the non-shared subscribers, to ensure that no subscriber gets multiple messages
// due to have both types of subscription matching the same filter.
func (s *Subscribers) MergeSharedSelected() {
	for client, sub := range s.SharedSelected {
		cls, ok := s.Subscriptions[client]
		if !ok {
			cls = sub
		}

		s.Subscriptions[client] = cls.Merge(sub)
	}
}

// TopicsIndex is a prefix/trie tree containing topic subscribers and retained messages.
type TopicsIndex struct {
	Retained *packets.Packets
	root     *particle // a leaf containing a message and more leaves.
}

// NewTopicsIndex returns a pointer to a new instance of Index.
func NewTopicsIndex() *TopicsIndex {
	return &TopicsIndex{
		Retained: packets.NewPackets(),
		root: &particle{
			particles:     newParticles(),
			subscriptions: NewSubscriptions(),
		},
	}
}

// Subscribe adds a new subscription for a client to a topic filter, returning
// true if the subscription was new.
func (x *TopicsIndex) Subscribe(client string, subscription packets.Subscription) bool {
	x.root.Lock()
	defer x.root.Unlock()

	var existed bool
	prefix, _ := isolateParticle(subscription.Filter, 0)
	if strings.EqualFold(prefix, SharePrefix) {
		group, _ := isolateParticle(subscription.Filter, 1)
		n := x.set(subscription.Filter, 2)
		_, existed = n.shared.Get(group, client)
		n.shared.Add(group, client, subscription)
	} else {
		n := x.set(subscription.Filter, 0)
		_, existed = n.subscriptions.Get(client)
		n.subscriptions.Add(client, subscription)
	}

	return !existed
}

// Unsubscribe removes a subscription filter for a client, returning true if the
// subscription existed.
func (x *TopicsIndex) Unsubscribe(filter, client string) bool {
	x.root.Lock()
	defer x.root.Unlock()

	var d int
	if strings.HasPrefix(filter, SharePrefix) {
		d = 2
	}

	particle := x.seek(filter, d)
	if particle == nil {
		return false
	}

	prefix, _ := isolateParticle(filter, 0)
	if strings.EqualFold(prefix, SharePrefix) {
		group, _ := isolateParticle(filter, 1)
		particle.shared.Delete(group, client)
	} else {
		particle.subscriptions.Delete(client)
	}

	x.trim(particle)
	return true
}

// RetainMessage saves a message payload to the end of a topic address. Returns
// 1 if a retained message was added, and -1 if the retained message was removed.
// 0 is returned if sequential empty payloads are received.
func (x *TopicsIndex) RetainMessage(pk packets.Packet) int64 {
	x.root.Lock()
	defer x.root.Unlock()

	n := x.set(pk.TopicName, 0)
	n.Lock()
	defer n.Unlock()
	if len(pk.Payload) > 0 {
		n.retainPath = pk.TopicName
		x.Retained.Add(pk.TopicName, pk)
		return 1
	}

	var out int64
	if pke, ok := x.Retained.Get(pk.TopicName); ok && len(pke.Payload) > 0 && pke.FixedHeader.Retain {
		out = -1 // if a retained packet existed, return -1
	}

	n.retainPath = ""
	x.Retained.Delete(pk.TopicName) // [MQTT-3.3.1-6] [MQTT-3.3.1-7]
	x.trim(n)

	return out
}

// set creates a topic address in the index and returns the final particle.
func (x *TopicsIndex) set(topic string, d int) *particle {
	var key string
	var hasNext = true
	n := x.root
	for hasNext {
		key, hasNext = isolateParticle(topic, d)
		d++

		p := n.particles.get(key)
		if p == nil {
			p = newParticle(key, n)
			n.particles.add(p)
		}
		n = p
	}

	return n
}

// seek finds the particle at a specific index in a topic filter.
func (x *TopicsIndex) seek(filter string, d int) *particle {
	var key string
	var hasNext = true
	n := x.root
	for hasNext {
		key, hasNext = isolateParticle(filter, d)
		n = n.particles.get(key)
		d++
		if n == nil {
			return nil
		}
	}

	return n
}

// trim removes empty filter particles from the index.
func (x *TopicsIndex) trim(n *particle) {
	for n.parent != nil && n.retainPath == "" && n.particles.len()+n.subscriptions.Len()+n.shared.Len() == 0 {
		key := n.key
		n = n.parent
		n.particles.delete(key)
	}
}

// Messages returns a slice of any retained messages which match a filter.
func (x *TopicsIndex) Messages(filter string) []packets.Packet {
	return x.scanMessages(filter, 0, nil, []packets.Packet{})
}

// scanMessages returns all retained messages on topics matching a given filter.
func (x *TopicsIndex) scanMessages(filter string, d int, n *particle, pks []packets.Packet) []packets.Packet {
	if n == nil {
		n = x.root
	}

	if len(filter) == 0 || x.Retained.Len() == 0 {
		return pks
	}

	if !strings.ContainsRune(filter, '#') && !strings.ContainsRune(filter, '+') {
		if pk, ok := x.Retained.Get(filter); ok {
			pks = append(pks, pk)
		}
		return pks
	}

	key, hasNext := isolateParticle(filter, d)
	if key == "+" || key == "#" || d == -1 {
		for _, adjacent := range n.particles.getAll() {
			if d == 0 && adjacent.key == SysPrefix {
				continue
			}

			if !hasNext {
				if adjacent.retainPath != "" {
					if pk, ok := x.Retained.Get(adjacent.retainPath); ok {
						pks = append(pks, pk)
					}
				}
			}

			if hasNext || (d >= 0 && key == "#") {
				pks = x.scanMessages(filter, d+1, adjacent, pks)
			}
		}
		return pks
	}

	if particle := n.particles.get(key); particle != nil {
		if hasNext {
			return x.scanMessages(filter, d+1, particle, pks)
		}

		if pk, ok := x.Retained.Get(particle.retainPath); ok {
			pks = append(pks, pk)
		}
	}

	return pks
}

// Subscribers returns a map of clients who are subscribed to matching filters,
// their subscription ids and highest qos.
func (x *TopicsIndex) Subscribers(topic string) *Subscribers {
	return x.scanSubscribers(topic, 0, nil, &Subscribers{
		Shared:         map[string]map[string]packets.Subscription{},
		SharedSelected: map[string]packets.Subscription{},
		Subscriptions:  map[string]packets.Subscription{},
	})
}

// scanSubscribers returns a list of client subscriptions matching an indexed topic address.
func (x *TopicsIndex) scanSubscribers(topic string, d int, n *particle, subs *Subscribers) *Subscribers {
	if n == nil {
		n = x.root
	}

	if len(topic) == 0 {
		return subs
	}

	key, hasNext := isolateParticle(topic, d)
	for _, partKey := range []string{key, "+", "#"} {
		if particle := n.particles.get(partKey); particle != nil { // [MQTT-3.3.2-3]
			x.gatherSubscriptions(topic, particle, subs)
			x.gatherSharedSubscriptions(particle, subs)
			if wild := particle.particles.get("#"); wild != nil && partKey != "#" && partKey != "+" {
				x.gatherSubscriptions(topic, wild, subs) // also match any subs where filter/# is filter as per 4.7.1.2
			}

			if hasNext {
				x.scanSubscribers(topic, d+1, particle, subs)
			}
		}
	}

	return subs
}

// gatherSubscriptions collects any matching subscriptions, and gathers any identifiers or highest qos values.
func (x *TopicsIndex) gatherSubscriptions(topic string, particle *particle, subs *Subscribers) {
	if subs.Subscriptions == nil {
		subs.Subscriptions = map[string]packets.Subscription{}
	}

	for client, sub := range particle.subscriptions.GetAll() {
		if len(sub.Filter) > 0 && topic[0] == '$' && (sub.Filter[0] == '+' || sub.Filter[0] == '#') { // don't match $ topics with top level wildcards [MQTT-4.7.1-1] [MQTT-4.7.1-2]
			continue
		}

		cls, ok := subs.Subscriptions[client]
		if !ok {
			cls = sub
		}

		subs.Subscriptions[client] = cls.Merge(sub)
	}
}

// gatherSharedSubscriptions gathers all shared subscriptions for a particle.
func (x *TopicsIndex) gatherSharedSubscriptions(particle *particle, subs *Subscribers) {
	if subs.Shared == nil {
		subs.Shared = map[string]map[string]packets.Subscription{}
	}

	for _, shares := range particle.shared.GetAll() {
		for client, sub := range shares {
			if _, ok := subs.Shared[sub.Filter]; !ok {
				subs.Shared[sub.Filter] = map[string]packets.Subscription{}
			}

			subs.Shared[sub.Filter][client] = sub
		}
	}
}

// isolateParticle extracts a particle between d / and d+1 / without allocations.
func isolateParticle(filter string, d int) (particle string, hasNext bool) {
	var next, end int
	for i := 0; end > -1 && i <= d; i++ {
		end = strings.IndexRune(filter, '/')

		switch {
		case d > -1 && i == d && end > -1:
			hasNext = true
			particle = filter[next:end]
		case end > -1:
			hasNext = false
			filter = filter[end+1:]
		default:
			hasNext = false
			particle = filter[next:]
		}
	}

	return
}

// IsSharedFilter returns true if the filter uses the share prefix.
func IsSharedFilter(filter string) bool {
	prefix, _ := isolateParticle(filter, 0)
	return strings.EqualFold(prefix, SharePrefix)
}

// IsValidFilter returns true if the filter is valid.
func IsValidFilter(filter string, forPublish bool) bool {
	if !forPublish && len(filter) == 0 { // publishing can accept zero-length topic filter if topic alias exists, so we don't enforce for publihs.
		return false // [MQTT-4.7.3-1]
	}

	if forPublish {
		if len(filter) >= len(SysPrefix) && strings.EqualFold(filter[0:len(SysPrefix)], SysPrefix) {
			// 4.7.2 Non-normative - The Server SHOULD prevent Clients from using such Topic Names [$SYS] to exchange messages with other Clients.
			return false
		}

		if strings.ContainsRune(filter, '+') || strings.ContainsRune(filter, '#') {
			return false //[MQTT-3.3.2-2]
		}
	}

	wildhash := strings.IndexRune(filter, '#')
	if wildhash >= 0 && wildhash != len(filter)-1 { // [MQTT-4.7.1-2]
		return false
	}

	prefix, hasNext := isolateParticle(filter, 0)
	if !hasNext && strings.EqualFold(prefix, SharePrefix) {
		return false // [MQTT-4.8.2-1]
	}

	if hasNext && strings.EqualFold(prefix, SharePrefix) {
		group, hasNext := isolateParticle(filter, 1)
		if !hasNext {
			return false // [MQTT-4.8.2-1]
		}

		if strings.ContainsRune(group, '+') || strings.ContainsRune(group, '#') {
			return false // [MQTT-4.8.2-2]
		}
	}

	return true
}

// particle is a child node on the tree.
type particle struct {
	key           string               // the key of the particle
	parent        *particle            // a pointer to the parent of the particle
	particles     particles            // a map of child particles
	subscriptions *Subscriptions       // a map of subscriptions made by clients to this ending address
	shared        *SharedSubscriptions // a map of shared subscriptions keyed on group name
	retainPath    string               // path of a retained message
	sync.Mutex                         // mutex for when making changes to the particle
}

// newParticle returns a pointer to a new instance of particle.
func newParticle(key string, parent *particle) *particle {
	return &particle{
		key:           key,
		parent:        parent,
		particles:     newParticles(),
		subscriptions: NewSubscriptions(),
		shared:        NewSharedSubscriptions(),
	}
}

// particles is a concurrency safe map of particles.
type particles struct {
	internal map[string]*particle
	sync.RWMutex
}

// newParticles returns a map of particles.
func newParticles() particles {
	return particles{
		internal: map[string]*particle{},
	}
}

// add adds a new particle.
func (p *particles) add(val *particle) {
	p.Lock()
	p.internal[val.key] = val
	p.Unlock()
}

// getAll returns all particles.
func (p *particles) getAll() map[string]*particle {
	p.RLock()
	defer p.RUnlock()
	m := map[string]*particle{}
	for k, v := range p.internal {
		m[k] = v
	}
	return m
}

// get returns a particle by id (key).
func (p *particles) get(id string) *particle {
	p.RLock()
	defer p.RUnlock()
	return p.internal[id]
}

// len returns the number of particles.
func (p *particles) len() int {
	p.RLock()
	defer p.RUnlock()
	val := len(p.internal)
	return val
}

// delete removes a particle.
func (p *particles) delete(id string) {
	p.Lock()
	defer p.Unlock()
	delete(p.internal, id)
}
