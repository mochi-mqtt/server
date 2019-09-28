package mqtt

import (
	"errors"
	"log"
	"net"

	"github.com/mochi-co/mqtt/auth"
	"github.com/mochi-co/mqtt/listeners"
	"github.com/mochi-co/mqtt/packets"
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
	// clients map[string]Client
}

// Clients
/*
type Clients struct {
	clients map[string]*Client
	subscriptions Subscriptions(cl.clients)
}

*/

// New returns a pointer to a new instance of the MQTT broker.
func New() *Server {
	return &Server{
		listeners: listeners.NewListeners(),
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

	// Create a new packets parser which will parse all packets for this cliet.
	p := packets.NewParser(c)

	// Pull the header from the first packet and check for a CONNECT message.
	fh := new(packets.FixedHeader)
	err := p.ReadFixedHeader(fh)
	if err != nil {
		log.Println("_A", err)
		return ErrReadConnectFixedHeader
	}

	// Read the first packet expecting a CONNECT message.
	pk, err := p.Read()
	if err != nil {
		log.Println("_B", err)
		return ErrReadConnectPacket
	}

	// Ensure first packet is a connect packet.
	msg, ok := pk.(*packets.ConnectPacket)
	if !ok {
		log.Println("_C")
		return ErrFirstPacketInvalid
	}

	// Ensure the packet conforms to MQTT CONNECT specifications.
	retcode, _ := msg.Validate()
	if retcode != packets.Accepted {
		log.Println("_D", retcode)
		return ErrReadConnectInvalid
	}

	// If a username and password has been provided, perform authentication.
	if msg.Username != "" && !ac.Authenticate(msg.Username, msg.Password) {
		retcode = packets.CodeConnectNotAuthorised
		log.Println("_E", retcode)
		return ErrConnectNotAuthorized
	}

	// Add the new client to the clients manager.
	// @TODO ...

	// Send a CONNACK back to the client.
	// @TODO s.writeClient(cl, connackPK)

	// Publish out any unacknowledged QOS messages still pending for the client.
	// @TODO ...

	// Block and listen for more packets, and end if an error or nil packet occurs.
	// @TODO ... s.readClient

	//	log.Println(msg, retcode)
	log.Println(msg, pk)
	return nil
}

// readClient reads new packets from a client connection.
func (s *Server) readClient(cl *client) error {
	var err error
	var pk packets.Packet
	fh := new(packets.FixedHeader)

	var i int

DONE:
	for {
		select {
		case <-cl.end:
			break DONE

		default:
			log.Println("CYCLE", i)
			i++
			if cl.p.Conn == nil {
				return ErrConnectionClosed
			}

			// Reset the keepalive read deadline.
			cl.p.RefreshDeadline(cl.keepalive)

			// Read in the fixed header of the packet.
			err = cl.p.ReadFixedHeader(fh)
			if err != nil {
				log.Println(">>A ", err)
				return ErrReadFixedHeader
			}

			// If it's a disconnect packet, begin the close process.
			if fh.Type == packets.Disconnect {
				return nil
			}

			// Otherwise read in the packet payload.
			pk, err = cl.p.Read()
			if err != nil {
				log.Println(">>B ", err)
				return ErrReadPacketPayload
			}

			// Validate the packet if necessary.
			_, err := pk.Validate()
			if err != nil {
				log.Println(">>C ", err)
				return ErrReadPacketValidation
			}

			// Process inbound packet.
			go s.processPacket(cl, pk)
		}
	}

	return nil
}

// writeClient writes packets to a client connection.
func (s *Server) writeClient(cl *client, pk packets.Packet) error {

	// encode packet (use buffer pool)
	// write packet
	// refresh deadline

	return nil
}

// closeClient closes a client connection and publishes any LWT messages.
func (s *Server) closeClient(cl *client) error {

	// close client connection
	// send LWT

	return nil
}

// processPacket processes an inbound packet for a client.
func (s *Server) processPacket(cl *client, pk packets.Packet) error {
	log.Println("PROCESSING PACKET", cl, pk)

	// Log read stats for $SYS.
	// @TODO ... //

	// switch on packet type
	//// connect
	//		stop
	//// disconnect
	// 		stop

	//// ping
	// 		pingresp
	// 		else stop

	//// publish
	//		retain if 1
	//		find valid subscribers
	//			upgrade copied packet
	// 			if (qos > 1) add packetID > cl.nextPacketID()
	//			write packet to client
	//			handle qos > s.processQOS(cl, pk)

	//// pub*
	//		handle qos > s.processQOS(cl, pk)

	//// subscribe
	// 		subscribe topics
	//		send subacks
	//		receive any retained messages

	//// unsubscribe
	//		unsubscribe topics
	//		send unsuback

	return nil
}

// processQOS handles the back and forth of QOS>0 packets.
func (s *Server) processQOS(cl *client, pk packets.Packet) error {

	// handle publish in/out
	// handle puback
	// handle pubrec
	// handle pubrel
	// handle pubcomp

	return nil
}
