package topics

// Indexer indexes filter subscriptions and retained messages.
type Indexer interface {

	// Subscribers returns the clients with filters matching a topic.
	Subscribers()

	// Subscribe adds a new filter subscription for a client.
	Subscribe()

	// Unsubscribe removes a filter subscription for a client.
	Unsubscribe()

	// Messages returns any message payloads retained for topics matching a
	// filter.
	Messages()

	// Retain retains a message payload for a topic.
	RetainMessage()
}
