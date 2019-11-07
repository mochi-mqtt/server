package mqtt

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/xid"

	"github.com/mochi-co/mqtt/auth"
	"github.com/mochi-co/mqtt/circ"
	"github.com/mochi-co/mqtt/debug"
	"github.com/mochi-co/mqtt/packets"
)

var (
	// defaultClientKeepalive is the default keepalive time in seconds.
	defaultClientKeepalive uint16 = 60

	// ErrInsufficientBytes indicates that there were not enough bytes
	// in the buffer to read/peek.
	ErrInsufficientBytes = fmt.Errorf("Insufficient bytes in buffer")
)

// Client contains information about a client known by the broker.
type client struct {
	sync.RWMutex

	// ac is a pointer to an auth controller inherited from the listener.
	ac auth.Controller

	// inFlight is a map of messages which are in-flight,awaiting completiong of
	// a QoS pattern.
	inFlight inFlight

	// subscriptions is a map of the subscription filters a client maintains.
	subscriptions map[string]byte

	// p is a packets processor which reads and writes packets.
	//p *Processor

	// state is the state of the client
	state clientState

	// conn is the net.Conn used to establish the connection.
	conn net.Conn

	// R is a reader for reading incoming bytes.
	r *circ.Reader

	// W is a writer for writing outgoing bytes.
	w *circ.Writer

	// fh is the FixedHeader from the last read packet.
	fh packets.FixedHeader

	// id is the client id.
	id string

	// listener is the id of the listener the client is connected to.
	listener string

	// user is the username the client authenticated with.
	user string

	// keepalive is the number of seconds the connection can stay open without
	// receiving a message from the client.
	keepalive uint16

	// cleanSession indicates if the client expects a cleansession.
	cleanSession bool

	// packetID is the current highest packetID.
	packetID uint32

	// lwt contains the last will and testament for the client.
	lwt lwt
}

// clientState tracks the state of the client.
type clientState struct {
	// started tracks the goroutines which have been started.
	started *sync.WaitGroup

	// endedW tracks when the writer has ended.
	endedW *sync.WaitGroup

	// endedR tracks when the reader has ended.
	endedR *sync.WaitGroup

	// end is a channel that indicates the client should be halted.
	end chan struct{}

	// done indicates that the client has closed.
	done int64

	// endOnce is called to ensure the close methods are only called once.
	endOnce *sync.Once

	// wasClosed indicates that the connection was closed deliberately.
	wasClosed bool
}

// newClient creates a new instance of client.
func newClient(c net.Conn, r *circ.Reader, w *circ.Writer) *client {
	cl := &client{
		conn: c,
		r:    r,
		w:    w,

		inFlight: inFlight{
			internal: make(map[uint16]*inFlightMessage),
		},
		subscriptions: make(map[string]byte),

		state: clientState{
			started: new(sync.WaitGroup),
			endedW:  new(sync.WaitGroup),
			endedR:  new(sync.WaitGroup),
			end:     make(chan struct{}),
			endOnce: new(sync.Once),
		},
	}

	return cl
}

func (cl *client) identify(lid string, pk *packets.ConnectPacket, ac auth.Controller) {
	cl.listener = lid
	cl.ac = ac
	cl.id = strings.Replace(pk.ClientIdentifier, "Jonathans-MacBook-Pro.local-worker0", "", -1)
	cl.user = pk.Username
	cl.keepalive = pk.Keepalive
	cl.cleanSession = pk.CleanSession

	if strings.Contains(cl.id, "sub") {
		cl.id = "\t\033[0;34m" + cl.id + "\033[0m"
	}

	cl.r.SetID(cl.id + " READER")
	cl.w.SetID(cl.id + " WRITER")

	// If no client id was provided, generate a new one.
	if cl.id == "" {
		cl.id = xid.New().String()
	}

	// if no deadline value was provided, set it to the default seconds.
	if cl.keepalive == 0 {
		cl.keepalive = defaultClientKeepalive
	}

	// If a last will and testament has been provided, record it.
	if pk.WillFlag {
		cl.lwt = lwt{
			topic:   pk.WillTopic,
			message: pk.WillMessage,
			qos:     pk.WillQos,
			retain:  pk.WillRetain,
		}
	}
}

// start begins the client goroutines reading and writing packets.
func (cl *client) start() {
	cl.state.started.Add(2)

	go func() {
		cl.state.started.Done()
		cl.w.WriteTo(cl.conn)
		cl.state.endedW.Done()
		debug.Printf("%s \033[1;33mGR WriteTo ended\033[0m\n", cl.id)

		//cl.close()
	}()
	cl.state.endedW.Add(1)

	go func() {
		cl.state.started.Done()
		cl.r.ReadFrom(cl.conn)
		cl.state.endedR.Done()

		debug.Printf("%s \033[1;33mGR ReadFrom ended\033[0m\n", cl.id)
		//cl.close()
	}()
	cl.state.endedR.Add(1)

	cl.state.started.Wait()
}

// close instructs the client to shut down all processing goroutines and disconnect.
func (cl *client) close() {

	// Signal the incoming read loop to stop.
	//atomic.StoreInt64(&cl.state.done, 1)
	/*
		debug.Println(cl.id, "SIGNALLING READER TO CLOSE")
		cl.r.Close()

		debug.Println(cl.id, "WAITING FOR READER TO CLOSE")
		cl.state.endedR.Wait()

		debug.Println(cl.id, "SIGNALLING WRITER TO CLOSE")
		//cl.w.Close()
		debug.Println(cl.id, "WAITING FOR WRITER TO CLOSE")
		//cl.state.endedW.Wait()
		rt, rh := cl.r.GetPos()
		debug.Println(cl.id, "WRITER CLOSED", rt, rh)

		// Cut off the connection if it's still up.
		if cl.conn != nil {
			//debug.Println(cl.id, "CLOSING CONNECTION")
			//	cl.conn.Close()
		}

		//cl.state.endedW.Wait()
	*/

	debug.Println(cl.id, "SIGNALLING READER TO CLOSE")
	cl.r.Close()
	debug.Println(cl.id, "READER CLOSE SENT")

	// Wait for the writing buffer to finish.
	debug.Println(cl.id, "SIGNALLING WRITER TO CLOSE")
	cl.w.Close()
	cl.state.endedW.Wait()

	// Cut off the connection if it's still up.
	if cl.conn != nil {
		cl.conn.Close()
	}

	// Wait for the reading buffer to close.
	cl.state.endedR.Wait()
	//cl.w.Close()

}

// readFixedHeader reads in the values of the next packet's fixed header.
func (cl *client) readFixedHeader(fh *packets.FixedHeader) error {

	// Peek the maximum message type and flags, and length.
	peeked, err := cl.r.Peek(1)
	if err != nil {
		return err
	}

	fmt.Println(cl.id, "READFH PEEKED", peeked)

	// Unpack message type and flags from byte 1.
	err = fh.Decode(peeked[0])
	if err != nil {
		// @SPEC [MQTT-2.2.2-2]
		// If invalid flags are received, the receiver MUST close the Network Connection.
		return err
		//return packets.ErrInvalidFlags
	}

	// The remaining length value can be up to 5 bytes. Peek through each byte
	// looking for continue values, and if found increase the peek. Otherwise
	// decode the bytes that were legit.
	buf := make([]byte, 0, 6)
	i := 1
	var b int64 = 2 // need this var later.
	for ; b < 6; b++ {
		peeked, err = cl.r.Peek(b)
		if err != nil {
			return err
		}

		// Add the byte to the length bytes slice.
		if i >= len(peeked) {
			return ErrInsufficientBytes
		}
		buf = append(buf, peeked[i])

		// If it's not a continuation flag, end here.
		if peeked[i] < 128 {
			break
		}

		// If i has reached 4 without a length terminator, return a protocol violation.
		i++
		if i == 4 {
			return packets.ErrOversizedLengthIndicator
		}
	}

	// Calculate and store the remaining length.
	rem, _ := binary.Uvarint(buf)
	fh.Remaining = int(rem)

	// Skip the number of used length bytes + first byte.
	err = cl.r.CommitTail(b)
	if err != nil {
		return err
	}

	// Set the fixed header in the parser.
	cl.fh = *fh

	return nil
}

// read reads new packets from a client connection.
func (cl *client) read(handler func(*client, packets.Packet) error) error {
	var err error
	var pk packets.Packet
	fh := new(packets.FixedHeader)
DONE:
	for {
		if atomic.LoadInt64(&cl.state.done) == 1 && cl.r.CapDelta() == 0 {
			break DONE
		}

		if cl.conn == nil {
			debug.Printf("%s \033[1;32mRR connection closed\033[0m\n", cl.id)
			return ErrConnectionClosed
		}

		// Reset the keepalive read deadline.
		cl.refreshDeadline(cl.keepalive)

		// Read in the fixed header of the packet.
		err = cl.readFixedHeader(fh)
		if err != nil {
			debug.Printf("%s \033[1;32mRR bad fixed header: %v\033[0m\n", cl.id, err)
			fmt.Printf("%+v\n", fh)
			return ErrReadFixedHeader
		}

		// If it's a disconnect packet, begin the close process.
		if fh.Type == packets.Disconnect {
			debug.Printf("%s \033[1;32mRR got disconnect %v\033[0m\n", cl.id)
			break DONE
		}

		// Otherwise read in the packet payload.
		pk, err = cl.readPacket()
		if err != nil {
			debug.Printf("%s \033[1;32mRR bad read packet: %v\033[0m\n", cl.id, err)
			return ErrReadPacketPayload
		}

		// Validate the packet if necessary.
		_, err = pk.Validate()
		if err != nil {
			debug.Printf("%s \033[1;32mRR validation failure: %v\033[0m\n", cl.id, err)
			return ErrReadPacketValidation
		}

		// Process inbound packet.
		handler(cl, pk)
	}

	return nil
}

// readPacket reads the remaining buffer into an MQTT packet.
func (cl *client) readPacket() (pk packets.Packet, err error) {

	switch cl.fh.Type {
	case packets.Connect:
		pk = &packets.ConnectPacket{FixedHeader: cl.fh}
	case packets.Connack:
		pk = &packets.ConnackPacket{FixedHeader: cl.fh}
	case packets.Publish:
		pk = &packets.PublishPacket{FixedHeader: cl.fh}
	case packets.Puback:
		pk = &packets.PubackPacket{FixedHeader: cl.fh}
	case packets.Pubrec:
		pk = &packets.PubrecPacket{FixedHeader: cl.fh}
	case packets.Pubrel:
		pk = &packets.PubrelPacket{FixedHeader: cl.fh}
	case packets.Pubcomp:
		pk = &packets.PubcompPacket{FixedHeader: cl.fh}
	case packets.Subscribe:
		pk = &packets.SubscribePacket{FixedHeader: cl.fh}
	case packets.Suback:
		pk = &packets.SubackPacket{FixedHeader: cl.fh}
	case packets.Unsubscribe:
		pk = &packets.UnsubscribePacket{FixedHeader: cl.fh}
	case packets.Unsuback:
		pk = &packets.UnsubackPacket{FixedHeader: cl.fh}
	case packets.Pingreq:
		pk = &packets.PingreqPacket{FixedHeader: cl.fh}
	case packets.Pingresp:
		pk = &packets.PingrespPacket{FixedHeader: cl.fh}
	case packets.Disconnect:
		pk = &packets.DisconnectPacket{FixedHeader: cl.fh}
	default:
		return pk, fmt.Errorf("No valid packet available; %v", cl.fh.Type)
	}

	bt, err := cl.r.Read(int64(cl.fh.Remaining))
	if err != nil {
		return pk, err
	}

	// Decode the remaining packet values using a fresh copy of the bytes,
	// otherwise the next packet will change the data of this one.
	// ----
	// This line is super important. If the bytes being decoded are not
	// in their own memory space, packets will get corrupted all over the place.
	err = pk.Decode(append([]byte{}, bt[:]...)) // <--- This MUST be a copy.
	if err != nil {
		return pk, err
	}

	return
}

// refreshDeadline refreshes the read/write deadline for the net.Conn connection.
func (cl *client) refreshDeadline(keepalive uint16) {
	if cl.conn != nil {
		expiry := time.Duration(keepalive+(keepalive/2)) * time.Second
		cl.conn.SetDeadline(time.Now().Add(expiry))
	}
}

// nextPacketID returns the next packet id for a client, looping back to 0
// if the maximum ID has been reached.
func (cl *client) nextPacketID() uint32 {
	i := atomic.LoadUint32(&cl.packetID)
	if i == uint32(65535) || i == uint32(0) {
		atomic.StoreUint32(&cl.packetID, 1)
		return 1
	}

	return atomic.AddUint32(&cl.packetID, 1)
}

// noteSubscription makes a note of a subscription for the client.
func (c *client) noteSubscription(filter string, qos byte) {
	c.Lock()
	c.subscriptions[filter] = qos
	c.Unlock()
}

// forgetSubscription forgests a subscription note for the client.
func (c *client) forgetSubscription(filter string) {
	c.Lock()
	delete(c.subscriptions, filter)
	c.Unlock()
}

// clients contains a map of the clients known by the broker.
type clients struct {
	sync.RWMutex

	// internal is a map of the clients known by the broker, keyed on client id.
	internal map[string]*client
}

// newClients returns an instance of clients.
func newClients() clients {
	return clients{
		internal: make(map[string]*client),
	}
}

// Add adds a new client to the clients map, keyed on client id.
func (cl *clients) add(val *client) {
	cl.Lock()
	cl.internal[val.id] = val
	cl.Unlock()
}

// Get returns the value of a client if it exists.
func (cl *clients) get(id string) (*client, bool) {
	cl.RLock()
	val, ok := cl.internal[id]
	cl.RUnlock()
	return val, ok
}

// Len returns the length of the clients map.
func (cl *clients) len() int {
	cl.RLock()
	val := len(cl.internal)
	cl.RUnlock()
	return val
}

// Delete removes a client from the internal map.
func (cl *clients) delete(id string) {
	cl.Lock()
	delete(cl.internal, id)
	cl.Unlock()
}

// getByListener returns clients matching a listener id.
func (cl *clients) getByListener(id string) []*client {
	clients := make([]*client, 0, cl.len())
	cl.RLock()
	for _, v := range cl.internal {
		if v.listener == id {
			clients = append(clients, v)
		}
	}
	cl.RUnlock()
	return clients
}

// lwt contains the last will and testament details for a client connection.
type lwt struct {

	// topic is the topic the will message shall be sent to.
	topic string

	// message is the message that shall be sent when the client disconnects.
	message []byte

	// qos is the quality of service byte desired for the will message.
	qos byte

	// retain indicates whether the will message should be retained.
	retain bool
}

// inFlightMessage contains data about a packet which is currently in-flight.
type inFlightMessage struct {

	// packet is the packet currently in-flight.
	packet packets.Packet

	// sent is the last time the message was sent (for retries) in unixtime.
	sent int64
}

// inFlight is a map of inFlightMessage keyed on packet id.
type inFlight struct {
	sync.RWMutex

	// internal contains the inflight messages.
	internal map[uint16]*inFlightMessage
}

// set stores the packet of an in-flight message, keyed on message id.
func (i *inFlight) set(key uint16, in *inFlightMessage) {
	i.Lock()
	i.internal[key] = in
	i.Unlock()
}

// get returns the value of an in-flight message if it exists.
func (i *inFlight) get(key uint16) (*inFlightMessage, bool) {
	i.RLock()
	val, ok := i.internal[key]
	i.RUnlock()
	return val, ok
}

// delete removes an in-flight message from the map.
func (i *inFlight) delete(key uint16) {
	i.Lock()
	delete(i.internal, key)
	i.Unlock()
}
