package clients

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/xid"

	"github.com/mochi-co/mqtt/internal/auth"
	"github.com/mochi-co/mqtt/internal/circ"
	"github.com/mochi-co/mqtt/internal/topics"
	//"github.com/mochi-co/mqtt/internal/debug"
	"github.com/mochi-co/mqtt/internal/packets"
)

var (
	defaultKeepalive uint16 = 60 // in seconds.

	ErrInsufficientBytes    = fmt.Errorf("Insufficient bytes in buffer")
	ErrReadPacketValidation = fmt.Errorf("Error validating packet")
	ErrConnectionClosed     = fmt.Errorf("Connection not open")
)

// Clients contains a map of the clients known by the broker.
type Clients struct {
	sync.RWMutex
	internal map[string]*Client // clients known by the broker, keyed on client id.
}

// New returns an instance of Clients.
func New() Clients {
	return Clients{
		internal: make(map[string]*Client),
	}
}

// Add adds a new client to the clients map, keyed on client id.
func (cl *Clients) Add(val *Client) {
	cl.Lock()
	cl.internal[val.id] = val
	cl.Unlock()
}

// Get returns the value of a client if it exists.
func (cl *Clients) Get(id string) (*Client, bool) {
	cl.RLock()
	val, ok := cl.internal[id]
	cl.RUnlock()
	return val, ok
}

// Len returns the length of the clients map.
func (cl *Clients) Len() int {
	cl.RLock()
	val := len(cl.internal)
	cl.RUnlock()
	return val
}

// Delete removes a client from the internal map.
func (cl *Clients) Delete(id string) {
	cl.Lock()
	delete(cl.internal, id)
	cl.Unlock()
}

// GetByListener returns clients matching a listener id.
func (cl *Clients) GetByListener(id string) []*Client {
	clients := make([]*Client, 0, cl.Len())
	cl.RLock()
	for _, v := range cl.internal {
		if v.listener == id {
			clients = append(clients, v)
		}
	}
	cl.RUnlock()
	return clients
}

// Client contains information about a client known by the broker.
type Client struct {
	sync.RWMutex
	ac            auth.Controller      // an auth controller inherited from the listener.
	inFlight      inFlight             // a map of in-flight qos messages.
	subscriptions topics.Subscriptions // a map of the subscription filters a client maintains.
	state         clientState          // the operational state of the client.
	conn          net.Conn             // the net.Conn used to establish the connection.
	r             *circ.Reader         // a reader for reading incoming bytes.
	w             *circ.Writer         // a writer for writing outgoing bytes.
	fh            packets.FixedHeader  // the FixedHeader from the last read packet.
	id            string               // the client id.
	listener      string               // the id of the listener the client is connected to.
	user          []byte               // the username the client authenticated with.
	keepalive     uint16               // the number of seconds the connection can wait.
	cleanSession  bool                 // indicates if the client expects a clean-session.
	packetID      uint32               // the current highest packetID.
	lwt           LWT                  // the last will and testament for the client.
}

// clientState tracks the state of the client.
type clientState struct {
	started *sync.WaitGroup // tracks the goroutines which have been started.
	endedW  *sync.WaitGroup // tracks when the writer has ended.
	endedR  *sync.WaitGroup // tracks when the reader has ended.
	end     chan struct{}   // a channel which indicates the client event loop should be halted.
	done    int64           // atomic counter which indicates that the client has closed.
	endOnce *sync.Once      // called to ensure the close methods are only called once.
	//wasClosed bool            //  indicates that the connection was closed deliberately.
}

// NewClient returns a new instance of Client.
func NewClient(c net.Conn, r *circ.Reader, w *circ.Writer) *Client {
	cl := &Client{
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

// Identify sets the identification values of a client instance.
func (cl *Client) Identify(lid string, pk *packets.Packet, ac auth.Controller) {
	cl.listener = lid
	cl.ac = ac
	cl.id = pk.ClientIdentifier

	// TBR
	//cl.id = strings.Replace(pk.ClientIdentifier, "Jonathans-MacBook-Pro.local-worker0", "", -1)
	//if strings.Contains(cl.id, "sub") {
	//	cl.id = "\t\033[0;34m" + cl.id + "\033[0m"
	//}
	cl.r.ID = cl.id + " READER"
	cl.w.ID = cl.id + " WRITER"

	cl.user = pk.Username
	cl.keepalive = pk.Keepalive
	cl.cleanSession = pk.CleanSession

	if cl.id == "" {
		cl.id = xid.New().String()
	}

	if cl.keepalive == 0 {
		cl.keepalive = defaultKeepalive
	}

	if pk.WillFlag {
		cl.lwt = LWT{
			topic:   pk.WillTopic,
			message: pk.WillMessage,
			qos:     pk.WillQos,
			retain:  pk.WillRetain,
		}
	}
}

// refreshDeadline refreshes the read/write deadline for the net.Conn connection.
func (cl *Client) refreshDeadline(keepalive uint16) {
	if cl.conn != nil {
		expiry := time.Duration(keepalive+(keepalive/2)) * time.Second
		cl.conn.SetDeadline(time.Now().Add(expiry))
	}
}

// nextPacketID returns the next packet id for a client, looping back to 0
// if the maximum ID has been reached.
func (cl *Client) nextPacketID() uint32 {
	i := atomic.LoadUint32(&cl.packetID)
	if i == uint32(65535) || i == uint32(0) {
		atomic.StoreUint32(&cl.packetID, 1)
		return 1
	}

	return atomic.AddUint32(&cl.packetID, 1)
}

// noteSubscription makes a note of a subscription for the client.
func (cl *Client) noteSubscription(filter string, qos byte) {
	cl.Lock()
	cl.subscriptions[filter] = qos
	cl.Unlock()
}

// forgetSubscription forgests a subscription note for the client.
func (cl *Client) forgetSubscription(filter string) {
	cl.Lock()
	delete(cl.subscriptions, filter)
	cl.Unlock()
}

// Start begins the client goroutines reading and writing packets.
func (cl *Client) Start() {
	cl.state.started.Add(2)

	go func() {
		cl.state.started.Done()
		cl.w.WriteTo(cl.conn)
		cl.state.endedW.Done()
		//debug.Printf("%s \033[1;33mGR WriteTo ended\033[0m\n", cl.id)
		//cl.close()
	}()
	cl.state.endedW.Add(1)

	go func() {
		cl.state.started.Done()
		cl.r.ReadFrom(cl.conn)
		cl.state.endedR.Done()
		//debug.Printf("%s \033[1;33mGR ReadFrom ended\033[0m\n", cl.id)
		//cl.close()
	}()
	cl.state.endedR.Add(1)
	cl.state.started.Wait()
}

// Stop instructs the client to shut down all processing goroutines and disconnect.
func (cl *Client) Stop() {
	/*
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
	*/
}

// readFixedHeader reads in the values of the next packet's fixed header.
func (cl *Client) ReadFixedHeader(fh *packets.FixedHeader) error {
	p, err := cl.r.Read(1)
	if err != nil {
		return err
	}

	err = fh.Decode(p[0])
	if err != nil {
		return err
	}

	// The remaining length value can be up to 5 bytes. Read through each byte
	// looking for continue values, and if found increase the read. Otherwise
	// decode the bytes that were legit.
	buf := make([]byte, 0, 6)
	i := 1
	n := 2
	for ; n < 6; n++ {
		p, err = cl.r.Read(n)
		if err != nil {
			return err
		}

		//if i >= len(p) {
		//	return ErrInsufficientBytes
		//}

		buf = append(buf, p[i])

		// If it's not a continuation flag, end here.
		if p[i] < 128 {
			break
		}

		// If i has reached 4 without a length terminator, return a protocol violation.
		i++
		if i == 4 {
			return packets.ErrOversizedLengthIndicator
		}
	}

	// Calculate and store the remaining length of the packet payload.
	rem, _ := binary.Uvarint(buf)
	fh.Remaining = int(rem)
	//cl.fh = *fh

	// Having successfully read n bytes, commit the tail forward.
	cl.r.CommitTail(n)

	return nil
}

// Read reads new packets from a client connection
func (cl *Client) Read(handler func(*Client, packets.Packet) error) error {
	for {
		if atomic.LoadInt64(&cl.state.done) == 1 && cl.r.CapDelta() == 0 {
			return nil
		}

		if cl.conn == nil {
			return ErrConnectionClosed
		}

		cl.refreshDeadline(cl.keepalive)

		var fh packets.FixedHeader
		err := cl.ReadFixedHeader(&fh)
		if err != nil {
			return err
		}

		if fh.Type == packets.Disconnect {
			return nil
		}

		pk, err := cl.readPacket(fh)
		if err != nil {
			return err
		}

		/*
			_, err = pk.Validate()
			if err != nil {
				return ErrReadPacketValidation
			}
		*/

		cl.r.CommitTail(pk.FixedHeader.Remaining)

		handler(cl, pk) // Process inbound packet.
	}
}

// readPacket reads the remaining buffer into an MQTT packet.
func (cl *Client) readPacket(fh packets.FixedHeader) (pk packets.Packet, err error) {
	pk.FixedHeader = fh

	p, err := cl.r.Read(pk.FixedHeader.Remaining)
	if err != nil {
		return pk, err
	}

	// Decode the remaining packet values using a fresh copy of the bytes,
	// otherwise the next packet will change the data of this one.
	px := append([]byte{}, p[:]...)

	switch pk.FixedHeader.Type {
	case packets.Connect:
		pk.ConnectDecode(px)
	case packets.Connack:
		err = pk.ConnackDecode(px)
	case packets.Publish:
		err = pk.PublishDecode(px)
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
	//	err = pk.PingreqDecode(px)
	case packets.Pingresp:
	//	err = pk.PingrespDecode(px)
	case packets.Disconnect:
	//	err = pk.DisconnectDecode(px)
	default:
		err = fmt.Errorf("No valid packet available; %v", cl.fh.Type)
	}

	return
}

// LWT contains the last will and testament details for a client connection.
type LWT struct {
	topic   string // the topic the will message shall be sent to.
	message []byte // the message that shall be sent when the client disconnects.
	qos     byte   //  the quality of service desired.
	retain  bool   // indicates whether the will message should be retained
}

// inFlightMessage contains data about a packet which is currently in-flight.
type inFlightMessage struct {
	packet *packets.Packet // the packet currently in-flight.
	sent   int64           // the last time the message was sent (for retries) in unixtime.
}

// inFlight is a map of inFlightMessage keyed on packet id.
type inFlight struct {
	sync.RWMutex
	internal map[uint16]*inFlightMessage // internal contains the inflight messages.
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
