package mqtt

import (
	"testing"
	"time"

	"net"

	"github.com/stretchr/testify/require"

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

	err := s.AddListener(listeners.NewMockListener("t1", ":1882"))
	require.NoError(t, err)

	// Add listener on existing id
	err = s.AddListener(listeners.NewMockListener("t1", ":1883"))
	require.Error(t, err)
	require.Equal(t, ErrListenerIDExists, err)
}

func BenchmarkAddListener(b *testing.B) {
	s := New()
	l := listeners.NewMockListener("t1", ":1882")
	for n := 0; n < b.N; n++ {
		err := s.AddListener(l)
		if err != nil {
			panic(err)
		}
		s.listeners.Delete("t1")
	}
}

func TestServe(t *testing.T) {
	s := New()
	require.NotNil(t, s)

	err := s.AddListener(listeners.NewMockListener("t1", ":1882"))
	require.NoError(t, err)

	s.Serve()
	time.Sleep(time.Millisecond)
	require.Equal(t, 1, s.listeners.Len())
	listener, ok := s.listeners.Get("t1")
	require.Equal(t, true, ok)
	require.Equal(t, true, listener.(*listeners.MockListener).Serving())
}

func BenchmarkServe(b *testing.B) {
	s := New()
	l := listeners.NewMockListener("t1", ":1882")
	err := s.AddListener(l)
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

	err := s.AddListener(listeners.NewMockListener("t1", ":1882"))
	require.NoError(t, err)
	s.Serve()
	time.Sleep(time.Millisecond)
	require.Equal(t, 1, s.listeners.Len())

	listener, ok := s.listeners.Get("t1")
	require.Equal(t, true, ok)
	require.Equal(t, true, listener.(*listeners.MockListener).Serving())

	s.Close()
	time.Sleep(time.Millisecond)
	require.Equal(t, false, listener.(*listeners.MockListener).Serving())
}

// This is not a super accurate benchmark, but you can extrapolate the values by
// subtracting add listener and delete.
func BenchmarkClose(b *testing.B) {
	s := New()

	for n := 0; n < b.N; n++ {
		err := s.AddListener(listeners.NewMockListener("t1", ":1882"))
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
	err := s.EstablishConnection(r)
	r.Close()
	require.Error(t, err)
	require.Equal(t, ErrReadConnectFixedHeader, err)

	// Error reading connect packet.
	r, w = net.Pipe()
	go func() {
		w.Write([]byte{byte(packets.Connect << 4), 17})
		w.Close()
	}()

	err = s.EstablishConnection(r)
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

	err = s.EstablishConnection(r)
	r.Close()
	require.Error(t, err)
	require.Equal(t, ErrFirstPacketInvalid, err)

	// Read a non-conforming connect packet.
	r, w = net.Pipe()
	go func() {
		w.Write([]byte{
			byte(packets.Connect << 4), 13, // Fixed header
			0, 2, // Protocol Name - MSB+LSB
			'M', 'Q', // Protocol Name
			4,     // Protocol Version
			2,     // Packet Flags
			0, 45, // Keepalive
			0, 3, // Client ID - MSB+LSB
			'z', 'e', 'n', // Client ID "zen"
		})
		w.Close()
	}()

	err = s.EstablishConnection(r)
	r.Close()
	require.Error(t, err)
	require.Equal(t, ErrReadConnectInvalid, err)

}
