package events

import (
	"github.com/mochi-co/mqtt/server/internal/clients"
	"github.com/mochi-co/mqtt/server/internal/packets"
)

type Events struct {
	OnMessage       // published message receieved.
	OnMessageModify // modify a received published message.
}

type Packet packets.Packet

type Client struct {
	ID       string
	Listener string
}

// FromClient returns an event client from a client.
func FromClient(cl clients.Client) Client {
	return Client{
		ID:       cl.ID,
		Listener: cl.Listener,
	}
}

// OnMessage function is called when a publish message is received. Note,
// this hook is ONLY called by connected client publishers, it is not triggered when
// using the direct s.Publish method. The function receives the sent message and the
// data of the client who published it. This function will block message dispatching
// until it returns. To minimise this, have the function open a new goroutine on the
// embedding side.
type OnMessage func(Client, Packet)

// OnMessageModify is the same as OnMessage except it allows the packet to be modified
// before it is dispatched to subscribers. If an error occurs, the original packet will
// be dispatched as if the event hook had not been triggered. Please implement your own
// error handling within the hook. This function will block message dispatching until it returns.
type OnMessageModify func(Client, Packet) (Packet, error)
