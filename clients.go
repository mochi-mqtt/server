// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package mqtt

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/xid"

	"github.com/mochi-mqtt/server/v2/packets"
)

const (
	defaultKeepalive             uint16 = 10 // the default connection keepalive value in seconds.
	defaultClientProtocolVersion byte   = 4  // the default mqtt protocol version of connecting clients (if somehow unspecified).
	minimumKeepalive             uint16 = 5  // the minimum recommended keepalive - values under with display a warning.
)

var (
	ErrMinimumKeepalive = errors.New("client keepalive is below minimum recommended value and may exhibit connection instability")
)

// ReadFn is the function signature for the function used for reading and processing new packets.
type ReadFn func(*Client, packets.Packet) error

// Clients contains a map of the clients known by the broker.
type Clients struct {
	internal map[string]*Client // clients known by the broker, keyed on client id.
	sync.RWMutex
}

// NewClients returns an instance of Clients.
func NewClients() *Clients {
	return &Clients{
		internal: make(map[string]*Client),
	}
}

// Add adds a new client to the clients map, keyed on client id.
func (cl *Clients) Add(val *Client) {
	cl.Lock()
	defer cl.Unlock()
	cl.internal[val.ID] = val
}

// GetAll returns all the clients.
func (cl *Clients) GetAll() map[string]*Client {
	cl.RLock()
	defer cl.RUnlock()
	m := map[string]*Client{}
	for k, v := range cl.internal {
		m[k] = v
	}
	return m
}

// Get returns the value of a client if it exists.
func (cl *Clients) Get(id string) (*Client, bool) {
	cl.RLock()
	defer cl.RUnlock()
	val, ok := cl.internal[id]
	return val, ok
}

// Len returns the length of the clients map.
func (cl *Clients) Len() int {
	cl.RLock()
	defer cl.RUnlock()
	val := len(cl.internal)
	return val
}

// Delete removes a client from the internal map.
func (cl *Clients) Delete(id string) {
	cl.Lock()
	defer cl.Unlock()
	delete(cl.internal, id)
}

// GetByListener returns clients matching a listener id.
//
// The capacity hint used to read cl.Len(), which takes
// RLock a second time while this function already holds it. sync.RWMutex
// documents that as prohibited - "if a goroutine holds a RWMutex for reading
// and another goroutine might call Lock, no goroutine should expect to be
// able to acquire a read lock until the initial read lock is released. In
// particular, this prohibits recursive read locking."
//
// A client connecting while a listener closes is exactly that interleaving:
// attachClient -> Clients.Delete -> Lock() queues a writer, RWMutex stops
// admitting new readers, and this nested RLock blocks forever behind the
// writer that is itself waiting on the RLock we still hold.
//
// The map is already guarded by the RLock held here, so read len directly.
func (cl *Clients) GetByListener(id string) []*Client {
	cl.RLock()
	defer cl.RUnlock()
	clients := make([]*Client, 0, len(cl.internal))
	for _, client := range cl.internal {
		if client.Net.Listener == id && !client.Closed() {
			clients = append(clients, client)
		}
	}
	return clients
}

// Client contains information about a client known by the broker.
type Client struct {
	Properties   ClientProperties // client properties
	State        ClientState      // the operational state of the client.
	Net          ClientConnection // network connection state of the client
	ID           string           // the client id.
	ops          *ops             // ops provides a reference to server ops.
	sync.RWMutex                  // mutex
}

// ClientConnection contains the connection transport and metadata for the client.
type ClientConnection struct {
	Conn     net.Conn      // the net.Conn used to establish the connection
	bconn    *bufio.Reader // a buffered net.Conn for reading packets
	outbuf   *bytes.Buffer // a buffer for writing packets
	Remote   string        // the remote address of the client
	Listener string        // listener id of the client
	Inline   bool          // if true, the client is the built-in 'inline' embedded client
}

// ClientProperties contains the properties which define the client behaviour.
type ClientProperties struct {
	Props           packets.Properties
	Will            Will
	Username        []byte
	ProtocolVersion byte
	Clean           bool
}

// Will contains the last will and testament details for a client connection.
type Will struct {
	Payload           []byte                 // -
	User              []packets.UserProperty // -
	TopicName         string                 // -
	Flag              uint32                 // 0,1
	WillDelayInterval uint32                 // -
	Qos               byte                   // -
	Retain            bool                   // -
}

// ClientState tracks the state of the client.
type ClientState struct {
	TopicAliases    TopicAliases         // a map of topic aliases
	stopCause       atomic.Value         // reason for stopping
	Inflight        *Inflight            // a map of in-flight qos messages
	Subscriptions   *Subscriptions       // a map of the subscription filters a client maintains
	disconnected    int64                // the time the client disconnected in unix time, for calculating expiry
	outbound        chan *packets.Packet // queue for pending outbound packets
	endOnce         sync.Once            // only end once
	isTakenOver     atomic.Bool          // used to identify orphaned clients
	packetID        uint32               // the current highest packetID
	open            context.Context      // indicate that the client is open for packet exchange
	cancelOpen      context.CancelFunc   // cancel function for open context
	outboundQty     int32                // number of messages currently in the outbound queue
	Keepalive       uint16               // the number of seconds the connection can wait
	ServerKeepalive bool                 // keepalive was set by the server
}

// newClient returns a new instance of Client. This is almost exclusively used by Server
// for creating new clients, but it lives here because it's not dependent.
func newClient(c net.Conn, o *ops) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	cl := &Client{
		State: ClientState{
			Inflight:      NewInflights(),
			Subscriptions: NewSubscriptions(),
			TopicAliases:  NewTopicAliases(o.options.Capabilities.TopicAliasMaximum),
			open:          ctx,
			cancelOpen:    cancel,
			Keepalive:     defaultKeepalive,
			outbound:      make(chan *packets.Packet, o.options.Capabilities.MaximumClientWritesPending),
		},
		Properties: ClientProperties{
			ProtocolVersion: defaultClientProtocolVersion, // default protocol version
		},
		ops: o,
	}

	if c != nil {
		cl.Net = ClientConnection{
			Conn:   c,
			bconn:  bufio.NewReaderSize(c, o.options.ClientNetReadBufferSize),
			Remote: c.RemoteAddr().String(),
		}
	}

	return cl
}

// WriteLoop ranges over pending outbound messages and writes them to the client connection.
func (cl *Client) WriteLoop() {
	for {
		select {
		case pk := <-cl.State.outbound:
			if err := cl.WritePacket(*pk); err != nil {
				// TODO : Figure out what to do with error
				cl.ops.log.Debug("failed publishing packet", "error", err, "client", cl.ID, "packet", pk)

				// A write that ran out of time under
				// ClientNetWriteTimeout has put part of a packet on the
				// wire, and there is no way to continue that stream: the
				// next packet written would land in the middle of the last
				// one. Continuing also leaves this loop taking the client
				// lock for every remaining queued packet, a timeout at a
				// time, while every other goroutine that needs that lock —
				// publishToClient taking a packet identifier — waits.
				if isTimeout(err) {
					atomic.AddInt32(&cl.State.outboundQty, -1)
					cl.Stop(err)
					return
				}
			}
			atomic.AddInt32(&cl.State.outboundQty, -1)
		case <-cl.State.open.Done():
			return
		}
	}
}

// ParseConnect parses the connect parameters and properties for a client.
func (cl *Client) ParseConnect(lid string, pk packets.Packet) {
	cl.Net.Listener = lid

	cl.Properties.ProtocolVersion = pk.ProtocolVersion
	cl.Properties.Username = pk.Connect.Username
	cl.Properties.Clean = pk.Connect.Clean
	cl.Properties.Props = pk.Properties.Copy(false)

	if cl.Properties.Props.ReceiveMaximum > cl.ops.options.Capabilities.MaximumInflight { // 3.3.4 Non-normative
		cl.Properties.Props.ReceiveMaximum = cl.ops.options.Capabilities.MaximumInflight
	}

	if pk.Connect.Keepalive <= minimumKeepalive {
		cl.ops.log.Warn(
			ErrMinimumKeepalive.Error(),
			"client", cl.ID,
			"keepalive", pk.Connect.Keepalive,
			"recommended", minimumKeepalive,
		)
	}

	cl.State.Keepalive = pk.Connect.Keepalive                                              // [MQTT-3.2.2-22]
	cl.State.Inflight.ResetReceiveQuota(int32(cl.ops.options.Capabilities.ReceiveMaximum)) // server receive max per client
	cl.State.Inflight.ResetSendQuota(int32(cl.Properties.Props.ReceiveMaximum))            // client receive max
	cl.State.TopicAliases.Outbound = NewOutboundTopicAliases(cl.Properties.Props.TopicAliasMaximum)

	cl.ID = pk.Connect.ClientIdentifier
	if cl.ID == "" {
		cl.ID = xid.New().String() // [MQTT-3.1.3-6] [MQTT-3.1.3-7]
		cl.Properties.Props.AssignedClientID = cl.ID
	}

	if pk.Connect.WillFlag {
		cl.Properties.Will = Will{
			Qos:               pk.Connect.WillQos,
			Retain:            pk.Connect.WillRetain,
			Payload:           pk.Connect.WillPayload,
			TopicName:         pk.Connect.WillTopic,
			WillDelayInterval: pk.Connect.WillProperties.WillDelayInterval,
			User:              pk.Connect.WillProperties.User,
		}
		if pk.Properties.SessionExpiryIntervalFlag &&
			pk.Properties.SessionExpiryInterval < pk.Connect.WillProperties.WillDelayInterval {
			cl.Properties.Will.WillDelayInterval = pk.Properties.SessionExpiryInterval
		}
		if pk.Connect.WillFlag {
			cl.Properties.Will.Flag = 1 // atomic for checking
		}
	}
}

// refreshDeadline refreshes the read deadline for the net.Conn connection.
//
// The read deadline and not both: [MQTT-3.1.2-22] is a rule about packets
// the client sends, and SetDeadline would apply it to writes as well. That
// was harmless while nothing else set a write deadline, and became a way
// for a client to switch one off the moment ClientNetWriteTimeout existed:
// this runs at the top of every pass of the read loop, so any packet
// needing no reply — a QoS 0 PUBLISH, a PUBACK, an UNSUBSCRIBE — cleared
// the deadline a queued write had armed. With keepalive 0 it cleared it
// outright; with keepalive 60 it pushed it 90 seconds out, and the next
// packet pushed it again.
func (cl *Client) refreshDeadline(keepalive uint16) {
	var expiry time.Time // nil time can be used to disable deadline if keepalive = 0
	if keepalive > 0 {
		expiry = time.Now().Add(time.Duration(keepalive+(keepalive/2)) * time.Second) // [MQTT-3.1.2-22]
	}

	if cl.Net.Conn != nil {
		_ = cl.Net.Conn.SetReadDeadline(expiry) // [MQTT-3.1.2-22]
	}
}

// writeDeadline is when a single write to this client must have completed.
//
// ClientNetWriteTimeout when it is set, and otherwise the same keepalive
// expiry refreshDeadline uses for reads - because that is the bound every
// release before this one gave a write, by arming both deadlines at once.
// Zero means no bound, which happens only when neither is configured.
func (cl *Client) writeDeadline() time.Time {
	if d := cl.ops.options.ClientNetWriteTimeout; d > 0 {
		return time.Now().Add(d)
	}
	if k := cl.State.Keepalive; k > 0 {
		return time.Now().Add(time.Duration(k+(k/2)) * time.Second) // [MQTT-3.1.2-22]
	}
	return time.Time{}
}

// NextPacketID returns the next available (unused) packet id for the client.
// If no unused packet ids are available, an error is returned and the client
// should be disconnected.
func (cl *Client) NextPacketID() (i uint32, err error) {
	cl.Lock()
	defer cl.Unlock()

	i = atomic.LoadUint32(&cl.State.packetID)
	started := i
	overflowed := false
	for {
		if overflowed && i == started {
			return 0, packets.ErrQuotaExceeded
		}

		if i >= cl.ops.options.Capabilities.maximumPacketID {
			overflowed = true
			i = 0
			continue
		}

		i++

		if _, ok := cl.State.Inflight.Get(uint16(i)); !ok {
			atomic.StoreUint32(&cl.State.packetID, i)
			return i, nil
		}
	}
}

// ResendInflightMessages attempts to resend any pending inflight messages to connected clients.
func (cl *Client) ResendInflightMessages(force bool) error {
	if cl.State.Inflight.Len() == 0 {
		return nil
	}

	for _, tk := range cl.State.Inflight.GetAll(false) {
		if tk.FixedHeader.Type == packets.Publish {
			tk.FixedHeader.Dup = true // [MQTT-3.3.1-1] [MQTT-3.3.1-3]
		}

		cl.ops.hooks.OnQosPublish(cl, tk, tk.Created, 0)
		err := cl.WritePacket(tk)
		if err != nil {
			return err
		}

		if tk.FixedHeader.Type == packets.Puback || tk.FixedHeader.Type == packets.Pubcomp {
			if ok := cl.State.Inflight.Delete(tk.PacketID); ok {
				cl.ops.hooks.OnQosComplete(cl, tk)
				atomic.AddInt64(&cl.ops.info.Inflight, -1)
			}
		}
	}

	return nil
}

// ClearInflights deletes all inflight messages for the client, e.g. for a disconnected user with a clean session.
func (cl *Client) ClearInflights() {
	for _, tk := range cl.State.Inflight.GetAll(false) {
		if ok := cl.State.Inflight.Delete(tk.PacketID); ok {
			cl.ops.hooks.OnQosDropped(cl, tk)
			atomic.AddInt64(&cl.ops.info.Inflight, -1)
		}
	}
}

// ClearExpiredInflights deletes any inflight messages which have expired.
func (cl *Client) ClearExpiredInflights(now, maximumExpiry int64) []uint16 {
	deleted := []uint16{}
	for _, tk := range cl.State.Inflight.GetAll(false) {
		expired := tk.ProtocolVersion == 5 && tk.Expiry > 0 && tk.Expiry < now // [MQTT-3.3.2-5]

		// If the maximum message expiry interval is set (greater than 0), and the message
		// retention period exceeds the maximum expiry, the message will be forcibly removed.
		enforced := maximumExpiry > 0 && now-tk.Created > maximumExpiry

		if expired || enforced {
			if ok := cl.State.Inflight.Delete(tk.PacketID); ok {
				cl.ops.hooks.OnQosDropped(cl, tk)
				atomic.AddInt64(&cl.ops.info.Inflight, -1)
				deleted = append(deleted, tk.PacketID)
			}
		}
	}

	return deleted
}

// Read reads incoming packets from the connected client and transforms them into
// packets to be handled by the packetHandler.
func (cl *Client) Read(packetHandler ReadFn) error {
	var err error

	for {
		if cl.Closed() {
			return nil
		}

		cl.refreshDeadline(cl.State.Keepalive)
		fh := new(packets.FixedHeader)
		err = cl.ReadFixedHeader(fh)
		if err != nil {
			return err
		}

		pk, err := cl.ReadPacket(fh)
		if err != nil {
			return err
		}

		err = packetHandler(cl, pk) // Process inbound packet.
		if err != nil {
			return err
		}
	}
}

// Stop instructs the client to shut down all processing goroutines and disconnect.
func (cl *Client) Stop(err error) {
	cl.State.endOnce.Do(func() {

		if cl.Net.Conn != nil {
			_ = cl.Net.Conn.Close() // omit close error
		}

		if err != nil {
			cl.State.stopCause.Store(err)
		}

		if cl.State.cancelOpen != nil {
			cl.State.cancelOpen()
		}

		atomic.StoreInt64(&cl.State.disconnected, time.Now().Unix())
	})
}

// StopCause returns the reason the client connection was stopped, if any.
func (cl *Client) StopCause() error {
	if cl.State.stopCause.Load() == nil {
		return nil
	}
	return cl.State.stopCause.Load().(error)
}

// StopTime returns the the time the client disconnected in unix time, else zero.
func (cl *Client) StopTime() int64 {
	return atomic.LoadInt64(&cl.State.disconnected)
}

// Closed returns true if client connection is closed.
func (cl *Client) Closed() bool {
	return cl.State.open == nil || cl.State.open.Err() != nil
}

func (cl *Client) IsTakenOver() bool {
	return cl.State.isTakenOver.Load()
}

// ReadFixedHeader reads in the values of the next packet's fixed header.
func (cl *Client) ReadFixedHeader(fh *packets.FixedHeader) error {
	if cl.Net.bconn == nil {
		return ErrConnectionClosed
	}

	b, err := cl.Net.bconn.ReadByte()
	if err != nil {
		return err
	}

	err = fh.Decode(b)
	if err != nil {
		return err
	}

	var bu int
	fh.Remaining, bu, err = packets.DecodeLength(cl.Net.bconn)
	if err != nil {
		return err
	}

	if cl.ops.options.Capabilities.MaximumPacketSize > 0 && uint32(fh.Remaining+1) > cl.ops.options.Capabilities.MaximumPacketSize {
		return packets.ErrPacketTooLarge // [MQTT-3.2.2-15]
	}

	atomic.AddInt64(&cl.ops.info.BytesReceived, int64(bu+1))
	return nil
}

// ReadPacket reads the remaining buffer into an MQTT packet.
func (cl *Client) ReadPacket(fh *packets.FixedHeader) (pk packets.Packet, err error) {
	atomic.AddInt64(&cl.ops.info.PacketsReceived, 1)

	pk.ProtocolVersion = cl.Properties.ProtocolVersion // inherit client protocol version for decoding
	pk.FixedHeader = *fh
	p := make([]byte, pk.FixedHeader.Remaining)
	n, err := io.ReadFull(cl.Net.bconn, p)
	if err != nil {
		return pk, err
	}

	atomic.AddInt64(&cl.ops.info.BytesReceived, int64(n))

	// Decode the remaining packet values using a fresh copy of the bytes,
	// otherwise the next packet will change the data of this one.
	px := append([]byte{}, p[:]...)
	switch pk.FixedHeader.Type {
	case packets.Connect:
		err = pk.ConnectDecode(px)
	case packets.Disconnect:
		err = pk.DisconnectDecode(px)
	case packets.Connack:
		err = pk.ConnackDecode(px)
	case packets.Publish:
		err = pk.PublishDecode(px)
		if err == nil {
			atomic.AddInt64(&cl.ops.info.MessagesReceived, 1)
		}
	case packets.Puback:
		err = pk.PubackDecode(px)
	case packets.Pubrec:
		err = pk.PubrecDecode(px)
	case packets.Pubrel:
		err = pk.PubrelDecode(px)
	case packets.Pubcomp:
		err = pk.PubcompDecode(px)
	case packets.Subscribe:
		err = pk.SubscribeDecode(px)
	case packets.Suback:
		err = pk.SubackDecode(px)
	case packets.Unsubscribe:
		err = pk.UnsubscribeDecode(px)
	case packets.Unsuback:
		err = pk.UnsubackDecode(px)
	case packets.Pingreq:
	case packets.Pingresp:
	case packets.Auth:
		err = pk.AuthDecode(px)
	default:
		err = fmt.Errorf("invalid packet type; %v", pk.FixedHeader.Type)
	}

	if err != nil {
		return pk, err
	}

	pk, err = cl.ops.hooks.OnPacketRead(cl, pk)
	return
}

// WritePacket encodes and writes a packet to the client.
func (cl *Client) WritePacket(pk packets.Packet) error {
	if cl.Closed() {
		return ErrConnectionClosed
	}

	if cl.Net.Conn == nil {
		return nil
	}

	if pk.Expiry > 0 {
		expiry := pk.Expiry - time.Now().Unix()
		if expiry < 1 {
			expiry = 1
		}
		pk.Properties.MessageExpiryInterval = uint32(expiry) // [MQTT-3.3.2-6]
	}

	pk.ProtocolVersion = cl.Properties.ProtocolVersion
	if pk.Mods.MaxSize == 0 { // NB we use this statement to embed client packet sizes in tests
		pk.Mods.MaxSize = cl.Properties.Props.MaximumPacketSize
	}

	if cl.Properties.Props.RequestProblemInfoFlag && cl.Properties.Props.RequestProblemInfo == 0x0 &&
		pk.FixedHeader.Type != packets.Publish {
		// [MQTT-3.1.2-29] the server must not send a Reason String or User
		// Properties on any packet other than PUBLISH, CONNACK or
		// DISCONNECT. PUBLISH is exempt: its User Properties are
		// application data forwarded from the publisher, not problem
		// information about a failure.
		pk.Mods.DisallowProblemInfo = true
	}

	if pk.FixedHeader.Type != packets.Connack || cl.Properties.Props.RequestResponseInfo == 0x1 || cl.ops.options.Capabilities.Compatibilities.AlwaysReturnResponseInfo {
		pk.Mods.AllowResponseInfo = true // [MQTT-3.1.2-28] we need to know which properties we can encode
	}

	pk = cl.ops.hooks.OnPacketEncode(cl, pk)

	var err error
	buf := new(bytes.Buffer)
	switch pk.FixedHeader.Type {
	case packets.Connect:
		err = pk.ConnectEncode(buf)
	case packets.Connack:
		err = pk.ConnackEncode(buf)
	case packets.Publish:
		err = pk.PublishEncode(buf)
	case packets.Puback:
		err = pk.PubackEncode(buf)
	case packets.Pubrec:
		err = pk.PubrecEncode(buf)
	case packets.Pubrel:
		err = pk.PubrelEncode(buf)
	case packets.Pubcomp:
		err = pk.PubcompEncode(buf)
	case packets.Subscribe:
		err = pk.SubscribeEncode(buf)
	case packets.Suback:
		err = pk.SubackEncode(buf)
	case packets.Unsubscribe:
		err = pk.UnsubscribeEncode(buf)
	case packets.Unsuback:
		err = pk.UnsubackEncode(buf)
	case packets.Pingreq:
		err = pk.PingreqEncode(buf)
	case packets.Pingresp:
		err = pk.PingrespEncode(buf)
	case packets.Disconnect:
		err = pk.DisconnectEncode(buf)
	case packets.Auth:
		err = pk.AuthEncode(buf)
	default:
		err = fmt.Errorf("%w: %v", packets.ErrNoValidPacketAvailable, pk.FixedHeader.Type)
	}
	if err != nil {
		return err
	}

	if pk.Mods.MaxSize > 0 && uint32(buf.Len()) > pk.Mods.MaxSize {
		return packets.ErrPacketTooLarge // [MQTT-3.1.2-24] [MQTT-3.1.2-25]
	}

	n, err := func() (int64, error) {
		cl.Lock()
		defer cl.Unlock()

		// Armed inside the lock and cleared before it is released, so that
		// the deadline belongs to this write and to no other. A connection
		// deadline is a property of the connection rather than of a write,
		// so arming it outside would let one goroutine clear the deadline
		// another is relying on.
		//
		// The lock is the reason this matters at all: every write below
		// happens while it is held, so a write that never completes holds
		// the client's lock for as long as the client stays connected, and
		// publishToClient blocks on that same lock before it can reach the
		// outbound queue that would have shed the client.
		// The connection is read once and held, so that the deadline is
		// cleared on the connection it was armed on rather than on
		// whatever cl.Net.Conn names by the time the write returns.
		// **Unset does not mean unbounded**, and this is the half that is
		// easy to get wrong. refreshDeadline used to arm SetDeadline, which
		// set the read AND write deadlines, so every release before this one
		// bounded a write to a stalled client at the keepalive expiry.
		// Arming only the read deadline there - which is what stops the read
		// loop clearing a write deadline a queued write is relying on - takes
		// that bound away, and a broker upgrading with this option unset
		// would be worse off than before: a write could block forever holding
		// the lock, which is the deadlock this option exists to prevent.
		//
		// So the option is an override, and its absence falls back to what
		// the keepalive already promised. A client with no keepalive gets no
		// bound, which is also what it got before.
		if d := cl.writeDeadline(); !d.IsZero() {
			if conn := cl.Net.Conn; conn != nil {
				if err := conn.SetWriteDeadline(d); err == nil {
					defer conn.SetWriteDeadline(time.Time{})
				}
			}
		}

		if len(cl.State.outbound) == 0 {
			if cl.Net.outbuf == nil {
				return buf.WriteTo(cl.Net.Conn)
			}

			// first write to buffer, then flush buffer
			n, _ := cl.Net.outbuf.Write(buf.Bytes()) // will always be successful
			err = cl.flushOutbuf()
			return int64(n), err
		}

		// there are more writes in the queue
		if cl.Net.outbuf == nil {
			if buf.Len() >= cl.ops.options.ClientNetWriteBufferSize {
				return buf.WriteTo(cl.Net.Conn)
			}
			cl.Net.outbuf = new(bytes.Buffer)
		}

		n, _ := cl.Net.outbuf.Write(buf.Bytes()) // will always be successful
		if cl.Net.outbuf.Len() < cl.ops.options.ClientNetWriteBufferSize {
			return int64(n), nil
		}

		err = cl.flushOutbuf()
		return int64(n), err
	}()
	if err != nil {
		return err
	}

	atomic.AddInt64(&cl.ops.info.BytesSent, n)
	atomic.AddInt64(&cl.ops.info.PacketsSent, 1)
	if pk.FixedHeader.Type == packets.Publish {
		atomic.AddInt64(&cl.ops.info.MessagesSent, 1)
	}

	cl.ops.hooks.OnPacketSent(cl, pk, buf.Bytes())

	return err
}

func (cl *Client) flushOutbuf() (err error) {
	if cl.Net.outbuf == nil {
		return
	}

	_, err = cl.Net.outbuf.WriteTo(cl.Net.Conn)
	if err == nil {
		cl.Net.outbuf = nil
	}
	return
}

// isTimeout reports whether an error is a network timeout, which is what a
// write that ran out of time under ClientNetWriteTimeout produces.
//
// net.Error's Timeout rather than errors.Is against os.ErrDeadlineExceeded:
// the two agree for a plain TCP write, and do not for every listener. A
// websocket connection writes through gorilla, whose buffered writev path
// yields "writev tcp …: i/o timeout" — a timeout by every definition that
// matters, and not that sentinel. Checking the sentinel silently covered
// one transport and not another.
func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}
