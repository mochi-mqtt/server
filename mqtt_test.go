package mqtt

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/mochi-co/mqtt/auth"
	"github.com/mochi-co/mqtt/listeners"
	"github.com/mochi-co/mqtt/packets"
)

func TestNew(t *testing.T) {
	s := New()
	require.NotNil(t, s)
	require.NotNil(t, s.listeners)
}

func BenchmarkNew(b *testing.B) {
	for n := 0; n < b.N; n++ {
		New()
	}
}

func TestAddListener(t *testing.T) {
	s := New()
	require.NotNil(t, s)

	err := s.AddListener(listeners.NewMockListener("t1", ":1882"), nil)
	require.NoError(t, err)

	// Add listener with config.
	err = s.AddListener(listeners.NewMockListener("t2", ":1882"), &listeners.Config{
		Auth: new(auth.Disallow),
	})
	require.NoError(t, err)
	l, ok := s.listeners.Get("t2")
	require.Equal(t, true, ok)
	require.Equal(t, new(auth.Disallow), l.(*listeners.MockListener).Config.Auth)

	// Add listener on existing id
	err = s.AddListener(listeners.NewMockListener("t1", ":1883"), nil)
	require.Error(t, err)
	require.Equal(t, ErrListenerIDExists, err)
}

func BenchmarkAddListener(b *testing.B) {
	s := New()
	l := listeners.NewMockListener("t1", ":1882")
	for n := 0; n < b.N; n++ {
		err := s.AddListener(l, nil)
		if err != nil {
			panic(err)
		}
		s.listeners.Delete("t1")
	}
}

func TestServe(t *testing.T) {
	s := New()
	require.NotNil(t, s)

	err := s.AddListener(listeners.NewMockListener("t1", ":1882"), nil)
	require.NoError(t, err)

	s.Serve()
	time.Sleep(time.Millisecond)
	require.Equal(t, 1, s.listeners.Len())
	listener, ok := s.listeners.Get("t1")
	require.Equal(t, true, ok)
	require.Equal(t, true, listener.(*listeners.MockListener).IsServing)
}

func BenchmarkServe(b *testing.B) {
	s := New()
	l := listeners.NewMockListener("t1", ":1882")
	err := s.AddListener(l, nil)
	if err != nil {
		panic(err)
	}
	for n := 0; n < b.N; n++ {
		s.Serve()
	}
}

func TestClose(t *testing.T) {
	s := New()
	require.NotNil(t, s)

	err := s.AddListener(listeners.NewMockListener("t1", ":1882"), nil)
	require.NoError(t, err)
	s.Serve()
	time.Sleep(time.Millisecond)
	require.Equal(t, 1, s.listeners.Len())

	listener, ok := s.listeners.Get("t1")
	require.Equal(t, true, ok)
	require.Equal(t, true, listener.(*listeners.MockListener).IsServing)

	s.Close()
	time.Sleep(time.Millisecond)
	require.Equal(t, false, listener.(*listeners.MockListener).IsServing)
}

// This is not a super accurate benchmark, but you can extrapolate the values by
// subtracting add listener and delete.
func BenchmarkClose(b *testing.B) {
	s := New()

	for n := 0; n < b.N; n++ {
		err := s.AddListener(listeners.NewMockListener("t1", ":1882"), nil)
		if err != nil {
			panic(err)
		}
		s.Close()
		s.listeners.Delete("t1")
	}
}

func TestEstablishConnection(t *testing.T) {
	s := New()

	// Error reading fixed header.
	r, w := net.Pipe()
	go func() {
		w.Write([]byte{99})
		w.Close()
	}()
	err := s.EstablishConnection(r, new(auth.Allow))
	r.Close()
	require.Error(t, err)
	require.Equal(t, ErrReadConnectFixedHeader, err)

	// Error reading connect packet.
	r, w = net.Pipe()
	go func() {
		w.Write([]byte{byte(packets.Connect << 4), 17})
		w.Close()
	}()
	err = s.EstablishConnection(r, new(auth.Allow))
	r.Close()
	require.Error(t, err)
	require.Equal(t, ErrReadConnectPacket, err)

	// Read a packet which isnt a connect packet.
	r, w = net.Pipe()
	go func() {
		w.Write([]byte{
			byte(packets.Connack << 4), 2, // fixed header
			0, // No existing session
			packets.Accepted,
		})
		w.Close()
	}()
	err = s.EstablishConnection(r, new(auth.Allow))
	r.Close()
	require.Error(t, err)
	require.Equal(t, ErrFirstPacketInvalid, err)

	// Read a non-conforming connect packet.
	r, w = net.Pipe()
	go func() {
		w.Write([]byte{
			byte(packets.Connect << 4), 13, // Fixed header
			0, 2, // Protocol Name - MSB+LSB
			'M', 'Q', // ** NON-CONFORMING Protocol Name
			4,     // Protocol Version
			2,     // Packet Flags
			0, 45, // Keepalive
			0, 3, // Client ID - MSB+LSB
			'z', 'e', 'n', // Client ID "zen"
		})
		w.Close()
	}()
	err = s.EstablishConnection(r, new(auth.Allow))
	r.Close()
	require.Error(t, err)
	require.Equal(t, ErrReadConnectInvalid, err)

	// Fail with a bad authentication details.
	r, w = net.Pipe()
	go func() {
		w.Write([]byte{
			byte(packets.Connect << 4), 28, // Fixed header
			0, 4, // Protocol Name - MSB+LSB
			'M', 'Q', 'T', 'T', // Protocol Name
			4,     // Protocol Version
			194,   // Packet Flags
			0, 20, // Keepalive
			0, 3, // Client ID - MSB+LSB
			'z', 'e', 'n', // Client ID "zen"
			0, 5, // Username MSB+LSB
			'm', 'o', 'c', 'h', 'i',
			0, 4, // Password MSB+LSB
			'a', 'b', 'c', 'd',
		})
		w.Close()
	}()
	err = s.EstablishConnection(r, new(auth.Disallow))
	r.Close()
	require.Error(t, err)
	require.Equal(t, ErrConnectNotAuthorized, err)

	// Fail creating a new client.
	// ...

}

func BenchmarkEstablishConnection(b *testing.B) {
	//	s := New()
	for n := 0; n < b.N; n++ {
		//
	}
}

func TestReadClient(t *testing.T) {
	s := New()

	// Error no connection
	r0, _ := net.Pipe()
	p := packets.NewParser(r0)
	p.Conn.Close()
	p.Conn = nil
	r0.Close()
	cl := newClient(p, new(packets.ConnectPacket))

	err := s.readClient(cl)
	require.Error(t, err)
	require.Equal(t, ErrConnectionClosed, err)

	// Fail on bad FixedHeader.
	r1, w1 := net.Pipe()
	go func() {
		w1.Write([]byte{99})
		w1.Close()
	}()
	p = packets.NewParser(r1)
	cl = newClient(p, new(packets.ConnectPacket))
	err = s.readClient(cl)
	r1.Close()
	require.Error(t, err)
	require.Equal(t, ErrReadFixedHeader, err)

	// Terminate on disconnect packet.
	r2, w2 := net.Pipe()
	go func() {
		w2.Write([]byte{packets.Disconnect << 4, 0})
		w2.Close()
	}()
	p = packets.NewParser(r2)
	cl = newClient(p, new(packets.ConnectPacket))
	err = s.readClient(cl)
	r2.Close()
	require.NoError(t, err)

	// Fail on bad packet payload.
	r3, w3 := net.Pipe()
	go func() {
		w3.Write([]byte{
			byte(packets.Publish << 4), 7, // Fixed header
			0, 5, // Topic Name - LSB+MSB
			'a', '/',
			0, 11, // Packet ID - LSB+MSB, // malformed packet id.
		})
		w3.Close()
	}()
	p = packets.NewParser(r3)
	cl = newClient(p, new(packets.ConnectPacket))
	err = s.readClient(cl)
	r3.Close()
	require.Error(t, err)
	require.Equal(t, ErrReadPacketPayload, err)

	// Fail on bad packet validation.
	r4, w4 := net.Pipe()
	go func() {
		w4.Write([]byte{
			byte(packets.Unsubscribe<<4) | 1<<1, 9, // Fixed header
			0, 0, // Packet ID - LSB+MSB
			0, 5, // Topic Name - LSB+MSB
			'a', '/', 'b', '/', 'c', // Topic Name
		})
		w4.Close()
	}()
	p = packets.NewParser(r4)
	cl = newClient(p, new(packets.ConnectPacket))
	err = s.readClient(cl)
	r4.Close()
	require.Error(t, err)
	require.Equal(t, ErrReadPacketValidation, err)

	// Good packet then break cleanly.
	r5, w5 := net.Pipe()
	p = packets.NewParser(r5)
	cl = newClient(p, new(packets.ConnectPacket))
	o := make(chan error)
	go func() {
		o <- s.readClient(cl)
	}()
	time.Sleep(time.Millisecond)
	go func() {
		b := []byte{
			byte(packets.Publish << 4), 18, // Fixed header
			0, 5, // Topic Name - LSB+MSB
			'a', '/', 'b', '/', 'c', // Topic Name
			'h', 'e', 'l', 'l', 'o', ' ', 'm', 'o', 'c', 'h', 'i', // Payload
		}
		w5.Write(b)
		close(cl.end)
	}()
	time.Sleep(time.Millisecond)
	require.NoError(t, <-o)
}
