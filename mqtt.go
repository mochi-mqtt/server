package mqtt

import (
	"errors"

	"github.com/mochi-co/mqtt/internal/debug"
	"github.com/mochi-co/mqtt/internal/listeners"
	"github.com/mochi-co/mqtt/internal/topics"
)

const (
	maxPacketID = 65535 // maxPacketID is the maximum value of a 16-bit packet ID.
)

var (
	ErrListenerIDExists       = errors.New("Listener id already exists")
	ErrReadConnectFixedHeader = errors.New("Error reading fixed header on CONNECT packet")
	ErrReadConnectPacket      = errors.New("Error reading CONNECT packet")
	ErrReadConnectInvalid     = errors.New("CONNECT packet was not valid")
	ErrConnectNotAuthorized   = errors.New("CONNECT packet was not authorized")
	ErrReadFixedHeader        = errors.New("Error reading fixed header")
	ErrReadPacketPayload      = errors.New("Error reading packet payload")
	ErrReadPacketValidation   = errors.New("Error validating packet")
	ErrConnectionClosed       = errors.New("Connection not open")
	ErrNoData                 = errors.New("No data")
	ErrACLNotAuthorized       = errors.New("ACL not authorized")
)

// Server is an MQTT broker server.
type Server struct {
	listeners listeners.Listeners // listeners listen for new connections.
	clients   clients             // clients known to the broker.
	topics    *topics.Index       // an index of topic subscriptions and retained messages.
}

// New returns a new instance of an MQTT broker.
func New() *Server {
	return &Server{
		listeners: listeners.New(),
		topics:    topics.New(),
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
	s.listeners.CloseAll(s.closeListenerClients)
	return nil
}

// closeListenerClients closes all clients on the specified listener.
func (s *Server) closeListenerClients(listener string) {
	clients := s.clients.getByListener(listener)
	for _, client := range clients {
		s.closeClient(client, false) // omit errors
	}

}

// closeClient closes a client connection and publishes any LWT messages.
func (s *Server) closeClient(cl *client, sendLWT bool) error {

	debug.Println(cl.id, "SERVER STOPS ISSUED >> ")

	// If an LWT message is set, publish it to the topic subscribers.
	/* // this currently loops forever on broken connection
	if sendLWT && cl.lwt.topic != "" {
		err := s.processPublish(cl, &packets.PublishPacket{
			FixedHeader: packets.FixedHeader{
				Type:   packets.Publish,
				Retain: cl.lwt.retain,
				Qos:    cl.lwt.qos,
			},
			TopicName: cl.lwt.topic,
			Payload:   cl.lwt.message,
		})
		if err != nil {
			return err
		}
	}
	*/

	// Stop listening for new packets.
	cl.close()

	return nil
}

// EstablishConnection establishes a new client connection with the broker.
func (s *Server) EstablishConnection(lid string, c net.Conn, ac auth.Controller) error {
	//client := newClient(c, circ.NewReader(0, 0), circ.NewWriter(0, 0))
	//client.start()
	return nil
}

/*
// EstablishConnection establishes a new client connection with the broker.
func (s *Server) EstablishConnection(lid string, c net.Conn, ac auth.Controller) error {
	client := newClient(c, circ.NewReader(0, 0), circ.NewWriter(0, 0))
	client.start()

	// Pull the header from the first packet and check for a CONNECT message.
	fh := new(packets.FixedHeader)
	err := client.readFixedHeader(fh)
	if err != nil {
		return ErrReadConnectFixedHeader
	}

	// Read the first packet expecting a CONNECT message.
	pk, err := client.readPacket()
	if err != nil {
		return ErrReadConnectPacket
	}

	// Ensure first packet is a connect packet.
	msg, ok := pk.(*packets.ConnectPacket)
	if !ok {
		return ErrReadConnectInvalid
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

	// Identify the new client with this connection and listener id.
	client.identify(lid, msg, ac)

	// If it's not a clean session and a client already exists with the same
	// id, inherit the session.
	var sessionPresent bool
	if existing, ok := s.clients.get(msg.ClientIdentifier); ok {
		existing.Lock()
		existing.close()
		if msg.CleanSession {
			for k := range existing.subscriptions {
				s.topics.Unsubscribe(k, existing.id)
			}
		} else {
			client.inFlight = existing.inFlight // Inherit from existing session.
			client.subscriptions = existing.subscriptions
			sessionPresent = true
		}
		existing.Unlock()
	}

	// Add the new client to the clients manager.
	s.clients.add(client)

	// Send a CONNACK back to the client.
	err = s.writeClient(client, &packets.ConnackPacket{
		FixedHeader: packets.FixedHeader{
			Type: packets.Connack,
		},
		SessionPresent: sessionPresent,
		ReturnCode:     retcode,
	})
	if err != nil {
		return err
	}

	// Resend any unacknowledged QOS messages still pending for the client.
	err = s.resendInflight(client)
	if err != nil {
		return err
	}

	// Block and listen for more packets, and end if an error or nil packet occurs.
	var sendLWT bool
	err = client.read(s.processPacket)
	if err != nil {
		sendLWT = true // Only send LWT on bad disconnect [MQTT-3.14.4-3]
	}

	debug.Println(client.id, "^^ CLIENT READ ENDED, ENDING", err)
	//debug.Println(client.id, "FINAL BUF", string(client.r.Get()))

	// Publish last will and testament then close.
	s.closeClient(client, sendLWT)
	debug.Println(client.id, "^^ CLIENT STOPPED")

	return err
}
*/
