package topics

import (
	"github.com/mochi-co/mqtt/packets"
)

// Indexer indexes filter subscriptions and retained messages.
type Indexer interface {

	// Subscribers returns the clients with filters matching a topic.
	Subscribers(topic string) Subscriptions

	// Subscribe adds a new filter subscription for a client.
	Subscribe(filter string, client string, qos byte)

	// Unsubscribe removes a filter subscription for a client.
	Unsubscribe(filter string, client string) bool

	// Messages returns message payloads retained for topics matching a filter.
	Messages(topic string) []*packets.PublishPacket

	// Retain retains a message payload for a topic.
	RetainMessage(packet *packets.PublishPacket)
}

// Subscriptions is a map of subscriptions keyed on client.
type Subscriptions map[string]byte
