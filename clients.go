package mqtt

import (
	"errors"
	"sync"

	"github.com/rs/xid"

	"github.com/mochi-co/mqtt/packets"
)

var (
	ErrConnectionClosed = errors.New("Connection not open")
)

const (
	// defaultKeepalive is the default keepalive time in seconds.
	defaultKeepalive uint16 = 60
)

// clients contains data about the clients known by the broker.
type clients struct {
}

// Client contains information about a client known by the broker.
type client struct {
	sync.RWMutex

	// p is a packets parser which reads incoming packets.
	p *packets.Parser

	// end is a channel that indicates the client should halt.
	end chan struct{}

	// done can be called to ensure the close methods are only called once.
	done *sync.Once

	// id is the client id.
	id string

	// user is the username the client authenticated with.
	user string

	// keepalive is the number of seconds the connection can stay open without
	// receiving a message from the client.
	keepalive uint16

	// cleanSession indicates if the client expects a cleansession.
	cleanSession bool
}

// newClient creates a new instance of client.
func newClient(p *packets.Parser, pk *packets.ConnectPacket) *client {

	cl := &client{
		p:    p,
		end:  make(chan struct{}),
		done: new(sync.Once),

		id:           pk.ClientIdentifier,
		user:         pk.Username,
		keepalive:    pk.Keepalive,
		cleanSession: pk.CleanSession,
	}

	// If no client id was provided, generate a new one.
	if cl.id == "" {
		cl.id = xid.New().String()
	}

	// if no deadline value was provided, set it to the default seconds.
	if cl.keepalive == 0 {
		cl.keepalive = defaultKeepalive
	}

	// If a last will and testament has been provided, record it.
	/*if pk.WillFlag {
		// @TODO ...
		client.will = lwt{
			topic:   pk.WillTopic,
			message: pk.WillMessage,
			qos:     pk.WillQos,
			retain:  pk.WillRetain,
		}
	}
	*/

	return cl
}
