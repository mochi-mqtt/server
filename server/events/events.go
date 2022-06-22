package events

import (
	"github.com/mochi-co/mqtt/server/internal/packets"
)

// Events provides callback handlers for different event hooks.
type Events struct {
	OnProcessMessage // published message receieved before evaluation.
	OnMessage        // published message receieved.
	OnError          // server error.
	OnConnect        // client connected.
	OnDisconnect     // client disconnected.
	OnSubscribe      // topic subscription created.
	OnUnsubscribe    // topic subscription removed.
}

// Packets is an alias for packets.Packet.
type Packet packets.Packet

// Client contains limited information about a connected client.
type Client struct {
	ID           string
	Remote       string
	Listener     string
	Username     []byte
	CleanSession bool
}

// Clientlike is an interface for Clients and client-like objects that
// are able to describe their client/listener IDs and remote address.
type Clientlike interface {
	Info() Client
}

// OnProcessMessage is called when a publish message is received, allowing modification
// of the packet data after ACL checking has occurred but before any data is evaluated
// for processing - e.g. for changing the Retain flag. Note, this hook is ONLY called
// by connected client publishers, it is not triggered when using the direct
// s.Publish method. The function receives the sent message and the
// data of the client who published it, and allows the packet to be modified
// before it is dispatched to subscribers. If no modification is required, return
// the original packet data. If an error occurs, the original packet will
// be dispatched as if the event hook had not been triggered.
// This function will block message dispatching until it returns. To minimise this,
// have the function open a new goroutine on the embedding side.
// The `mqtt.ErrRejectPacket` error can be returned to reject and abandon any further
// processing of the packet.
type OnProcessMessage func(Client, Packet) (Packet, error)

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

// OnError is called when errors that will not be passed to
// OnDisconnect are handled by the server.
type OnError func(Client, error)

// OnSubscribe is called when a new subscription filter for a client is created.
type OnSubscribe func(filter string, cl Client, qos byte)

// OnUnsubscribe is called when an existing subscription filter for a client is removed.
type OnUnsubscribe func(filter string, cl Client)
