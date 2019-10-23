package mqtt

import (
	"bufio"
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/mochi-co/mqtt/packets"
)

func newBufioReader(c net.Conn) *bufio.Reader {
	return bufio.NewReaderSize(c, 512)
}

func newBufioWriter(c net.Conn) *bufio.Writer {
	return bufio.NewWriterSize(c, 512)
}

func TestNewParser(t *testing.T) {
	conn := new(MockNetConn)
	p := NewParser(conn, newBufioReader(conn), newBufioWriter(conn))
	require.NotNil(t, p.R)
}

func BenchmarkNewParser(b *testing.B) {
	conn := new(MockNetConn)
	r, w := new(bufio.Reader), new(bufio.Writer)

	for n := 0; n < b.N; n++ {
		NewParser(conn, r, w)
	}
}

func TestRefreshDeadline(t *testing.T) {
	conn := new(MockNetConn)
	p := NewParser(conn, newBufioReader(conn), newBufioWriter(conn))

	dl := p.Conn.(*MockNetConn).Deadline
	p.RefreshDeadline(10)

	require.NotEqual(t, dl, p.Conn.(*MockNetConn).Deadline)
}

func BenchmarkRefreshDeadline(b *testing.B) {
	conn := new(MockNetConn)
	p := NewParser(conn, newBufioReader(conn), newBufioWriter(conn))

	for n := 0; n < b.N; n++ {
		p.RefreshDeadline(10)
	}
}

/*
type fixedHeaderTable struct {
	rawBytes    []byte
	header      packets.FixedHeader
	packetError bool
	flagError   bool
}

var fixedHeaderExpected = []fixedHeaderTable{
	{
		rawBytes: []byte{packets.Connect << 4, 0x00},
		header:   packets.FixedHeader{packets.Connect, false, 0, false, 0}, // Type byte, Dup bool, Qos byte, Retain bool, Remaining int
	},
	{
		rawBytes: []byte{packets.Connack << 4, 0x00},
		header:   packets.FixedHeader{packets.Connack, false, 0, false, 0},
	},
	{
		rawBytes: []byte{packets.Publish << 4, 0x00},
		header:   packets.FixedHeader{packets.Publish, false, 0, false, 0},
	},
	{
		rawBytes: []byte{packets.Publish<<4 | 1<<1, 0x00},
		header:   packets.FixedHeader{packets.Publish, false, 1, false, 0},
	},
	{
		rawBytes: []byte{packets.Publish<<4 | 1<<1 | 1, 0x00},
		header:   packets.FixedHeader{packets.Publish, false, 1, true, 0},
	},
	{
		rawBytes: []byte{packets.Publish<<4 | 2<<1, 0x00},
		header:   packets.FixedHeader{packets.Publish, false, 2, false, 0},
	},
	{
		rawBytes: []byte{Publish<<4 | 2<<1 | 1, 0x00},
		header:   FixedHeader{Publish, false, 2, true, 0},
	},
	{
		rawBytes: []byte{Publish<<4 | 1<<3, 0x00},
		header:   FixedHeader{Publish, true, 0, false, 0},
	},
	{
		rawBytes: []byte{Publish<<4 | 1<<3 | 1, 0x00},
		header:   FixedHeader{Publish, true, 0, true, 0},
	},
	{
		rawBytes: []byte{Publish<<4 | 1<<3 | 1<<1 | 1, 0x00},
		header:   FixedHeader{Publish, true, 1, true, 0},
	},
	{
		rawBytes: []byte{Publish<<4 | 1<<3 | 2<<1 | 1, 0x00},
		header:   FixedHeader{Publish, true, 2, true, 0},
	},
	{
		rawBytes: []byte{Puback << 4, 0x00},
		header:   FixedHeader{Puback, false, 0, false, 0},
	},
	{
		rawBytes: []byte{Pubrec << 4, 0x00},
		header:   FixedHeader{Pubrec, false, 0, false, 0},
	},
	{
		rawBytes: []byte{Pubrel<<4 | 1<<1, 0x00},
		header:   FixedHeader{Pubrel, false, 1, false, 0},
	},
	{
		rawBytes: []byte{Pubcomp << 4, 0x00},
		header:   FixedHeader{Pubcomp, false, 0, false, 0},
	},
	{
		rawBytes: []byte{Subscribe<<4 | 1<<1, 0x00},
		header:   FixedHeader{Subscribe, false, 1, false, 0},
	},
	{
		rawBytes: []byte{Suback << 4, 0x00},
		header:   FixedHeader{Suback, false, 0, false, 0},
	},
	{
		rawBytes: []byte{Unsubscribe<<4 | 1<<1, 0x00},
		header:   FixedHeader{Unsubscribe, false, 1, false, 0},
	},
	{
		rawBytes: []byte{Unsuback << 4, 0x00},
		header:   FixedHeader{Unsuback, false, 0, false, 0},
	},
	{
		rawBytes: []byte{Pingreq << 4, 0x00},
		header:   FixedHeader{Pingreq, false, 0, false, 0},
	},
	{
		rawBytes: []byte{Pingresp << 4, 0x00},
		header:   FixedHeader{Pingresp, false, 0, false, 0},
	},
	{
		rawBytes: []byte{Disconnect << 4, 0x00},
		header:   FixedHeader{Disconnect, false, 0, false, 0},
	},

	// remaining length
	{
		rawBytes: []byte{Publish << 4, 0x0a},
		header:   FixedHeader{Publish, false, 0, false, 10},
	},
	{
		rawBytes: []byte{Publish << 4, 0x80, 0x04},
		header:   FixedHeader{Publish, false, 0, false, 512},
	},
	{
		rawBytes: []byte{Publish << 4, 0xd2, 0x07},
		header:   FixedHeader{Publish, false, 0, false, 978},
	},
	{
		rawBytes: []byte{Publish << 4, 0x86, 0x9d, 0x01},
		header:   FixedHeader{Publish, false, 0, false, 20102},
	},
	{
		rawBytes:    []byte{Publish << 4, 0xd5, 0x86, 0xf9, 0x9e, 0x01},
		header:      FixedHeader{Publish, false, 0, false, 333333333},
		packetError: true,
	},

	// Invalid flags for packet
	{
		rawBytes:  []byte{Connect<<4 | 1<<3, 0x00},
		header:    FixedHeader{Connect, true, 0, false, 0},
		flagError: true,
	},
	{
		rawBytes:  []byte{Connect<<4 | 1<<1, 0x00},
		header:    FixedHeader{Connect, false, 1, false, 0},
		flagError: true,
	},
	{
		rawBytes:  []byte{Connect<<4 | 1, 0x00},
		header:    FixedHeader{Connect, false, 0, true, 0},
		flagError: true,
	},
}
*/

func TestParserReadFixedHeader(t *testing.T) {
	conn := new(MockNetConn)

	// Test null data.
	p := NewParser(conn, newBufioReader(conn), newBufioWriter(conn))
	fh := new(packets.FixedHeader)
	err := p.ReadFixedHeader(fh)
	require.Error(t, err)

	// Test insufficient peeking.
	fh = new(packets.FixedHeader)
	p.R = bufio.NewReader(bytes.NewReader([]byte{packets.Connect << 4}))
	err = p.ReadFixedHeader(fh)
	require.Error(t, err)

	fh = new(packets.FixedHeader)
	p.R = bufio.NewReader(bytes.NewReader([]byte{packets.Connect << 4, 0x00}))
	err = p.ReadFixedHeader(fh)
	require.NoError(t, err)

	/*


		// Test expected bytes.
		for i, wanted := range fixedHeaderExpected {
			fh := new(packets.FixedHeader)
			p := NewParser(conn, newBufioReader(conn), newBufioWriter(conn))
			b := wanted.rawBytes
			p.R = bufio.NewReader(bytes.NewReader(b))

			err := p.ReadFixedHeader(fh)
			if wanted.packetError || wanted.flagError {
				require.Error(t, err, "Expected error [i:%d] %v", i, wanted.rawBytes)
			} else {
				require.NoError(t, err, "Error reading fixedheader [i:%d] %v", i, wanted.rawBytes)
				require.Equal(t, wanted.header.Type, p.FixedHeader.Type, "Mismatched fixedheader type [i:%d] %v", i, wanted.rawBytes)
				require.Equal(t, wanted.header.Dup, p.FixedHeader.Dup, "Mismatched fixedheader dup [i:%d] %v", i, wanted.rawBytes)
				require.Equal(t, wanted.header.Qos, p.FixedHeader.Qos, "Mismatched fixedheader qos [i:%d] %v", i, wanted.rawBytes)
				require.Equal(t, wanted.header.Retain, p.FixedHeader.Retain, "Mismatched fixedheader retain [i:%d] %v", i, wanted.rawBytes)
			}
		}
	*/
}

func BenchmarkReadFixedHeader(b *testing.B) {
	conn := new(MockNetConn)
	fh := new(packets.FixedHeader)
	p := NewParser(conn, newBufioReader(conn), newBufioWriter(conn))

	var rn bytes.Reader = *bytes.NewReader([]byte{packets.Connect << 4, 0x00})
	var rc bytes.Reader
	for n := 0; n < b.N; n++ {
		rc = rn
		p.R.Reset(&rc)
		err := p.ReadFixedHeader(fh)
		if err != nil {
			panic(err)
		}
	}
}

func TestRead(t *testing.T) {
	conn := new(MockNetConn)

	var fh packets.FixedHeader
	p := NewParser(conn, newBufioReader(conn), newBufioWriter(conn))
	p.R = bufio.NewReader(bytes.NewReader([]byte{
		byte(packets.Publish << 4), 18, // Fixed header
		0, 5, // Topic Name - LSB+MSB
		'a', '/', 'b', '/', 'c', // Topic Name
		'h', 'e', 'l', 'l', 'o', ' ', 'm', 'o', 'c', 'h', 'i', // Payload
	}))
	err := p.ReadFixedHeader(&fh)
	require.NoError(t, err)

	pko, err := p.Read()
	require.NoError(t, err)
	require.Equal(t, &packets.PublishPacket{
		FixedHeader: packets.FixedHeader{
			Type:      packets.Publish,
			Remaining: 18,
		},
		TopicName: "a/b/c",
		Payload:   []byte("hello mochi"),
	}, pko)

	/*
		for code, pt := range expectedPackets {
			for i, wanted := range pt {
				if wanted.primary {
					var fh packets.FixedHeader
					b := wanted.rawBytes
					p := NewParser(conn, newBufioReader(conn), newBufioWriter(conn))
					p.R = bufio.NewReader(bytes.NewReader(b))

					err := p.ReadFixedHeader(&fh)
					if wanted.failFirst != nil {
						require.Error(t, err, "Expected error reading fixedheader [i:%d] %s - %s", i, wanted.desc, Names[code])
					} else {
						require.NoError(t, err, "Error reading fixedheader [i:%d] %s - %s", i, wanted.desc, Names[code])
					}

					pko, err := p.Read()

					if wanted.expect != nil {
						require.Error(t, err, "Expected error reading packet [i:%d] %s - %s", i, wanted.desc, Names[code])
						if err != nil {
							require.Equal(t, err, wanted.expect, "Mismatched packet error [i:%d] %s - %s", i, wanted.desc, Names[code])
						}
					} else {
						require.NoError(t, err, "Error reading packet [i:%d] %s - %s", i, wanted.desc, Names[code])
						require.Equal(t, wanted.packet, pko, "Mismatched packet final [i:%d] %s - %s", i, wanted.desc, Names[code])
					}
				}
			}
		}
	*/
}

func TestReadFail(t *testing.T) {
	conn := new(MockNetConn)

	var fh packets.FixedHeader
	p := NewParser(conn, newBufioReader(conn), newBufioWriter(conn))
	p.R = bufio.NewReader(bytes.NewReader([]byte{
		byte(packets.Publish << 4), 3, // Fixed header
		0, 5, // Topic Name - LSB+MSB
		'a', '/',
	}))
	err := p.ReadFixedHeader(&fh)
	require.NoError(t, err)
	_, err = p.Read()
	require.Error(t, err)
}

func BenchmarkRead(b *testing.B) {
	conn := new(MockNetConn)
	p := NewParser(conn, newBufioReader(conn), newBufioWriter(conn))

	pkb := []byte{
		byte(packets.Publish << 4), 18, // Fixed header
		0, 5, // Topic Name - LSB+MSB
		'a', '/', 'b', '/', 'c', // Topic Name
		'h', 'e', 'l', 'l', 'o', ' ', 'm', 'o', 'c', 'h', 'i', // Payload
	}

	p.R = bufio.NewReader(bytes.NewReader(pkb))
	var fh packets.FixedHeader
	err := p.ReadFixedHeader(&fh)
	if err != nil {
		panic(err)
	}

	var rn bytes.Reader = *bytes.NewReader(pkb)
	var rc bytes.Reader
	for n := 0; n < b.N; n++ {
		rc = rn
		p.R.Reset(&rc)
		p.R.Discard(2)
		_, err := p.Read()
		if err != nil {
			panic(err)
		}
	}

}

// This is a super important test. It checks whether or not subsequent packets
// mutate each other. This happens when you use a single byte buffer for decoding
// multiple packets.
func TestReadPacketNoOverwrite(t *testing.T) {
	pk1 := []byte{
		byte(packets.Publish << 4), 12, // Fixed header
		0, 5, // Topic Name - LSB+MSB
		'a', '/', 'b', '/', 'c', // Topic Name
		'h', 'e', 'l', 'l', 'o', // Payload
	}

	pk2 := []byte{
		byte(packets.Publish << 4), 14, // Fixed header
		0, 5, // Topic Name - LSB+MSB
		'x', '/', 'y', '/', 'z', // Topic Name
		'y', 'a', 'h', 'a', 'l', 'l', 'o', // Payload
	}

	r, w := net.Pipe()
	p := NewParser(r, newBufioReader(r), newBufioWriter(w))
	go func() {
		w.Write(pk1)
		w.Write(pk2)
		w.Close()
	}()

	var fh packets.FixedHeader
	err := p.ReadFixedHeader(&fh)
	require.NoError(t, err)
	o1, err := p.Read()
	require.NoError(t, err)
	require.Equal(t, []byte{'h', 'e', 'l', 'l', 'o'}, o1.(*packets.PublishPacket).Payload)
	require.Equal(t, []byte{'h', 'e', 'l', 'l', 'o'}, pk1[9:])
	require.NoError(t, err)

	err = p.ReadFixedHeader(&fh)
	require.NoError(t, err)
	o2, err := p.Read()
	require.NoError(t, err)

	require.Equal(t, []byte{'y', 'a', 'h', 'a', 'l', 'l', 'o'}, o2.(*packets.PublishPacket).Payload)
	require.Equal(t, []byte{'h', 'e', 'l', 'l', 'o'}, o1.(*packets.PublishPacket).Payload, "o1 payload was mutated")
}

func TestReadPacketNil(t *testing.T) {

	conn := new(MockNetConn)
	var fh packets.FixedHeader
	p := NewParser(conn, newBufioReader(conn), newBufioWriter(conn))

	// Check for un-specified packet.
	// Create a ping request packet with a false fixedheader type code.
	pk := &packets.PingreqPacket{FixedHeader: packets.FixedHeader{Type: packets.Pingreq}}

	pk.FixedHeader.Type = 99
	p.R = bufio.NewReader(bytes.NewReader([]byte{0, 0}))

	err := p.ReadFixedHeader(&fh)
	_, err = p.Read()

	require.Error(t, err, "Expected error reading packet")

}

func TestReadPacketReadOverflow(t *testing.T) {
	conn := new(MockNetConn)
	var fh packets.FixedHeader
	p := NewParser(conn, newBufioReader(conn), newBufioWriter(conn))

	// Check for un-specified packet.
	// Create a ping request packet with a false fixedheader type code.
	pk := &packets.PingreqPacket{FixedHeader: packets.FixedHeader{Type: packets.Pingreq}}

	pk.FixedHeader.Type = 99
	p.R = bufio.NewReader(bytes.NewReader([]byte{byte(packets.Connect << 4), 0}))

	err := p.ReadFixedHeader(&fh)

	p.FixedHeader.Remaining = 999999 // overflow buffer
	_, err = p.Read()

	require.Error(t, err, "Expected error reading packet")
}

func TestReadPacketReadAllFail(t *testing.T) {
	conn := new(MockNetConn)
	var fh packets.FixedHeader
	p := NewParser(conn, newBufioReader(conn), newBufioWriter(conn))

	// Check for un-specified packet.
	// Create a ping request packet with a false fixedheader type code.
	pk := &packets.PingreqPacket{FixedHeader: packets.FixedHeader{Type: packets.Pingreq}}

	pk.FixedHeader.Type = 99
	p.R = bufio.NewReader(bytes.NewReader([]byte{byte(packets.Connect << 4), 0}))

	err := p.ReadFixedHeader(&fh)

	p.FixedHeader.Remaining = 1 // overflow buffer
	_, err = p.Read()

	require.Error(t, err, "Expected error reading packet")
}

// MockNetConn satisfies the net.Conn interface.
type MockNetConn struct {
	ID       string
	Deadline time.Time
}

// Read reads bytes from the net io.reader.
func (m *MockNetConn) Read(b []byte) (n int, err error) {
	return 0, nil
}

// Read writes bytes to the net io.writer.
func (m *MockNetConn) Write(b []byte) (n int, err error) {
	return 0, nil
}

// Close closes the net.Conn connection.
func (m *MockNetConn) Close() error {
	return nil
}

// LocalAddr returns the local address of the request.
func (m *MockNetConn) LocalAddr() net.Addr {
	return new(MockNetAddr)
}

// RemoteAddr returns the remove address of the request.
func (m *MockNetConn) RemoteAddr() net.Addr {
	return new(MockNetAddr)
}

// SetDeadline sets the request deadline.
func (m *MockNetConn) SetDeadline(t time.Time) error {
	m.Deadline = t
	return nil
}

// SetReadDeadline sets the read deadline.
func (m *MockNetConn) SetReadDeadline(t time.Time) error {
	return nil
}

// SetWriteDeadline sets the write deadline.
func (m *MockNetConn) SetWriteDeadline(t time.Time) error {
	return nil
}

// MockNetAddr satisfies net.Addr interface.
type MockNetAddr struct{}

// Network returns the network protocol.
func (m *MockNetAddr) Network() string {
	return "tcp"
}

// String returns the network address.
func (m *MockNetAddr) String() string {
	return "127.0.0.1"
}
