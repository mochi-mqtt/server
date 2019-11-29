package mqtt

import (
	"io/ioutil"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/mochi-co/mqtt/internal/auth"
	"github.com/mochi-co/mqtt/internal/circ"
	"github.com/mochi-co/mqtt/internal/clients"
	"github.com/mochi-co/mqtt/internal/listeners"
	"github.com/mochi-co/mqtt/internal/packets"
)

/*
 * Server
 */

func TestNew(t *testing.T) {
	s := New()
	require.NotNil(t, s)
	require.NotNil(t, s.Listeners)
	require.NotNil(t, s.Clients)
	require.NotNil(t, s.Topics)
}

func BenchmarkNew(b *testing.B) {
	for n := 0; n < b.N; n++ {
		New()
	}
}

func TestServerAddListener(t *testing.T) {
	s := New()
	require.NotNil(t, s)

	err := s.AddListener(listeners.NewMockListener("t1", ":1882"), nil)
	require.NoError(t, err)

	// Add listener with config.
	err = s.AddListener(listeners.NewMockListener("t2", ":1882"), &listeners.Config{
		Auth: new(auth.Disallow),
	})
	require.NoError(t, err)
	l, ok := s.Listeners.Get("t2")
	require.Equal(t, true, ok)
	require.Equal(t, new(auth.Disallow), l.(*listeners.MockListener).Config.Auth)

	// Add listener on existing id
	err = s.AddListener(listeners.NewMockListener("t1", ":1883"), nil)
	require.Error(t, err)
	require.Equal(t, ErrListenerIDExists, err)
}

func BenchmarkServerAddListener(b *testing.B) {
	s := New()
	l := listeners.NewMockListener("t1", ":1882")
	for n := 0; n < b.N; n++ {
		err := s.AddListener(l, nil)
		if err != nil {
			panic(err)
		}
		s.Listeners.Delete("t1")
	}
}

func TestServerServe(t *testing.T) {
	s := New()
	require.NotNil(t, s)

	err := s.AddListener(listeners.NewMockListener("t1", ":1882"), nil)
	require.NoError(t, err)

	s.Serve()
	time.Sleep(time.Millisecond)
	require.Equal(t, 1, s.Listeners.Len())
	listener, ok := s.Listeners.Get("t1")
	require.Equal(t, true, ok)
	require.Equal(t, true, listener.(*listeners.MockListener).IsServing)
}

func BenchmarkServerServe(b *testing.B) {
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

/*

 * Server Establish Connection

 */
func TestServerEstablishConnectionOKCleanSession(t *testing.T) {
	s := New()

	// Existing conneciton with subscription.
	c, _ := net.Pipe()
	cl := clients.NewClient(c, circ.NewReader(256, 8), circ.NewWriter(256, 8))
	cl.ID = "mochi"
	cl.Subscriptions = map[string]byte{
		"a/b/c": 1,
	}
	s.Clients.Add(cl)

	r, w := net.Pipe()
	o := make(chan error)
	go func() {
		o <- s.EstablishConnection("tcp", r, new(auth.Allow))
	}()

	go func() {
		w.Write([]byte{
			byte(packets.Connect << 4), 17, // Fixed header
			0, 4, // Protocol Name - MSB+LSB
			'M', 'Q', 'T', 'T', // Protocol Name
			4,     // Protocol Version
			2,     // Packet Flags - clean session
			0, 45, // Keepalive
			0, 5, // Client ID - MSB+LSB
			'm', 'o', 'c', 'h', 'i', // Client ID
		})
		w.Write([]byte{byte(packets.Disconnect << 4), 0})
	}()

	// Receive the Connack
	recv := make(chan []byte)
	go func() {
		buf, err := ioutil.ReadAll(w)
		if err != nil {
			panic(err)
		}
		recv <- buf
	}()

	clw, ok := s.Clients.Get("mochi")
	require.Equal(t, true, ok)
	clw.Stop()

	errx := <-o
	require.NoError(t, errx)
	require.Equal(t, []byte{
		byte(packets.Connack << 4), 2,
		0, packets.Accepted,
	}, <-recv)
	require.Empty(t, clw.Subscriptions)

	w.Close()
}

func TestServerEstablishConnectionOKInheritSession(t *testing.T) {
	s := New()

	c, _ := net.Pipe()
	cl := clients.NewClient(c, circ.NewReader(256, 8), circ.NewWriter(256, 8))
	cl.ID = "mochi"
	cl.Subscriptions = map[string]byte{
		"a/b/c": 1,
	}
	s.Clients.Add(cl)

	r, w := net.Pipe()
	o := make(chan error)
	go func() {
		o <- s.EstablishConnection("tcp", r, new(auth.Allow))
	}()

	go func() {
		w.Write([]byte{
			byte(packets.Connect << 4), 17, // Fixed header
			0, 4, // Protocol Name - MSB+LSB
			'M', 'Q', 'T', 'T', // Protocol Name
			4,     // Protocol Version
			0,     // Packet Flags
			0, 45, // Keepalive
			0, 5, // Client ID - MSB+LSB
			'm', 'o', 'c', 'h', 'i', // Client ID
		})
		w.Write([]byte{byte(packets.Disconnect << 4), 0})
	}()

	// Receive the Connack
	recv := make(chan []byte)
	go func() {
		buf, err := ioutil.ReadAll(w)
		if err != nil {
			panic(err)
		}
		recv <- buf
	}()

	clw, ok := s.Clients.Get("mochi")
	require.Equal(t, true, ok)
	clw.Stop()

	errx := <-o
	require.NoError(t, errx)
	require.Equal(t, []byte{
		byte(packets.Connack << 4), 2,
		1, packets.Accepted,
	}, <-recv)
	require.NotEmpty(t, clw.Subscriptions)

	w.Close()
}

func TestServerEstablishConnectionBadFixedHeader(t *testing.T) {
	s := New()

	r, w := net.Pipe()
	go func() {
		w.Write([]byte{packets.Connect<<4 | 1<<1, 0x00, 0x00})
		w.Close()
	}()

	err := s.EstablishConnection("tcp", r, new(auth.Allow))

	r.Close()
	require.Error(t, err)
	require.Equal(t, packets.ErrInvalidFlags, err)
}

func TestServerEstablishConnectionInvalidPacket(t *testing.T) {
	s := New()

	r, w := net.Pipe()

	go func() {
		w.Write([]byte{0, 0})
		w.Close()
	}()

	err := s.EstablishConnection("tcp", r, new(auth.Allow))
	r.Close()

	require.Error(t, err)
}

func TestServerEstablishConnectionNotConnectPacket(t *testing.T) {
	s := New()

	r, w := net.Pipe()

	go func() {
		w.Write([]byte{byte(packets.Connack << 4), 2, 0, packets.Accepted})
		w.Close()
	}()

	err := s.EstablishConnection("tcp", r, new(auth.Allow))

	r.Close()
	require.Error(t, err)
	require.Equal(t, ErrReadConnectInvalid, err)
}

func TestServerEstablishConnectionBadAuth(t *testing.T) {
	s := New()

	r, w := net.Pipe()
	o := make(chan error)
	go func() {
		o <- s.EstablishConnection("tcp", r, new(auth.Disallow))
	}()

	go func() {
		w.Write([]byte{
			byte(packets.Connect << 4), 30, // Fixed header
			0, 4, // Protocol Name - MSB+LSB
			'M', 'Q', 'T', 'T', // Protocol Name
			4,     // Protocol Version
			194,   // Packet Flags
			0, 20, // Keepalive
			0, 5, // Client ID - MSB+LSB
			'm', 'o', 'c', 'h', 'i', // Client ID
			0, 5, // Username MSB+LSB
			'm', 'o', 'c', 'h', 'i',
			0, 4, // Password MSB+LSB
			'a', 'b', 'c', 'd',
		})
	}()

	// Receive the Connack
	recv := make(chan []byte)
	go func() {
		buf, err := ioutil.ReadAll(w)
		if err != nil {
			panic(err)
		}
		recv <- buf
	}()

	errx := <-o
	time.Sleep(time.Millisecond)
	r.Close()
	require.NoError(t, errx)
	require.Equal(t, []byte{
		byte(packets.Connack << 4), 2,
		0, packets.CodeConnectBadAuthValues,
	}, <-recv)
}

func TestServerEstablishConnectionPromptSendLWT(t *testing.T) {
	s := New()

	r, w := net.Pipe()
	o := make(chan error)
	go func() {
		o <- s.EstablishConnection("tcp", r, new(auth.Allow))
	}()

	go func() {
		w.Write([]byte{
			byte(packets.Connect << 4), 17, // Fixed header
			0, 4, // Protocol Name - MSB+LSB
			'M', 'Q', 'T', 'T', // Protocol Name
			4,     // Protocol Version
			2,     // Packet Flags - clean session
			0, 45, // Keepalive
			0, 5, // Client ID - MSB+LSB
			'm', 'o', 'c', 'h', 'i', // Client ID,
		})
		w.Write([]byte{0, 0}) // invalid packet
	}()

	// Receive the Connack
	go func() {
		_, err := ioutil.ReadAll(w)
		if err != nil {
			panic(err)
		}
	}()

	require.Error(t, <-o)
}

func TestWriteClient(t *testing.T) {
	s := New()
	r, w := net.Pipe()

	cl := clients.NewClient(w, circ.NewReader(256, 8), circ.NewWriter(256, 8))
	cl.ID = "mochi"
	cl.Start()
	defer cl.Stop()

	err := s.writeClient(cl, &packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type:      packets.Pubcomp,
			Remaining: 2,
		},
		PacketID: 14,
	})
	require.NoError(t, err)

	recv := make(chan []byte)
	go func() {
		buf, err := ioutil.ReadAll(r)
		if err != nil {
			panic(err)
		}
		recv <- buf
	}()
	time.Sleep(time.Millisecond)
	w.Close()

	require.Equal(t, []byte{
		byte(packets.Pubcomp << 4), 2, // Fixed header
		0, 14, // Packet ID - LSB+MSB
	}, <-recv)
}

func TestWriteClientError(t *testing.T) {
	s := New()
	w, _ := net.Pipe()

	cl := clients.NewClient(w, circ.NewReader(256, 8), circ.NewWriter(256, 8))
	cl.ID = "mochi"

	err := s.writeClient(cl, new(packets.Packet))
	require.Error(t, err)

}

/*

func TestResendInflight(t *testing.T) {
	s, _, _, cl := setupClient("zen")
	cl.inFlight.set(1, &inFlightMessage{
		packet: &packets.PublishPacket{
			FixedHeader: packets.FixedHeader{
				Type:   packets.Publish,
				Qos:    1,
				Retain: true,
				Dup:    true,
			},
			TopicName: "a/b/c",
			Payload:   []byte("hello"),
			PacketID:  1,
		},
		sent: time.Now().Unix(),
	})

	err := s.resendInflight(cl)
	require.NoError(t, err)
	require.Equal(t, []byte{
		byte(packets.Publish<<4 | 11), 14, // Fixed header QoS : 1
		0, 5, // Topic Name - LSB+MSB
		'a', '/', 'b', '/', 'c', // Topic Name
		0, 1, // packet id from qos=1
		'h', 'e', 'l', 'l', 'o', // Payload)
	}, cl.p.W.Get()[:16])
}

func TestResendInflightWriteError(t *testing.T) {
	s, _, _, cl := setupClient("zen")
	cl.inFlight.set(1, &inFlightMessage{
		packet: &packets.PublishPacket{},
	})

	cl.p.W.Close()
	err := s.resendInflight(cl)
	require.Error(t, err)
}
*/
