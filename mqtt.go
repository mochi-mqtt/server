package mqtt

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/mochi-co/mqtt/internal/auth"
	"github.com/mochi-co/mqtt/internal/circ"
	"github.com/mochi-co/mqtt/internal/clients"
	"github.com/mochi-co/mqtt/internal/listeners"
	"github.com/mochi-co/mqtt/internal/packets"
	"github.com/mochi-co/mqtt/internal/topics"
)

const (
	Version = "0.0.1" // the server version.

	maxPacketID = 65535 // the maximum value of a 16-bit packet ID.
)

var (
	ErrListenerIDExists     = errors.New("Listener id already exists")
	ErrReadConnectInvalid   = errors.New("Connect packet was not valid")
	ErrConnectNotAuthorized = errors.New("Connect packet was not authorized")
	ErrInvalidTopic         = errors.New("Cannot publish to $ and $SYS topics")
)

// Server is an MQTT broker server.
type Server struct {
	bytepool  circ.BytesPool      // a byte pool for packet bytes.
	Listeners listeners.Listeners // listeners listen for new connections.
	Clients   clients.Clients     // clients known to the broker.
	Topics    *topics.Index       // an index of topic subscriptions and retained messages.
	System    System              // values commonly found in $SYS topics.
}

// New returns a new instance of an MQTT broker.
func New() *Server {
	return &Server{
		bytepool:  circ.NewBytesPool(circ.DefaultBufferSize),
		Listeners: listeners.New(),
		Clients:   clients.New(),
		Topics:    topics.New(),
		System: System{
			Version: Version,
			Started: time.Now().Unix(),
		},
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

// EstablishConnection establishes a new client connection with the broker.
func (s *Server) EstablishConnection(lid string, c net.Conn, ac auth.Controller) error {
	//cl := clients.NewClient(c, circ.NewReader(0, 0), circ.NewWriter(0, 0))
	xbr := s.bytepool.Get()
	xbw := s.bytepool.Get()
	cl := clients.NewClient(c,
		circ.NewReaderFromSlice(0, xbr),
		circ.NewWriterFromSlice(0, xbw),
	)

	cl.Start()

	fh := new(packets.FixedHeader)
	err := cl.ReadFixedHeader(fh)
	if err != nil {
		return err
	}

	pk, err := cl.ReadPacket(fh)
	if err != nil {
		return err
	}

	if pk.FixedHeader.Type != packets.Connect {
		return ErrReadConnectInvalid
	}

	cl.Identify(lid, pk, ac)

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
			cl.InFlight = existing.InFlight // Inherit from existing session.
			cl.Subscriptions = existing.Subscriptions
			sessionPresent = true
		}
		existing.Unlock()
	}

	s.Clients.Add(cl) // Overwrite any existing client with the same name.

	err = s.writeClient(cl, packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Connack,
		},
		SessionPresent: sessionPresent,
		ReturnCode:     retcode,
	})
	if err != nil || retcode != packets.Accepted {
		return err
	}

	cl.ResendInflight(true)

	err = cl.Read(s.processPacket)
	if err != nil {
		s.closeClient(cl, true)
	}

	s.bytepool.Put(xbr)
	s.bytepool.Put(xbw)
	return err
}

// writeClient writes packets to a client connection.
func (s *Server) writeClient(cl *clients.Client, pk packets.Packet) error {
	fmt.Println(">>", cl.ID, pk.FixedHeader.Type, pk.FixedHeader.Qos, pk.PacketID)
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

	fmt.Println("<<", cl.ID, pk.FixedHeader.Type, pk.FixedHeader.Qos, pk.PacketID)
	switch pk.FixedHeader.Type {
	case packets.Connect:
		return s.processConnect(cl, pk)
	case packets.Disconnect:
		return s.processDisconnect(cl, pk)
	case packets.Pingreq:
		return s.processPingreq(cl, pk)
	case packets.Publish:
		r, err := pk.PublishValidate()
		if r != packets.Accepted {
			return err
		}
		return s.processPublish(cl, pk)
	case packets.Puback:
		return s.processPuback(cl, pk)
	case packets.Pubrec:
		return s.processPubrec(cl, pk)
	case packets.Pubrel:
		return s.processPubrel(cl, pk)
	case packets.Pubcomp:
		return s.processPubcomp(cl, pk)
	case packets.Subscribe:
		r, err := pk.SubscribeValidate()
		if r != packets.Accepted {
			return err
		}
		return s.processSubscribe(cl, pk)
	case packets.Unsubscribe:
		r, err := pk.UnsubscribeValidate()
		if r != packets.Accepted {
			return err
		}
		return s.processUnsubscribe(cl, pk)
	default:
		return fmt.Errorf("No valid packet available; %v", pk.FixedHeader.Type)
	}
}

// processConnect processes a Connect packet. The packet cannot be used to
// establish a new connection on an existing connection. See EstablishConnection
// instead.
func (s *Server) processConnect(cl *clients.Client, pk packets.Packet) error {
	s.closeClient(cl, true)
	return nil
}

// processDisconnect processes a Disconnect packet.
func (s *Server) processDisconnect(cl *clients.Client, pk packets.Packet) error {
	s.closeClient(cl, false)
	return nil
}

// processPingreq processes a Pingreq packet.
func (s *Server) processPingreq(cl *clients.Client, pk packets.Packet) error {
	err := s.writeClient(cl, packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Pingresp,
		},
	})
	if err != nil {
		return err
	}

	return nil
}

// processPublish processes a Publish packet.
func (s *Server) processPublish(cl *clients.Client, pk packets.Packet) error {
	if len(pk.TopicName) > 0 && pk.TopicName[0:3] == "$SYS" {
		return nil
	}

	if !cl.AC.ACL(cl.Username, pk.TopicName, true) {
		return nil
	}

	if pk.FixedHeader.Retain {
		s.Topics.RetainMessage(pk.PublishCopy())
	}

	if pk.FixedHeader.Qos > 0 {
		ack := packets.Packet{
			FixedHeader: packets.FixedHeader{
				Type: packets.Puback,
			},
			PacketID: pk.PacketID,
		}

		if pk.FixedHeader.Qos == 2 {
			ack.FixedHeader.Type = packets.Pubrec
		}

		s.writeClient(cl, ack)
		// omit errors in case of broken connection / LWT publish. ack send failures
		// will be handled by in-flight resending on next reconnect.
	}

	subs := s.Topics.Subscribers(pk.TopicName)
	for id, qos := range subs {
		if client, ok := s.Clients.Get(id); ok {
			out := pk.PublishCopy()
			if qos > out.FixedHeader.Qos { // Inherit higher desired qos values.
				out.FixedHeader.Qos = qos
			}

			if out.FixedHeader.Qos > 0 { // If QoS required, save to inflight index.
				if out.PacketID == 0 {
					out.PacketID = uint16(client.NextPacketID())
				}

				// If a message has a QoS, we need to ensure it is delivered to
				// the client at some point, one way or another. Store the publish
				// packet in the client's inflight queue and attempt to redeliver
				// if an appropriate ack is not received (or if the client is offline).
				client.InFlight.Set(out.PacketID, clients.InFlightMessage{
					Packet: out,
					Sent:   time.Now().Unix(),
				})
				fmt.Println(client.ID, "SETB", out.FixedHeader.Type, out.FixedHeader.Qos, out.PacketID)
			}

			s.writeClient(client, out)
		}
	}

	return nil
}

// processPuback processes a Puback packet.
func (s *Server) processPuback(cl *clients.Client, pk packets.Packet) error {
	cl.InFlight.Delete(pk.PacketID)
	return nil
}

// processPubrec processes a Pubrec packet.
func (s *Server) processPubrec(cl *clients.Client, pk packets.Packet) error {
	out := packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Pubrel,
			Qos:  1,
		},
		PacketID: pk.PacketID,
	}

	err := s.writeClient(cl, out)
	if err != nil {
		return err
	}

	return nil
}

// processPubrel processes a Pubrel packet.
func (s *Server) processPubrel(cl *clients.Client, pk packets.Packet) error {
	out := packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Pubcomp,
		},
		PacketID: pk.PacketID,
	}

	err := s.writeClient(cl, out)
	if err != nil {
		return err
	}
	cl.InFlight.Delete(pk.PacketID)

	return nil
}

// processPubcomp processes a Pubcomp packet.
func (s *Server) processPubcomp(cl *clients.Client, pk packets.Packet) error {
	cl.InFlight.Delete(pk.PacketID)
	return nil
}

// processSubscribe processes a Subscribe packet.
func (s *Server) processSubscribe(cl *clients.Client, pk packets.Packet) error {
	retCodes := make([]byte, len(pk.Topics))
	for i := 0; i < len(pk.Topics); i++ {
		if !cl.AC.ACL(cl.Username, pk.Topics[i], false) {
			retCodes[i] = packets.ErrSubAckNetworkError
		} else {
			s.Topics.Subscribe(pk.Topics[i], cl.ID, pk.Qoss[i])
			cl.NoteSubscription(pk.Topics[i], pk.Qoss[i])
			retCodes[i] = pk.Qoss[i]
		}
	}

	err := s.writeClient(cl, packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Suback,
		},
		PacketID:    pk.PacketID,
		ReturnCodes: retCodes,
	})
	if err != nil {
		return err
	}

	// Publish out any retained messages matching the subscription filter.
	for i := 0; i < len(pk.Topics); i++ {
		fmt.Println(cl.ID, "retained for", pk.Topics[i])
		for _, pkv := range s.Topics.Messages(pk.Topics[i]) {
			fmt.Println(">>", cl.ID, pkv.FixedHeader.Type, pkv.FixedHeader.Qos, pkv.PacketID)
			err := s.writeClient(cl, pkv)
			if err != nil {
				fmt.Println("writeClient got err", err)
				//return err
			}
		}
	}

	return nil
}

// processUnsubscribe processes an unsubscribe packet.
func (s *Server) processUnsubscribe(cl *clients.Client, pk packets.Packet) error {
	for i := 0; i < len(pk.Topics); i++ {
		s.Topics.Unsubscribe(pk.Topics[i], cl.ID)
		cl.ForgetSubscription(pk.Topics[i])
	}

	err := s.writeClient(cl, packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Unsuback,
		},
		PacketID: pk.PacketID,
	})
	if err != nil {
		return err
	}

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
	for _, cl := range clients {
		s.closeClient(cl, false) // omit errors
	}

}

// closeClient closes a client connection and publishes any LWT messages.
func (s *Server) closeClient(cl *clients.Client, sendLWT bool) error {
	if sendLWT && cl.LWT.Topic != "" {
		err := s.processPublish(cl, packets.Packet{
			FixedHeader: packets.FixedHeader{
				Type:   packets.Publish,
				Retain: cl.LWT.Retain,
				Qos:    cl.LWT.Qos,
			},
			TopicName: cl.LWT.Topic,
			Payload:   cl.LWT.Message,
		})

		if err != nil {
			return err
		}
	}

	cl.Stop()

	return nil
}

// System contains atomic counters and values for various server statistics
// commonly found in $SYS topics.
type System struct {
	BytesRecv           int64  // The total number of bytes received in all packets.
	BytesSent           int64  // The total number of bytes sent to clients.
	ClientsConnected    int64  // The number of currently connected clients.
	ClientsDisconnected int64  // The number of disconnected non-cleansession clients.
	ClientsMax          int64  // The maximum number of clients that have been concurrently connected.
	ClientsTotal        int64  // The sum of all clients, connected and disconnected.
	MessagesRecv        int64  // The total number of packets received.
	MessagesSent        int64  // The total number of packets sent.
	PublishDropped      int64  // The number of in-flight publish messages which were dropped.
	PublishRecv         int64  // The total number of received publish packets.
	PublishSent         int64  // The total number of sent publish packets.
	Retained            int64  // The number of messages currently retained.
	InFlight            int64  // The number of messages currently in-flight.
	Subscriptions       int64  // The total number of filter subscriptions.
	Started             int64  // The time the server started in unix seconds.
	Version             string // The current version of the server.
}
