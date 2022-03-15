package server

import (
	//"errors"
	"fmt"
	"io/ioutil"
	"net"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/mochi-co/mqtt/server/events"
	"github.com/mochi-co/mqtt/server/internal/circ"
	"github.com/mochi-co/mqtt/server/internal/clients"
	"github.com/mochi-co/mqtt/server/internal/packets"
	"github.com/mochi-co/mqtt/server/internal/topics"
	"github.com/mochi-co/mqtt/server/listeners"
	"github.com/mochi-co/mqtt/server/listeners/auth"
	"github.com/mochi-co/mqtt/server/persistence"
	"github.com/mochi-co/mqtt/server/system"
)

const defaultPort = ":18882"

func setupClient() (s *Server, cl *clients.Client, r net.Conn, w net.Conn) {
	s = New()
	s.Store = new(persistence.MockStore)
	r, w = net.Pipe()
	cl = clients.NewClient(w, circ.NewReader(256, 8), circ.NewWriter(256, 8), s.System)
	cl.ID = "mochi"
	cl.AC = new(auth.Allow)
	cl.Start()
	return
}

func setupServerClient(s *Server) (cl *clients.Client, r net.Conn, w net.Conn) {
	r, w = net.Pipe()
	cl = clients.NewClient(w, circ.NewReader(256, 8), circ.NewWriter(256, 8), s.System)
	cl.ID = "mochi"
	cl.AC = new(auth.Allow)
	cl.Start()
	return
}

func TestNew(t *testing.T) {
	s := New()
	require.NotNil(t, s)
	require.NotNil(t, s.Listeners)
	require.NotNil(t, s.Clients)
	require.NotNil(t, s.Topics)
	require.Nil(t, s.Store)
	require.NotEmpty(t, s.System.Version)
	require.Equal(t, true, s.System.Started > 0)
}

func BenchmarkNew(b *testing.B) {
	for n := 0; n < b.N; n++ {
		New()
	}
}

func TestServerAddStore(t *testing.T) {
	s := New()
	require.NotNil(t, s)

	p := new(persistence.MockStore)
	err := s.AddStore(p)
	require.NoError(t, err)
	require.Equal(t, p, s.Store)
}

func TestServerAddStoreFailure(t *testing.T) {
	s := New()
	require.NotNil(t, s)

	p := new(persistence.MockStore)
	p.FailOpen = true
	err := s.AddStore(p)
	require.Error(t, err)
}

func BenchmarkServerAddStore(b *testing.B) {
	s := New()
	p := new(persistence.MockStore)
	for n := 0; n < b.N; n++ {
		s.AddStore(p)
	}
}

func TestServerAddListener(t *testing.T) {
	s := New()
	require.NotNil(t, s)
	err := s.AddListener(listeners.NewMockListener("t1", defaultPort), nil)
	require.NoError(t, err)

	// Add listener with config.
	err = s.AddListener(listeners.NewMockListener("t2", defaultPort), &listeners.Config{
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

func TestServerAddListenerFailure(t *testing.T) {
	s := New()
	require.NotNil(t, s)
	m := listeners.NewMockListener("t1", ":1882")
	m.ErrListen = true
	err := s.AddListener(m, nil)
	require.Error(t, err)
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

	err = s.Serve()
	require.NoError(t, err)
	time.Sleep(time.Millisecond)
	require.Equal(t, 1, s.Listeners.Len())
	listener, ok := s.Listeners.Get("t1")
	require.Equal(t, true, ok)
	require.Equal(t, true, listener.(*listeners.MockListener).IsServing())
}

func TestServerServeFail(t *testing.T) {
	s := New()
	s.Store = new(persistence.MockStore)
	s.Store.(*persistence.MockStore).Fail = map[string]bool{
		"read_subs": true,
	}

	err := s.Serve()
	require.Error(t, err)
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

func TestServerEstablishConnectionOKCleanSession(t *testing.T) {
	s := New()

	// Existing connection with subscription.
	c, _ := net.Pipe()
	cl := clients.NewClient(c, circ.NewReader(256, 8), circ.NewWriter(256, 8), s.System)
	cl.ID = "mochi"
	cl.Subscriptions = map[string]byte{
		"a/b/c": 1,
	}
	s.Clients.Add(cl)
	s.Topics.Subscribe("a/b/c", cl.ID, 0)

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

func TestServerEventOnConnect(t *testing.T) {
	r, w := net.Pipe()
	s, cl, _, _ := setupClient()
	s.Clients.Add(cl)

	var hookedClient events.Client
	var hookedPacket events.Packet
	s.Events.OnConnect = func(cl events.Client, pk events.Packet) {
		hookedClient = cl
		hookedPacket = pk
	}

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

	time.Sleep(10 * time.Millisecond)

	require.Equal(t, events.Client{
		ID:       "mochi",
		Listener: "tcp",
	}, hookedClient)

	require.Equal(t, events.Packet(packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type:      packets.Connect,
			Remaining: 17,
		},
		ProtocolName:     []byte{'M', 'Q', 'T', 'T'},
		ProtocolVersion:  4,
		CleanSession:     true,
		Keepalive:        45,
		ClientIdentifier: "mochi",
	}), hookedPacket)

}

func TestServerEventOnDisconnect(t *testing.T) {
	r, w := net.Pipe()
	s, cl, _, _ := setupClient()
	s.Clients.Add(cl)

	var hookedClient events.Client
	var hookedErr error
	s.Events.OnDisconnect = func(cl events.Client, err error) {
		hookedClient = cl
		hookedErr = err
	}

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

	require.Equal(t, events.Client{
		ID:       "mochi",
		Listener: "tcp",
	}, hookedClient)

	require.Equal(t, nil, hookedErr)
}

func TestServerEventOnDisconnectOnError(t *testing.T) {
	r, w := net.Pipe()
	s, cl, _, _ := setupClient()
	s.Clients.Add(cl)

	var hookedClient events.Client
	var hookedErr error
	s.Events.OnDisconnect = func(cl events.Client, err error) {
		hookedClient = cl
		hookedErr = err
	}

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

		w.Write([]byte{0, 0})
	}()

	// Receive the Connack
	go func() {
		_, err := ioutil.ReadAll(w)
		if err != nil {
			panic(err)
		}
	}()

	clw, ok := s.Clients.Get("mochi")
	require.Equal(t, true, ok)
	clw.Stop()

	errx := <-o
	require.Error(t, errx)
	require.Equal(t, "No valid packet available; 0", errx.Error())

	require.Equal(t, errx, hookedErr)

	require.Equal(t, events.Client{
		ID:       "mochi",
		Listener: "tcp",
	}, hookedClient)

}

func TestServerEstablishConnectionOKInheritSession(t *testing.T) {
	s := New()

	c, _ := net.Pipe()
	cl := clients.NewClient(c, circ.NewReader(256, 8), circ.NewWriter(256, 8), s.System)
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

func TestServerEstablishConnectionReadPacketErr(t *testing.T) {
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
			194,   // Packet Flags
			0, 20, // Keepalive
			0, 5, // Client ID - MSB+LSB
			'm', 'o', 'c', 'h', 'i', // Client ID
		})
	}()

	errx := <-o
	time.Sleep(time.Millisecond)
	r.Close()
	require.Error(t, errx)
}

func TestServerOnDisconnectErr(t *testing.T) {

}

func TestServerWriteClient(t *testing.T) {
	s, cl, r, w := setupClient()
	cl.ID = "mochi"
	defer cl.Stop()

	err := s.writeClient(cl, packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type:      packets.Pubcomp,
			Remaining: 2,
		},
		PacketID: 14,
	})
	require.NoError(t, err)

	ack := make(chan []byte)
	go func() {
		buf, err := ioutil.ReadAll(r)
		if err != nil {
			panic(err)
		}
		ack <- buf
	}()
	time.Sleep(time.Millisecond)
	w.Close()

	require.Equal(t, []byte{
		byte(packets.Pubcomp << 4), 2,
		0, 14,
	}, <-ack)
}

func TestServerWriteClientError(t *testing.T) {
	s := New()
	w, _ := net.Pipe()
	cl := clients.NewClient(w, circ.NewReader(256, 8), circ.NewWriter(256, 8), s.System)
	cl.ID = "mochi"

	err := s.writeClient(cl, packets.Packet{})
	require.Error(t, err)
}

func TestServerProcessFailure(t *testing.T) {
	s, cl, _, _ := setupClient()
	err := s.processPacket(cl, packets.Packet{})
	require.Error(t, err)
}

func TestServerProcessConnect(t *testing.T) {
	s, cl, _, _ := setupClient()
	err := s.processPacket(cl, packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Connect,
		},
	})
	require.NoError(t, err)
}

func TestServerProcessDisconnect(t *testing.T) {
	s, cl, _, _ := setupClient()
	err := s.processPacket(cl, packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Disconnect,
		},
	})
	require.NoError(t, err)
}

func TestServerProcessPingreq(t *testing.T) {
	s, cl, r, w := setupClient()

	recv := make(chan []byte)
	go func() {
		buf, err := ioutil.ReadAll(r)
		if err != nil {
			panic(err)
		}
		recv <- buf
	}()

	err := s.processPacket(cl, packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Pingreq,
		},
	})

	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)
	w.Close()

	require.Equal(t, []byte{
		byte(packets.Pingresp << 4), 0,
	}, <-recv)
}

func TestServerProcessPingreqError(t *testing.T) {
	s, cl, _, _ := setupClient()

	cl.Stop()
	err := s.processPacket(cl, packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Pingreq,
		},
	})
	require.Error(t, err)
}

func TestServerProcessPublishInvalid(t *testing.T) {
	s, cl, _, _ := setupClient()

	err := s.processPacket(cl, packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Publish,
			Qos:  1,
		},
		PacketID: 0,
	})

	require.Error(t, err)
}

func TestServerProcessPublishQoS1Retain(t *testing.T) {
	s, cl1, r1, w1 := setupClient()
	cl1.ID = "mochi1"
	s.Clients.Add(cl1)

	cl2, r2, w2 := setupServerClient(s)
	cl2.ID = "mochi2"
	s.Clients.Add(cl2)

	s.Topics.Subscribe("a/b/+", cl2.ID, 0)
	s.Topics.Subscribe("a/+/c", cl2.ID, 1)

	ack1 := make(chan []byte)
	go func() {
		buf, err := ioutil.ReadAll(r1)
		if err != nil {
			panic(err)
		}
		ack1 <- buf
	}()

	ack2 := make(chan []byte)
	go func() {
		buf, err := ioutil.ReadAll(r2)
		if err != nil {
			panic(err)
		}
		ack2 <- buf
	}()

	require.Equal(t, int64(0), atomic.LoadInt64(&s.System.PublishRecv))

	err := s.processPacket(cl1, packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type:   packets.Publish,
			Qos:    1,
			Retain: true,
		},
		TopicName: "a/b/c",
		Payload:   []byte("hello"),
		PacketID:  12,
	})

	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)
	w1.Close()
	w2.Close()

	require.Equal(t, []byte{
		byte(packets.Puback << 4), 2,
		0, 12,
	}, <-ack1)

	require.Equal(t, []byte{
		byte(packets.Publish<<4 | 2 | 3), 14,
		0, 5,
		'a', '/', 'b', '/', 'c',
		0, 1,
		'h', 'e', 'l', 'l', 'o',
	}, <-ack2)

	require.Equal(t, int64(1), atomic.LoadInt64(&s.System.Retained))
}

func TestServerProcessPublishQoS2(t *testing.T) {
	s, cl1, r1, w1 := setupClient()
	s.Clients.Add(cl1)

	ack1 := make(chan []byte)
	go func() {
		buf, err := ioutil.ReadAll(r1)
		if err != nil {
			panic(err)
		}
		ack1 <- buf
	}()

	err := s.processPacket(cl1, packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Publish,
			Qos:  2,
		},
		TopicName: "a/b/c",
		Payload:   []byte("hello"),
		PacketID:  12,
	})

	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)
	w1.Close()

	require.Equal(t, []byte{
		byte(packets.Pubrec << 4), 2, // Fixed header
		0, 12, // Packet ID - LSB+MSB
	}, <-ack1)

	require.Equal(t, int64(0), atomic.LoadInt64(&s.System.Retained))
}

func TestServerProcessPublishUnretainByEmptyPayload(t *testing.T) {
	s, cl1, r1, w1 := setupClient()
	s.Clients.Add(cl1)

	ack1 := make(chan []byte)
	go func() {
		buf, err := ioutil.ReadAll(r1)
		if err != nil {
			panic(err)
		}
		ack1 <- buf
	}()

	err := s.processPacket(cl1, packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type:   packets.Publish,
			Retain: true,
		},
		TopicName: "a/b/c",
		Payload:   []byte{},
	})

	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)
	w1.Close()

	require.Equal(t, int64(0), atomic.LoadInt64(&s.System.Retained))
}

func TestServerProcessPublishOfflineQueuing(t *testing.T) {
	s, cl1, r1, w1 := setupClient()
	cl1.ID = "mochi1"
	s.Clients.Add(cl1)

	// Start and stop the receiver client
	cl2, _, _ := setupServerClient(s)
	cl2.ID = "mochi2"
	s.Clients.Add(cl2)
	s.Topics.Subscribe("qos0", cl2.ID, 0)
	s.Topics.Subscribe("qos1", cl2.ID, 1)
	s.Topics.Subscribe("qos2", cl2.ID, 2)
	cl2.Stop()

	ack1 := make(chan []byte)
	go func() {
		buf, err := ioutil.ReadAll(r1)
		if err != nil {
			panic(err)
		}
		ack1 <- buf
	}()

	for i := 0; i < 3; i++ {
		err := s.processPacket(cl1, packets.Packet{
			FixedHeader: packets.FixedHeader{
				Type: packets.Publish,
				Qos:  byte(i),
			},
			TopicName: "qos" + strconv.Itoa(i),
			Payload:   []byte("hello"),
			PacketID:  uint16(i),
		})
		require.NoError(t, err)
	}

	require.Equal(t, int64(2), atomic.LoadInt64(&s.System.Inflight))

	time.Sleep(10 * time.Millisecond)
	w1.Close()

	require.Equal(t, []byte{
		byte(packets.Puback << 4), 2, // Qos1 Ack
		0, 1,
		byte(packets.Pubrec << 4), 2, // Qos2 Ack
		0, 2,
	}, <-ack1)

	queued := cl2.Inflight.GetAll()
	require.Equal(t, 2, len(queued))
	require.Equal(t, "qos1", queued[1].Packet.TopicName)
	require.Equal(t, "qos2", queued[2].Packet.TopicName)

	// Reconnect the receiving client and get queued messages.
	r, w := net.Pipe()
	o := make(chan error)
	go func() {
		o <- s.EstablishConnection("tcp", r, new(auth.Allow))
	}()

	go func() {
		w.Write([]byte{
			byte(packets.Connect << 4), 18, // Fixed header
			0, 4, // Protocol Name - MSB+LSB
			'M', 'Q', 'T', 'T', // Protocol Name
			4,     // Protocol Version
			0,     // Packet Flags
			0, 45, // Keepalive
			0, 6, // Client ID - MSB+LSB
			'm', 'o', 'c', 'h', 'i', '2', // Client ID
		})
		w.Write([]byte{byte(packets.Disconnect << 4), 0})
	}()

	recv := make(chan []byte)
	go func() {
		buf, err := ioutil.ReadAll(w)
		if err != nil {
			panic(err)
		}
		recv <- buf
	}()

	clw, ok := s.Clients.Get("mochi2")
	require.Equal(t, true, ok)
	clw.Stop()

	errx := <-o
	require.NoError(t, errx)
	ret := <-recv

	wanted := []byte{
		byte(packets.Connack << 4), 2,
		1, packets.Accepted,
		byte(packets.Publish<<4 | 1<<1 | 1<<3), 13,
		0, 4,
		'q', 'o', 's', '1',
		0, 1,
		'h', 'e', 'l', 'l', 'o',
		byte(packets.Publish<<4 | 2<<1 | 1<<3), 13,
		0, 4,
		'q', 'o', 's', '2',
		0, 2,
		'h', 'e', 'l', 'l', 'o',
	}

	require.Equal(t, len(wanted), len(ret))
	require.Equal(t, true, (ret[4] == byte(packets.Publish<<4|1<<1|1<<3) || ret[4] == byte(packets.Publish<<4|2<<1|1<<3)))

	w.Close()

}

func TestServerProcessPublishSystemPrefix(t *testing.T) {
	s, cl, _, _ := setupClient()
	s.Clients.Add(cl)

	err := s.processPacket(cl, packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Publish,
		},
		TopicName: "$SYS/stuff",
		Payload:   []byte("hello"),
	})

	require.NoError(t, err)
	require.Equal(t, int64(0), s.System.BytesSent)
}

func TestServerProcessPublishBadACL(t *testing.T) {
	s, cl, _, _ := setupClient()
	cl.AC = new(auth.Disallow)
	s.Clients.Add(cl)

	err := s.processPacket(cl, packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Publish,
		},
		TopicName: "a/b/c",
		Payload:   []byte("hello"),
	})

	require.NoError(t, err)
}

func TestServerProcessPublishWriteAckError(t *testing.T) {
	s, cl, _, _ := setupClient()
	cl.Stop()

	err := s.processPacket(cl, packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Publish,
			Qos:  1,
		},
		TopicName: "a/b/c",
		Payload:   []byte("hello"),
	})

	require.Error(t, err)
}

func TestServerPublishInline(t *testing.T) {
	s, cl1, r1, w1 := setupClient()
	cl1.ID = "inline"
	s.Clients.Add(cl1)
	s.Topics.Subscribe("a/b/+", cl1.ID, 0)
	go s.inlineClient()

	ack1 := make(chan []byte)
	go func() {
		buf, err := ioutil.ReadAll(r1)
		if err != nil {
			panic(err)
		}
		ack1 <- buf
	}()

	err := s.Publish("a/b/c", []byte("hello"), false)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)
	w1.Close()

	require.Equal(t, []byte{
		byte(packets.Publish << 4), 12,
		0, 5,
		'a', '/', 'b', '/', 'c',
		'h', 'e', 'l', 'l', 'o',
	}, <-ack1)

	require.Equal(t, int64(14), s.System.BytesSent)

	close(s.inline.done)
}

func TestServerPublishInlineRetain(t *testing.T) {
	s, cl1, r1, w1 := setupClient()
	cl1.ID = "inline"

	ack1 := make(chan []byte)
	go func() {
		buf, err := ioutil.ReadAll(r1)
		if err != nil {
			panic(err)
		}
		ack1 <- buf
	}()

	err := s.Publish("a/b/c", []byte("hello"), true)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	s.Clients.Add(cl1)
	s.Topics.Subscribe("a/b/+", cl1.ID, 0)
	go s.inlineClient()

	time.Sleep(10 * time.Millisecond)

	w1.Close()

	require.Equal(t, []byte{
		byte(packets.Publish << 4), 12,
		0, 5,
		'a', '/', 'b', '/', 'c',
		'h', 'e', 'l', 'l', 'o',
	}, <-ack1)

	require.Equal(t, int64(14), s.System.BytesSent)

	close(s.inline.done)
}

func TestServerPublishInlineSysTopicError(t *testing.T) {
	s, _, _, _ := setupClient()

	err := s.Publish("$SYS/stuff", []byte("hello"), false)
	require.Error(t, err)
	require.Equal(t, int64(0), s.System.BytesSent)
}

func TestServerEventOnMessage(t *testing.T) {
	s, cl1, r1, w1 := setupClient()
	s.Clients.Add(cl1)
	s.Topics.Subscribe("a/b/+", cl1.ID, 0)

	var hookedPacket events.Packet
	var hookedClient events.Client
	s.Events.OnMessage = func(cl events.Client, pk events.Packet) (events.Packet, error) {
		hookedClient = cl
		hookedPacket = pk
		return pk, nil
	}

	ack1 := make(chan []byte)
	go func() {
		buf, err := ioutil.ReadAll(r1)
		if err != nil {
			panic(err)
		}
		ack1 <- buf
	}()

	pk1 := packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Publish,
		},
		TopicName: "a/b/c",
		Payload:   []byte("hello"),
	}
	err := s.processPacket(cl1, pk1)

	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	require.Equal(t, events.Client{
		ID:       "mochi",
		Listener: "",
	}, hookedClient)

	require.Equal(t, events.Packet(pk1), hookedPacket)

	w1.Close()

	require.Equal(t, []byte{
		byte(packets.Publish << 4), 12,
		0, 5,
		'a', '/', 'b', '/', 'c',
		'h', 'e', 'l', 'l', 'o',
	}, <-ack1)

	require.Equal(t, int64(14), s.System.BytesSent)
}

func TestServerProcessPublishHookOnMessageModify(t *testing.T) {
	s, cl1, r1, w1 := setupClient()
	s.Clients.Add(cl1)
	s.Topics.Subscribe("a/b/+", cl1.ID, 0)

	var hookedPacket events.Packet
	var hookedClient events.Client
	s.Events.OnMessage = func(cl events.Client, pk events.Packet) (events.Packet, error) {
		hookedPacket = pk
		hookedPacket.Payload = []byte("world")
		hookedClient = cl
		return hookedPacket, nil
	}

	ack1 := make(chan []byte)
	go func() {
		buf, err := ioutil.ReadAll(r1)
		if err != nil {
			panic(err)
		}
		ack1 <- buf
	}()

	pk1 := packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Publish,
		},
		TopicName: "a/b/c",
		Payload:   []byte("hello"),
	}
	err := s.processPacket(cl1, pk1)

	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	require.Equal(t, events.Client{
		ID:       "mochi",
		Listener: "",
	}, hookedClient)

	w1.Close()

	require.Equal(t, []byte{
		byte(packets.Publish << 4), 12,
		0, 5,
		'a', '/', 'b', '/', 'c',
		'w', 'o', 'r', 'l', 'd',
	}, <-ack1)

	require.Equal(t, int64(14), s.System.BytesSent)
}

func TestServerProcessPublishHookOnMessageModifyError(t *testing.T) {
	s, cl1, r1, w1 := setupClient()
	s.Clients.Add(cl1)
	s.Topics.Subscribe("a/b/+", cl1.ID, 0)

	s.Events.OnMessage = func(cl events.Client, pk events.Packet) (events.Packet, error) {
		pkx := pk
		pkx.Payload = []byte("world")
		return pkx, fmt.Errorf("error")
	}

	ack1 := make(chan []byte)
	go func() {
		buf, err := ioutil.ReadAll(r1)
		if err != nil {
			panic(err)
		}
		ack1 <- buf
	}()

	pk1 := packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Publish,
		},
		TopicName: "a/b/c",
		Payload:   []byte("hello"),
	}
	err := s.processPacket(cl1, pk1)

	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	w1.Close()

	require.Equal(t, []byte{
		byte(packets.Publish << 4), 12,
		0, 5,
		'a', '/', 'b', '/', 'c',
		'h', 'e', 'l', 'l', 'o',
	}, <-ack1)

	require.Equal(t, int64(14), s.System.BytesSent)
}

func TestServerProcessPublishHookOnMessageAllowClients(t *testing.T) {
	s, cl1, r1, w1 := setupClient()
	cl1.ID = "allowed"
	s.Clients.Add(cl1)
	s.Topics.Subscribe("a/b/c", cl1.ID, 0)

	cl2, r2, w2 := setupServerClient(s)
	cl2.ID = "not_allowed"
	s.Clients.Add(cl2)
	s.Topics.Subscribe("a/b/c", cl2.ID, 0)
	s.Topics.Subscribe("d/e/f", cl2.ID, 0)

	s.Events.OnMessage = func(cl events.Client, pk events.Packet) (events.Packet, error) {
		hookedPacket := pk
		if pk.TopicName == "a/b/c" {
			hookedPacket.AllowClients = []string{"allowed"}
		}
		return hookedPacket, nil
	}

	ack1 := make(chan []byte)
	go func() {
		buf, err := ioutil.ReadAll(r1)
		if err != nil {
			panic(err)
		}
		ack1 <- buf
	}()

	ack2 := make(chan []byte)
	go func() {
		buf, err := ioutil.ReadAll(r2)
		if err != nil {
			panic(err)
		}
		ack2 <- buf
	}()

	pk1 := packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Publish,
		},
		TopicName: "a/b/c",
		Payload:   []byte("hello"),
	}
	err := s.processPacket(cl1, pk1)
	require.NoError(t, err)

	pk2 := packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Publish,
		},
		TopicName: "d/e/f",
		Payload:   []byte("a"),
	}
	err = s.processPacket(cl1, pk2)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	w1.Close()
	w2.Close()

	require.Equal(t, []byte{
		byte(packets.Publish << 4), 12,
		0, 5,
		'a', '/', 'b', '/', 'c',
		'h', 'e', 'l', 'l', 'o',
	}, <-ack1)

	require.Equal(t, []byte{
		byte(packets.Publish << 4), 8,
		0, 5,
		'd', '/', 'e', '/', 'f',
		'a',
	}, <-ack2)

	require.Equal(t, int64(24), s.System.BytesSent)
}

func TestServerProcessPuback(t *testing.T) {
	s, cl, _, _ := setupClient()
	cl.Inflight.Set(11, clients.InflightMessage{Packet: packets.Packet{PacketID: 11}, Sent: 0})
	atomic.AddInt64(&s.System.Inflight, 1)

	err := s.processPacket(cl, packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type:      packets.Puback,
			Remaining: 2,
		},
		PacketID: 11,
	})
	require.NoError(t, err)
	require.Equal(t, int64(0), atomic.LoadInt64(&s.System.Inflight))

	_, ok := cl.Inflight.Get(11)
	require.Equal(t, false, ok)
}

func TestServerProcessPubrec(t *testing.T) {
	s, cl, r, w := setupClient()
	cl.Inflight.Set(12, clients.InflightMessage{Packet: packets.Packet{PacketID: 12}, Sent: 0})
	atomic.AddInt64(&s.System.Inflight, 1)

	recv := make(chan []byte)
	go func() {
		buf, err := ioutil.ReadAll(r)
		if err != nil {
			panic(err)
		}
		recv <- buf
	}()

	err := s.processPacket(cl, packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Pubrec,
		},
		PacketID: 12,
	})

	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)
	w.Close()
	require.Equal(t, int64(1), atomic.LoadInt64(&s.System.Inflight))

	require.Equal(t, []byte{
		byte(packets.Pubrel<<4) | 2, 2,
		0, 12,
	}, <-recv)

}

func TestServerProcessPubrecError(t *testing.T) {
	s, cl, _, _ := setupClient()
	cl.Stop()
	cl.Inflight.Set(12, clients.InflightMessage{Packet: packets.Packet{PacketID: 12}, Sent: 0})

	err := s.processPacket(cl, packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Pubrec,
		},
		PacketID: 12,
	})
	require.Error(t, err)
}

func TestServerProcessPubrel(t *testing.T) {
	s, cl, r, w := setupClient()
	cl.Inflight.Set(10, clients.InflightMessage{Packet: packets.Packet{PacketID: 10}, Sent: 0})
	atomic.AddInt64(&s.System.Inflight, 1)

	recv := make(chan []byte)
	go func() {
		buf, err := ioutil.ReadAll(r)
		if err != nil {
			panic(err)
		}
		recv <- buf
	}()

	err := s.processPacket(cl, packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Pubrel,
		},
		PacketID: 10,
	})

	require.NoError(t, err)
	require.Equal(t, int64(0), atomic.LoadInt64(&s.System.Inflight))
	time.Sleep(10 * time.Millisecond)
	w.Close()

	require.Equal(t, []byte{
		byte(packets.Pubcomp << 4), 2,
		0, 10,
	}, <-recv)
}

func TestServerProcessPubrelError(t *testing.T) {
	s, cl, _, _ := setupClient()
	cl.Stop()
	cl.Inflight.Set(12, clients.InflightMessage{Packet: packets.Packet{PacketID: 12}, Sent: 0})

	err := s.processPacket(cl, packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Pubrel,
		},
		PacketID: 12,
	})
	require.Error(t, err)
}

func TestServerProcessPubcomp(t *testing.T) {
	s, cl, _, _ := setupClient()
	cl.Inflight.Set(11, clients.InflightMessage{Packet: packets.Packet{PacketID: 11}, Sent: 0})
	atomic.AddInt64(&s.System.Inflight, 1)

	err := s.processPacket(cl, packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type:      packets.Pubcomp,
			Remaining: 2,
		},
		PacketID: 11,
	})
	require.NoError(t, err)
	require.Equal(t, int64(0), atomic.LoadInt64(&s.System.Inflight))

	_, ok := cl.Inflight.Get(11)
	require.Equal(t, false, ok)
}

func TestServerProcessSubscribeInvalid(t *testing.T) {
	s, cl, _, _ := setupClient()

	err := s.processPacket(cl, packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Subscribe,
			Qos:  1,
		},
		PacketID: 0,
	})

	require.Error(t, err)
}

func TestServerProcessSubscribe(t *testing.T) {
	s, cl, r, w := setupClient()

	s.Topics.RetainMessage(packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type:   packets.Publish,
			Retain: true,
		},
		TopicName: "a/b/c",
		Payload:   []byte("hello"),
	})
	require.Equal(t, 1, len(s.Topics.Messages("a/b/c")))

	recv := make(chan []byte)
	go func() {
		buf, err := ioutil.ReadAll(r)
		if err != nil {
			panic(err)
		}
		recv <- buf
	}()

	err := s.processPacket(cl, packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Subscribe,
		},
		PacketID: 10,
		Topics:   []string{"a/b/c", "d/e/f"},
		Qoss:     []byte{0, 1},
	})
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)
	w.Close()

	require.Equal(t, []byte{
		byte(packets.Suback << 4), 4, // Fixed header
		0, 10, // Packet ID - LSB+MSB
		0, // Return Code QoS 0
		1, // Return Code QoS 1

		byte(packets.Publish<<4 | 1), 12, // Fixed header
		0, 5, // Topic Name - LSB+MSB
		'a', '/', 'b', '/', 'c', // Topic Name
		'h', 'e', 'l', 'l', 'o', // Payload
	}, <-recv)

	require.Contains(t, cl.Subscriptions, "a/b/c")
	require.Contains(t, cl.Subscriptions, "d/e/f")
	require.Equal(t, byte(0), cl.Subscriptions["a/b/c"])
	require.Equal(t, byte(1), cl.Subscriptions["d/e/f"])
	require.Equal(t, topics.Subscriptions{cl.ID: 0}, s.Topics.Subscribers("a/b/c"))
	require.Equal(t, topics.Subscriptions{cl.ID: 1}, s.Topics.Subscribers("d/e/f"))
}

func TestServerProcessSubscribeFailACL(t *testing.T) {
	s, cl, r, w := setupClient()
	cl.AC = new(auth.Disallow)

	recv := make(chan []byte)
	go func() {
		buf, err := ioutil.ReadAll(r)
		if err != nil {
			panic(err)
		}
		recv <- buf
	}()

	err := s.processPacket(cl, packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Subscribe,
		},
		PacketID: 10,
		Topics:   []string{"a/b/c", "d/e/f"},
		Qoss:     []byte{0, 1},
	})

	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)
	w.Close()

	require.Equal(t, []byte{
		byte(packets.Suback << 4), 4,
		0, 10,
		packets.ErrSubAckNetworkError,
		packets.ErrSubAckNetworkError,
	}, <-recv)

	require.Empty(t, s.Topics.Subscribers("a/b/c"))
	require.Empty(t, s.Topics.Subscribers("d/e/f"))
}

func TestServerProcessSubscribeWriteError(t *testing.T) {
	s, cl, _, _ := setupClient()
	cl.Stop()

	err := s.processPacket(cl, packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Subscribe,
		},
		PacketID: 10,
		Topics:   []string{"a/b/c", "d/e/f"},
		Qoss:     []byte{0, 1},
	})

	require.Error(t, err)
}

func TestServerProcessUnsubscribeInvalid(t *testing.T) {
	s, cl, _, _ := setupClient()

	err := s.processPacket(cl, packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Unsubscribe,
			Qos:  1,
		},
		PacketID: 0,
	})

	require.Error(t, err)
}

func TestServerProcessUnsubscribe(t *testing.T) {
	s, cl, r, w := setupClient()
	s.Clients.Add(cl)
	s.Topics.Subscribe("a/b/c", cl.ID, 0)
	s.Topics.Subscribe("d/e/f", cl.ID, 1)
	s.Topics.Subscribe("a/b/+", cl.ID, 2)
	cl.NoteSubscription("a/b/c", 0)
	cl.NoteSubscription("d/e/f", 1)
	cl.NoteSubscription("a/b/+", 2)

	recv := make(chan []byte)
	go func() {
		buf, err := ioutil.ReadAll(r)
		if err != nil {
			panic(err)
		}
		recv <- buf
	}()

	err := s.processPacket(cl, packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Unsubscribe,
		},
		PacketID: 12,
		Topics:   []string{"a/b/c", "d/e/f"},
	})

	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)
	w.Close()

	require.Equal(t, []byte{
		byte(packets.Unsuback << 4), 2,
		0, 12,
	}, <-recv)

	require.NotEmpty(t, s.Topics.Subscribers("a/b/c"))
	require.Empty(t, s.Topics.Subscribers("d/e/f"))
	require.NotContains(t, cl.Subscriptions, "a/b/c")
	require.NotContains(t, cl.Subscriptions, "d/e/f")

	require.NotEmpty(t, s.Topics.Subscribers("a/b/+"))
	require.Contains(t, cl.Subscriptions, "a/b/+")
}

func TestServerProcessUnsubscribeWriteError(t *testing.T) {
	s, cl, _, _ := setupClient()
	cl.Stop()

	err := s.processPacket(cl, packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Unsubscribe,
		},
		PacketID: 12,
		Topics:   []string{"a/b/c", "d/e/f"},
	})

	require.Error(t, err)
}

func TestEventLoop(t *testing.T) {
	s := New()
	s.sysTicker = time.NewTicker(2 * time.Millisecond)

	go func() {
		s.eventLoop()
	}()
	time.Sleep(time.Millisecond * 3)
	close(s.done)

}

func TestServerClose(t *testing.T) {
	s, cl, _, _ := setupClient()
	cl.Listener = "t1"
	s.Clients.Add(cl)

	p := new(persistence.MockStore)
	err := s.AddStore(p)
	require.NoError(t, err)

	err = s.AddListener(listeners.NewMockListener("t1", ":1882"), nil)
	require.NoError(t, err)
	s.Serve()
	time.Sleep(time.Millisecond)
	require.Equal(t, 1, s.Listeners.Len())

	listener, ok := s.Listeners.Get("t1")
	require.Equal(t, true, ok)
	require.Equal(t, true, listener.(*listeners.MockListener).IsServing())

	s.Close()
	time.Sleep(time.Millisecond)
	require.Equal(t, false, listener.(*listeners.MockListener).IsServing())
	require.Equal(t, true, p.Closed)
}

func TestServerCloseClientLWT(t *testing.T) {
	s, cl1, _, _ := setupClient()
	cl1.Listener = "t1"
	cl1.LWT = clients.LWT{
		Topic:   "a/b/c",
		Message: []byte{'h', 'e', 'l', 'l', 'o'},
	}
	s.Clients.Add(cl1)

	cl2, r2, w2 := setupServerClient(s)
	cl2.ID = "mochi2"
	s.Clients.Add(cl2)

	s.Topics.Subscribe("a/b/c", cl2.ID, 0)

	ack2 := make(chan []byte)
	go func() {
		buf, err := ioutil.ReadAll(r2)
		if err != nil {
			panic(err)
		}
		ack2 <- buf
	}()

	err := s.closeClient(cl1, true)
	require.NoError(t, err)
	time.Sleep(time.Millisecond)
	w2.Close()

	require.Equal(t, []byte{
		byte(packets.Publish << 4), 12,
		0, 5,
		'a', '/', 'b', '/', 'c',
		'h', 'e', 'l', 'l', 'o',
	}, <-ack2)
}

func TestServerCloseClientClosed(t *testing.T) {
	s, cl, _, _ := setupClient()
	cl.Listener = "t1"
	cl.LWT = clients.LWT{
		Qos:     1,
		Topic:   "a/b/c",
		Message: []byte{'h', 'e', 'l', 'l', 'o'},
	}
	cl.Stop()

	err := s.closeClient(cl, true)
	require.NoError(t, err)
}

func TestServerReadStore(t *testing.T) {
	s := New()
	require.NotNil(t, s)

	s.Store = new(persistence.MockStore)
	err := s.readStore()
	require.NoError(t, err)

	require.Equal(t, int64(100), s.System.Started)
	require.Equal(t, topics.Subscriptions{"test": 1}, s.Topics.Subscribers("a/b/c"))

	cl1, ok := s.Clients.Get("client1")
	require.Equal(t, true, ok)

	msg, ok := cl1.Inflight.Get(100)
	require.Equal(t, true, ok)
	require.Equal(t, []byte{'y', 'e', 's'}, msg.Packet.Payload)

}

func TestServerReadStoreFailures(t *testing.T) {
	s := New()
	require.NotNil(t, s)

	s.Store = new(persistence.MockStore)
	s.Store.(*persistence.MockStore).Fail = map[string]bool{
		"read_subs":     true,
		"read_clients":  true,
		"read_inflight": true,
		"read_retained": true,
		"read_info":     true,
	}

	err := s.readStore()
	require.Error(t, err)
	delete(s.Store.(*persistence.MockStore).Fail, "read_info")

	err = s.readStore()
	require.Error(t, err)
	delete(s.Store.(*persistence.MockStore).Fail, "read_subs")

	err = s.readStore()
	require.Error(t, err)
	delete(s.Store.(*persistence.MockStore).Fail, "read_clients")

	err = s.readStore()
	require.Error(t, err)
	delete(s.Store.(*persistence.MockStore).Fail, "read_inflight")

	err = s.readStore()
	require.Error(t, err)
	delete(s.Store.(*persistence.MockStore).Fail, "read_retained")
}

func TestServerLoadServerInfo(t *testing.T) {
	s := New()
	require.NotNil(t, s)

	s.System.Version = "original"

	s.loadServerInfo(persistence.ServerInfo{
		Info: system.Info{
			Version: "test",
			Started: 100,
		},
		ID: persistence.KServerInfo,
	})

	require.Equal(t, "original", s.System.Version)
	require.Equal(t, int64(100), s.System.Started)
}

func TestServerLoadSubscriptions(t *testing.T) {
	s := New()
	require.NotNil(t, s)

	cl := clients.NewClientStub(s.System)
	cl.ID = "test"
	s.Clients.Add(cl)

	subs := []persistence.Subscription{
		{
			ID:     "test:a/b/c",
			Client: "test",
			Filter: "a/b/c",
			QoS:    1,
			T:      persistence.KSubscription,
		},
		{
			ID:     "test:d/e/f",
			Client: "test",
			Filter: "d/e/f",
			QoS:    0,
			T:      persistence.KSubscription,
		},
	}

	s.loadSubscriptions(subs)
	require.Equal(t, topics.Subscriptions{"test": 1}, s.Topics.Subscribers("a/b/c"))
	require.Equal(t, topics.Subscriptions{"test": 0}, s.Topics.Subscribers("d/e/f"))
}

func TestServerLoadClients(t *testing.T) {
	s := New()
	require.NotNil(t, s)

	clients := []persistence.Client{
		{
			ID:       "cl_client1",
			ClientID: "client1",
			T:        persistence.KClient,
			Listener: "tcp1",
		},
		{
			ID:       "cl_client2",
			ClientID: "client2",
			T:        persistence.KClient,
			Listener: "tcp1",
		},
	}

	s.loadClients(clients)

	cl1, ok := s.Clients.Get("client1")
	require.Equal(t, true, ok)
	require.NotNil(t, cl1)

	cl2, ok2 := s.Clients.Get("client2")
	require.Equal(t, true, ok2)
	require.NotNil(t, cl2)

}

func TestServerLoadInflight(t *testing.T) {
	s := New()
	require.NotNil(t, s)

	msgs := []persistence.Message{
		{
			ID:        "client1_if_0",
			T:         persistence.KInflight,
			Client:    "client1",
			PacketID:  0,
			TopicName: "a/b/c",
			Payload:   []byte{'h', 'e', 'l', 'l', 'o'},
			Sent:      100,
			Resends:   0,
		},
		{
			ID:        "client1_if_100",
			T:         persistence.KInflight,
			Client:    "client1",
			PacketID:  100,
			TopicName: "d/e/f",
			Payload:   []byte{'y', 'e', 's'},
			Sent:      200,
			Resends:   1,
		},
	}

	w, _ := net.Pipe()
	defer w.Close()
	c1 := clients.NewClient(w, nil, nil, nil)
	c1.ID = "client1"
	s.Clients.Add(c1)

	s.loadInflight(msgs)

	cl1, ok := s.Clients.Get("client1")
	require.Equal(t, true, ok)
	require.Equal(t, "client1", cl1.ID)

	msg, ok := cl1.Inflight.Get(100)
	require.Equal(t, true, ok)
	require.Equal(t, []byte{'y', 'e', 's'}, msg.Packet.Payload)

}

func TestServerLoadRetained(t *testing.T) {
	s := New()
	require.NotNil(t, s)

	msgs := []persistence.Message{
		{
			ID: "client1_ret_200",
			T:  persistence.KRetained,
			FixedHeader: persistence.FixedHeader{
				Retain: true,
			},
			PacketID:  200,
			TopicName: "a/b/c",
			Payload:   []byte{'h', 'e', 'l', 'l', 'o'},
			Sent:      100,
			Resends:   0,
		},
		{
			ID: "client1_ret_300",
			T:  persistence.KRetained,
			FixedHeader: persistence.FixedHeader{
				Retain: true,
			},
			PacketID:  100,
			TopicName: "d/e/f",
			Payload:   []byte{'y', 'e', 's'},
			Sent:      200,
			Resends:   1,
		},
	}

	s.loadRetained(msgs)

	require.Equal(t, 1, len(s.Topics.Messages("a/b/c")))
	require.Equal(t, 1, len(s.Topics.Messages("d/e/f")))

	msg := s.Topics.Messages("a/b/c")
	require.Equal(t, []byte{'h', 'e', 'l', 'l', 'o'}, msg[0].Payload)

	msg = s.Topics.Messages("d/e/f")
	require.Equal(t, []byte{'y', 'e', 's'}, msg[0].Payload)
}

func TestServerResendClientInflight(t *testing.T) {
	s := New()
	s.Store = new(persistence.MockStore)
	require.NotNil(t, s)

	r, w := net.Pipe()
	cl := clients.NewClient(r, circ.NewReader(128, 8), circ.NewWriter(128, 8), new(system.Info))
	cl.Start()
	s.Clients.Add(cl)

	o := make(chan []byte)
	go func() {
		buf, err := ioutil.ReadAll(w)
		require.NoError(t, err)
		o <- buf
	}()

	pk1 := packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Publish,
			Qos:  1,
		},
		TopicName: "a/b/c",
		Payload:   []byte("hello"),
		PacketID:  11,
	}

	cl.Inflight.Set(pk1.PacketID, clients.InflightMessage{
		Packet: pk1,
		Sent:   time.Now().Unix(),
	})

	err := s.ResendClientInflight(cl, true)
	require.NoError(t, err)

	time.Sleep(time.Millisecond)
	r.Close()

	rcv := <-o
	require.Equal(t, []byte{
		byte(packets.Publish<<4 | 1<<1 | 1<<3), 14,
		0, 5,
		'a', '/', 'b', '/', 'c',
		0, 11,
		'h', 'e', 'l', 'l', 'o',
	}, rcv)

	m := cl.Inflight.GetAll()
	require.Equal(t, 1, m[11].Resends) // index is packet id

}

func TestServerResendClientInflightBackoff(t *testing.T) {
	s := New()
	s.Store = new(persistence.MockStore)
	require.NotNil(t, s)

	r, w := net.Pipe()
	cl := clients.NewClient(r, circ.NewReader(128, 8), circ.NewWriter(128, 8), new(system.Info))
	cl.Start()
	s.Clients.Add(cl)

	o := make(chan []byte)
	go func() {
		buf, err := ioutil.ReadAll(w)
		require.NoError(t, err)
		o <- buf
	}()

	pk1 := packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Publish,
			Qos:  1,
		},
		TopicName: "a/b/c",
		Payload:   []byte("hello"),
		PacketID:  11,
	}

	cl.Inflight.Set(pk1.PacketID, clients.InflightMessage{
		Packet:  pk1,
		Sent:    time.Now().Unix(),
		Resends: 0,
	})

	err := s.ResendClientInflight(cl, true)
	require.NoError(t, err)

	time.Sleep(time.Millisecond)

	// Attempt to send twice, but backoff should kick in stopping second resend.
	err = s.ResendClientInflight(cl, false)
	require.NoError(t, err)

	r.Close()

	rcv := <-o
	require.Equal(t, []byte{
		byte(packets.Publish<<4 | 1<<1 | 1<<3), 14,
		0, 5,
		'a', '/', 'b', '/', 'c',
		0, 11,
		'h', 'e', 'l', 'l', 'o',
	}, rcv)

	m := cl.Inflight.GetAll()
	require.Equal(t, 1, m[11].Resends) // index is packet id
}

func TestServerResendClientInflightNoMessages(t *testing.T) {
	s := New()
	s.Store = new(persistence.MockStore)
	require.NotNil(t, s)

	r, _ := net.Pipe()
	cl := clients.NewClient(r, circ.NewReader(128, 8), circ.NewWriter(128, 8), new(system.Info))
	out := []packets.Packet{}
	err := s.ResendClientInflight(cl, true)
	require.NoError(t, err)
	require.Equal(t, 0, len(out))
	r.Close()
}

func TestServerResendClientInflightDropMessage(t *testing.T) {
	s := New()
	s.Store = new(persistence.MockStore)
	require.NotNil(t, s)

	r, _ := net.Pipe()
	cl := clients.NewClient(r, circ.NewReader(128, 8), circ.NewWriter(128, 8), new(system.Info))

	pk1 := packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type: packets.Publish,
			Qos:  1,
		},
		TopicName: "a/b/c",
		Payload:   []byte("hello"),
		PacketID:  11,
	}

	cl.Inflight.Set(pk1.PacketID, clients.InflightMessage{
		Packet:  pk1,
		Sent:    time.Now().Unix(),
		Resends: inflightMaxResends,
	})

	err := s.ResendClientInflight(cl, true)
	require.NoError(t, err)
	r.Close()

	m := cl.Inflight.GetAll()
	require.Equal(t, 0, len(m))
	require.Equal(t, int64(1), atomic.LoadInt64(&s.System.PublishDropped))
}

func TestServerResendClientInflightError(t *testing.T) {
	s := New()
	require.NotNil(t, s)

	r, _ := net.Pipe()
	cl := clients.NewClient(r, circ.NewReader(128, 8), circ.NewWriter(128, 8), new(system.Info))

	cl.Inflight.Set(1, clients.InflightMessage{
		Packet: packets.Packet{},
		Sent:   time.Now().Unix(),
	})
	r.Close()
	err := s.ResendClientInflight(cl, true)
	require.Error(t, err)
}
