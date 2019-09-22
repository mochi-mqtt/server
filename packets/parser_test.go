package packets

import (
	"bufio"
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const localAddr = "127.0.0.1"

type netAddress struct {
	val []byte
}

func (n *netAddress) String() string {
	return localAddr
}

func (n *netAddress) Network() string {
	return "tcp"
}

type connTester struct {
	id       string
	deadline time.Time
}

func (c *connTester) Read(b []byte) (n int, err error) {
	return 0, nil
}

func (c *connTester) Write(b []byte) (n int, err error) {
	return 0, nil
}

func (c *connTester) Close() error {
	return nil
}

func (c *connTester) LocalAddr() net.Addr {
	return &netAddress{}
}

func (c *connTester) RemoteAddr() net.Addr {
	return &netAddress{}
}

func (c *connTester) SetDeadline(t time.Time) error {
	c.deadline = t
	return nil
}

func (c *connTester) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *connTester) SetWriteDeadline(t time.Time) error {
	return nil
}

func TestNewParser(t *testing.T) {
	conn := new(connTester)
	p := NewParser(conn)

	require.NotNil(t, p.R)
}

func BenchmarkNewParser(b *testing.B) {
	conn := new(connTester)

	for n := 0; n < b.N; n++ {
		NewParser(conn)
	}
}

func TestRefreshDeadline(t *testing.T) {
	conn := new(connTester)
	p := NewParser(conn)

	dl := p.Conn.(*connTester).deadline
	p.RefreshDeadline(10)

	require.NotEqual(t, dl, p.Conn.(*connTester).deadline)
}

func BenchmarkRefreshDeadline(b *testing.B) {
	conn := new(connTester)
	p := NewParser(conn)

	for n := 0; n < b.N; n++ {
		p.RefreshDeadline(10)
	}
}

func TestReset(t *testing.T) {
	conn := &connTester{id: "a"}
	p := NewParser(conn)

	require.Equal(t, "a", p.Conn.(*connTester).id)

	conn2 := &connTester{id: "b"}
	p.Reset(conn2)
	require.Equal(t, "b", p.Conn.(*connTester).id)
}

func BenchmarkReset(b *testing.B) {
	conn := &connTester{id: "a"}
	conn2 := &connTester{id: "b"}
	p := NewParser(conn)

	for n := 0; n < b.N; n++ {
		p.Reset(conn2)
	}
}

func TestReadFixedHeader(t *testing.T) {

	conn := new(connTester)

	// Test null data.
	fh := new(FixedHeader)
	p := NewParser(conn)
	err := p.ReadFixedHeader(fh)
	require.Error(t, err)

	// Test expected bytes.
	for i, wanted := range fixedHeaderExpected {
		fh := new(FixedHeader)
		p := NewParser(conn)
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
}

func BenchmarkReadFixedHeader(b *testing.B) {
	conn := new(connTester)
	fh := new(FixedHeader)
	p := NewParser(conn)

	var rn bytes.Reader = *bytes.NewReader(fixedHeaderExpected[0].rawBytes)
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
	conn := new(connTester)

	for code, pt := range expectedPackets {
		for i, wanted := range pt {
			if wanted.primary {
				var fh FixedHeader
				b := wanted.rawBytes
				p := NewParser(conn)
				p.R = bufio.NewReader(bytes.NewReader(b))

				err := p.ReadFixedHeader(&fh)
				if wanted.failFirst != nil {
					require.Error(t, err, "Expected error reading fixedheader [i:%d] %s - %d", i, wanted.desc, code)
				} else {
					require.NoError(t, err, "Error reading fixedheader [i:%d] %s - %d", i, wanted.desc, code)
				}

				pko, err := p.Read()
				if wanted.expect != nil {
					require.Error(t, err, "Expected error reading packet [i:%d] %s - %d", i, wanted.desc, code)
					if err != nil {
						require.Equal(t, err.Error(), wanted.expect, "Mismatched packet error [i:%d] %s - %d", i, wanted.desc, code)
					}

				} else {
					require.NoError(t, err, "Error reading packet [i:%d] %s - %d", i, wanted.desc, code)
					require.Equal(t, wanted.packet, pko, "Mismatched packet final [i:%d] %s - %d", i, wanted.desc, code)
				}
			}
		}
	}
}

func BenchmarkRead(b *testing.B) {
	conn := new(connTester)
	p := NewParser(conn)

	p.R = bufio.NewReader(bytes.NewReader(expectedPackets[Publish][1].rawBytes))
	var fh FixedHeader
	err := p.ReadFixedHeader(&fh)
	if err != nil {
		panic(err)
	}

	var rn bytes.Reader = *bytes.NewReader(expectedPackets[Publish][1].rawBytes)
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

func TestReadPacketNil(t *testing.T) {

	conn := &connTester{}
	var fh FixedHeader
	p := NewParser(conn)

	// Check for un-specified packet.
	// Create a ping request packet with a false fixedheader type code.
	pk := newPacket(Pingreq).(*PingreqPacket)

	pk.FixedHeader.Type = 99
	p.R = bufio.NewReader(bytes.NewReader([]byte{0, 0}))

	err := p.ReadFixedHeader(&fh)
	_, err = p.Read()

	require.Error(t, err, "Expected error reading packet")

}

func TestReadPacketReadOverflow(t *testing.T) {
	conn := &connTester{}
	var fh FixedHeader
	p := NewParser(conn)

	// Check for un-specified packet.
	// Create a ping request packet with a false fixedheader type code.
	pk := newPacket(Pingreq).(*PingreqPacket)

	pk.FixedHeader.Type = 99
	p.R = bufio.NewReader(bytes.NewReader([]byte{byte(Connect << 4), 0}))

	err := p.ReadFixedHeader(&fh)

	p.FixedHeader.Remaining = 999999 // overflow buffer
	_, err = p.Read()

	require.Error(t, err, "Expected error reading packet")
}

func TestReadPacketReadAllFail(t *testing.T) {
	conn := &connTester{}
	var fh FixedHeader
	p := NewParser(conn)

	// Check for un-specified packet.
	// Create a ping request packet with a false fixedheader type code.
	pk := newPacket(Pingreq).(*PingreqPacket)

	pk.FixedHeader.Type = 99
	p.R = bufio.NewReader(bytes.NewReader([]byte{byte(Connect << 4), 0}))

	err := p.ReadFixedHeader(&fh)

	p.FixedHeader.Remaining = 1 // overflow buffer
	_, err = p.Read()

	require.Error(t, err, "Expected error reading packet")
}
