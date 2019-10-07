package mqtt

import (
	"bufio"
	"errors"
	"log"
	"net"

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

	// Add the new client to the clients manager.
	client := newClient(p, msg)
	// ... handle session takeover
	s.clients.add(client)

	// Send a CONNACK back to the client.
	err = s.writeClient(client, &packets.ConnackPacket{
		FixedHeader:    packets.NewFixedHeader(packets.Connack),
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

// processPacket processes an inbound packet for a client.
func (s *Server) processPacket(cl *client, pk packets.Packet) error {

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
			return err
		}

	case *packets.PublishPacket:

		// @TODO ... Publish ACL here.

		// If message is retained, add it to the retained messages index.
		if msg.Retain {
			s.topics.RetainMessage(msg)
		}

		// Get all the clients who have a subscription matching the publish
		// packet's topic.
		subs := s.topics.Subscribers(msg.TopicName)
		for id, qos := range subs {
			if client, ok := s.clients.get(id); ok {

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
					return err
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

			err := s.writeClient(cl, out)
			if err != nil {
				s.closeClient(cl, true)
				return err
			}

			cl.inFlight.set(out.PacketID, out)
		}

	case *packets.PubrelPacket:
		if _, ok := cl.inFlight.get(msg.PacketID); ok {
			out := &packets.PubcompPacket{
				FixedHeader: packets.NewFixedHeader(packets.Pubcomp),
				PacketID:    msg.PacketID,
			}

			err := s.writeClient(cl, out)
			if err != nil {
				s.closeClient(cl, true)
				return err
			}

			cl.inFlight.delete(msg.PacketID)
		}

	case *packets.PubcompPacket:
		if _, ok := cl.inFlight.get(msg.PacketID); ok {
			cl.inFlight.delete(msg.PacketID)
		}

	case *packets.SubscribePacket:
		retCodes := make([]byte, len(msg.Topics))
		for i := 0; i < len(msg.Topics); i++ {
			// @TODO ... Subscribe ACL here.
			//		retCodes[i] = packets.ErrSubAckNetworkError
			s.topics.Subscribe(msg.Topics[i], cl.id, msg.Qoss[i])
			retCodes[i] = msg.Qoss[i]
		}

		// Acknowledge the subscriptions with a Suback packet.
		err := s.writeClient(cl, &packets.SubackPacket{
			FixedHeader: packets.NewFixedHeader(packets.Suback),
			PacketID:    msg.PacketID,
			ReturnCodes: retCodes,
		})

		if err != nil {
			s.closeClient(cl, true)
			return err
		}

		// Publish out any retained messages matching the subscription filter.
		for i := 0; i < len(msg.Topics); i++ {
			messages := s.topics.Messages(msg.Topics[i])
			for _, pkv := range messages {
				err := s.writeClient(cl, pkv)
				if err != nil {
					log.Println("write err B", err)
					s.closeClient(cl, true)
					return err
				}
			}
		}

	case *packets.UnsubscribePacket:
		for i := 0; i < len(msg.Topics); i++ {
			s.topics.Unsubscribe(msg.Topics[i], cl.id)
		}

		// Acknowledge the unsubscriptions with a UNSUBACK packet.
		err := s.writeClient(cl, &packets.UnsubackPacket{
			FixedHeader: packets.NewFixedHeader(packets.Unsuback),
			PacketID:    msg.PacketID,
		})
		if err != nil {
			s.closeClient(cl, true)
			return err
		}
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
