// package server provides a MQTT 3.1.1 compliant MQTT server.
package server

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/mochi-co/mqtt/server/events"
	"github.com/mochi-co/mqtt/server/internal/circ"
	"github.com/mochi-co/mqtt/server/internal/clients"
	"github.com/mochi-co/mqtt/server/internal/packets"
	"github.com/mochi-co/mqtt/server/internal/topics"
	"github.com/mochi-co/mqtt/server/internal/utils"
	"github.com/mochi-co/mqtt/server/listeners"
	"github.com/mochi-co/mqtt/server/listeners/auth"
	"github.com/mochi-co/mqtt/server/persistence"
	"github.com/mochi-co/mqtt/server/system"
)

const (
	// Version indicates the current server version.
	Version = "1.1.1"

	// defaultInflightTTL is the number of seconds a pending inflight message should last.
	defaultInflightTTL int64 = 60 * 60 * 24
)

var (
	// ErrListenerIDExists indicates that a listener with the same id already exists.
	ErrListenerIDExists = errors.New("listener id already exists")

	// ErrReadConnectInvalid indicates that the connection packet was invalid.
	ErrReadConnectInvalid = errors.New("connect packet was not valid")

	// ErrConnectNotAuthorized indicates that the connection packet had incorrect auth values.
	ErrConnectNotAuthorized = errors.New("connect packet was not authorized")

	// ErrInvalidTopic indicates that the specified topic was not valid.
	ErrInvalidTopic = errors.New("cannot publish to $ and $SYS topics")

	// ErrRejectPacket indicates that a packet should be dropped instead of processed.
	ErrRejectPacket = errors.New("packet rejected")

	// ErrClientDisconnect indicates that a client disconnected from the server.
	ErrClientDisconnect = errors.New("client disconnected")

	// ErrClientReconnect indicates that a client attempted to reconnect while still connected.
	ErrClientReconnect = errors.New("client sent connect while connected")

	// ErrServerShutdown is propagated when the server shuts down.
	ErrServerShutdown = errors.New("server is shutting down")

	// ErrSessionReestablished indicates that an existing client was replaced by a newly connected
	// client. The existing client is disconnected.
	ErrSessionReestablished = errors.New("client session re-established")

	// ErrConnectionFailed indicates that a client connection attempt failed for other reasons.
	ErrConnectionFailed = errors.New("connection attempt failed")

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
	inline               inlineMessages       // channels for direct publishing.
	Events               events.Events        // overrideable event hooks.
	Store                persistence.Store    // a persistent storage backend if desired.
	Options              *Options             // configurable server options.
	Listeners            *listeners.Listeners // listeners are network interfaces which listen for new connections.
	Clients              *clients.Clients     // clients which are known to the broker.
	Topics               *topics.Index        // an index of topic filter subscriptions and retained messages.
	System               *system.Info         // values about the server commonly found in $SYS topics.
	bytepool             *circ.BytesPool      // a byte pool for incoming and outgoing packets.
	sysTicker            *time.Ticker         // the interval ticker for sending updating $SYS topics.
	inflightExpiryTicker *time.Ticker         // the interval ticker for cleaning up expired messages.
	inflightResendTicker *time.Ticker         // the interval ticker for resending unresolved inflight messages.
	done                 chan bool            // indicate that the server is ending.
}

// Options contains configurable options for the server.
type Options struct {
	// BufferSize overrides the default buffer size (circ.DefaultBufferSize) for the client buffers.
	BufferSize int

	// BufferBlockSize overrides the default buffer block size (DefaultBlockSize) for the client buffers.
	BufferBlockSize int

	// InflightTTL specifies the duration that a queued inflight message should exist before being purged.
	InflightTTL int64
}

// inlineMessages contains channels for handling inline (direct) publishing.
type inlineMessages struct {
	done chan bool           // indicate that the server is ending.
	pub  chan packets.Packet // a channel of packets to publish to clients
}

// New returns a new instance of MQTT server with no options.
// This method has been deprecated and will be removed in a future release.
// Please use NewServer instead.
func New() *Server {
	return NewServer(nil)
}

// NewServer returns a new instance of an MQTT broker with optional values where applicable.
func NewServer(opts *Options) *Server {
	if opts == nil {
		opts = new(Options)
	}

	if opts.InflightTTL < 1 {
		opts.InflightTTL = defaultInflightTTL
	}

	s := &Server{
		done:     make(chan bool),
		bytepool: circ.NewBytesPool(opts.BufferSize),
		Clients:  clients.New(),
		Topics:   topics.New(),
		System: &system.Info{
			Version: Version,
			Started: time.Now().Unix(),
		},
		sysTicker:            time.NewTicker(SysTopicInterval * time.Millisecond),
		inflightExpiryTicker: time.NewTicker(time.Duration(opts.InflightTTL) * time.Second),
		inflightResendTicker: time.NewTicker(time.Duration(10) * time.Second),
		inline: inlineMessages{
			done: make(chan bool),
			pub:  make(chan packets.Packet, 4096),
		},
		Events:  events.Events{},
		Options: opts,
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
	s.Store.SetInflightTTL(s.Options.InflightTTL)

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
		case <-s.inflightExpiryTicker.C:
			s.clearExpiredInflights(time.Now().Unix())
		case <-s.inflightResendTicker.C:
			s.resendPendingInflights()
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

// readConnectionPacket reads the first incoming header for a connection, and if
// acceptable, returns the valid connection packet.
func (s *Server) readConnectionPacket(cl *clients.Client) (pk packets.Packet, err error) {
	fh := new(packets.FixedHeader)
	err = cl.ReadFixedHeader(fh)
	if err != nil {
		return
	}

	pk, err = cl.ReadPacket(fh)
	if err != nil {
		return
	}

	if pk.FixedHeader.Type != packets.Connect {
		return pk, ErrReadConnectInvalid
	}

	return
}

// onError is a pass-through method which triggers the OnError
// event hook (if applicable), and returns the provided error.
func (s *Server) onError(cl events.Client, err error) error {
	if err == nil {
		return err
	}
	// Note: if the error originates from a real cause, it will
	// have been captured as the StopCause. The two cases ignored
	// below are ordinary consequences of closing the connection.
	// If one of these ordinary conditions stops the connection,
	// then the client closed or broke the connection.
	if s.Events.OnError != nil &&
		!errors.Is(err, io.EOF) {
		s.Events.OnError(cl, err)
	}

	return err
}

// onStorage is a pass-through method which delegates errors from
// the persistent storage adapter to the onError event hook.
func (s *Server) onStorage(cl events.Clientlike, err error) {
	if err == nil {
		return
	}

	_ = s.onError(cl.Info(), fmt.Errorf("storage: %w", err))
}

// EstablishConnection establishes a new client when a listener
// accepts a new connection.
func (s *Server) EstablishConnection(lid string, c net.Conn, ac auth.Controller) error {
	xbr := s.bytepool.Get() // Get byte buffer from pools for receiving packet data.
	xbw := s.bytepool.Get() // and for sending.
	defer s.bytepool.Put(xbr)
	defer s.bytepool.Put(xbw)

	cl := clients.NewClient(c,
		circ.NewReaderFromSlice(s.Options.BufferBlockSize, xbr),
		circ.NewWriterFromSlice(s.Options.BufferBlockSize, xbw),
		s.System,
	)

	cl.Start()
	defer cl.ClearBuffers()
	defer cl.Stop(nil)

	pk, err := s.readConnectionPacket(cl)
	if err != nil {
		return s.onError(cl.Info(), fmt.Errorf("read connection: %w", err))
	}

	ackCode, err := pk.ConnectValidate()
	if err != nil {
		if err := s.ackConnection(cl, ackCode, false); err != nil {
			return s.onError(cl.Info(), fmt.Errorf("invalid connection send ack: %w", err))
		}
		return s.onError(cl.Info(), fmt.Errorf("validate connection packet: %w", err))
	}

	cl.Identify(lid, pk, ac) // Set client identity values from the connection packet.

	if !ac.Authenticate(pk.Username, pk.Password) {
		if err := s.ackConnection(cl, packets.CodeConnectBadAuthValues, false); err != nil {
			return s.onError(cl.Info(), fmt.Errorf("invalid connection send ack: %w", err))
		}
		return s.onError(cl.Info(), ErrConnectionFailed)
	}

	atomic.AddInt64(&s.System.ConnectionsTotal, 1)
	atomic.AddInt64(&s.System.ClientsConnected, 1)
	defer atomic.AddInt64(&s.System.ClientsConnected, -1)
	defer atomic.AddInt64(&s.System.ClientsDisconnected, 1)

	sessionPresent := s.inheritClientSession(pk, cl)
	s.Clients.Add(cl)

	err = s.ackConnection(cl, ackCode, sessionPresent)
	if err != nil {
		return s.onError(cl.Info(), fmt.Errorf("ack connection packet: %w", err))
	}

	if sessionPresent {
		err = s.ResendClientInflight(cl, true)
		if err != nil {
			s.onError(cl.Info(), fmt.Errorf("resend in flight: %w", err)) // pass-through, no return.
		}
	}

	if s.Store != nil {
		s.onStorage(cl, s.Store.WriteClient(persistence.Client{
			ID:       "cl_" + cl.ID,
			ClientID: cl.ID,
			T:        persistence.KClient,
			Listener: cl.Listener,
			Username: cl.Username,
			LWT:      persistence.LWT(cl.LWT),
		}))
	}

	if s.Events.OnConnect != nil {
		s.Events.OnConnect(cl.Info(), events.Packet(pk))
	}

	if err := cl.Read(s.processPacket); err != nil {
		s.sendLWT(cl)
		cl.Stop(err)
	}

	err = cl.StopCause() // Determine true cause of stop.

	if cl.CleanSession {
		s.clearAbandonedInflights(cl)
	}

	if s.Events.OnDisconnect != nil {
		s.Events.OnDisconnect(cl.Info(), err)
	}

	return err
}

// ackConnection returns a Connack packet to a client.
func (s *Server) ackConnection(cl *clients.Client, ack byte, present bool) error {
	return s.writeClient(cl, packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Connack,
		},
		SessionPresent: present,
		ReturnCode:     ack,
	})
}

// inheritClientSession inherits the state of an existing client sharing the same
// connection ID. If cleanSession is true, the state of any previously existing client
// session is abandoned.
func (s *Server) inheritClientSession(pk packets.Packet, cl *clients.Client) bool {
	if existing, ok := s.Clients.Get(pk.ClientIdentifier); ok {
		existing.Lock()
		defer existing.Unlock()

		existing.Stop(ErrSessionReestablished) // Issue a stop on the old client.

		// Per [MQTT-3.1.2-6]:
		// If CleanSession is set to 1, the Client and Server MUST discard any previous Session and start a new one.
		// The state associated with a CleanSession MUST NOT be reused in any subsequent session.
		if pk.CleanSession || existing.CleanSession {
			s.unsubscribeClient(existing)
			s.clearAbandonedInflights(existing)
			return false
		}

		cl.Inflight = existing.Inflight // Take address of existing session.
		cl.Subscriptions = existing.Subscriptions
		return true

	} else {
		atomic.AddInt64(&s.System.ClientsTotal, 1)
		if atomic.LoadInt64(&s.System.ClientsConnected) > atomic.LoadInt64(&s.System.ClientsMax) {
			atomic.AddInt64(&s.System.ClientsMax, 1)
		}
		return false
	}
}

// unsubscribeClient unsubscribes a client from all of their subscriptions.
func (s *Server) unsubscribeClient(cl *clients.Client) {
	for k := range cl.Subscriptions {
		delete(cl.Subscriptions, k)
		if s.Topics.Unsubscribe(k, cl.ID) {
			if s.Events.OnUnsubscribe != nil {
				s.Events.OnUnsubscribe(k, cl.Info())
			}
			atomic.AddInt64(&s.System.Subscriptions, -1)
		}
	}
}

// writeClient writes packets to a client connection.
func (s *Server) writeClient(cl *clients.Client, pk packets.Packet) error {
	_, err := cl.WritePacket(pk)
	if err != nil {
		return fmt.Errorf("write: %w", err)
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
	s.sendLWT(cl)
	cl.Stop(ErrClientReconnect)
	return nil
}

// processDisconnect processes a Disconnect packet.
func (s *Server) processDisconnect(cl *clients.Client, pk packets.Packet) error {
	cl.Stop(ErrClientDisconnect)
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
			Type:   packets.Publish,
			Retain: retain,
		},
		TopicName: topic,
		Payload:   payload,
	}

	if retain {
		s.retainMessage(&s.inline, pk)
	}

	// handoff packet to s.inline.pub channel for writing to client buffers
	// to avoid blocking the calling function.
	s.inline.pub <- pk

	return nil
}

// Info provides pseudo-client information for the inline messages processor.
// It provides a 'client' to which inline retained messages can be assigned.
func (*inlineMessages) Info() events.Client {
	return events.Client{
		ID:       "inline",
		Remote:   "inline",
		Listener: "inline",
	}
}

// processPublish processes a Publish packet.
func (s *Server) processPublish(cl *clients.Client, pk packets.Packet) error {
	if len(pk.TopicName) >= 4 && pk.TopicName[0:4] == "$SYS" {
		return nil // Clients can't publish to $SYS topics, so fail silently as per spec.
	}

	if !cl.AC.ACL(cl.Username, pk.TopicName, true) {
		return nil
	}

	// if an OnProcessMessage hook exists, potentially modify the packet.
	if s.Events.OnProcessMessage != nil {
		pkx, err := s.Events.OnProcessMessage(cl.Info(), events.Packet(pk))
		if err == nil {
			pk = packets.Packet(pkx) // Only use the new package changes if there's no errors.
		} else {
			// If the ErrRejectPacket is return, abandon processing the packet.
			if err == ErrRejectPacket {
				return nil
			}

			if s.Events.OnError != nil {
				s.Events.OnError(cl.Info(), err)
			}
		}
	}

	if pk.FixedHeader.Retain {
		s.retainMessage(cl, pk)
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
		s.onError(cl.Info(), s.writeClient(cl, ack))
	}

	// if an OnMessage hook exists, potentially modify the packet.
	if s.Events.OnMessage != nil {
		if pkx, err := s.Events.OnMessage(cl.Info(), events.Packet(pk)); err == nil {
			pk = packets.Packet(pkx)
		}
	}

	// write packet to the byte buffers of any clients with matching topic filters.
	s.publishToSubscribers(pk)

	return nil
}

// retainMessage adds a message to a topic, and if a persistent store is provided,
// adds the message to the store so it can be reloaded if necessary.
func (s *Server) retainMessage(cl events.Clientlike, pk packets.Packet) {
	out := pk.PublishCopy()
	r := s.Topics.RetainMessage(out)
	atomic.AddInt64(&s.System.Retained, r)

	if s.Store != nil {
		id := "ret_" + out.TopicName
		if r == 1 {
			s.onStorage(cl, s.Store.WriteRetained(persistence.Message{
				ID:          id,
				T:           persistence.KRetained,
				FixedHeader: persistence.FixedHeader(out.FixedHeader),
				TopicName:   out.TopicName,
				Payload:     out.Payload,
			}))
		} else {
			s.onStorage(cl, s.Store.DeleteRetained(id))
		}
	}
}

// publishToSubscribers publishes a publish packet to all subscribers with
// matching topic filters.
func (s *Server) publishToSubscribers(pk packets.Packet) {
	for id, qos := range s.Topics.Subscribers(pk.TopicName) {
		if client, ok := s.Clients.Get(id); ok {

			// If the AllowClients value is set, only deliver the packet if the subscribed
			// client exists in the AllowClients value. For use with the OnMessage event hook
			// in cases where you want to publish messages to clients selectively.
			if pk.AllowClients != nil && !utils.InSliceString(pk.AllowClients, id) {
				continue
			}

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
					Packet:  out,
					Created: time.Now().Unix(),
					Sent:    sent,
				})
				if q {
					atomic.AddInt64(&s.System.Inflight, 1)
				}

				if s.Store != nil {
					s.onStorage(client, s.Store.WriteInflight(persistence.Message{
						ID:          persistentID(client, out),
						T:           persistence.KInflight,
						FixedHeader: persistence.FixedHeader(out.FixedHeader),
						TopicName:   out.TopicName,
						Payload:     out.Payload,
						Sent:        sent,
					}))
				}
			}

			s.onError(client.Info(), s.writeClient(client, out))
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
		s.onStorage(cl, s.Store.DeleteInflight(persistentID(cl, pk)))
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
		s.onStorage(cl, s.Store.DeleteInflight(persistentID(cl, pk)))
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
		s.onStorage(cl, s.Store.DeleteInflight(persistentID(cl, pk)))
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
			r := s.Topics.Subscribe(pk.Topics[i], cl.ID, pk.Qoss[i])
			if r {
				if s.Events.OnSubscribe != nil {
					s.Events.OnSubscribe(pk.Topics[i], cl.Info(), pk.Qoss[i])
				}
				atomic.AddInt64(&s.System.Subscriptions, 1)
			}
			cl.NoteSubscription(pk.Topics[i], pk.Qoss[i])
			retCodes[i] = pk.Qoss[i]

			if s.Store != nil {
				s.onStorage(cl, s.Store.WriteSubscription(persistence.Subscription{
					ID:     "sub_" + cl.ID + ":" + pk.Topics[i],
					T:      persistence.KSubscription,
					Filter: pk.Topics[i],
					Client: cl.ID,
					QoS:    pk.Qoss[i],
				}))
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

	// Publish out any retained messages matching the subscription filter and the user has
	// been allowed to subscribe to.
	for i := 0; i < len(pk.Topics); i++ {
		if retCodes[i] == packets.ErrSubAckNetworkError {
			continue
		}

		for _, pkv := range s.Topics.Messages(pk.Topics[i]) {
			s.onError(cl.Info(), s.writeClient(cl, pkv))
		}
	}

	return nil
}

// processUnsubscribe processes an unsubscribe packet.
func (s *Server) processUnsubscribe(cl *clients.Client, pk packets.Packet) error {
	for i := 0; i < len(pk.Topics); i++ {
		q := s.Topics.Unsubscribe(pk.Topics[i], cl.ID)
		if q {
			if s.Events.OnUnsubscribe != nil {
				s.Events.OnUnsubscribe(pk.Topics[i], cl.Info())
			}
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

// atomicItoa reads an *int64 and formats a decimal string.
func atomicItoa(ptr *int64) string {
	return strconv.FormatInt(atomic.LoadInt64(ptr), 10)
}

// persistentID return a string combining the client and packet
// identifiers for use with the persistence layer.
func persistentID(client *clients.Client, pk packets.Packet) string {
	return "if_" + client.ID + "_" + pk.FormatID()
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

	uptime := time.Now().Unix() - atomic.LoadInt64(&s.System.Started)
	atomic.StoreInt64(&s.System.Uptime, uptime)
	topics := map[string]string{
		"$SYS/broker/version":                   s.System.Version,
		"$SYS/broker/uptime":                    atomicItoa(&s.System.Uptime),
		"$SYS/broker/timestamp":                 atomicItoa(&s.System.Started),
		"$SYS/broker/load/bytes/received":       atomicItoa(&s.System.BytesRecv),
		"$SYS/broker/load/bytes/sent":           atomicItoa(&s.System.BytesSent),
		"$SYS/broker/clients/connected":         atomicItoa(&s.System.ClientsConnected),
		"$SYS/broker/clients/disconnected":      atomicItoa(&s.System.ClientsDisconnected),
		"$SYS/broker/clients/maximum":           atomicItoa(&s.System.ClientsMax),
		"$SYS/broker/clients/total":             atomicItoa(&s.System.ClientsTotal),
		"$SYS/broker/connections/total":         atomicItoa(&s.System.ConnectionsTotal),
		"$SYS/broker/messages/received":         atomicItoa(&s.System.MessagesRecv),
		"$SYS/broker/messages/sent":             atomicItoa(&s.System.MessagesSent),
		"$SYS/broker/messages/publish/dropped":  atomicItoa(&s.System.PublishDropped),
		"$SYS/broker/messages/publish/received": atomicItoa(&s.System.PublishRecv),
		"$SYS/broker/messages/publish/sent":     atomicItoa(&s.System.PublishSent),
		"$SYS/broker/messages/retained/count":   atomicItoa(&s.System.Retained),
		"$SYS/broker/messages/inflight":         atomicItoa(&s.System.Inflight),
		"$SYS/broker/subscriptions/count":       atomicItoa(&s.System.Subscriptions),
	}

	for topic, payload := range topics {
		pk.TopicName = topic
		pk.Payload = []byte(payload)
		q := s.Topics.RetainMessage(pk.PublishCopy())
		atomic.AddInt64(&s.System.Retained, q)
		s.publishToSubscribers(pk)
	}

	if s.Store != nil {
		s.onStorage(&s.inline, s.Store.WriteServerInfo(persistence.ServerInfo{
			Info: *s.System,
			ID:   persistence.KServerInfo,
		}))
	}
}

// ResendClientInflight attempts to resend all undelivered inflight messages
// to a client.
func (s *Server) ResendClientInflight(cl *clients.Client, force bool) error {
	if atomic.LoadUint32(&cl.State.Done) == 1 || cl.Inflight.Len() == 0 {
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
				s.onStorage(cl, s.Store.DeleteInflight(persistentID(cl, tk.Packet)))
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
			s.onStorage(cl, s.Store.WriteInflight(persistence.Message{
				ID:          persistentID(cl, tk.Packet),
				T:           persistence.KInflight,
				FixedHeader: persistence.FixedHeader(tk.Packet.FixedHeader),
				TopicName:   tk.Packet.TopicName,
				Payload:     tk.Packet.Payload,
				Sent:        tk.Sent,
				Resends:     tk.Resends,
			}))
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
		cl.Stop(ErrServerShutdown)
	}
}

// sendLWT issues an LWT message to a topic when a client disconnects.
func (s *Server) sendLWT(cl *clients.Client) error {
	if cl.LWT.Topic != "" {
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
			return s.onError(cl.Info(), fmt.Errorf("send lwt: %s %w; %+v", cl.ID, err, cl.LWT))
		}
	}

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
		if s.Topics.Subscribe(sub.Filter, sub.Client, sub.QoS) {
			if cl, ok := s.Clients.Get(sub.Client); ok {
				cl.NoteSubscription(sub.Filter, sub.QoS)
				if s.Events.OnSubscribe != nil {
					s.Events.OnSubscribe(sub.Filter, cl.Info(), sub.QoS)
				}
			}
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
				Created: msg.Created,
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

// clearExpiredInflights deletes all inflight messages older than server inflight TTL.
func (s *Server) clearExpiredInflights(dt int64) {
	expiry := dt - s.Options.InflightTTL

	for _, client := range s.Clients.GetAll() {
		deleted := client.Inflight.ClearExpired(expiry)
		atomic.AddInt64(&s.System.Inflight, deleted*-1)
	}

	if s.Store != nil {
		s.Store.ClearExpiredInflight(expiry)
	}
}

// clearAbandonedInflights deletes all inflight messages for a disconnected user (eg. with a clean session).
func (s *Server) clearAbandonedInflights(cl *clients.Client) {
	for i := range cl.Inflight.GetAll() {
		cl.Inflight.Delete(i)
		atomic.AddInt64(&s.System.Inflight, -1)
	}
}

// resendPendingInflights attempts resends of any pending and due inflight messages.
func (s *Server) resendPendingInflights() {
	for _, client := range s.Clients.GetAll() {
		err := s.ResendClientInflight(client, false)
		if err != nil {
			// TODO add log-level debugging.
			continue
		}
	}
}
