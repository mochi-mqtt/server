package packets

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

// Packet is the base interface that all MQTT packets must implement.
type Packet interface {

	// Encode encodes a packet into a byte buffer.
	Encode(*bytes.Buffer) error

	// Decode decodes a byte array into a packet struct.
	Decode([]byte) error

	// Validate the packet. Returns a error code and error if not valid.
	Validate() (byte, error)
}

// Parser is an MQTT packet parser that reads and writes MQTT payloads to a
// buffered IO stream.
type Parser struct {

	// sync mutex to avoid access collisions.
	sync.RWMutex

	// Conn is the net.Conn used to establish the connection.
	Conn net.Conn

	// R is a bufio reader for peeking and reading incoming packets.
	R *bufio.Reader

	// FixedHeader is the fixed header from the last seen packet.
	FixedHeader FixedHeader
}

// NewParser returns an instance of Parser for a connection.
func NewParser(c net.Conn) *Parser {
	return &Parser{
		Conn: c,
		R:    bufio.NewReaderSize(c, 512),
	}
}

// RefreshDeadline refreshes the read/write deadline for the net.Conn connection.
func (p *Parser) RefreshDeadline(keepalive uint16) {
	expiry := time.Duration(keepalive+(keepalive/2)) * time.Second
	p.Conn.SetDeadline(time.Now().UTC().Add(expiry))
}

// Reset sets the new destinations for the read and write buffers.
func (p *Parser) Reset(c net.Conn) {
	p.Lock()
	defer p.Unlock()
	p.Conn = c
}

// ReadFixedHeader reads in the values of the next packet's fixed header.
func (p *Parser) ReadFixedHeader(fh *FixedHeader) error {

	// Peek the maximum message type and flags, and length.
	peeked, err := p.R.Peek(1)
	if err != nil {
		return err
	}

	// Unpack message type and flags from byte 1.
	err = fh.decode(peeked[0])
	if err != nil {

		// @SPEC [MQTT-2.2.2-2]
		// If invalid flags are received, the receiver MUST close the Network Connection.
		return ErrInvalidFlags
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

		// Add the byte to the length bytes slice.
		buf = append(buf, peeked[i])

		// If it's not a continuation flag, end here.
		if peeked[i] < 128 {
			break
		}

		// If i has reached 4 without a length terminator, throw a protocol violation.
		i++
		if i == 4 {
			return ErrOversizedLengthIndicator
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
func (p *Parser) Read() (pk Packet, err error) {

	switch p.FixedHeader.Type {
	case Connect:
		pk = &ConnectPacket{FixedHeader: p.FixedHeader}
	case Connack:
		pk = &ConnackPacket{FixedHeader: p.FixedHeader}
	case Publish:
		pk = &PublishPacket{FixedHeader: p.FixedHeader}
	case Puback:
		pk = &PubackPacket{FixedHeader: p.FixedHeader}
	case Pubrec:
		pk = &PubrecPacket{FixedHeader: p.FixedHeader}
	case Pubrel:
		pk = &PubrelPacket{FixedHeader: p.FixedHeader}
	case Pubcomp:
		pk = &PubcompPacket{FixedHeader: p.FixedHeader}
	case Subscribe:
		pk = &SubscribePacket{FixedHeader: p.FixedHeader}
	case Suback:
		pk = &SubackPacket{FixedHeader: p.FixedHeader}
	case Unsubscribe:
		pk = &UnsubscribePacket{FixedHeader: p.FixedHeader}
	case Unsuback:
		pk = &UnsubackPacket{FixedHeader: p.FixedHeader}
	case Pingreq:
		pk = &PingreqPacket{FixedHeader: p.FixedHeader}
	case Pingresp:
		pk = &PingrespPacket{FixedHeader: p.FixedHeader}
	case Disconnect:
		pk = &DisconnectPacket{FixedHeader: p.FixedHeader}
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

	// Decode the remaining packet values.
	err = pk.Decode(bt)
	if err != nil {
		return pk, err
	}

	// If peeking was successful, discard the rest of the packet now it's been read.
	if peeked {
		p.R.Discard(p.FixedHeader.Remaining)
	}

	// Validate the packet to ensure spec compliance.
	_, err = pk.Validate()
	if err != nil {
		return pk, err
	}

	return
}
