package mqtt

import (
	"errors"
	"fmt"
	"net"

	"github.com/mochi-co/mqtt/internal/auth"
	"github.com/mochi-co/mqtt/internal/circ"
	"github.com/mochi-co/mqtt/internal/clients"
	"github.com/mochi-co/mqtt/internal/listeners"
	"github.com/mochi-co/mqtt/internal/packets"
	"github.com/mochi-co/mqtt/internal/topics"

	dbg "github.com/mochi-co/debug"
)

const (
	maxPacketID = 65535 // maxPacketID is the maximum value of a 16-bit packet ID.
)

var (
	ErrListenerIDExists     = errors.New("Listener id already exists")
	ErrReadConnectInvalid   = errors.New("Connect packet was not valid")
	ErrConnectNotAuthorized = errors.New("Connect packet was not authorized")
)

/*
var (
	ErrListenerIDExists       = errors.New("Listener id already exists")
	ErrReadConnectFixedHeader = errors.New("Error reading fixed header on CONNECT packet")
	ErrReadConnectPacket      = errors.New("Error reading CONNECT packet")
	ErrReadConnectInvalid     = errors.New("CONNECT packet was not valid")

	ErrReadFixedHeader        = errors.New("Error reading fixed header")
	ErrReadPacketPayload      = errors.New("Error reading packet payload")
	ErrReadPacketValidation   = errors.New("Error validating packet")
	ErrConnectionClosed       = errors.New("Connection not open")
	ErrNoData                 = errors.New("No data")
	ErrACLNotAuthorized       = errors.New("ACL not authorized")
)
*/

// Server is an MQTT broker server.
type Server struct {
	Listeners listeners.Listeners // listeners listen for new connections.
	Clients   clients.Clients     // clients known to the broker.
	Topics    *topics.Index       // an index of topic subscriptions and retained messages.
}

// New returns a new instance of an MQTT broker.
func New() *Server {
	fmt.Println()
	return &Server{
		Listeners: listeners.New(),
		Clients:   clients.New(),
		Topics:    topics.New(),
	}
}

// AddListener adds a new network listener to the server.
func (s *Server) AddListener(listener listeners.Listener, config *listeners.Config) error {
	if _, ok := s.Listeners.Get(listener.ID()); ok {
		return ErrListenerIDExists
	}

	if config != nil {
		listener.SetConfig(config)
	}

	s.Listeners.Add(listener)

	return nil
}

// Serve begins the event loops for establishing client connections on all
// attached listeners.
func (s *Server) Serve() error {
	s.Listeners.ServeAll(s.EstablishConnection)
	return nil
}

// Close attempts to gracefully shutdown the server, all listeners, and clients.
func (s *Server) Close() error {
	s.Listeners.CloseAll(s.closeListenerClients)
	return nil
}

// closeListenerClients closes all clients on the specified listener.
func (s *Server) closeListenerClients(listener string) {
	clients := s.Clients.GetByListener(listener)
	for _, client := range clients {
		s.closeClient(client, false) // omit errors
	}

}

// closeClient closes a client connection and publishes any LWT messages.
func (s *Server) closeClient(cl *clients.Client, sendLWT bool) error {

	//debug.Println(cl.ID, "SERVER STOPS ISSUED >> ")

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
	cl.Stop()

	return nil
}

// EstablishConnection establishes a new client connection with the broker.
func (s *Server) EstablishConnection(lid string, c net.Conn, ac auth.Controller) error {
	client := clients.NewClient(c, circ.NewReader(0, 0), circ.NewWriter(0, 0))
	client.Start()
	dbg.Println(dbg.Bold + ">> Establishing Connection")

	fh := new(packets.FixedHeader)
	err := client.ReadFixedHeader(fh)
	if err != nil {
		return err
	}

	pk, err := client.ReadPacket(fh)
	if err != nil {
		return err
	}

	if pk.FixedHeader.Type != packets.Connect {
		return ErrReadConnectInvalid
	}

	client.Identify(lid, pk, ac)
	retcode, _ := pk.ConnectValidate()
	if !ac.Authenticate(pk.Username, pk.Password) {
		retcode = packets.CodeConnectBadAuthValues
	}
	var sessionPresent bool
	if existing, ok := s.Clients.Get(pk.ClientIdentifier); ok {
		existing.Lock()
		existing.Stop()
		if pk.CleanSession {
			for k := range existing.Subscriptions {
				delete(existing.Subscriptions, k)
				s.Topics.Unsubscribe(k, existing.ID)
			}
		} else {
			client.InFlight = existing.InFlight // Inherit from existing session.
			client.Subscriptions = existing.Subscriptions
			sessionPresent = true
		}
		existing.Unlock()
	}

	// Add the new client to the clients manager.
	s.Clients.Add(client)

	// Send a CONNACK back to the client with retcode.
	err = s.writeClient(client, &packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Connack,
		},
		SessionPresent: sessionPresent,
		ReturnCode:     retcode,
	})

	if err != nil || retcode != packets.Accepted {
		return err
	}

	// Resend any unacknowledged QOS messages still pending for the client.
	//err = s.ResendInflight(client)
	//if err != nil {
	//	return err
	//}

	// Block and listen for more packets, and end if an error or nil packet occurs.
	var sendLWT bool
	err = client.Read(s.processPacket)
	if err != nil {
		sendLWT = true // Only send LWT on bad disconnect [MQTT-3.14.4-3]
	}

	// Publish last will and testament then close.
	dbg.Println(dbg.Bold+">> Client ended"+dbg.Reset, dbg.Underline+client.ID+dbg.Reset, err)
	s.closeClient(client, sendLWT)

	return err
}

// writeClient writes packets to a client connection.
func (s *Server) writeClient(cl *clients.Client, pk *packets.Packet) error {
	_, err := cl.WritePacket(pk)
	if err != nil {
		return err
	}

	// Log $SYS stats.
	// @TODO ...

	return nil
}

// processPacket processes an inbound packet for a client. Since the method is
// typically called as a goroutine, errors are mostly for test checking purposes.
func (s *Server) processPacket(cl *clients.Client, pk packets.Packet) error {
	dbg.Printf("%s %+v\n", dbg.Green+">> Processing", pk)

	return nil
}
