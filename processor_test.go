package mqtt

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/mochi-co/mqtt/packets"
	"github.com/mochi-co/mqtt/processors/circ"
)

func TestNewProcessor(t *testing.T) {
	conn := new(MockNetConn)
	p := NewProcessor(conn, circ.NewReader(16), circ.NewWriter(16))
	require.NotNil(t, p.R)
}

func BenchmarkNewProcessor(b *testing.B) {
	conn := new(MockNetConn)
	r := circ.NewReader(16)
	w := circ.NewWriter(16)

	for n := 0; n < b.N; n++ {
		NewProcessor(conn, r, w)
	}
}

func TestProcessorRefreshDeadline(t *testing.T) {
	conn := new(MockNetConn)
	p := NewProcessor(conn, circ.NewReader(16), circ.NewWriter(16))

	dl := p.Conn.(*MockNetConn).Deadline
	p.RefreshDeadline(10)

	require.NotEqual(t, dl, p.Conn.(*MockNetConn).Deadline)
}

func BenchmarkProcessorRefreshDeadline(b *testing.B) {
	conn := new(MockNetConn)
	p := NewProcessor(conn, circ.NewReader(16), circ.NewWriter(16))

	for n := 0; n < b.N; n++ {
		p.RefreshDeadline(10)
	}
}

func TestProcessorReadFixedHeader(t *testing.T) {
	conn := new(MockNetConn)

	p := NewProcessor(conn, circ.NewReader(16), circ.NewWriter(16))

	// Test null data.
	fh := new(packets.FixedHeader)
	err := p.ReadFixedHeader(fh)
	require.Error(t, err)

	// Test insufficient peeking.
	fh = new(packets.FixedHeader)
	p.R.Set([]byte{packets.Connect << 4}, 0, 1)
	p.R.SetPos(0, 1)
	err = p.ReadFixedHeader(fh)
	require.Error(t, err)

	fh = new(packets.FixedHeader)
	p.R.Set([]byte{packets.Connect << 4, 0x00}, 0, 2)
	p.R.SetPos(0, 2)
	err = p.ReadFixedHeader(fh)
	require.NoError(t, err)

	tail, head := p.R.GetPos()
	require.Equal(t, int64(2), tail)
	require.Equal(t, int64(2), head)
}

func TestProcessorRead(t *testing.T) {
	conn := new(MockNetConn)

	var fh packets.FixedHeader
	p := NewProcessor(conn, circ.NewReader(32), circ.NewWriter(32))
	p.R.Set([]byte{
		byte(packets.Publish << 4), 18, // Fixed header
		0, 5, // Topic Name - LSB+MSB
		'a', '/', 'b', '/', 'c', // Topic Name
		'h', 'e', 'l', 'l', 'o', ' ', 'm', 'o', 'c', 'h', 'i', // Payload
	}, 0, 20)
	p.R.SetPos(0, 20)

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
}

func TestProcessorReadFail(t *testing.T) {
	conn := new(MockNetConn)
	p := NewProcessor(conn, circ.NewReader(32), circ.NewWriter(32))
	p.R.Set([]byte{
		byte(packets.Publish << 4), 3, // Fixed header
		0, 5, // Topic Name - LSB+MSB
		'a', '/',
	}, 0, 6)
	p.R.SetPos(0, 8)

	var fh packets.FixedHeader
	err := p.ReadFixedHeader(&fh)
	require.NoError(t, err)
	_, err = p.Read()
	require.Error(t, err)
}

// This is a super important test. It checks whether or not subsequent packets
// mutate each other. This happens when you use a single byte buffer for decoding
// multiple packets.
func TestProcessorReadPacketNoOverwrite(t *testing.T) {
	conn := new(MockNetConn)

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

	p := NewProcessor(conn, circ.NewReader(32), circ.NewWriter(32))
	p.R.Set(pk1, 0, len(pk1))
	p.R.SetPos(0, int64(len(pk1)))
	var fh packets.FixedHeader
	err := p.ReadFixedHeader(&fh)
	require.NoError(t, err)
	o1, err := p.Read()
	require.NoError(t, err)
	require.Equal(t, []byte{'h', 'e', 'l', 'l', 'o'}, o1.(*packets.PublishPacket).Payload)
	require.Equal(t, []byte{'h', 'e', 'l', 'l', 'o'}, pk1[9:])

	p.R.Set(pk2, 0, len(pk2))
	p.R.SetPos(0, int64(len(pk2)))

	err = p.ReadFixedHeader(&fh)
	require.NoError(t, err)
	o2, err := p.Read()
	require.NoError(t, err)
	require.Equal(t, []byte{'y', 'a', 'h', 'a', 'l', 'l', 'o'}, o2.(*packets.PublishPacket).Payload)
	require.Equal(t, []byte{'h', 'e', 'l', 'l', 'o'}, o1.(*packets.PublishPacket).Payload, "o1 payload was mutated")
}

func TestProcessorReadPacketNil(t *testing.T) {

	conn := new(MockNetConn)
	p := NewProcessor(conn, circ.NewReader(32), circ.NewWriter(32))
	var fh packets.FixedHeader

	// Check for un-specified packet.
	// Create a ping request packet with a false fixedheader type code.
	pk := &packets.PingreqPacket{FixedHeader: packets.FixedHeader{Type: packets.Pingreq}}

	pk.FixedHeader.Type = 99
	p.R.Set([]byte{0, 0}, 0, 2)
	p.R.SetPos(0, 2)

	err := p.ReadFixedHeader(&fh)
	require.NoError(t, err)
	_, err = p.Read()
	require.Error(t, err)

}

func TestProcessorReadPacketReadOverflow(t *testing.T) {
	conn := new(MockNetConn)
	p := NewProcessor(conn, circ.NewReader(32), circ.NewWriter(32))
	var fh packets.FixedHeader

	// Check for un-specified packet.
	// Create a ping request packet with a false fixedheader type code.
	pk := &packets.PingreqPacket{FixedHeader: packets.FixedHeader{Type: packets.Pingreq}}

	pk.FixedHeader.Type = 99
	p.R.Set([]byte{byte(packets.Connect << 4), 0}, 0, 2)
	p.R.SetPos(0, 2)

	err := p.ReadFixedHeader(&fh)
	require.NoError(t, err)

	p.FixedHeader.Remaining = 999999 // overflow buffer
	_, err = p.Read()
	require.Error(t, err)
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
