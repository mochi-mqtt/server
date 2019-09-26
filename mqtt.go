package mqtt

import (
	"errors"
	"log"
	"net"

	"github.com/mochi-co/mqtt/listeners"
	"github.com/mochi-co/mqtt/packets"
)

var (
	ErrListenerIDExists       = errors.New("Listener id already exists")
	ErrReadConnectFixedHeader = errors.New("Error reading fixed header on CONNECT packet")
	ErrReadConnectPacket      = errors.New("Error reading CONNECT packet")
	ErrFirstPacketInvalid     = errors.New("First packet was not CONNECT packet")
	ErrReadConnectInvalid     = errors.New("CONNECT packet was not valid")
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
}

// New returns a pointer to a new instance of the MQTT broker.
func New() *Server {
	return &Server{
		listeners: listeners.NewListeners(),
	}
}

// AddListener adds a new network listener to the server.
func (s *Server) AddListener(listener listeners.Listener) error {
	if _, ok := s.listeners.Get(listener.ID()); ok {
		return ErrListenerIDExists
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
func (s *Server) EstablishConnection(c net.Conn) error {
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
	// @TODO ...

	//	log.Println(msg, retcode)
	log.Println(msg, pk)
	return nil
}
