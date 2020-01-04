package server

import (
	"io/ioutil"
	"net"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/mochi-co/mqtt/server/internal/circ"
	"github.com/mochi-co/mqtt/server/internal/clients"
	"github.com/mochi-co/mqtt/server/internal/packets"
	"github.com/mochi-co/mqtt/server/internal/topics"
	"github.com/mochi-co/mqtt/server/listeners"
	"github.com/mochi-co/mqtt/server/listeners/auth"
)

const defaultPort = ":18882"

func setupClient() (s *Server, cl *clients.Client, r net.Conn, w net.Conn) {
	s = New()
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
	require.NotEmpty(t, s.System.Version)
	require.Equal(t, true, s.System.Started > 0)
}

func BenchmarkNew(b *testing.B) {
	for n := 0; n < b.N; n++ {
		New()
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

func TestServerEstablishConnectionOKCleanSession(t *testing.T) {
	s := New()

	// Existing conneciton with subscription.
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

	_, cl2, r2, w2 := setupClient()
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

func TestServerProcessPublishOfflineQueuing(t *testing.T) {
	s, cl1, r1, w1 := setupClient()
	cl1.ID = "mochi1"
	s.Clients.Add(cl1)

	// Start and stop the receiver client
	_, cl2, _, _ := setupClient()
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

	err := s.AddListener(listeners.NewMockListener("t1", ":1882"), nil)
	require.NoError(t, err)
	s.Serve()
	time.Sleep(time.Millisecond)
	require.Equal(t, 1, s.Listeners.Len())

	listener, ok := s.Listeners.Get("t1")
	require.Equal(t, true, ok)
	require.Equal(t, true, listener.(*listeners.MockListener).IsServing)

	s.Close()
	time.Sleep(time.Millisecond)
	require.Equal(t, false, listener.(*listeners.MockListener).IsServing)
}

func TestServerCloseClientLWT(t *testing.T) {
	s, cl1, _, _ := setupClient()
	cl1.Listener = "t1"
	cl1.LWT = clients.LWT{
		Topic:   "a/b/c",
		Message: []byte{'h', 'e', 'l', 'l', 'o'},
	}
	s.Clients.Add(cl1)

	_, cl2, r2, w2 := setupClient()
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
