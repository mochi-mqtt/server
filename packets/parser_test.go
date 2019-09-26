package packets

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mochi-co/mqtt/listeners"
)

func TestNewParser(t *testing.T) {
	conn := new(listeners.MockNetConn)
	p := NewParser(conn)

	require.NotNil(t, p.R)
}

func BenchmarkNewParser(b *testing.B) {
	conn := new(listeners.MockNetConn)

	for n := 0; n < b.N; n++ {
		NewParser(conn)
	}
}

func TestRefreshDeadline(t *testing.T) {
	conn := new(listeners.MockNetConn)
	p := NewParser(conn)

	dl := p.Conn.(*listeners.MockNetConn).Deadline
	p.RefreshDeadline(10)

	require.NotEqual(t, dl, p.Conn.(*listeners.MockNetConn).Deadline)
}

func BenchmarkRefreshDeadline(b *testing.B) {
	conn := new(listeners.MockNetConn)
	p := NewParser(conn)

	for n := 0; n < b.N; n++ {
		p.RefreshDeadline(10)
	}
}

func TestReset(t *testing.T) {
	conn := &listeners.MockNetConn{ID: "a"}
	p := NewParser(conn)

	require.Equal(t, "a", p.Conn.(*listeners.MockNetConn).ID)

	conn2 := &listeners.MockNetConn{ID: "b"}
	p.Reset(conn2)
	require.Equal(t, "b", p.Conn.(*listeners.MockNetConn).ID)
}

func BenchmarkReset(b *testing.B) {
	conn := &listeners.MockNetConn{ID: "a"}
	conn2 := &listeners.MockNetConn{ID: "b"}
	p := NewParser(conn)

	for n := 0; n < b.N; n++ {
		p.Reset(conn2)
	}
}

func TestReadFixedHeader(t *testing.T) {

	conn := new(listeners.MockNetConn)

	// Test null data.
	p := NewParser(conn)
	fh := new(FixedHeader)
	err := p.ReadFixedHeader(fh)
	require.Error(t, err)

	// Test insufficient peeking.
	fh = new(FixedHeader)
	p.R = bufio.NewReader(bytes.NewReader([]byte{Connect << 4}))
	err = p.ReadFixedHeader(fh)
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
	conn := new(listeners.MockNetConn)
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
	conn := new(listeners.MockNetConn)

	for code, pt := range expectedPackets {
		for i, wanted := range pt {
			if wanted.primary {
				var fh FixedHeader
				b := wanted.rawBytes
				p := NewParser(conn)
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

	// Fail decoder
	var fh FixedHeader
	p := NewParser(conn)
	p.R = bufio.NewReader(bytes.NewReader([]byte{
		byte(Publish << 4), 3, // Fixed header
		0, 5, // Topic Name - LSB+MSB
		'a', '/',
	}))
	err := p.ReadFixedHeader(&fh)
	require.NoError(t, err)
	_, err = p.Read()
	require.Error(t, err)
}

func BenchmarkRead(b *testing.B) {
	conn := new(listeners.MockNetConn)
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

	conn := new(listeners.MockNetConn)
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
	conn := new(listeners.MockNetConn)
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
	conn := new(listeners.MockNetConn)
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
