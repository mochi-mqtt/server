package mqtt

import (
	"log"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/mochi-co/mqtt/packets"
)

func TestNewClient(t *testing.T) {
	r, _ := net.Pipe()
	p := packets.NewParser(r)
	r.Close()
	pk := &packets.ConnectPacket{
		FixedHeader: packets.FixedHeader{
			Type:      packets.Connect,
			Remaining: 16,
		},
		ProtocolName:     "MQTT",
		ProtocolVersion:  4,
		CleanSession:     true,
		Keepalive:        60,
		ClientIdentifier: "zen3",
	}

	cl := newClient(p, pk)
	require.NotNil(t, cl)
	require.Equal(t, pk.Keepalive, cl.keepalive)
	require.Equal(t, pk.CleanSession, cl.cleanSession)
	require.Equal(t, pk.ClientIdentifier, cl.id)

	// Autogenerate id.
	pk = new(packets.ConnectPacket)
	cl = newClient(p, pk)
	require.NotNil(t, cl)
	require.NotEmpty(t, cl.id)

	// Autoset keepalive
	pk = new(packets.ConnectPacket)
	cl = newClient(p, pk)
	require.NotNil(t, cl)
	require.Equal(t, defaultKeepalive, cl.keepalive)
}

func BenchmarkNewClient(b *testing.B) {
	r, _ := net.Pipe()
	p := packets.NewParser(r)
	r.Close()
	pk := new(packets.ConnectPacket)

	for n := 0; n < b.N; n++ {
		newClient(p, pk)
	}
}

func TestClientRead(t *testing.T) {

	// Error no connection
	r0, _ := net.Pipe()
	p := packets.NewParser(r0)
	p.Conn.Close()
	p.Conn = nil
	r0.Close()
	cl := newClient(p, new(packets.ConnectPacket))
	err := cl.read()
	require.Error(t, err)
	require.Equal(t, ErrConnectionClosed, err)

	// Fail on bad FixedHeader.
	r, w := net.Pipe()
	go func() {
		w.Write([]byte{99})
		w.Close()
	}()
	p = packets.NewParser(r)
	cl = newClient(p, new(packets.ConnectPacket))
	err = cl.read()
	r.Close()
	require.Error(t, err)
	require.Equal(t, ErrReadFixedHeader, err)

	// Terminate on disconnect packet.
	r1, w1 := net.Pipe()
	go func() {
		w1.Write([]byte{packets.Disconnect << 4, 0})
		w1.Close()
	}()
	p = packets.NewParser(r1)
	cl = newClient(p, new(packets.ConnectPacket))
	err = cl.read()
	r1.Close()
	require.NoError(t, err)

	// Fail on bad packet payload.
	r2, w2 := net.Pipe()
	go func() {
		w2.Write([]byte{
			byte(packets.Publish << 4), 7, // Fixed header
			0, 5, // Topic Name - LSB+MSB
			'a', '/',
			0, 11, // Packet ID - LSB+MSB, // malformed packet id.
		})
		w2.Close()
	}()
	p = packets.NewParser(r2)
	cl = newClient(p, new(packets.ConnectPacket))
	err = cl.read()
	r2.Close()
	require.Error(t, err)
	require.Equal(t, ErrReadPacketPayload, err)

	// Fail on bad packet validation.
	r3, w3 := net.Pipe()
	go func() {
		w3.Write([]byte{
			byte(packets.Unsubscribe<<4) | 1<<1, 9, // Fixed header
			0, 0, // Packet ID - LSB+MSB
			0, 5, // Topic Name - LSB+MSB
			'a', '/', 'b', '/', 'c', // Topic Name
		})
		w3.Close()
	}()
	p = packets.NewParser(r3)
	cl = newClient(p, new(packets.ConnectPacket))
	err = cl.read()
	r3.Close()
	require.Error(t, err)
	require.Equal(t, ErrReadPacketValidation, err)

	// Good packet.
	r4, w4 := net.Pipe()
	go func() {
		b := []byte{
			byte(packets.Publish << 4), 18, // Fixed header
			0, 5, // Topic Name - LSB+MSB
			'a', '/', 'b', '/', 'c', // Topic Name
			'h', 'e', 'l', 'l', 'o', ' ', 'm', 'o', 'c', 'h', 'i', // Payload
		}
		log.Println("---", b)
		w4.Write(b)
		w4.Close()
	}()
	p = packets.NewParser(r4)
	cl = newClient(p, new(packets.ConnectPacket))
	o := make(chan error)
	go func() {
		o <- cl.read()
	}()
	time.Sleep(time.Millisecond)
	close(cl.end)
	r4.Close()
	require.NoError(t, <-o)

}
