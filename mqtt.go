package mqtt

import (
	"bufio"
	"bytes"
	"errors"
	"net"
	"time"

	"github.com/mochi-co/mqtt/auth"
	"github.com/mochi-co/mqtt/listeners"
	"github.com/mochi-co/mqtt/packets"
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
	ErrACLNotAuthorized       = errors.New("ACL not authorized")

	// clientKeepalive is the default keepalive time in seconds.
	clientKeepalive uint16 = 60

	// rwBufSize is the size of client read/write buffers.
	rwBufSize = 512
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

// Close attempts to gracefully shutdown the server, all listeners, and clients.
func (s *Server) Close() error {
	s.listeners.CloseAll(listeners.MockCloser)
	return nil
}

// EstablishConnection establishes a new client connection with the broker.
func (s *Server) EstablishConnection(c net.Conn, ac auth.Controller) error {

	// Create a new packets parser which will parse all packets for this client,
	// using buffered writers and readers.
	p := packets.NewParser(
		c,
		bufio.NewReaderSize(c, rwBufSize),
		bufio.NewWriterSize(c, rwBufSize),
	)

	// Pull the header from the first packet and check for a CONNECT message.
	fh := new(packets.FixedHeader)
	err := p.ReadFixedHeader(fh)
	if err != nil {
		return ErrReadConnectFixedHeader
	}
	// Read the first packet expecting a CONNECT message.
	pk, err := p.Read()
	if err != nil {
		return ErrReadConnectPacket
	}

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

	// Create a new client with this connection.
	client := newClient(p, msg, ac)

	// If it's not a clean session and a client already exists with the same
	// id, inherit the session.
	if existing, ok := s.clients.get(msg.ClientIdentifier); ok {
		existing.close()
		existing.Lock()
		if msg.CleanSession {
			for k := range existing.subscriptions {
				s.topics.Unsubscribe(k, existing.id)
			}
		} else {
			client.inFlight = existing.inFlight // Inherit from existing session.
			client.subscriptions = existing.subscriptions
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
		SessionPresent: false, //msg.CleanSession,
		ReturnCode:     retcode,
	})
	if err != nil {
		return err
	}

	// Publish out any unacknowledged QOS messages still pending for the client.
	// @TODO ...

	// Block and listen for more packets, and end if an error or nil packet occurs.
	err = s.readClient(client)
	if err != nil {
		return err
	}

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
			break DONE

		default:
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

// writeClient writes packets to a client connection.
func (s *Server) writeClient(cl *client, pk packets.Packet) error {

	// Ensure Writer is open.
	if cl.p.W == nil {
		return ErrConnectionClosed
	}

	buf := new(bytes.Buffer)
	err := pk.Encode(buf)
	if err != nil {
		return err
	}

	// Write packet to client.
	_, err = buf.WriteTo(cl.p.W)
	if err != nil {
		return err
	}

	err = cl.p.W.Flush()
	if err != nil {
		return err
	}

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
