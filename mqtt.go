package mqtt

import (
	"github.com/mochi-co/mqtt/listeners"
	"github.com/mochi-co/mqtt/topics"
)

const (
	// maxPacketID is the maximum value of a 16-bit packet ID.
	maxPacketID = 65535
)

// Broker is an MQTT broker server.
type Broker struct {
	listeners listeners.Listeners // listeners listen for new connections.
	clients   clients             // clients known to the broker.
	topics    topics.Index        // an index of topic subscriptions and retained messages.
}

// New returns a new instance of an MQTT broker.
func New() *Broker {
	return &Broker{
		listeners: listeners.NewListeners(),
		topics:    topics.New(),
	}
}
