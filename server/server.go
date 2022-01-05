// packet server provides a MQTT 3.1.1 compliant MQTT server.
package server

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/mochi-co/mqtt/server/events"
	"github.com/mochi-co/mqtt/server/internal/circ"
	"github.com/mochi-co/mqtt/server/internal/clients"
	"github.com/mochi-co/mqtt/server/internal/packets"
	"github.com/mochi-co/mqtt/server/internal/topics"
	"github.com/mochi-co/mqtt/server/listeners"
	"github.com/mochi-co/mqtt/server/listeners/auth"
	"github.com/mochi-co/mqtt/server/persistence"
	"github.com/mochi-co/mqtt/server/system"
)

const (
	Version = "1.0.0" // the server version.
)

var (
	ErrListenerIDExists     = errors.New("Listener id already exists")
	ErrReadConnectInvalid   = errors.New("Connect packet was not valid")
	ErrConnectNotAuthorized = errors.New("Connect packet was not authorized")
	ErrInvalidTopic         = errors.New("Cannot publish to $ and $SYS topics")

	// SysTopicInterval is the number of milliseconds between $SYS topic publishes.
	SysTopicInterval time.Duration = 30000

	// inflightResendBackoff is a slice of seconds, which determines the
	// interval between inflight resend attempts.
	inflightResendBackoff = []int64{0, 1, 2, 10, 60, 120, 600, 3600, 21600}

	// inflightMaxResends is the maximum number of times to try resending QoS promises.
	inflightMaxResends = 6
)

// Server is an MQTT broker server. It should be created with server.New()
// in order to ensure all the internal fields are correctly populated.
type Server struct {
	Listeners *listeners.Listeners // listeners are network interfaces which listen for new connections.
	Clients   *clients.Clients     // clients which are known to the broker.
	Topics    *topics.Index        // an index of topic filter subscriptions and retained messages.
	System    *system.Info         // values about the server commonly found in $SYS topics.
	Store     persistence.Store    // a persistent storage backend if desired.
	done      chan bool            // indicate that the server is ending.
	bytepool  circ.BytesPool       // a byte pool for incoming and outgoing packets.
	sysTicker *time.Ticker         // the interval ticker for sending updating $SYS topics.
	inline    inlineMessages       // channels for direct publishing.
	Events    events.Events        // overrideable event hooks.
}

// inlineMessages contains channels for handling inline (direct) publishing.
type inlineMessages struct {
	done chan bool           // indicate that the server is ending.
	pub  chan packets.Packet // a channel of packets to publish to clients
}

// New returns a new instance of an MQTT broker.
func New() *Server {
	s := &Server{
		done:     make(chan bool),
		bytepool: circ.NewBytesPool(circ.DefaultBufferSize),
		Clients:  clients.New(),
		Topics:   topics.New(),
		System: &system.Info{
			Version: Version,
			Started: time.Now().Unix(),
		},
		sysTicker: time.NewTicker(SysTopicInterval * time.Millisecond),
		inline: inlineMessages{
			done: make(chan bool),
			pub:  make(chan packets.Packet, 1024),
		},
		Events: events.Events{},
	}

	// Expose server stats using the system listener so it can be used in the
	// dashboard and other more experimental listeners.
	s.Listeners = listeners.New(s.System)

	return s
}

// AddStore assigns a persistent storage backend to the server. This must be
// called before calling server.Server().
func (s *Server) AddStore(p persistence.Store) error {
	s.Store = p
	err := s.Store.Open()
	if err != nil {
		return err
	}

	return nil
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
	err := listener.Listen(s.System)
	if err != nil {
		return err
	}

	return nil
}

// Serve starts the event loops responsible for establishing client connections
// on all attached listeners, and publishing the system topics.
func (s *Server) Serve() error {
	if s.Store != nil {
		err := s.readStore()
		if err != nil {
			return err
		}
	}

	go s.eventLoop()                            // spin up event loop for issuing $SYS values and closing server.
	go s.inlineClient()                         // spin up inline client for direct message publishing.
	s.Listeners.ServeAll(s.EstablishConnection) // start listening on all listeners.
	s.publishSysTopics()                        // begin publishing $SYS system values.

	return nil
}

// eventLoop loops forever, running various server processes at different intervals.
func (s *Server) eventLoop() {
	for {
		select {
		case <-s.done:
			s.sysTicker.Stop()
			close(s.inline.done)
			return
		case <-s.sysTicker.C:
			s.publishSysTopics()
		}
	}
}

// inlineClient loops forever, sending directly-published messages
// from the Publish method to subscribers.
func (s *Server) inlineClient() {
	for {
		select {
		case <-s.inline.done:
			close(s.inline.pub)
			return
		case pk := <-s.inline.pub:
			s.publishToSubscribers(pk)
		}
	}
}

// EstablishConnection establishes a new client when a listener
// accepts a new connection.
func (s *Server) EstablishConnection(lid string, c net.Conn, ac auth.Controller) error {
	xbr := s.bytepool.Get() // Get byte buffer from pools for receiving packet data.
	xbw := s.bytepool.Get() // and for sending.

	cl := clients.NewClient(c,
		circ.NewReaderFromSlice(0, xbr),
		circ.NewWriterFromSlice(0, xbw),
		s.System,
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

	atomic.AddInt64(&s.System.ConnectionsTotal, 1)
	atomic.AddInt64(&s.System.ClientsConnected, 1)

	var sessionPresent bool
	if existing, ok := s.Clients.Get(pk.ClientIdentifier); ok {
		existing.Lock()
		if atomic.LoadInt64(&existing.State.Done) == 1 {
			atomic.AddInt64(&s.System.ClientsDisconnected, -1)
		}
		existing.Stop()
		if pk.CleanSession {
			for k := range existing.Subscriptions {
				delete(existing.Subscriptions, k)
				q := s.Topics.Unsubscribe(k, existing.ID)
				if q {
					atomic.AddInt64(&s.System.Subscriptions, -1)
				}
			}
		} else {
			cl.Inflight = existing.Inflight // Inherit from existing session.
			cl.Subscriptions = existing.Subscriptions
			sessionPresent = true
		}
		existing.Unlock()
	} else {
		atomic.AddInt64(&s.System.ClientsTotal, 1)
		if atomic.LoadInt64(&s.System.ClientsConnected) > atomic.LoadInt64(&s.System.ClientsMax) {
			atomic.AddInt64(&s.System.ClientsMax, 1)
		}
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

	s.ResendClientInflight(cl, true)

	if s.Store != nil {
		s.Store.WriteClient(persistence.Client{
			ID:       "cl_" + cl.ID,
			ClientID: cl.ID,
			T:        persistence.KClient,
			Listener: cl.Listener,
			Username: cl.Username,
			LWT:      persistence.LWT(cl.LWT),
		})
	}

	err = cl.Read(s.processPacket)
	if err != nil {
		s.closeClient(cl, true)
	}

	s.bytepool.Put(xbr) // Return byte buffers to pools when the client has finished.
	s.bytepool.Put(xbw)

	atomic.AddInt64(&s.System.ClientsConnected, -1)
	atomic.AddInt64(&s.System.ClientsDisconnected, 1)

	return err
}

// writeClient writes packets to a client connection.
func (s *Server) writeClient(cl *clients.Client, pk packets.Packet) error {
	_, err := cl.WritePacket(pk)
	if err != nil {
		return err
	}

	return nil
}

// processPacket processes an inbound packet for a client. Since the method is
// typically called as a goroutine, errors are primarily for test checking purposes.
func (s *Server) processPacket(cl *clients.Client, pk packets.Packet) error {
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

// Publish creates a publish packet from a payload and sends it to the inline.pub
// channel, where it is written directly to the outgoing byte buffers of any
// clients subscribed to the given topic. Because the message is written directly
// within the server, QoS is inherently 2 (exactly once).
func (s *Server) Publish(topic string, payload []byte, retain bool) error {
	if len(topic) >= 4 && topic[0:4] == "$SYS" {
		return ErrInvalidTopic
	}

	pk := packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Publish,
		},
		TopicName: topic,
		Payload:   payload,
	}

	if retain {
		s.retainMessage(pk)
	}

	// handoff packet to s.inline.pub channel for writing to client buffers
	// to avoid blocking the calling function.
	s.inline.pub <- pk

	return nil
}

// processPublish processes a Publish packet.
func (s *Server) processPublish(cl *clients.Client, pk packets.Packet) error {
	if len(pk.TopicName) >= 4 && pk.TopicName[0:4] == "$SYS" {
		return nil // Clients can't publish to $SYS topics, so fail silently as per spec.
	}

	if !cl.AC.ACL(cl.Username, pk.TopicName, true) {
		return nil
	}

	if pk.FixedHeader.Retain {
		s.retainMessage(pk)
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

		// omit errors in case of broken connection / LWT publish. ack send failures
		// will be handled by in-flight resending on next reconnect.
		s.writeClient(cl, ack)
	}

	// If an OnMessage hook exists, trigger it.
	if s.Events.OnMessage != nil {
		s.Events.OnMessage(events.FromClient(*cl), events.Packet(pk))
	}

	// if an OnMessageModify hook exists, potentially modify the packet.
	if s.Events.OnMessageModify != nil {
		if pkx, err := s.Events.OnMessageModify(events.FromClient(*cl), events.Packet(pk)); err == nil {
			pk = packets.Packet(pkx)
		}
	}

	// write packet to the byte buffers of any clients with matching topic filters.
	s.publishToSubscribers(pk)

	return nil
}

// retainMessage adds a message to a topic, and if a persistent store is provided,
// adds the message to the store so it can be reloaded if necessary.
func (s *Server) retainMessage(pk packets.Packet) {
	out := pk.PublishCopy()
	q := s.Topics.RetainMessage(out)
	atomic.AddInt64(&s.System.Retained, q)
	if s.Store != nil {
		if q == 1 {
			s.Store.WriteRetained(persistence.Message{
				ID:          "ret_" + out.TopicName,
				T:           persistence.KRetained,
				FixedHeader: persistence.FixedHeader(out.FixedHeader),
				TopicName:   out.TopicName,
				Payload:     out.Payload,
			})
		} else {
			s.Store.DeleteRetained("ret_" + out.TopicName)
		}
	}
}

// publishToSubscribers publishes a publish packet to all subscribers with
// matching topic filters.
func (s *Server) publishToSubscribers(pk packets.Packet) {
	for id, qos := range s.Topics.Subscribers(pk.TopicName) {
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
				sent := time.Now().Unix()
				q := client.Inflight.Set(out.PacketID, clients.InflightMessage{
					Packet: out,
					Sent:   sent,
				})
				if q {
					atomic.AddInt64(&s.System.Inflight, 1)
				}

				if s.Store != nil {
					s.Store.WriteInflight(persistence.Message{
						ID:          "if_" + client.ID + "_" + strconv.Itoa(int(out.PacketID)),
						T:           persistence.KRetained,
						FixedHeader: persistence.FixedHeader(out.FixedHeader),
						TopicName:   out.TopicName,
						Payload:     out.Payload,
						Sent:        sent,
					})
				}
			}

			s.writeClient(client, out)
		}
	}
}

// processPuback processes a Puback packet.
func (s *Server) processPuback(cl *clients.Client, pk packets.Packet) error {
	q := cl.Inflight.Delete(pk.PacketID)
	if q {
		atomic.AddInt64(&s.System.Inflight, -1)
	}
	if s.Store != nil {
		s.Store.DeleteInflight("if_" + cl.ID + "_" + strconv.Itoa(int(pk.PacketID)))
	}
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
	q := cl.Inflight.Delete(pk.PacketID)
	if q {
		atomic.AddInt64(&s.System.Inflight, -1)
	}

	if s.Store != nil {
		s.Store.DeleteInflight("if_" + cl.ID + "_" + strconv.Itoa(int(pk.PacketID)))
	}

	return nil
}

// processPubcomp processes a Pubcomp packet.
func (s *Server) processPubcomp(cl *clients.Client, pk packets.Packet) error {
	q := cl.Inflight.Delete(pk.PacketID)
	if q {
		atomic.AddInt64(&s.System.Inflight, -1)
	}
	if s.Store != nil {
		s.Store.DeleteInflight("if_" + cl.ID + "_" + strconv.Itoa(int(pk.PacketID)))
	}
	return nil
}

// processSubscribe processes a Subscribe packet.
func (s *Server) processSubscribe(cl *clients.Client, pk packets.Packet) error {
	retCodes := make([]byte, len(pk.Topics))
	for i := 0; i < len(pk.Topics); i++ {
		if !cl.AC.ACL(cl.Username, pk.Topics[i], false) {
			retCodes[i] = packets.ErrSubAckNetworkError
		} else {
			q := s.Topics.Subscribe(pk.Topics[i], cl.ID, pk.Qoss[i])
			if q {
				atomic.AddInt64(&s.System.Subscriptions, 1)
			}
			cl.NoteSubscription(pk.Topics[i], pk.Qoss[i])
			retCodes[i] = pk.Qoss[i]

			if s.Store != nil {
				s.Store.WriteSubscription(persistence.Subscription{
					ID:     "sub_" + cl.ID + ":" + pk.Topics[i],
					T:      persistence.KSubscription,
					Filter: pk.Topics[i],
					Client: cl.ID,
					QoS:    pk.Qoss[i],
				})
			}
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
		for _, pkv := range s.Topics.Messages(pk.Topics[i]) {
			s.writeClient(cl, pkv) // omit errors, prefer continuing.
		}
	}

	return nil
}

// processUnsubscribe processes an unsubscribe packet.
func (s *Server) processUnsubscribe(cl *clients.Client, pk packets.Packet) error {
	for i := 0; i < len(pk.Topics); i++ {
		q := s.Topics.Unsubscribe(pk.Topics[i], cl.ID)
		if q {
			atomic.AddInt64(&s.System.Subscriptions, -1)
		}
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

// publishSysTopics publishes the current values to the server $SYS topics.
// Due to the int to string conversions this method is not as cheap as
// some of the others so the publishing interval should be set appropriately.
func (s *Server) publishSysTopics() {
	pk := packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type:   packets.Publish,
			Retain: true,
		},
	}

	s.System.Uptime = time.Now().Unix() - s.System.Started
	topics := map[string]string{
		"$SYS/broker/version":                   s.System.Version,
		"$SYS/broker/uptime":                    strconv.Itoa(int(s.System.Uptime)),
		"$SYS/broker/timestamp":                 strconv.Itoa(int(s.System.Started)),
		"$SYS/broker/load/bytes/received":       strconv.Itoa(int(s.System.BytesRecv)),
		"$SYS/broker/load/bytes/sent":           strconv.Itoa(int(s.System.BytesSent)),
		"$SYS/broker/clients/connected":         strconv.Itoa(int(s.System.ClientsConnected)),
		"$SYS/broker/clients/disconnected":      strconv.Itoa(int(s.System.ClientsDisconnected)),
		"$SYS/broker/clients/maximum":           strconv.Itoa(int(s.System.ClientsMax)),
		"$SYS/broker/clients/total":             strconv.Itoa(int(s.System.ClientsTotal)),
		"$SYS/broker/connections/total":         strconv.Itoa(int(s.System.ConnectionsTotal)),
		"$SYS/broker/messages/received":         strconv.Itoa(int(s.System.MessagesRecv)),
		"$SYS/broker/messages/sent":             strconv.Itoa(int(s.System.MessagesSent)),
		"$SYS/broker/messages/publish/dropped":  strconv.Itoa(int(s.System.PublishDropped)),
		"$SYS/broker/messages/publish/received": strconv.Itoa(int(s.System.PublishRecv)),
		"$SYS/broker/messages/publish/sent":     strconv.Itoa(int(s.System.PublishSent)),
		"$SYS/broker/messages/retained/count":   strconv.Itoa(int(s.System.Retained)),
		"$SYS/broker/messages/inflight":         strconv.Itoa(int(s.System.Inflight)),
		"$SYS/broker/subscriptions/count":       strconv.Itoa(int(s.System.Subscriptions)),
	}

	for topic, payload := range topics {
		pk.TopicName = topic
		pk.Payload = []byte(payload)
		q := s.Topics.RetainMessage(pk.PublishCopy())
		atomic.AddInt64(&s.System.Retained, q)
		s.publishToSubscribers(pk)
	}

	if s.Store != nil {
		s.Store.WriteServerInfo(persistence.ServerInfo{
			Info: *s.System,
			ID:   persistence.KServerInfo,
		})
	}
}

// ResendClientInflight attempts to resend all undelivered inflight messages
// to a client.
func (s *Server) ResendClientInflight(cl *clients.Client, force bool) error {
	if cl.Inflight.Len() == 0 {
		return nil
	}

	nt := time.Now().Unix()
	for _, tk := range cl.Inflight.GetAll() {
		if tk.Resends >= inflightMaxResends { // After a reasonable time, drop inflight packets.
			cl.Inflight.Delete(tk.Packet.PacketID)
			if tk.Packet.FixedHeader.Type == packets.Publish {
				atomic.AddInt64(&s.System.PublishDropped, 1)
			}

			if s.Store != nil {
				s.Store.DeleteInflight("if_" + cl.ID + "_" + strconv.Itoa(int(tk.Packet.PacketID)))
			}

			continue
		}

		// Only continue if the resend backoff time has passed and there's a backoff time.
		if !force && (nt-tk.Sent < inflightResendBackoff[tk.Resends] || len(inflightResendBackoff) < tk.Resends) {
			continue
		}

		if tk.Packet.FixedHeader.Type == packets.Publish {
			tk.Packet.FixedHeader.Dup = true
		}

		tk.Resends++
		tk.Sent = nt
		cl.Inflight.Set(tk.Packet.PacketID, tk)
		_, err := cl.WritePacket(tk.Packet)
		if err != nil {
			return err
		}

		if s.Store != nil {
			s.Store.WriteInflight(persistence.Message{
				ID:          "if_" + cl.ID + "_" + strconv.Itoa(int(tk.Packet.PacketID)),
				T:           persistence.KRetained,
				FixedHeader: persistence.FixedHeader(tk.Packet.FixedHeader),
				TopicName:   tk.Packet.TopicName,
				Payload:     tk.Packet.Payload,
				Sent:        tk.Sent,
				Resends:     tk.Resends,
			})
		}
	}

	return nil
}

// Close attempts to gracefully shutdown the server, all listeners, clients, and stores.
func (s *Server) Close() error {
	close(s.done)
	s.Listeners.CloseAll(s.closeListenerClients)

	if s.Store != nil {
		s.Store.Close()
	}

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
		s.processPublish(cl, packets.Packet{
			FixedHeader: packets.FixedHeader{
				Type:   packets.Publish,
				Retain: cl.LWT.Retain,
				Qos:    cl.LWT.Qos,
			},
			TopicName: cl.LWT.Topic,
			Payload:   cl.LWT.Message,
		})
	}

	cl.Stop()

	return nil
}

// readStore reads in any data from the persistent datastore (if applicable).
func (s *Server) readStore() error {
	info, err := s.Store.ReadServerInfo()
	if err != nil {
		return fmt.Errorf("load server info; %w", err)
	}
	s.loadServerInfo(info)

	clients, err := s.Store.ReadClients()
	if err != nil {
		return fmt.Errorf("load clients; %w", err)
	}
	s.loadClients(clients)

	subs, err := s.Store.ReadSubscriptions()
	if err != nil {
		return fmt.Errorf("load subscriptions; %w", err)
	}
	s.loadSubscriptions(subs)

	inflight, err := s.Store.ReadInflight()
	if err != nil {
		return fmt.Errorf("load inflight; %w", err)
	}
	s.loadInflight(inflight)

	retained, err := s.Store.ReadRetained()
	if err != nil {
		return fmt.Errorf("load retained; %w", err)
	}
	s.loadRetained(retained)

	return nil
}

// loadServerInfo restores server info from the datastore.
func (s *Server) loadServerInfo(v persistence.ServerInfo) {
	version := s.System.Version
	s.System = &v.Info
	s.System.Version = version
}

// loadSubscriptions restores subscriptions from the datastore.
func (s *Server) loadSubscriptions(v []persistence.Subscription) {
	for _, sub := range v {
		s.Topics.Subscribe(sub.Filter, sub.Client, sub.QoS)
		if cl, ok := s.Clients.Get(sub.Client); ok {
			cl.NoteSubscription(sub.Filter, sub.QoS)
		}
	}
}

// loadClients restores clients from the datastore.
func (s *Server) loadClients(v []persistence.Client) {
	for _, c := range v {
		cl := clients.NewClientStub(s.System)
		cl.ID = c.ClientID
		cl.Listener = c.Listener
		cl.Username = c.Username
		cl.LWT = clients.LWT(c.LWT)
		s.Clients.Add(cl)
	}
}

// loadInflight restores inflight messages from the datastore.
func (s *Server) loadInflight(v []persistence.Message) {
	for _, msg := range v {
		if client, ok := s.Clients.Get(msg.Client); ok {
			client.Inflight.Set(msg.PacketID, clients.InflightMessage{
				Packet: packets.Packet{
					FixedHeader: packets.FixedHeader(msg.FixedHeader),
					PacketID:    msg.PacketID,
					TopicName:   msg.TopicName,
					Payload:     msg.Payload,
				},
				Sent:    msg.Sent,
				Resends: msg.Resends,
			})
		}
	}
}

// loadRetained restores retained messages from the datastore.
func (s *Server) loadRetained(v []persistence.Message) {
	for _, msg := range v {
		s.Topics.RetainMessage(packets.Packet{
			FixedHeader: packets.FixedHeader(msg.FixedHeader),
			TopicName:   msg.TopicName,
			Payload:     msg.Payload,
		})
	}
}
