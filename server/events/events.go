package events

import (
	"github.com/mochi-co/mqtt/server/internal/clients"
	"github.com/mochi-co/mqtt/server/internal/packets"
)

// Events provides callback handlers for different event hooks.
type Events struct {
	OnMessage    // published message receieved.
	OnConnect    // client connected.
	OnDisconnect // client disconnected.
}

// Packets is an alias for packets.Packet.
type Packet packets.Packet

// Client contains limited information about a connected client.
type Client struct {
	ID       string
	Listener string
}

// FromClient returns an event client from a client.
func FromClient(cl *clients.Client) Client {
	return Client{
		ID:       cl.ID,
		Listener: cl.Listener,
	}
}

// OnMessage function is called when a publish message is received. Note,
// this hook is ONLY called by connected client publishers, it is not triggered when
// using the direct s.Publish method. The function receives the sent message and the
// data of the client who published it, and allows the packet to be modified
// before it is dispatched to subscribers. If no modification is required, return
// the original packet data. If an error occurs, the original packet will
// be dispatched as if the event hook had not been triggered.
// This function will block message dispatching until it returns. To minimise this,
// have the function open a new goroutine on the embedding side.
type OnMessage func(Client, Packet) (Packet, error)

// OnConnect is called when a client successfully connects to the broker.
type OnConnect func(Client, Packet)

// OnDisconnect is called when a client disconnects to the broker. An error value
// is passed to the function if the client disconnected abnormally, otherwise it
// will be nil on a normal disconnect.
type OnDisconnect func(Client, error)
