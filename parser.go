package mqtt

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/mochi-co/mqtt/packets"
)

// BufWriter is an interface for satisfying a bufio.Writer. This is mainly
// in place to allow testing.
type BufWriter interface {

	// Write writes a byte buffer.
	Write(p []byte) (nn int, err error)

	// Flush flushes the buffer.
	Flush() error
}

// Parser is an MQTT packet parser that reads and writes MQTT payloads to a
// buffered IO stream.
type Parser struct {
	sync.RWMutex

	// Conn is the net.Conn used to establish the connection.
	Conn net.Conn

	// R is a bufio reader for peeking and reading incoming packets.
	R *bufio.Reader

	// W is a bufio writer for writing outgoing packets.
	W BufWriter

	// FixedHeader is the fixed header from the last seen packet.
	FixedHeader packets.FixedHeader
}

// NewParser returns an instance of Parser for a connection.
func NewParser(c net.Conn, r *bufio.Reader, w BufWriter) *Parser {
	return &Parser{
		Conn: c,
		R:    r,
		W:    w,
	}
}

// RefreshDeadline refreshes the read/write deadline for the net.Conn connection.
func (p *Parser) RefreshDeadline(keepalive uint16) {
	if p.Conn != nil {
		expiry := time.Duration(keepalive+(keepalive/2)) * time.Second
		p.Conn.SetDeadline(time.Now().Add(expiry))
	}
}

// ReadFixedHeader reads in the values of the next packet's fixed header.
func (p *Parser) ReadFixedHeader(fh *packets.FixedHeader) error {

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
	//p.fhBuffer = p.fhBuffer[:0]
	buf := make([]byte, 0, 6)
	i := 1
	b := 2
	for ; b < 6; b++ {
		peeked, err = p.R.Peek(b)
		if err != nil {
			return err
		}

		// Add the byte to the length bytes slice.
		buf = append(buf, peeked[i])

		// If it's not a continuation flag, end here.
		if peeked[i] < 128 {
			break
		}

		// If i has reached 4 without a length terminator, throw a protocol violation.
		i++
		if i == 4 {
			return packets.ErrOversizedLengthIndicator
		}
	}

	// Calculate and store the remaining length.
	rem, _ := binary.Uvarint(buf)
	fh.Remaining = int(rem)

	// Discard the number of used length bytes + first byte.
	p.R.Discard(b)

	// Set the fixed header in the parser.
	p.FixedHeader = *fh

	return nil
}

// Read reads the remaining buffer into an MQTT packet.
func (p *Parser) Read() (pk packets.Packet, err error) {

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
		return pk, errors.New("No valid packet available; " + string(p.FixedHeader.Type))
	}

	// Attempt to peek the rest of the packet.
	peeked := true

	bt, err := p.R.Peek(p.FixedHeader.Remaining)
	if err != nil {

		// Only try to continue if reading is still possible.
		if err != bufio.ErrBufferFull {
			return pk, err
		}

		// If it didn't work, read the buffer directly, and if that still doesn't
		// work, then throw an error.
		peeked = false
		bt = make([]byte, p.FixedHeader.Remaining)
		_, err := io.ReadFull(p.R, bt)
		if err != nil {
			return pk, err
		}
	}

	// If peeking was successful, discard the rest of the packet now it's been read.
	if peeked {
		p.R.Discard(p.FixedHeader.Remaining)
	}

	// Decode the remaining packet values using a fresh copy of the bytes,
	// otherwise the next packet will change the data of this one.
	// 🚨 DANGER!! This line is super important. If the bytes being decoded are not
	// in their own memory space, packets will get corrupted all over the place.
	err = pk.Decode(append([]byte{}, bt[:]...)) // <--- MUST BE A COPY.
	if err != nil {
		return pk, err
	}

	return
}
