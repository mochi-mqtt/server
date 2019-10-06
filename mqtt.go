package mqtt

import (
	"bufio"
	"errors"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/xid"

	"github.com/mochi-co/mqtt/auth"
	"github.com/mochi-co/mqtt/listeners"
	"github.com/mochi-co/mqtt/packets"
	"github.com/mochi-co/mqtt/pools"
	"github.com/mochi-co/mqtt/topics"
	"github.com/mochi-co/mqtt/topics/trie"
)

const (
	// maxPacketID is the maximum value of a packet ID.
	// Control Packets MUST contain a non-zero 16-bit Packet Identifier [MQTT-2.3.1-1].
	maxPacketID = 65535
)

var (
	ErrListenerIDExists       = errors.New("Listener id already exists")
	ErrReadConnectFixedHeader = errors.New("Error reading fixed header on CONNECT packet")
	ErrReadConnectPacket      = errors.New("Error reading CONNECT packet")
	ErrFirstPacketInvalid     = errors.New("First packet was not CONNECT packet")
	ErrReadConnectInvalid     = errors.New("CONNECT packet was not valid")
	ErrConnectNotAuthorized   = errors.New("CONNECT packet was not authorized")
	ErrReadFixedHeader        = errors.New("Error reading fixed header")
	ErrReadPacketPayload      = errors.New("Error reading packet payload")
	ErrReadPacketValidation   = errors.New("Error validating packet")
	ErrConnectionClosed       = errors.New("Connection not open")
	ErrNoData                 = errors.New("No data")

	// clientKeepalive is the default keepalive time in seconds.
	clientKeepalive uint16 = 60

	// rwBufSize is the size of client read/write buffers.
	rwBufSize = 512
)

/*
	ErrListenerInvalid        = errors.New("listener validation failed")
	ErrListenerIDExists       = errors.New("listener id already exists")
	ErrPortInUse              = errors.New("port already in use")
	ErrFailedListening        = errors.New("couldnt start net listener")
	ErrIDNotSet               = errors.New("id not set")
	ErrListenerNotFound       = errors.New("listener id not found")
	ErrFailedInitializing     = errors.New("failed initializing")
	ErrFailedServingTCP       = errors.New("error serving tcp listener")
	ErrFailedServingWS        = errors.New("error serving websocket listener")
	ErrAcceptConnection       = errors.New("error accepting connection")
	ErrEstablishingConnection = errors.New("error establishing connection")
	ErrCloseConnection        = errors.New("error closing connection")

	ErrReadConnectPacket      = errors.New("error reading CONNECT packet")
	ErrFirstPacketInvalid     = errors.New("first packet was not CONNECT packet")
	ErrReadConnectInvalid     = errors.New("CONNECT packet was not valid")
	ErrParsingRemoteOrigin    = errors.New("error parsing remote origin from websocket")
	ErrFailedConnack          = errors.New("failed sending CONNACK packet")
*/

// Server is an MQTT broker server.
type Server struct {

	// listeners is a map of listeners, which listen for new connections.
	listeners listeners.Listeners

	// clients is a map of clients known to the broker.
	clients clients

	// topics is an index of topic subscriptions and retained messages.
	topics topics.Indexer

	// buffers is a pool of bytes.buffer.
	buffers pools.BytesBuffersPool
}

// New returns a pointer to a new instance of the MQTT broker.
func New() *Server {
	return &Server{
		listeners: listeners.NewListeners(),
		clients:   newClients(),
		topics:    trie.New(),
		buffers:   pools.NewBytesBuffersPool(),
	}
}

// AddListener adds a new network listener to the server.
func (s *Server) AddListener(listener listeners.Listener, config *listeners.Config) error {
	if _, ok := s.listeners.Get(listener.ID()); ok {
		return ErrListenerIDExists
	}

	if config != nil {
		listener.SetConfig(config)
	}

	s.listeners.Add(listener)

	return nil
}

// Serve begins the event loops for establishing client connections on all
// attached listeners.
func (s *Server) Serve() error {
	s.listeners.ServeAll(s.EstablishConnection)
	return nil
}

// Close attempts to gracefully shutdown the server, all listeners, and clients.
func (s *Server) Close() error {
	s.listeners.CloseAll(listeners.MockCloser)
	return nil
}

// EstablishConnection establishes a new client connection with the broker.
func (s *Server) EstablishConnection(c net.Conn, ac auth.Controller) error {
	log.Println("connecting")

	// Create a new packets parser which will parse all packets for this client,
	// using buffered writers and readers.
	p := packets.NewParser(
		c,
		bufio.NewReaderSize(c, rwBufSize),
		bufio.NewWriterSize(c, rwBufSize),
	)

	log.Println("....")
	// Pull the header from the first packet and check for a CONNECT message.
	fh := new(packets.FixedHeader)
	err := p.ReadFixedHeader(fh)
	if err != nil {
		return ErrReadConnectFixedHeader
	}
	log.Println("read fh")
	// Read the first packet expecting a CONNECT message.
	pk, err := p.Read()
	if err != nil {
		return ErrReadConnectPacket
	}
	log.Println("read pk")
	// Ensure first packet is a connect packet.
	msg, ok := pk.(*packets.ConnectPacket)
	if !ok {
		return ErrFirstPacketInvalid
	}

	// Ensure the packet conforms to MQTT CONNECT specifications.
	retcode, _ := msg.Validate()
	if retcode != packets.Accepted {
		return ErrReadConnectInvalid
	}

	// If a username and password has been provided, perform authentication.
	if msg.Username != "" && !ac.Authenticate(msg.Username, msg.Password) {
		retcode = packets.CodeConnectNotAuthorised
		return ErrConnectNotAuthorized
	}

	// Add the new client to the clients manager.
	client := newClient(p, msg)
	// ... handle session takeover
	s.clients.add(client)

	log.Println("connected", client.id)

	// Send a CONNACK back to the client.
	err = s.writeClient(client, &packets.ConnackPacket{
		FixedHeader:    packets.NewFixedHeader(packets.Connack),
		SessionPresent: false, //msg.CleanSession,
		ReturnCode:     retcode,
	})
	if err != nil {
		log.Println("write err", err)
		return err
	}

	log.Println("connack sent", client.id)

	// Publish out any unacknowledged QOS messages still pending for the client.
	// @TODO ...

	// Block and listen for more packets, and end if an error or nil packet occurs.
	err = s.readClient(client)
	if err != nil {
		log.Println("read err", err)
		return err
	}

	log.Println("ended", client.id)

	return nil
}

// readClient reads new packets from a client connection.
func (s *Server) readClient(cl *client) error {
	var err error
	var pk packets.Packet
	fh := new(packets.FixedHeader)

DONE:
	for {
		select {
		case <-cl.end:
			log.Println("done ending")
			break DONE

		default:
			log.Println("iterating")
			if cl.p.Conn == nil {
				return ErrConnectionClosed
			}

			// Reset the keepalive read deadline.
			cl.p.RefreshDeadline(cl.keepalive)

			// Read in the fixed header of the packet.
			err = cl.p.ReadFixedHeader(fh)
			if err != nil {
				return ErrReadFixedHeader
			}

			// If it's a disconnect packet, begin the close process.
			if fh.Type == packets.Disconnect {
				return nil
			}

			// Otherwise read in the packet payload.
			pk, err = cl.p.Read()
			if err != nil {
				return ErrReadPacketPayload
			}

			// Validate the packet if necessary.
			_, err := pk.Validate()
			if err != nil {
				return ErrReadPacketValidation
			}

			// Process inbound packet.
			go s.processPacket(cl, pk)
		}
	}

	log.Println("returning")

	return nil
}

// processPacket processes an inbound packet for a client.
func (s *Server) processPacket(cl *client, pk packets.Packet) error {
	log.Println("PROCESSING PACKET", cl, pk)

	// Log read stats for $SYS.
	// @TODO ...

	// Process the packet depending on the detected type.
	switch msg := pk.(type) { //pk :=
	case *packets.ConnectPacket:

		// [MQTT-3.1.0-2]
		// The server is required to disconnect the client if a second connect
		// packet is sent.
		s.closeClient(cl, true)

	case *packets.DisconnectPacket:

		// Connection is ending gracefully, so close client and return.
		s.closeClient(cl, true)

	case *packets.PingreqPacket:
		// Client has sent a ping to keep the connection alive, so send a
		// response back to acknowledge receipt.
		err := s.writeClient(cl, &packets.PingrespPacket{
			FixedHeader: packets.NewFixedHeader(packets.Pingresp),
		})
		if err != nil {
			s.closeClient(cl, true)
		}

	case *packets.PublishPacket:
		log.Println(msg)

		// If message is retained, add it to the retained messages index.
		if msg.Retain {
			log.Println("RETAIN", msg.Retain)
			s.topics.RetainMessage(msg)
		}

		// Get all the clients who have a subscription matching the publish
		// packet's topic.
		subs := s.topics.Subscribers(msg.TopicName)
		for id, qos := range subs {
			if client, ok := s.clients.get(id); ok {
				log.Println(client, id, qos)

				// Make a copy of the packet to send to client.
				out := msg.Copy()

				// If the client subscription has a higher qos, inherit it.
				if qos > out.Qos {
					out.Qos = qos
				}

				// If QoS byte is set, ensure the message has an id.
				if out.Qos > 0 && out.PacketID == 0 {
					out.PacketID = uint16(client.nextPacketID())
				}

				// Write the publish packet out to the receiving client.
				err := s.writeClient(client, out)
				if err != nil {
					s.closeClient(client, true)
				}

				// If QoS byte is set, save as message to inflight index so we
				// can track delivery.
				if out.Qos > 0 {
					client.inFlight.set(out.PacketID, out)
				}
			}
		}

	case *packets.PubackPacket:
		cl.inFlight.delete(msg.PacketID)

	case *packets.PubrecPacket:
		if _, ok := cl.inFlight.get(msg.PacketID); ok {
			out := &packets.PubrelPacket{
				FixedHeader: packets.NewFixedHeader(packets.Pubrel),
				PacketID:    msg.PacketID,
			}
			log.Println("READY", out)
			err := s.writeClient(cl, out)
			if err != nil {
				s.closeClient(cl, true)
			}
			log.Println("WRITTEN", msg)
			cl.inFlight.set(out.PacketID, out)
		}

	case *packets.PubrelPacket:
		if _, ok := cl.inFlight.get(msg.PacketID); ok {
			out := &packets.PubcompPacket{
				FixedHeader: packets.NewFixedHeader(packets.Pubcomp),
				PacketID:    msg.PacketID,
			}
			log.Println("READY", out)
			err := s.writeClient(cl, out)
			if err != nil {
				s.closeClient(cl, true)
			}
			log.Println("WRITTEN", msg)
			cl.inFlight.delete(msg.PacketID)
		}

	case *packets.PubcompPacket:
		if _, ok := cl.inFlight.get(msg.PacketID); ok {
			cl.inFlight.delete(msg.PacketID)
		}

	case *packets.SubscribePacket:

	case *packets.UnsubscribePacket:

	}

	return nil
}

// writeClient writes packets to a client connection.
func (s *Server) writeClient(cl *client, pk packets.Packet) error {

	// Ensure Writer is open.
	if cl.p.W == nil {
		return ErrConnectionClosed
	}

	// Encode packet to a pooled byte buffer.
	buf := s.buffers.Get()
	defer s.buffers.Put(buf)
	err := pk.Encode(buf)
	if err != nil {
		return err
	}

	log.Println("==", buf.Bytes())

	// Write packet to client.
	_, err = buf.WriteTo(cl.p.W)
	if err != nil {
		return err
	}

	err = cl.p.W.Flush()
	if err != nil {
		return err
	}
	log.Println("WRITE CLIENT", cl.id)

	// Refresh deadline to keep the connection alive.
	cl.p.RefreshDeadline(cl.keepalive)

	// Log $SYS stats.
	// @TODO ...

	return nil
}

// closeClient closes a client connection and publishes any LWT messages.
func (s *Server) closeClient(cl *client, sendLWT bool) error {

	// Stop listening for new packets.
	cl.close()

	// Send LWT to subscribers.
	// @TODO ...

	return nil
}

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
			internal: make(map[uint16]*inFlightMessage),
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
	log.Println("closing", cl)
	cl.done.Do(func() {

		// Signal to stop lsitening for packets.
		close(cl.end)

		// Close the network connection.
		cl.p.Conn.Close() // Error is irrelevant so can be ommited here.
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
