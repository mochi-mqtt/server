package trie

import (
	"strings"
	"sync"

	"github.com/mochi-co/mqtt/packets"
	"github.com/mochi-co/mqtt/topics"
)

// Index is a prefix/trie tree containing topic subscribers and retained messages.
type Index struct {
	Root *Leaf
}

// New returns a pointer to a new instance of Index.
func New() *Index {
	return &Index{
		Root: &Leaf{
			Leaves:  make(map[string]*Leaf),
			Clients: make(map[string]byte),
		},
	}
}

// Subscribe creates a subscription filter for a client.
func (x *Index) Subscribe(filter, client string, qos byte) {
	n := x.poperate(filter)
	n.Lock()
	n.Clients[client] = qos
	n.Filter = filter
	n.Unlock()
}

// RetainMessage saves a message payload to the end of a topic branch.
func (x *Index) RetainMessage(msg *packets.PublishPacket) {
	n := x.poperate(msg.TopicName)
	n.Lock()
	n.Message = msg
	n.Unlock()
}

// poperate iterates and populates through a topic/filter path, instantiating
// leaves as it goes and returning the final leaf in the branch.
// poperate is a more enjoyable word than iterpop.
func (x *Index) poperate(topic string) *Leaf {
	var d int
	var particle string
	var hasNext = true

	n := x.Root
	for hasNext {
		particle, hasNext = isolateParticle(topic, d)
		d++

		n.Lock()
		child, _ := n.Leaves[particle]
		if child == nil {
			child = &Leaf{
				Key:     particle,
				Parent:  n,
				Leaves:  make(map[string]*Leaf),
				Clients: make(map[string]byte),
			}
			n.Leaves[particle] = child
		}
		n.Unlock()
		n = child
	}

	return n
}

// Unsubscribe removes a subscription filter for a client. Returns true if an
// unsubscribe action sucessful.
func (x *Index) Unsubscribe(filter, client string) bool {

	// Walk to end leaf.
	var d int
	var particle string
	var hasNext = true
	e := x.Root
	for hasNext {
		particle, hasNext = isolateParticle(filter, d)
		d++
		e, _ = e.Leaves[particle]

		// If the topic part doesn't exist in the tree, there's nothing
		// left to do.
		if e == nil {
			return false
		}
	}

	// Step backward removing client and orphaned leaves.
	var key string
	var orphaned bool
	for e.Parent != nil {
		key = e.Key

		// Wipe the client from this leaf.
		delete(e.Clients, client)

		// If this leaf is empty, note it as orphaned.
		orphaned = len(e.Clients) == 0 && len(e.Leaves) == 0

		// Traverse up the branch.
		e = e.Parent

		// If the leaf we just came from was empty, delete it.
		if orphaned {
			delete(e.Leaves, key)
		}
	}

	return true
}

// Subscribers returns a map of clients who are subscribed to matching filters.
func (x *Index) Subscribers(topic string) topics.Subscription {
	return x.Root.scanSubscribers(topic, 0, make(topics.Subscription))
}

// Messages returns a slice of retained topic messages which match a filter.
func (x *Index) Messages(filter string) []*packets.PublishPacket {
	return x.Root.scanMessages(filter, 0, make([]*packets.PublishPacket, 0, 32))
}

// Leaf is a child node on the tree.
type Leaf struct {
	sync.RWMutex

	// Key contains the key that was used to create the leaf.
	Key string

	// Parent is a pointer to the parent node for the leaf.
	Parent *Leaf

	// Leafs is a map of child nodes, keyed on particle id.
	Leaves map[string]*Leaf

	// Clients is a map of client ids subscribed to the topic.
	Clients map[string]byte

	// Filter is the path of the topic filter being matched.
	Filter string

	// Message is a message which has been retained for a specific topic.
	Message *packets.PublishPacket
}

// scanSubscribers recursively steps through a branch of leaves finding clients who
// have subscription filters matching a topic, and their highest QoS byte.
func (l *Leaf) scanSubscribers(topic string, d int, clients topics.Subscription) topics.Subscription {
	l.RLock()
	part, hasNext := isolateParticle(topic, d)

	// For either the topic part, a +, or a #, follow the branch.
	for _, particle := range []string{part, "+", "#"} {
		if child, ok := l.Leaves[particle]; ok {

			// We're only interested in getting clients from the final
			// element in the topic, or those with wildhashes.
			if !hasNext || particle == "#" {

				// Capture the highest QOS byte for any client with a filter
				// matching the topic.
				for client, qos := range child.Clients {
					if ex, ok := clients[client]; !ok || ex < qos {
						clients[client] = qos
					}
				}

				// Make sure we also capture any client who are listening
				// to this topic via path/#
				if !hasNext {
					if extra, ok := child.Leaves["#"]; ok {
						for client, qos := range extra.Clients {
							if ex, ok := clients[client]; !ok || ex < qos {
								clients[client] = qos
							}
						}
					}
				}
			}

			// If this branch has hit a wildhash, just return immediately.
			if particle == "#" {
				return clients
			} else if hasNext {
				// Otherwise continue down the branch.
				clients = child.scanSubscribers(topic, d+1, clients)
			}
		}
	}

	l.RUnlock()
	return clients
}

// scanMessages recursively steps through a branch of leaves finding retained messages
// that match a topic filter. Setting `d` to -1 will enable wildhash mode, and will
// recursively check ALL child leaves in every subsequent branch.
func (l *Leaf) scanMessages(filter string, d int, messages []*packets.PublishPacket) []*packets.PublishPacket {

	l.RLock()

	// If a wildhash mode has been set, continue recursively checking through all
	// child leaves regardless of their particle key.
	if d == -1 {
		for _, child := range l.Leaves {
			if child.Message != nil {
				messages = append(messages, child.Message)
			}
			messages = child.scanMessages(filter, -1, messages)
		}
		return messages
	}

	// Otherwise, we'll get the particle for d in the filter.
	particle, hasNext := isolateParticle(filter, d)

	// If there's no more particles after this one, then take the messages from
	// these topics.
	if !hasNext {

		// If it's a specific particle, we only need the single message.
		if child, ok := l.Leaves[particle]; ok {
			if child.Message != nil {
				messages = append(messages, child.Message)
			}

		} else if particle == "+" || particle == "#" {
			// Otherwise, if it's a wildcard or wildhash, get messages from all
			// the child leaves. This wildhash captures messages on the actual
			// wildhash position, whereas the d == -1 block collects subsequent
			// messages further down the branch.
			for _, child := range l.Leaves {
				if child.Message != nil {
					messages = append(messages, child.Message)
				}
			}
		}
	} else {

		// If it's not the last particle, branch out to the next leaves, scanning
		// all available if it's a wildcard, or just one if it's a specific particle.
		if particle == "+" {
			for _, child := range l.Leaves {
				messages = child.scanMessages(filter, d+1, messages)
			}
		} else if child, ok := l.Leaves[particle]; ok {
			messages = child.scanMessages(filter, d+1, messages)
		}
	}

	// If the particle was a wildhash, scan all the child leaves setting the
	// d value to wildhash mode.
	if particle == "#" {
		for _, child := range l.Leaves {
			messages = child.scanMessages(filter, -1, messages)
		}
	}

	l.RUnlock()

	return messages
}

// isolateParticle extracts a particle between d / and d+1 / without allocations.
func isolateParticle(filter string, d int) (particle string, hasNext bool) {
	var next, end int
	for i := 0; end > -1 && i <= d; i++ {
		end = strings.IndexRune(filter, '/')
		if d > -1 && i == d && end > -1 {
			hasNext = true
			particle = filter[next:end]
		} else if end > -1 {
			hasNext = false
			filter = filter[end+1:]
		} else {
			hasNext = false
			particle = filter[next:]
		}
	}

	return
}
