package mqtt

import (
	"encoding/binary"
	"log"
	"net"
	"time"

	"github.com/mochi-co/mqtt/packets"
	"github.com/mochi-co/mqtt/processors/circ"
)

// Processor reads and writes bytes to a network connection.
type Processor struct {

	// Conn is the net.Conn used to establish the connection.
	Conn net.Conn

	// R is a reader for reading incoming bytes.
	R *circ.Reader

	// W is a writer for writing outgoing bytes.
	W *circ.Writer

	// FixedHeader is the FixedHeader from the last read packet.
	FixedHeader packets.FixedHeader
}

// NewProcessor returns a new instance of Processor.
func NewProcessor(c net.Conn, r *circ.Reader, w *circ.Writer) *Processor {
	return &Processor{
		Conn: c,
		R:    r,
		W:    w,
	}
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

	log.Println("Peeked", peeked)

	// Unpack message type and flags from byte 1.
	err = fh.Decode(peeked[0])
	if err != nil {

		// @SPEC [MQTT-2.2.2-2]
		// If invalid flags are received, the receiver MUST close the Network Connection.
		return packets.ErrInvalidFlags
	}

	log.Printf("FH %+v\n", fh)

	// The remaining length value can be up to 5 bytes. Peek through each byte
	// looking for continue values, and if found increase the peek. Otherwise
	// decode the bytes that were legit.
	//p.fhBuffer = p.fhBuffer[:0]
	buf := make([]byte, 0, 6)
	i := 1
	//var b int64 = 2
	for b := int64(2); b < 6; b++ {
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
	//p.R.Discard(b)

	// Set the fixed header in the parser.
	p.FixedHeader = *fh

	return nil

}
