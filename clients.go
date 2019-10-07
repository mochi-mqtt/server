package mqtt

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/xid"

	"github.com/mochi-co/mqtt/packets"
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

// Client contains information about a client known by the broker.
type client struct {
	sync.RWMutex

	// p is a packets parser which reads incoming packets.
	p *packets.Parser

	// end is a channel that indicates the client should be halted.
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

	// packetID is the current highest packetID.
	packetID uint32

	// inFlight is a map of messages which are in-flight,awaiting completiong of
	// a QoS pattern.
	inFlight inFlight

	// wasClosed indicates that the connection was closed deliberately.
	wasClosed bool
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
		inFlight: inFlight{
			internal: make(map[uint16]*inFlightMessage, 2),
		},
	}

	// If no client id was provided, generate a new one.
	if cl.id == "" {
		cl.id = xid.New().String()
	}

	// if no deadline value was provided, set it to the default seconds.
	if cl.keepalive == 0 {
		cl.keepalive = clientKeepalive
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

// close attempts to gracefully close a client connection.
func (cl *client) close() {
	cl.done.Do(func() {
		close(cl.end) // Signal to stop listening for packets.

		// Close the network connection.
		cl.p.Conn.Close() // Error is irrelevant so can be ommitted here.
		cl.p.Conn = nil

	})
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
func (i *inFlight) set(key uint16, pk packets.Packet) {
	i.Lock()
	i.internal[key] = &inFlightMessage{
		packet: pk,
		sent:   time.Now().Unix(),
	}
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
