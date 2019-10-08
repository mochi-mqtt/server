package mqtt

import (
	"sync"
	"sync/atomic"

	"github.com/rs/xid"

	"github.com/mochi-co/mqtt/auth"
	"github.com/mochi-co/mqtt/packets"
)

var (
	// defaultClientKeepalive is the default keepalive time in seconds.
	defaultClientKeepalive uint16 = 60
)

// clients contains a map of the clients known by the broker.
type clients struct {
	sync.RWMutex

	// internal is a map of the clients known by the broker, keyed on client id.
	internal map[string]*client
}

// newClients returns an instance of clients.
func newClients() clients {
	return clients{
		internal: make(map[string]*client),
	}
}

// Add adds a new client to the clients map, keyed on client id.
func (cl *clients) add(val *client) {
	cl.Lock()
	cl.internal[val.id] = val
	cl.Unlock()
}

// Get returns the value of a client if it exists.
func (cl *clients) get(id string) (*client, bool) {
	cl.RLock()
	val, ok := cl.internal[id]
	cl.RUnlock()
	return val, ok
}

// Len returns the length of the clients map.
func (cl *clients) len() int {
	cl.RLock()
	val := len(cl.internal)
	cl.RUnlock()
	return val
}

// Delete removes a client from the internal map.
func (cl *clients) delete(id string) {
	cl.Lock()
	delete(cl.internal, id)
	cl.Unlock()
}

// getByListener returns clients matching a listener id.
func (cl *clients) getByListener(id string) []*client {
	clients := make([]*client, 0, cl.len())
	cl.RLock()
	for _, v := range cl.internal {
		if v.listener == id {
			clients = append(clients, v)
		}
	}
	cl.RUnlock()
	return clients
}

// Client contains information about a client known by the broker.
type client struct {
	sync.RWMutex

	// p is a packets parser which reads incoming packets.
	p *packets.Parser

	// ac is a pointer to an auth controller inherited from the listener.
	ac auth.Controller

	// end is a channel that indicates the client should be halted.
	end chan struct{}

	// done can be called to ensure the close methods are only called once.
	done *sync.Once

	// id is the client id.
	id string

	// listener is the id of the listener the client is connected to.
	listener string

	// user is the username the client authenticated with.
	user string

	// keepalive is the number of seconds the connection can stay open without
	// receiving a message from the client.
	keepalive uint16

	// cleanSession indicates if the client expects a cleansession.
	cleanSession bool

	// packetID is the current highest packetID.
	packetID uint32

	// lwt contains the last will and testament for the client.
	lwt lwt

	// inFlight is a map of messages which are in-flight,awaiting completiong of
	// a QoS pattern.
	inFlight inFlight

	// subscriptions is a map of the subscription filters a client maintains.
	subscriptions map[string]byte

	// wasClosed indicates that the connection was closed deliberately.
	wasClosed bool
}

// newClient creates a new instance of client.
func newClient(p *packets.Parser, pk *packets.ConnectPacket, ac auth.Controller) *client {
	cl := &client{
		p:    p,
		ac:   ac,
		end:  make(chan struct{}),
		done: new(sync.Once),

		id:           pk.ClientIdentifier,
		user:         pk.Username,
		keepalive:    pk.Keepalive,
		cleanSession: pk.CleanSession,
		inFlight: inFlight{
			internal: make(map[uint16]*inFlightMessage),
		},
		subscriptions: make(map[string]byte),
	}

	// If no client id was provided, generate a new one.
	if cl.id == "" {
		cl.id = xid.New().String()
	}

	// if no deadline value was provided, set it to the default seconds.
	if cl.keepalive == 0 {
		cl.keepalive = defaultClientKeepalive
	}

	// If a last will and testament has been provided, record it.
	if pk.WillFlag {
		cl.lwt = lwt{
			topic:   pk.WillTopic,
			message: pk.WillMessage,
			qos:     pk.WillQos,
			retain:  pk.WillRetain,
		}
	}

	return cl
}

// nextPacketID returns the next packet id for a client, looping back to 0
// if the maximum ID has been reached.
func (cl *client) nextPacketID() uint32 {
	i := atomic.LoadUint32(&cl.packetID)
	if i == uint32(65535) || i == uint32(0) {
		atomic.StoreUint32(&cl.packetID, 1)
		return 1
	}

	return atomic.AddUint32(&cl.packetID, 1)
}

// noteSubscription makes a note of a subscription for the client.
func (c *client) noteSubscription(filter string, qos byte) {
	c.Lock()
	c.subscriptions[filter] = qos
	c.Unlock()
}

// forgetSubscription forgests a subscription note for the client.
func (c *client) forgetSubscription(filter string) {
	c.Lock()
	delete(c.subscriptions, filter)
	c.Unlock()
}

// close attempts to gracefully close a client connection.
func (cl *client) close() {
	cl.done.Do(func() {
		close(cl.end) // Signal to stop listening for packets.

		// Close the network connection.
		cl.p.Conn.Close() // Error is irrelevant so can be ommitted here.
		cl.p.Conn = nil
	})
}

// lwt contains the last will and testament details for a client connection.
type lwt struct {

	// topic is the topic the will message shall be sent to.
	topic string

	// message is the message that shall be sent when the client disconnects.
	message []byte

	// qos is the quality of service byte desired for the will message.
	qos byte

	// retain indicates whether the will message should be retained.
	retain bool
}

// inFlightMessage contains data about a packet which is currently in-flight.
type inFlightMessage struct {

	// packet is the packet currently in-flight.
	packet packets.Packet

	// sent is the last time the message was sent (for retries) in unixtime.
	sent int64
}

// inFlight is a map of inFlightMessage keyed on packet id.
type inFlight struct {
	sync.RWMutex

	// internal contains the inflight messages.
	internal map[uint16]*inFlightMessage
}

// set stores the packet of an in-flight message, keyed on message id.
func (i *inFlight) set(key uint16, in *inFlightMessage) {
	i.Lock()
	i.internal[key] = in
	i.Unlock()
}

// get returns the value of an in-flight message if it exists.
func (i *inFlight) get(key uint16) (*inFlightMessage, bool) {
	i.RLock()
	val, ok := i.internal[key]
	i.RUnlock()
	return val, ok
}

// delete removes an in-flight message from the map.
func (i *inFlight) delete(key uint16) {
	i.Lock()
	delete(i.internal, key)
	i.Unlock()
}
