package trie

import (
	"strings"
	//"github.com/mochi-co/mqtt/packets"
)

// Index is a prefix/trie tree containing topic subscribers and retained messages.
type Index struct {
	Root *Leaf
}

// Leaf is a child node on the tree.
type Leaf struct {
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
	parts := strings.Split(filter, "/")

	n := x.Root
	for i := 0; i < len(parts); i++ {
		child, _ := n.Leaves[parts[i]]
		if child == nil {
			child = &Leaf{
				Key:     parts[i],
				Parent:  n,
				Leaves:  make(map[string]*Leaf),
				Clients: make(map[string]byte),
			}
			n.Leaves[parts[i]] = child
		}

		n = child
	}

	n.Clients[client] = qos
	n.Filter = filter

}

// Unsubscribe removes a subscription filter for a client.
func (x *Index) Unsubscribe(filter, client string) bool {
	parts := strings.Split(filter, "/")

	// Walk to end leaf.
	e := x.Root
	for i := 0; i < len(parts); i++ {
		e, _ = e.Leaves[parts[i]]
		if e == nil {
			return false
		}
	}

	// Step backward removing client and orphaned leaves.
	var orphaned bool
	var key string
	for e.Parent != nil {
		key = e.Key
		delete(e.Clients, client)
		orphaned = len(e.Clients) == 0 && len(e.Leaves) == 0
		e = e.Parent
		if orphaned {
			delete(e.Leaves, key)
		}
	}

	return true
}

type clientR struct {
	id  string
	qos byte
}

// Subscribers returns a map of clients who are subscribed to matching filters.
func (x *Index) Subscribers(topic string) []clientR {
	parts := strings.Split(topic, "/")
	clients := x.Root.scanBranch(parts, 0, []clientR{})
	return clients
}

func (l *Leaf) scanBranch(parts []string, d int, clients []clientR) []clientR {
	ix := len(parts)
	if d < ix {
		for _, particle := range []string{parts[d], "+", "#"} {
			if child, ok := l.Leaves[particle]; ok {
				if d == ix-1 || particle == "#" {
					for client, qos := range child.Clients {
						clients = append(clients, clientR{
							id:  client,
							qos: qos,
						})
					}
					if d == ix-1 {
						if extra, ok := child.Leaves["#"]; ok {
							for client, qos := range extra.Clients {
								clients = append(clients, clientR{
									id:  client,
									qos: qos,
								})
							}
						}
					}
				}
				if particle == "#" {
					return clients
				} else {
					clients = child.scanBranch(parts, d+1, clients)
				}
			}
		}
	}

	return clients
}
