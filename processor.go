package mqtt

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/mochi-co/mqtt/circ"
	"github.com/mochi-co/mqtt/packets"
)

var (
	// ErrInsufficientBytes indicates that there were not enough bytes
	// in the buffer to read/peek.
	ErrInsufficientBytes = fmt.Errorf("Insufficient bytes in buffer")
)

// Processor reads and writes bytes to a network connection.
type Processor struct {

	// Conn is the net.Conn used to establish the connection.
	Conn net.Conn

	// R is a reader for reading incoming bytes.
	R *circ.Reader

	// W is a writer for writing outgoing bytes.
	W *circ.Writer

	// started tracks the goroutines which have been started.
	started *sync.WaitGroup

	// endedW tracks when the writer has ended.
	endedW *sync.WaitGroup

	// endedR tracks when the reader has ended.
	endedR *sync.WaitGroup

	// FixedHeader is the FixedHeader from the last read packet.
	FixedHeader packets.FixedHeader
}

// NewProcessor returns a new instance of Processor.
func NewProcessor(c net.Conn, r *circ.Reader, w *circ.Writer) *Processor {
	return &Processor{
		Conn:    c,
		R:       r,
		W:       w,
		started: new(sync.WaitGroup),
		endedW:  new(sync.WaitGroup),
		endedR:  new(sync.WaitGroup),
	}

}

// Start spins up the reader and writer goroutines.
func (p *Processor) Start() {
	p.started.Add(2)

	go func(start, end *sync.WaitGroup) {
		defer p.endedW.Done()
		p.started.Done()
		p.W.WriteTo(p.Conn)
	}(p.started, p.endedW)
	p.endedW.Add(1)

	go func(start, end *sync.WaitGroup) {
		defer p.endedR.Done()
		p.started.Done()
		p.R.ReadFrom(p.Conn)
	}(p.started, p.endedR)
	p.endedR.Add(1)

	p.started.Wait()
	fmt.Println("Started OK")
}

// Stop stops the processor goroutines.
func (p *Processor) Stop() {
	p.R.Close()
	p.W.Close()
	p.endedW.Wait()

	if p.Conn != nil {
		p.Conn.Close()
	}
	p.endedR.Wait()
}

// RefreshDeadline refreshes the read/write deadline for the net.Conn connection.
func (p *Processor) RefreshDeadline(keepalive uint16) {
	if p.Conn != nil {
		expiry := time.Duration(keepalive+(keepalive/2)) * time.Second
		p.Conn.SetDeadline(time.Now().Add(expiry))
	}
}

// ReadFixedHeader reads in the values of the next packet's fixed header.
func (p *Processor) ReadFixedHeader(fh *packets.FixedHeader) error {

	// Peek the maximum message type and flags, and length.
	peeked, err := p.R.Peek(1)
	if err != nil {
		return err
	}

	// Unpack message type and flags from byte 1.
	err = fh.Decode(peeked[0])
	if err != nil {

		// @SPEC [MQTT-2.2.2-2]
		// If invalid flags are received, the receiver MUST close the Network Connection.
		return packets.ErrInvalidFlags
	}

	// The remaining length value can be up to 5 bytes. Peek through each byte
	// looking for continue values, and if found increase the peek. Otherwise
	// decode the bytes that were legit.
	buf := make([]byte, 0, 6)
	i := 1
	var b int64 = 2 // need this var later.
	for ; b < 6; b++ {
		peeked, err = p.R.Peek(b)
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
	err = p.R.CommitTail(b)
	if err != nil {
		return err
	}

	fmt.Println("FIXEDHEADER READ", *fh)

	// Set the fixed header in the parser.
	p.FixedHeader = *fh

	return nil

}

// Read reads the remaining buffer into an MQTT packet.
func (p *Processor) Read() (pk packets.Packet, err error) {

	switch p.FixedHeader.Type {
	case packets.Connect:
		pk = &packets.ConnectPacket{FixedHeader: p.FixedHeader}
	case packets.Connack:
		pk = &packets.ConnackPacket{FixedHeader: p.FixedHeader}
	case packets.Publish:
		pk = &packets.PublishPacket{FixedHeader: p.FixedHeader}
	case packets.Puback:
		pk = &packets.PubackPacket{FixedHeader: p.FixedHeader}
	case packets.Pubrec:
		pk = &packets.PubrecPacket{FixedHeader: p.FixedHeader}
	case packets.Pubrel:
		pk = &packets.PubrelPacket{FixedHeader: p.FixedHeader}
	case packets.Pubcomp:
		pk = &packets.PubcompPacket{FixedHeader: p.FixedHeader}
	case packets.Subscribe:
		pk = &packets.SubscribePacket{FixedHeader: p.FixedHeader}
	case packets.Suback:
		pk = &packets.SubackPacket{FixedHeader: p.FixedHeader}
	case packets.Unsubscribe:
		pk = &packets.UnsubscribePacket{FixedHeader: p.FixedHeader}
	case packets.Unsuback:
		pk = &packets.UnsubackPacket{FixedHeader: p.FixedHeader}
	case packets.Pingreq:
		pk = &packets.PingreqPacket{FixedHeader: p.FixedHeader}
	case packets.Pingresp:
		pk = &packets.PingrespPacket{FixedHeader: p.FixedHeader}
	case packets.Disconnect:
		pk = &packets.DisconnectPacket{FixedHeader: p.FixedHeader}
	default:
		return pk, fmt.Errorf("No valid packet available; %v", p.FixedHeader.Type)
	}

	bt, err := p.R.Read(int64(p.FixedHeader.Remaining))
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
