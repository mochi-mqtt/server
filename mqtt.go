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
	//dbg "github.com/mochi-co/debug"
)

const (
	maxPacketID = 65535 // maxPacketID is the maximum value of a 16-bit packet ID.
)

var (
	ErrListenerIDExists     = errors.New("Listener id already exists")
	ErrReadConnectInvalid   = errors.New("Connect packet was not valid")
	ErrConnectNotAuthorized = errors.New("Connect packet was not authorized")
)

// Server is an MQTT broker server.
type Server struct {
	bytepool  circ.BytesPool
	Listeners listeners.Listeners // listeners listen for new connections.
	Clients   clients.Clients     // clients known to the broker.
	Topics    *topics.Index       // an index of topic subscriptions and retained messages.
}

// New returns a new instance of an MQTT broker.
func New() *Server {
	return &Server{
		bytepool:  circ.NewBytesPool(circ.DefaultBufferSize),
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

// EstablishConnection establishes a new client connection with the broker.
func (s *Server) EstablishConnection(lid string, c net.Conn, ac auth.Controller) error {
	//client := clients.NewClient(c, circ.NewReader(0, 0), circ.NewWriter(0, 0))
	client := clients.NewClient(c,
		circ.NewReaderFromSlice(0, s.bytepool.Get()),
		circ.NewWriterFromSlice(0, s.bytepool.Get()),
	)

	client.Start()

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

	s.Clients.Add(client)

	err = s.writeClient(client, packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Connack,
		},
		SessionPresent: sessionPresent,
		ReturnCode:     retcode,
	})

	if err != nil || retcode != packets.Accepted {
		return err
	}

	err = s.ResendInflight(client)
	if err != nil {
		return err
	}

	var sendLWT bool
	err = client.Read(s.processPacket)
	if err != nil {
		sendLWT = true // Only send LWT on bad disconnect [MQTT-3.14.4-3]
	}

	s.closeClient(client, sendLWT)

	return err
}

// writeClient writes packets to a client connection.
func (s *Server) writeClient(cl *clients.Client, pk packets.Packet) error {
	_, err := cl.WritePacket(pk)
	if err != nil {
		return err
	}

	// Log $SYS stats.
	// @TODO ...

	return nil
}

// ResendInflight republishes any inflight messages to the client.
func (s *Server) ResendInflight(cl *clients.Client) error {
	for _, pk := range cl.InFlight.GetAll() {
		err := s.writeClient(cl, pk.Packet)
		if err != nil {
			return err
		}
	}

	return nil
}

// processPacket processes an inbound packet for a client. Since the method is
// typically called as a goroutine, errors are mostly for test checking purposes.
func (s *Server) processPacket(cl *clients.Client, pk packets.Packet) (close bool, err error) {
	switch pk.FixedHeader.Type {
	case packets.Connect:
		return s.processConnect(cl, pk)
	case packets.Disconnect:
		return s.processDisconnect(cl, pk)
	case packets.Pingreq:
		return s.processPingreq(cl, pk)
	case packets.Publish:
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
		return s.processSubscribe(cl, pk)
	case packets.Unsubscribe:
		return s.processUnsubscribe(cl, pk)
	default:
		return false, fmt.Errorf("No valid packet available; %v", pk.FixedHeader.Type)
	}
}

// processConnect processes a Connect packet. The packet cannot be used to
// establish a new connection on an existing connection. See EstablishConnection
// instead.
func (s *Server) processConnect(cl *clients.Client, pk packets.Packet) (close bool, err error) {
	s.closeClient(cl, true)
	return
}

// processDisconnect processes a Disconnect packet.
func (s *Server) processDisconnect(cl *clients.Client, pk packets.Packet) (close bool, err error) {
	s.closeClient(cl, true)
	return true, nil
}

// processPingreq processes a Pingreq packet.
func (s *Server) processPingreq(cl *clients.Client, pk packets.Packet) (close bool, err error) {
	err = s.writeClient(cl, packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Pingresp,
		},
	})

	return
}

// processPublish processes a Publish packet.
func (s *Server) processPublish(cl *clients.Client, pk packets.Packet) (close bool, err error) {
	if !cl.AC.ACL(cl.Username, pk.TopicName, true) {
		return
	}

	if pk.FixedHeader.Retain {
		s.Topics.RetainMessage(pk)
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
			cl.InFlight.Set(pk.PacketID, clients.InFlightMessage{
				Packet: ack,
				Sent:   time.Now().Unix(),
			})
		}

		err = s.writeClient(cl, ack)
		if err != nil {
			return
		}
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

				client.InFlight.Set(out.PacketID, clients.InFlightMessage{
					Packet: out,
					Sent:   time.Now().Unix(),
				})
			}

			s.writeClient(client, out)
			// omit errors; they are averted through manual packet value setting.
		}
	}

	return
}

// processPuback processes a Puback packet.
func (s *Server) processPuback(cl *clients.Client, pk packets.Packet) (close bool, err error) {
	cl.InFlight.Delete(pk.PacketID)
	return
}

// processPubrec processes a Pubrec packet.
func (s *Server) processPubrec(cl *clients.Client, pk packets.Packet) (close bool, err error) {
	if _, ok := cl.InFlight.Get(pk.PacketID); ok {
		out := packets.Packet{
			FixedHeader: packets.FixedHeader{
				Type: packets.Pubrel,
				Qos:  1,
			},
			PacketID: pk.PacketID,
		}

		cl.InFlight.Set(out.PacketID, clients.InFlightMessage{
			Packet: out,
			Sent:   time.Now().Unix(),
		})
		err = s.writeClient(cl, out)
	}

	return
}

// processPubrel processes a Pubrel packet.
func (s *Server) processPubrel(cl *clients.Client, pk packets.Packet) (close bool, err error) {
	if _, ok := cl.InFlight.Get(pk.PacketID); ok {
		out := packets.Packet{
			FixedHeader: packets.FixedHeader{
				Type: packets.Pubcomp,
			},
			PacketID: pk.PacketID,
		}

		err = s.writeClient(cl, out)
		cl.InFlight.Delete(pk.PacketID)
	}
	return
}

// processPubcomp processes a Pubcomp packet.
func (s *Server) processPubcomp(cl *clients.Client, pk packets.Packet) (close bool, err error) {
	if _, ok := cl.InFlight.Get(pk.PacketID); ok {
		cl.InFlight.Delete(pk.PacketID)
	}
	return
}

// processSubscribe processes a Subscribe packet.
func (s *Server) processSubscribe(cl *clients.Client, pk packets.Packet) (close bool, err error) {
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

	err = s.writeClient(cl, packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Suback,
		},
		PacketID:    pk.PacketID,
		ReturnCodes: retCodes,
	})
	if err != nil {
		return
	}

	// Publish out any retained messages matching the subscription filter.
	for i := 0; i < len(pk.Topics); i++ {
		for _, pkv := range s.Topics.Messages(pk.Topics[i]) {
			err := s.writeClient(cl, pkv)
			if err != nil {
				return false, err
			}
		}
	}

	return
}

// processUnsubscribe processes an unsubscribe packet.
func (s *Server) processUnsubscribe(cl *clients.Client, pk packets.Packet) (close bool, err error) {
	for i := 0; i < len(pk.Topics); i++ {
		s.Topics.Unsubscribe(pk.Topics[i], cl.ID)
		cl.ForgetSubscription(pk.Topics[i])
	}

	err = s.writeClient(cl, packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Unsuback,
		},
		PacketID: pk.PacketID,
	})
	if err != nil {
		return
	}

	return
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
	// If an LWT message is set, publish it to the topic subscribers.

	/*
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
