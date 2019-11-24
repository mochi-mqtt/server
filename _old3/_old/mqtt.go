package mqtt

import (
	"bytes"
	"errors"
	"net"
	"time"

	//"github.com/davecgh/go-spew/spew"

	"github.com/mochi-co/mqtt/auth"
	"github.com/mochi-co/mqtt/circ"
	"github.com/mochi-co/mqtt/listeners"
	"github.com/mochi-co/mqtt/packets"
	"github.com/mochi-co/mqtt/topics"
	"github.com/mochi-co/mqtt/topics/trie"

	"github.com/mochi-co/mqtt/debug"
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

	// listeners is a map of listeners, which listen for new connections.
	listeners listeners.Listeners

	// clients is a map of clients known to the broker.
	clients clients

	// topics is an index of topic subscriptions and retained messages.
	topics topics.Indexer
}

// transitMsg contains data to be sent to the inbound channel.
type transitMsg struct {

	// client is the client who received or will receive the message.
	client *client

	// packet is the packet sent or received.
	packet packets.Packet
}

// New returns a pointer to a new instance of the MQTT broker.
func New() *Server {
	return &Server{
		listeners: listeners.NewListeners(),
		clients:   newClients(),
		topics:    trie.New(),
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

	// Publish last will and testament then close.
	s.closeClient(client, sendLWT)
	debug.Println(client.id, "^^ CLIENT STOPPED")

	return err
}

// writeClient writes packets to a client connection.
func (s *Server) writeClient(cl *client, pk packets.Packet) error {
	cl.w.Lock()
	defer cl.w.Unlock()

	if cl.w == nil {
		return ErrConnectionClosed
	}

	buf := new(bytes.Buffer)
	err := pk.Encode(buf)
	if err != nil {
		return err
	}

	// Write packet to client.
	//time.Sleep(time.Millisecond * 500)
	//debug.Printf("%s \033[1;33mWRITING PACKET %+v \033[0m\n", cl.id, pk)
	debug.Printf("%s \033[1;33mWRITING BYTES %s\033[0m\n", cl.id, string(buf.Bytes()))
	_, err = cl.w.Write(buf.Bytes())
	//_, err = buf.WriteTo(cl.w)
	if err != nil {
		return err
	}

	// Refresh deadline to keep the connection alive.
	cl.refreshDeadline(cl.keepalive)

	// Log $SYS stats.
	// @TODO ...

	return nil
}

// processPacket processes an inbound packet for a client. Since the method is
// typically called as a goroutine, errors are mostly for test checking purposes.
func (s *Server) processPacket(cl *client, pk packets.Packet) error {

	// Log read stats for $SYS.
	// @TODO ...

	// Process the packet depending on the detected type.
	switch msg := pk.(type) {
	case *packets.ConnectPacket:
		return s.processConnect(cl, msg)

	case *packets.DisconnectPacket:
		return s.processDisconnect(cl, msg)

	case *packets.PingreqPacket:
		return s.processPingreq(cl, msg)

	case *packets.PublishPacket:
		debug.Printf("%s \033[1;32mPROCESSING PUB PACKET %+v\033[0m\n", cl.id, string(pk.(*packets.PublishPacket).Payload))
		return s.processPublish(cl, msg)

	case *packets.PubackPacket:
		return s.processPuback(cl, msg)

	case *packets.PubrecPacket:
		return s.processPubrec(cl, msg)

	case *packets.PubrelPacket:
		return s.processPubrel(cl, msg)

	case *packets.PubcompPacket:
		return s.processPubcomp(cl, msg)

	case *packets.SubscribePacket:
		return s.processSubscribe(cl, msg)

	case *packets.UnsubscribePacket:
		return s.processUnsubscribe(cl, msg)
	}

	return nil
}

// resendInflight republishes any inflight messages to the client.
func (s *Server) resendInflight(cl *client) error {
	cl.RLock()
	msgs := cl.inFlight.internal
	cl.RUnlock()
	for _, msg := range msgs {
		err := s.writeClient(cl, msg.packet)
		if err != nil {
			return err
		}
	}

	return nil
}

// processConnect processes a Connect packet. The packet cannot be used to
// establish a new connection on an existing connection. See EstablishConnection
// instead.
func (s *Server) processConnect(cl *client, pk *packets.ConnectPacket) error {
	s.closeClient(cl, true)
	return nil
}

// processDisconnect processes a Disconnect packet.
func (s *Server) processDisconnect(cl *client, pk *packets.DisconnectPacket) error {
	s.closeClient(cl, true)
	return nil
}

// processPingreq processes a Pingreq packet.
func (s *Server) processPingreq(cl *client, pk *packets.PingreqPacket) error {
	err := s.writeClient(cl, &packets.PingrespPacket{
		FixedHeader: packets.FixedHeader{
			Type: packets.Pingresp,
		},
	})
	if err != nil {
		s.closeClient(cl, true)
		return err
	}

	return nil
}

// processPublish processes a Publish packet.
func (s *Server) processPublish(cl *client, pk *packets.PublishPacket) error {

	// Perform Access control check for publish (write).
	aclOK := cl.ac.ACL(cl.user, pk.TopicName, true)

	// If message is retained, add it to the retained messages index.
	if pk.Retain && aclOK {
		s.topics.RetainMessage(pk)
	}

	// Send appropriate Ack back to client. In v5 this will include a
	// auth return code.
	if pk.Qos == 1 {
		err := s.writeClient(cl, &packets.PubackPacket{
			FixedHeader: packets.FixedHeader{
				Type: packets.Puback,
			},
			PacketID: pk.PacketID,
		})
		if err != nil {
			s.closeClient(cl, true)
			return err
		}
	} else if pk.Qos == 2 {
		rec := &packets.PubrecPacket{
			FixedHeader: packets.FixedHeader{
				Type: packets.Pubrec,
			},
			PacketID: pk.PacketID,
		}
		cl.inFlight.set(pk.PacketID, &inFlightMessage{
			packet: rec,
			sent:   time.Now().Unix(),
		})
		err := s.writeClient(cl, rec)
		if err != nil {
			s.closeClient(cl, true)
			return err
		}
	}

	// Don't propogate message if client has no permission to do so.
	if !aclOK {
		return ErrACLNotAuthorized
	}

	// Get all the clients who have a subscription matching the publish
	// packet's topic.
	subs := s.topics.Subscribers(pk.TopicName)
	for id, qos := range subs {
		if client, ok := s.clients.get(id); ok {

			// Make a copy of the packet to send to client.
			out := pk.Copy()

			// If the client subscription has a higher qos, inherit it.
			if qos > out.Qos {
				out.Qos = qos
			}

			// If QoS byte is set, save as message to inflight index so we
			// can track delivery.
			if out.Qos > 0 {
				client.inFlight.set(out.PacketID, &inFlightMessage{
					packet: out,
					sent:   time.Now().Unix(),
				})

				// If QoS byte is set, ensure the message has an id.
				if out.PacketID == 0 {
					out.PacketID = uint16(client.nextPacketID())
				}
			}

			// Write the publish packet out to the receiving client.
			err := s.writeClient(client, out)
			if err != nil {
				s.closeClient(client, true)
				return err
			}
		}
	}

	return nil
}

// processPuback processes a Puback packet.
func (s *Server) processPuback(cl *client, pk *packets.PubackPacket) error {
	cl.inFlight.delete(pk.PacketID)
	return nil
}

// processPubrec processes a Pubrec packet.
func (s *Server) processPubrec(cl *client, pk *packets.PubrecPacket) error {
	if _, ok := cl.inFlight.get(pk.PacketID); ok {
		out := &packets.PubrelPacket{
			FixedHeader: packets.FixedHeader{
				Type: packets.Pubrel,
				Qos:  1,
			},
			PacketID: pk.PacketID,
		}

		cl.inFlight.set(out.PacketID, &inFlightMessage{
			packet: out,
			sent:   time.Now().Unix(),
		})
		err := s.writeClient(cl, out)
		if err != nil {
			s.closeClient(cl, true)
			return err
		}
	}
	return nil
}

// processPubrel processes a Pubrel packet.
func (s *Server) processPubrel(cl *client, pk *packets.PubrelPacket) error {
	if _, ok := cl.inFlight.get(pk.PacketID); ok {
		out := &packets.PubcompPacket{
			FixedHeader: packets.FixedHeader{
				Type: packets.Pubcomp,
			},
			PacketID: pk.PacketID,
		}

		err := s.writeClient(cl, out)
		if err != nil {
			s.closeClient(cl, true)
			return err
		}

		cl.inFlight.delete(pk.PacketID)
	}
	return nil
}

// processPubcomp processes a Pubcomp packet.
func (s *Server) processPubcomp(cl *client, pk *packets.PubcompPacket) error {
	if _, ok := cl.inFlight.get(pk.PacketID); ok {
		cl.inFlight.delete(pk.PacketID)
	}
	return nil
}

// processSubscribe processes a Subscribe packet.
func (s *Server) processSubscribe(cl *client, pk *packets.SubscribePacket) error {
	retCodes := make([]byte, len(pk.Topics))

	for i := 0; i < len(pk.Topics); i++ {
		if !cl.ac.ACL(cl.user, pk.Topics[i], false) {
			retCodes[i] = packets.ErrSubAckNetworkError
		} else {
			s.topics.Subscribe(pk.Topics[i], cl.id, pk.Qoss[i])
			cl.noteSubscription(pk.Topics[i], pk.Qoss[i])
			retCodes[i] = pk.Qoss[i]
		}
	}

	err := s.writeClient(cl, &packets.SubackPacket{
		FixedHeader: packets.FixedHeader{
			Type: packets.Suback,
		},
		PacketID:    pk.PacketID,
		ReturnCodes: retCodes,
	})
	if err != nil {
		s.closeClient(cl, true)
		return err
	}

	// Publish out any retained messages matching the subscription filter.
	for i := 0; i < len(pk.Topics); i++ {
		messages := s.topics.Messages(pk.Topics[i])
		for _, pkv := range messages {
			err := s.writeClient(cl, pkv)
			if err != nil {
				s.closeClient(cl, true)
				return err
			}
		}
	}

	return nil
}

// processUnsubscribe processes an unsubscribe packet.
func (s *Server) processUnsubscribe(cl *client, pk *packets.UnsubscribePacket) error {
	for i := 0; i < len(pk.Topics); i++ {
		s.topics.Unsubscribe(pk.Topics[i], cl.id)
		cl.forgetSubscription(pk.Topics[i])
	}

	err := s.writeClient(cl, &packets.UnsubackPacket{
		FixedHeader: packets.FixedHeader{
			Type: packets.Unsuback,
		},
		PacketID: pk.PacketID,
	})
	if err != nil {
		s.closeClient(cl, true)
		return err
	}

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
