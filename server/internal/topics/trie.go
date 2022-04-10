package topics

import (
	"strings"
	"sync"

	"github.com/mochi-co/mqtt/server/internal/packets"
)

// Subscriptions is a map of subscriptions keyed on client.
type Subscriptions map[string]byte

// Index is a prefix/trie tree containing topic subscribers and retained messages.
type Index struct {
	mu   sync.RWMutex // a mutex for locking the whole index.
	Root *Leaf        // a leaf containing a message and more leaves.
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

// RetainMessage saves a message payload to the end of a topic branch. Returns
// 1 if a retained message was added, and -1 if the retained message was removed.
// 0 is returned if sequential empty payloads are received.
func (x *Index) RetainMessage(msg packets.Packet) int64 {
	x.mu.Lock()
	defer x.mu.Unlock()
	n := x.poperate(msg.TopicName)

	// If there is a payload, we can store it.
	if len(msg.Payload) > 0 {
		n.Message = msg
		return 1
	}

	// Otherwise, we are unsetting it.
	// If there was a previous retained message, return -1 instead of 0.
	var r int64 = 0
	if len(n.Message.Payload) > 0 && n.Message.FixedHeader.Retain == true {
		r = -1
	}
	x.unpoperate(msg.TopicName, "", true)

	return r
}

// Subscribe creates a subscription filter for a client. Returns true if the
// subscription was new.
func (x *Index) Subscribe(filter, client string, qos byte) bool {
	x.mu.Lock()
	defer x.mu.Unlock()

	n := x.poperate(filter)
	_, ok := n.Clients[client]
	n.Clients[client] = qos
	n.Filter = filter

	return !ok
}

// Unsubscribe removes a subscription filter for a client. Returns true if an
// unsubscribe action successful and the subscription existed.
func (x *Index) Unsubscribe(filter, client string) bool {
	x.mu.Lock()
	defer x.mu.Unlock()

	n := x.poperate(filter)
	_, ok := n.Clients[client]

	return x.unpoperate(filter, client, false) && ok
}

// unpoperate steps backward through a trie sequence and removes any orphaned
// nodes. If a client id is specified, it will unsubscribe a client. If message
// is true, it will delete a retained message.
func (x *Index) unpoperate(filter string, client string, message bool) bool {
	var d int // Walk to end leaf.
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
	var end = true
	for e.Parent != nil {
		key = e.Key

		// Wipe the client from this leaf if it's the filter end.
		if end {
			if client != "" {
				delete(e.Clients, client)
			}
			if message {
				e.Message = packets.Packet{}
			}
			end = false
		}

		// If this leaf is empty, note it as orphaned.
		orphaned = len(e.Clients) == 0 && len(e.Leaves) == 0 && !e.Message.FixedHeader.Retain

		// Traverse up the branch.
		e = e.Parent

		// If the leaf we just came from was empty, delete it.
		if orphaned {
			delete(e.Leaves, key)
		}
	}

	return true

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
		n = child
	}

	return n
}

// Subscribers returns a map of clients who are subscribed to matching filters.
func (x *Index) Subscribers(topic string) Subscriptions {
	x.mu.RLock()
	defer x.mu.RUnlock()
	return x.Root.scanSubscribers(topic, 0, make(Subscriptions))
}

// Messages returns a slice of retained topic messages which match a filter.
func (x *Index) Messages(filter string) []packets.Packet {
	// ReLeaf("messages", x.Root, 0)
	x.mu.RLock()
	defer x.mu.RUnlock()
	return x.Root.scanMessages(filter, 0, make([]packets.Packet, 0, 32))
}

// Leaf is a child node on the tree.
type Leaf struct {
	Message packets.Packet   // a message which has been retained for a specific topic.
	Key     string           // the key that was used to create the leaf.
	Filter  string           // the path of the topic filter being matched.
	Parent  *Leaf            // a pointer to the parent node for the leaf.
	Leaves  map[string]*Leaf // a map of child nodes, keyed on particle id.
	Clients map[string]byte  // a map of client ids subscribed to the topic.
}

// scanSubscribers recursively steps through a branch of leaves finding clients who
// have subscription filters matching a topic, and their highest QoS byte.
func (l *Leaf) scanSubscribers(topic string, d int, clients Subscriptions) Subscriptions {
	part, hasNext := isolateParticle(topic, d)

	// For either the topic part, a +, or a #, follow the branch.
	for _, particle := range []string{part, "+", "#"} {

		// Topics beginning with the reserved $ character are restricted from
		// being returned for top level wildcards.
		if d == 0 && len(part) > 0 && part[0] == '$' && (particle == "+" || particle == "#") {
			continue
		}

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
				clients = child.scanSubscribers(topic, d+1, clients)
			}
		}
	}

	return clients
}

// scanMessages recursively steps through a branch of leaves finding retained messages
// that match a topic filter. Setting `d` to -1 will enable wildhash mode, and will
// recursively check ALL child leaves in every subsequent branch.
func (l *Leaf) scanMessages(filter string, d int, messages []packets.Packet) []packets.Packet {

	// If a wildhash mode has been set, continue recursively checking through all
	// child leaves regardless of their particle key.
	if d == -1 {
		for _, child := range l.Leaves {
			if child.Message.FixedHeader.Retain {
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

		// Wildcards and Wildhashes must be checked first, otherwise they
		// may be detected as standard particles, and not act properly.
		if particle == "+" || particle == "#" {

			// Otherwise, if it's a wildcard or wildhash, get messages from all
			// the child leaves. This wildhash captures messages on the actual
			// wildhash position, whereas the d == -1 block collects subsequent
			// messages further down the branch.
			for _, child := range l.Leaves {
				if d == 0 && len(child.Key) > 0 && child.Key[0] == '$' {
					continue
				}
				if child.Message.FixedHeader.Retain {
					messages = append(messages, child.Message)
				}
			}
		} else if child, ok := l.Leaves[particle]; ok {
			if child.Message.FixedHeader.Retain {
				messages = append(messages, child.Message)
			}
		}

	} else {

		// If it's not the last particle, branch out to the next leaves, scanning
		// all available if it's a wildcard, or just one if it's a specific particle.
		if particle == "+" {
			for _, child := range l.Leaves {
				if d == 0 && len(child.Key) > 0 && child.Key[0] == '$' {
					continue
				}
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
			if d == 0 && len(child.Key) > 0 && child.Key[0] == '$' {
				continue
			}
			messages = child.scanMessages(filter, -1, messages)
		}
	}

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

// ReLeaf is a dev function for showing the trie leafs.
/*
func ReLeaf(m string, leaf *Leaf, d int) {
	for k, v := range leaf.Leaves {
		fmt.Println(m, d, strings.Repeat("  ", d), k)
		ReLeaf(m, v, d+1)
	}
}
*/
