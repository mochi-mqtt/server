// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 J. Blake / mochi-co
// SPDX-FileContributor: mochi-co

package mqtt

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/xid"

	"github.com/mochi-co/mqtt/v2/packets"
)

const (
	defaultKeepalive             uint16 = 10 // the default connection keepalive value in seconds
	defaultClientProtocolVersion byte   = 4  // the default mqtt protocol version of connecting clients (if somehow unspecified).
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
func (cl *Clients) GetByListener(id string) []*Client {
	cl.RLock()
	defer cl.RUnlock()
	clients := make([]*Client, 0, cl.Len())
	for _, client := range cl.internal {
		if client.Net.Listener == id && atomic.LoadUint32(&client.State.done) == 0 {
			clients = append(clients, client)
		}
	}
	return clients
}

// Client contains information about a client known by the broker.
type Client struct {
	Properties   ClientProperties // client properties
	State        ClientState      // the operational state of the client.
	Net          ClientConnection // network connection state of the clinet
	ID           string           // the client id.
	ops          *ops             // ops provides a reference to server ops.
	sync.RWMutex                  // mutex
}

// ClientConnection contains the connection transport and metadata for the client.
type ClientConnection struct {
	Conn     net.Conn          // the net.Conn used to establish the connection
	bconn    *bufio.ReadWriter // a buffered net.Conn for reading packets
	Remote   string            // the remote address of the client
	Listener string            // listener id of the client
	Inline   bool              // client is an inline programmetic client
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

// State tracks the state of the client.
type ClientState struct {
	TopicAliases  TopicAliases         // a map of topic aliases
	stopCause     atomic.Value         // reason for stopping
	Inflight      *Inflight            // a map of in-flight qos messages
	Subscriptions *Subscriptions       // a map of the subscription filters a client maintains
	disconnected  int64                // the time the client disconnected in unix time, for calculating expiry
	outbound      chan *packets.Packet // queue for pending outbound packets
	endOnce       sync.Once            // only end once
	isTakenOver   uint32               // used to identify orphaned clients
	packetID      uint32               // the current highest packetID
	done          uint32               // atomic counter which indicates that the client has closed
	outboundQty   int32                // number of messages currently in the outbound queue
	keepalive     uint16               // the number of seconds the connection can wait
}

// newClient returns a new instance of Client. This is almost exclusively used by Server
// for creating new clients, but it lives here because it's not dependent.
func newClient(c net.Conn, o *ops) *Client {
	cl := &Client{
		State: ClientState{
			Inflight:      NewInflights(),
			Subscriptions: NewSubscriptions(),
			TopicAliases:  NewTopicAliases(o.options.Capabilities.TopicAliasMaximum),
			keepalive:     defaultKeepalive,
			outbound:      make(chan *packets.Packet, o.options.Capabilities.MaximumClientWritesPending),
		},
		Properties: ClientProperties{
			ProtocolVersion: defaultClientProtocolVersion, // default protocol version
		},
		ops: o,
	}

	if c != nil {
		cl.Net = ClientConnection{
			Conn: c,
			bconn: bufio.NewReadWriter(
				bufio.NewReaderSize(c, o.options.ClientNetReadBufferSize),
				bufio.NewWriterSize(c, o.options.ClientNetReadBufferSize),
			),
			Remote: c.RemoteAddr().String(),
		}
	}

	cl.refreshDeadline(cl.State.keepalive)

	return cl
}

// WriteLoop ranges over pending outbound messages and writes them to the client connection.
func (cl *Client) WriteLoop() {
	for pk := range cl.State.outbound {
		if err := cl.WritePacket(*pk); err != nil {
			cl.ops.log.Debug().Err(err).Str("client", cl.ID).Interface("packet", pk).Msg("failed publishing packet")
		}
		atomic.AddInt32(&cl.State.outboundQty, -1)
	}
}

// ParseConnect parses the connect parameters and properties for a client.
func (cl *Client) ParseConnect(lid string, pk packets.Packet) {
	cl.Net.Listener = lid

	cl.Properties.ProtocolVersion = pk.ProtocolVersion
	cl.Properties.Username = pk.Connect.Username
	cl.Properties.Clean = pk.Connect.Clean
	cl.Properties.Props = pk.Properties.Copy(false)

	cl.State.Inflight.ResetReceiveQuota(int32(cl.ops.options.Capabilities.ReceiveMaximum)) // server receive max per client
	cl.State.Inflight.ResetSendQuota(int32(cl.Properties.Props.ReceiveMaximum))            // client receive max

	cl.State.TopicAliases.Outbound = NewOutboundTopicAliases(cl.Properties.Props.TopicAliasMaximum)

	cl.ID = pk.Connect.ClientIdentifier
	if cl.ID == "" {
		cl.ID = xid.New().String() // [MQTT-3.1.3-6] [MQTT-3.1.3-7]
		cl.Properties.Props.AssignedClientID = cl.ID
	}

	cl.State.keepalive = cl.ops.options.Capabilities.ServerKeepAlive
	if pk.Connect.Keepalive > 0 {
		cl.State.keepalive = pk.Connect.Keepalive // [MQTT-3.2.2-22]
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

	cl.refreshDeadline(cl.State.keepalive)
}

// refreshDeadline refreshes the read/write deadline for the net.Conn connection.
func (cl *Client) refreshDeadline(keepalive uint16) {
	var expiry time.Time // nil time can be used to disable deadline if keepalive = 0
	if keepalive > 0 {
		expiry = time.Now().Add(time.Duration(keepalive+(keepalive/2)) * time.Second) // [MQTT-3.1.2-22]
	}

	if cl.Net.Conn != nil {
		_ = cl.Net.Conn.SetDeadline(expiry) // [MQTT-3.1.2-22]
	}
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

// ClearInflights deletes all inflight messages for the client, eg. for a disconnected user with a clean session.
func (cl *Client) ClearInflights(now, maximumExpiry int64) []uint16 {
	deleted := []uint16{}
	for _, tk := range cl.State.Inflight.GetAll(false) {
		if (tk.Expiry > 0 && tk.Expiry < now) || tk.Created+maximumExpiry < now {
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
		if atomic.LoadUint32(&cl.State.done) == 1 {
			return nil
		}

		cl.refreshDeadline(cl.State.keepalive)
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

		if cl.State.outbound != nil {
			close(cl.State.outbound)
		}

		atomic.StoreUint32(&cl.State.done, 1)
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

// Closed returns true if client connection is closed.
func (cl *Client) Closed() bool {
	return atomic.LoadUint32(&cl.State.done) == 1
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
	if atomic.LoadUint32(&cl.State.done) == 1 {
		return ErrConnectionClosed
	}

	if cl.Net.Conn == nil {
		return nil
	}

	if pk.Expiry > 0 {
		pk.Properties.MessageExpiryInterval = uint32(pk.Expiry - time.Now().Unix()) // [MQTT-3.3.2-6]
	}

	pk.ProtocolVersion = cl.Properties.ProtocolVersion
	if pk.Mods.MaxSize == 0 { // NB we use this statement to embed client packet sizes in tests
		pk.Mods.MaxSize = cl.Properties.Props.MaximumPacketSize
	}

	if cl.Properties.Props.RequestProblemInfoFlag && cl.Properties.Props.RequestProblemInfo == 0x0 {
		pk.Mods.DisallowProblemInfo = true // [MQTT-3.1.2-29] strict, no problem info on any packet if set
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

	nb := net.Buffers{buf.Bytes()}
	n, err := nb.WriteTo(cl.Net.Conn)
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
