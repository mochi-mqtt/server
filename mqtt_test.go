package mqtt

import (
	"bufio"
	"errors"
	"io/ioutil"
	//"log"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/mochi-co/mqtt/auth"
	"github.com/mochi-co/mqtt/listeners"
	"github.com/mochi-co/mqtt/packets"
	"github.com/mochi-co/mqtt/topics"
)

func newBufioReader(c net.Conn) *bufio.Reader {
	return bufio.NewReaderSize(c, 512)
}

func newBufioWriter(c net.Conn) *bufio.Writer {
	return bufio.NewWriterSize(c, 512)
}

type quietWriter struct {
	b        []byte
	f        [][]byte
	writes   int
	errAfter int
}

func (q *quietWriter) Write(b []byte) (int, error) {
	q.writes++
	if len(q.b) > 0 {
		q.b = append(q.b, b...)
	} else {
		q.b = b
	}

	if q.errAfter == -1 || (q.errAfter > 0 && q.writes > q.errAfter) {
		return len(b), errors.New("error")
	}

	return len(b), nil
}

func (q *quietWriter) Flush() error {
	q.f = append(q.f, q.b)
	q.b = []byte{}
	return nil
}

func setupClient(id string) (s *Server, r net.Conn, w net.Conn, cl *client) {
	s = New()
	r, w = net.Pipe()
	cl = newClient(
		packets.NewParser(r, newBufioReader(r), newBufioWriter(w)),
		&packets.ConnectPacket{
			ClientIdentifier: id,
		},
		new(auth.Allow),
	)
	return
}

/*

 * Server

 */

func TestNew(t *testing.T) {
	s := New()
	require.NotNil(t, s)
	require.NotNil(t, s.listeners)
	require.NotNil(t, s.clients)
	//	log.Println(s)
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
	l, ok := s.listeners.Get("t2")
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
		s.listeners.Delete("t1")
	}
}

func TestServerServe(t *testing.T) {
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

func TestServerClose(t *testing.T) {
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
func BenchmarkServerClose(b *testing.B) {
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

/*

 * Server Establish Connection

 */

func TestServerEstablishConnectionOKCleanSession(t *testing.T) {
	r2, w2 := net.Pipe()
	s := New()
	s.clients.internal["zen"] = newClient(
		packets.NewParser(r2, newBufioReader(r2), newBufioWriter(w2)),
		&packets.ConnectPacket{ClientIdentifier: "zen"},
		new(auth.Allow),
	)
	s.clients.internal["zen"].subscriptions = map[string]byte{
		"a/b/c": 1,
	}

	r, w := net.Pipe()
	o := make(chan error)
	go func() {
		o <- s.EstablishConnection(r, new(auth.Allow))
	}()

	go func() {
		w.Write([]byte{
			byte(packets.Connect << 4), 15, // Fixed header
			0, 4, // Protocol Name - MSB+LSB
			'M', 'Q', 'T', 'T', // Protocol Name
			4,     // Protocol Version
			2,     // Packet Flags
			0, 45, // Keepalive
			0, 3, // Client ID - MSB+LSB
			'z', 'e', 'n', // Client ID "zen"
		})
		w.Write([]byte{byte(packets.Disconnect << 4), 0})
		r.Close()
		s.clients.internal["zen"].close()
	}()

	recv := make(chan []byte)
	go func() {
		buf, err := ioutil.ReadAll(w)
		if err != nil {
			panic(err)
		}
		recv <- buf
	}()

	require.NoError(t, <-o)
	require.Equal(t, []byte{byte(packets.Connack << 4), 2, 0, packets.Accepted}, <-recv)
	w.Close()
}

func TestServerEstablishConnectionOKInheritSession(t *testing.T) {
	r2, w2 := net.Pipe()
	s := New()
	s.clients.internal["zen"] = newClient(
		packets.NewParser(r2, newBufioReader(r2), newBufioWriter(w2)),
		&packets.ConnectPacket{ClientIdentifier: "zen"},
		new(auth.Allow),
	)
	subs := map[string]byte{
		"a/b/c": 1,
	}
	s.clients.internal["zen"].subscriptions = subs

	r, w := net.Pipe()
	o := make(chan error)
	go func() {
		o <- s.EstablishConnection(r, new(auth.Allow))
	}()

	go func() {
		w.Write([]byte{
			byte(packets.Connect << 4), 15, // Fixed header
			0, 4, // Protocol Name - MSB+LSB
			'M', 'Q', 'T', 'T', // Protocol Name
			4,     // Protocol Version
			0,     // Packet Flags
			0, 45, // Keepalive
			0, 3, // Client ID - MSB+LSB
			'z', 'e', 'n', // Client ID "zen"
		})
		w.Write([]byte{byte(packets.Disconnect << 4), 0})
		r.Close()
		s.clients.internal["zen"].close()
	}()

	recv := make(chan []byte)
	go func() {
		buf, err := ioutil.ReadAll(w)
		if err != nil {
			panic(err)
		}
		recv <- buf
	}()

	require.NoError(t, <-o)
	require.Equal(t, []byte{byte(packets.Connack << 4), 2, 0, packets.Accepted}, <-recv)
	require.Equal(t, subs, s.clients.internal["zen"].subscriptions)
	w.Close()
}

func TestServerEstablishConnectionBadFixedHeader(t *testing.T) {
	r, w := net.Pipe()

	go func() {
		w.Write([]byte{99})
		w.Close()
	}()

	s := New()
	err := s.EstablishConnection(r, new(auth.Allow))

	r.Close()
	require.Error(t, err)
	require.Equal(t, ErrReadConnectFixedHeader, err)
}

func TestServerEstablishConnectionBadConnectPacket(t *testing.T) {
	r, w := net.Pipe()

	go func() {
		w.Write([]byte{byte(packets.Connect << 4), 17})
		w.Close()
	}()

	s := New()
	err := s.EstablishConnection(r, new(auth.Allow))
	r.Close()

	require.Error(t, err)
	require.Equal(t, ErrReadConnectPacket, err)
}

func TestServerEstablishConnectionNotConnectPacket(t *testing.T) {
	r, w := net.Pipe()

	go func() {
		w.Write([]byte{
			byte(packets.Connack << 4), 2, // fixed header
			0, // No existing session
			packets.Accepted,
		})
		w.Close()
	}()

	s := New()
	err := s.EstablishConnection(r, new(auth.Allow))

	r.Close()
	require.Error(t, err)
	require.Equal(t, ErrFirstPacketInvalid, err)
}

func TestServerEstablishConnectionInvalidConnectPacket(t *testing.T) {
	r, w := net.Pipe()

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

	s := New()
	err := s.EstablishConnection(r, new(auth.Allow))
	r.Close()
	require.Error(t, err)
	require.Equal(t, ErrReadConnectInvalid, err)
}

func TestServerEstablishConnectionBadAuth(t *testing.T) {
	r, w := net.Pipe()

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

	s := New()
	err := s.EstablishConnection(r, new(auth.Disallow))

	r.Close()
	require.Error(t, err)
	require.Equal(t, ErrConnectNotAuthorized, err)
}

func TestServerEstablishConnectionWriteClientError(t *testing.T) {
	r, w := net.Pipe()

	go func() {
		w.Write([]byte{
			byte(packets.Connect << 4), 15, // Fixed header
			0, 4, // Protocol Name - MSB+LSB
			'M', 'Q', 'T', 'T', // Protocol Name
			4,     // Protocol Version
			0,     // Packet Flags
			0, 60, // Keepalive
			0, 3, // Client ID - MSB+LSB
			'z', 'e', 'n', // Client ID "zen"
		})

	}()

	o := make(chan error)
	go func() {
		s := New()
		o <- s.EstablishConnection(r, new(auth.Allow))
	}()

	time.Sleep(5 * time.Millisecond)

	w.Close()
	require.Error(t, <-o)
	r.Close()
}

func TestServerEstablishConnectionReadClientError(t *testing.T) {
	r, w := net.Pipe()

	o := make(chan error)
	go func() {
		s := New()
		o <- s.EstablishConnection(r, new(auth.Allow))
	}()

	go func() {
		w.Write([]byte{
			byte(packets.Connect << 4), 15, // Fixed header
			0, 4, // Protocol Name - MSB+LSB
			'M', 'Q', 'T', 'T', // Protocol Name
			4,     // Protocol Version
			2,     // Packet Flags
			0, 45, // Keepalive
			0, 3, // Client ID - MSB+LSB
			'z', 'e', 'n', // Client ID "zen"
		})
	}()

	go func() {
		_, err := ioutil.ReadAll(w)
		if err != nil {
			panic(err)
		}
		w.Close()
	}()

	time.Sleep(10 * time.Millisecond)
	r.Close()
	require.Error(t, <-o)
}

func TestResendInflight(t *testing.T) {
	s, _, _, cl := setupClient("zen")
	cl.p.W = new(quietWriter)

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
	}, cl.p.W.(*quietWriter).f[0])
}

func TestResendInflightWriteError(t *testing.T) {
	s, _, _, cl := setupClient("zen")
	cl.p.W = &quietWriter{errAfter: -1}
	cl.inFlight.set(1, &inFlightMessage{
		packet: &packets.PublishPacket{},
	})

	err := s.resendInflight(cl)
	require.Error(t, err)
}

/*

 * Server Read Client

 */

func TestServerReadClientOK(t *testing.T) {
	s, r, w, cl := setupClient("zen")

	go func() {
		w.Write([]byte{
			byte(packets.Publish << 4), 18, // Fixed header
			0, 5, // Topic Name - LSB+MSB
			'a', '/', 'b', '/', 'c', // Topic Name
			'h', 'e', 'l', 'l', 'o', ' ', 'm', 'o', 'c', 'h', 'i', // Payload
		})
		close(cl.end)
	}()

	time.Sleep(10 * time.Millisecond)

	o := make(chan error)
	go func() {
		o <- s.readClient(cl)
	}()

	require.NoError(t, <-o)
	w.Close()
	r.Close()
}

func TestServerReadClientNoConn(t *testing.T) {
	s, r, _, cl := setupClient("zen")
	cl.p.Conn.Close()
	cl.p.Conn = nil

	r.Close()
	err := s.readClient(cl)
	require.Error(t, err)
	require.Equal(t, ErrConnectionClosed, err)
}

func TestServerReadClientBadFixedHeader(t *testing.T) {
	s, r, w, cl := setupClient("zen")

	go func() {
		w.Write([]byte{99})
		w.Close()
	}()

	err := s.readClient(cl)
	r.Close()
	require.Error(t, err)
	require.Equal(t, ErrReadFixedHeader, err)
}

func TestServerReadClientDisconnect(t *testing.T) {
	s, r, w, cl := setupClient("zen")

	go func() {
		w.Write([]byte{packets.Disconnect << 4, 0})
		w.Close()
	}()

	err := s.readClient(cl)
	r.Close()
	require.NoError(t, err)
}

func TestServerReadClientBadPacketPayload(t *testing.T) {
	s, r, w, cl := setupClient("zen")

	go func() {
		w.Write([]byte{
			byte(packets.Publish << 4), 7, // Fixed header
			0, 5, // Topic Name - LSB+MSB
			'a', '/',
			0, 11, // Packet ID - LSB+MSB, // malformed packet id.
		})
		w.Close()
	}()

	err := s.readClient(cl)
	r.Close()
	require.Error(t, err)
	require.Equal(t, ErrReadPacketPayload, err)
}

func TestServerReadClientBadPacketValidation(t *testing.T) {
	s, r, w, cl := setupClient("zen")

	go func() {
		w.Write([]byte{
			byte(packets.Unsubscribe<<4) | 1<<1, 9, // Fixed header
			0, 0, // Packet ID - LSB+MSB
			0, 5, // Topic Name - LSB+MSB
			'a', '/', 'b', '/', 'c', // Topic Name
		})
		w.Close()
	}()

	err := s.readClient(cl)
	r.Close()
	require.Error(t, err)
	require.Equal(t, ErrReadPacketValidation, err)
}

/*

 * Server Write Client

 */

func TestServerWriteClient(t *testing.T) {
	s, _, _, cl := setupClient("zen")
	cl.p.W = new(quietWriter)
	s.clients.add(cl)

	err := s.writeClient(cl, &packets.PublishPacket{
		FixedHeader: packets.FixedHeader{
			Type:      packets.Publish,
			Remaining: 18,
		},
		TopicName: "a/b/c",
		Payload:   []byte("hello mochi"),
	})
	require.NoError(t, err)
	require.Equal(t, []byte{byte(packets.Publish << 4), 18, // Fixed header
		0, 5, // Topic Name - LSB+MSB
		'a', '/', 'b', '/', 'c', // Topic Name
		'h', 'e', 'l', 'l', 'o', ' ', 'm', 'o', 'c', 'h', 'i',
	}, cl.p.W.(*quietWriter).f[0])
}

func TestServerWriteClientBadEncode(t *testing.T) {
	s, _, _, cl := setupClient("zen")
	s.clients.add(cl)

	err := s.writeClient(cl, &packets.PublishPacket{
		FixedHeader: packets.FixedHeader{
			Qos: 1,
		},
		PacketID: 0,
	})
	require.Error(t, err)
}

func TestServerWriteClientNilFlush(t *testing.T) {
	s, _, _, cl := setupClient("zen")
	s.clients.add(cl)

	err := s.writeClient(cl, &packets.PublishPacket{
		FixedHeader: packets.FixedHeader{
			Qos: 1,
		},
		PacketID: 0,
	})

	require.Error(t, err)
}

func TestServerWriteClientNilWriter(t *testing.T) {
	s, _, _, cl := setupClient("zen")
	s.clients.add(cl)
	cl.p.W = nil

	err := s.writeClient(cl, &packets.PublishPacket{})
	require.Error(t, err)
	require.Equal(t, ErrConnectionClosed, err)
}

func TestServerWriteClientWriteError(t *testing.T) {
	s, _, _, cl := setupClient("zen")
	cl.p.W = &quietWriter{errAfter: -1}
	s.clients.add(cl)

	err := s.writeClient(cl, &packets.PublishPacket{})
	require.Error(t, err)
}

/*

 * Server Close Client

 */

func TestServerCloseClient(t *testing.T) { // as opposed to client.close
	s, _, _, cl := setupClient("zen")
	s.clients.add(cl)

	require.Contains(t, s.clients.internal, "zen")

	// close the client connection.
	err := s.closeClient(s.clients.internal["zen"], true)
	require.NoError(t, err)
	var ok bool
	select {
	case _, ok = <-s.clients.internal["zen"].end:
	}
	require.Equal(t, false, ok)
	require.Nil(t, s.clients.internal["zen"].p.Conn)
}

func TestServerCloseClientLWT(t *testing.T) { // as opposed to client.close
	s, _, _, c1 := setupClient("zen")
	c1.p.W = new(quietWriter)
	s.clients.add(c1)
	require.Contains(t, s.clients.internal, "zen")
	c1.lwt = lwt{
		topic:   "a/b/c",
		message: []byte{'h', 'e', 'l', 'l', 'o'},
	}

	_, _, _, c2 := setupClient("zen2")
	c2.p.W = new(quietWriter)
	s.clients.add(c2)
	require.Contains(t, s.clients.internal, "zen2")
	s.topics.Subscribe("a/b/c", c2.id, 0)

	// close the client connection.
	err := s.closeClient(s.clients.internal["zen"], true)
	require.NoError(t, err)

	require.Equal(t, []byte{
		byte(packets.Publish << 4), 12, // Fixed header QoS : 1
		0, 5, // Topic Name - LSB+MSB
		'a', '/', 'b', '/', 'c', // Topic Name
		'h', 'e', 'l', 'l', 'o', // Payload
	}, c2.p.W.(*quietWriter).f[0])
}

func TestServerCloseClientLWTWriteError(t *testing.T) { // as opposed to client.close
	s, _, _, c1 := setupClient("zen")
	c1.p.W = new(quietWriter)
	s.clients.add(c1)
	require.Contains(t, s.clients.internal, "zen")
	c1.lwt = lwt{
		topic:   "a/b/c",
		message: []byte{'h', 'e', 'l', 'l', 'o'},
	}

	_, _, _, c2 := setupClient("zen2")
	c2.p.W = &quietWriter{errAfter: -1}
	s.clients.add(c2)
	require.Contains(t, s.clients.internal, "zen2")
	s.topics.Subscribe("a/b/c", c2.id, 0)

	// close the client connection.
	err := s.closeClient(s.clients.internal["zen"], true)
	require.Error(t, err)
}

/*

 * Server Process Packets

 */

func TestServerProcessPacket(t *testing.T) {
	s, _, _, cl := setupClient("zen")
	err := s.processPacket(cl, nil)
	require.NoError(t, err)
}

func BenchmarkServerProcessPacket(b *testing.B) {
	s, _, _, cl := setupClient("zen")
	for n := 0; n < b.N; n++ {
		s.processPacket(cl, nil)
	}
}

func TestServerProcessConnect(t *testing.T) {
	s, _, _, cl := setupClient("zen")
	cl.p.W = new(quietWriter)

	err := s.processPacket(cl, &packets.ConnectPacket{
		FixedHeader: packets.FixedHeader{
			Type: packets.Connect,
		},
	})

	require.NoError(t, err)
	var ok bool
	select {
	case _, ok = <-cl.end:
	}
	require.Equal(t, false, ok)
}

func BenchmarkServerProcessConnect(b *testing.B) {
	s, _, _, cl := setupClient("zen")
	pk := &packets.ConnectPacket{
		FixedHeader: packets.FixedHeader{
			Type: packets.Connect,
		},
	}
	for n := 0; n < b.N; n++ {
		s.processConnect(cl, pk)
	}
}

func TestServerProcessDisconnect(t *testing.T) {
	s, _, _, cl := setupClient("zen")
	cl.p.W = new(quietWriter)

	err := s.processPacket(cl, &packets.DisconnectPacket{
		FixedHeader: packets.FixedHeader{
			Type: packets.Disconnect,
		},
	})

	require.NoError(t, err)
	var ok bool
	select {
	case _, ok = <-cl.end:
	}
	require.Equal(t, false, ok)
}

func BenchmarkServerProcessDisconnect(b *testing.B) {
	s, _, _, cl := setupClient("zen")
	cl.p.W = new(quietWriter)

	pk := &packets.DisconnectPacket{
		FixedHeader: packets.FixedHeader{
			Type: packets.Disconnect,
		},
	}
	for n := 0; n < b.N; n++ {
		s.processDisconnect(cl, pk)
	}
}

func TestServerProcessPingreq(t *testing.T) {
	s, _, _, cl := setupClient("zen")
	cl.p.W = new(quietWriter)

	err := s.processPacket(cl, &packets.PingreqPacket{
		FixedHeader: packets.FixedHeader{
			Type: packets.Pingreq,
		},
	})
	require.NoError(t, err)
	require.Equal(t, []byte{
		byte(packets.Pingresp << 4), 0,
	}, cl.p.W.(*quietWriter).f[0])
}

func BenchmarkServerProcessPingreq(b *testing.B) {
	s, _, _, cl := setupClient("zen")
	cl.p.W = &quietWriter{}

	pk := &packets.PingreqPacket{
		FixedHeader: packets.FixedHeader{
			Type: packets.Pingreq,
		},
	}
	for n := 0; n < b.N; n++ {
		s.processPingreq(cl, pk)
	}
}

func TestServerProcessPingreqWriteError(t *testing.T) {
	s, _, _, cl := setupClient("zen")
	cl.p.W = &quietWriter{errAfter: -1}

	err := s.processPacket(cl, &packets.PingreqPacket{
		FixedHeader: packets.FixedHeader{
			Type: packets.Pingreq,
		},
	})
	require.Error(t, err)
}

func TestServerProcessPublishQOS1(t *testing.T) {
	s, _, _, c1 := setupClient("c1")
	_, _, _, c2 := setupClient("c2")
	c1.p.W = new(quietWriter)
	c2.p.W = new(quietWriter)

	s.clients.add(c1)
	s.clients.add(c2)
	s.topics.Subscribe("a/b/+", c2.id, 0)
	s.topics.Subscribe("a/+/c", c2.id, 1)
	require.Nil(t, c1.inFlight.internal[1])

	err := s.processPacket(c1, &packets.PublishPacket{
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
	require.Equal(t, []byte{
		byte(packets.Puback << 4), 2, // Fixed header
		0, 12, // Packet ID - LSB+MSB
	}, c1.p.W.(*quietWriter).f[0])

	require.Equal(t, []byte{
		byte(packets.Publish<<4 | 2), 14, // Fixed header QoS : 1
		0, 5, // Topic Name - LSB+MSB
		'a', '/', 'b', '/', 'c', // Topic Name
		0, 1, // packet id from qos=1
		'h', 'e', 'l', 'l', 'o', // Payload
	}, c2.p.W.(*quietWriter).f[0])
}

func BenchmarkServerProcessPublish(b *testing.B) {
	s, _, _, cl := setupClient("zen")
	cl.p.W = &quietWriter{}

	pk := &packets.PublishPacket{
		FixedHeader: packets.FixedHeader{
			Type: packets.Publish,
		},
		TopicName: "a/b/c",
		Payload:   []byte("hello"),
	}
	for n := 0; n < b.N; n++ {
		err := s.processPublish(cl, pk)
		if err != nil {
			panic(err)
		}
	}
}

func TestServerProcessPublishQOS1WriteError(t *testing.T) {
	s, _, _, cl := setupClient("zen")
	cl.p.W = &quietWriter{errAfter: -1}
	err := s.processPacket(cl, &packets.PublishPacket{
		FixedHeader: packets.FixedHeader{
			Type: packets.Publish,
			Qos:  1,
		},
		TopicName: "a/b/c",
		Payload:   []byte("hello"),
		PacketID:  12,
	})

	require.Error(t, err)
}

func TestServerProcessPublishWritePubsError(t *testing.T) {
	s, _, _, c1 := setupClient("c1")
	_, _, _, c2 := setupClient("c2")
	c2.p.W = &quietWriter{errAfter: -1}

	s.clients.add(c1)
	s.clients.add(c2)
	s.topics.Subscribe("a/b/+", c2.id, 0)
	s.topics.Subscribe("a/+/c", c2.id, 1)
	require.Nil(t, c1.inFlight.internal[1])

	err := s.processPacket(c1, &packets.PublishPacket{
		FixedHeader: packets.FixedHeader{
			Type: packets.Publish,
		},
		TopicName: "a/b/c",
		Payload:   []byte("hello2"),
	})
	require.Error(t, err)

}
func TestServerProcessPublishQOS2(t *testing.T) {
	s, _, _, c1 := setupClient("c1")
	_, _, _, c2 := setupClient("c2")
	c1.p.W = new(quietWriter)
	c2.p.W = new(quietWriter)

	err := s.processPacket(c1, &packets.PublishPacket{
		FixedHeader: packets.FixedHeader{
			Type:   packets.Publish,
			Qos:    2,
			Retain: true,
		},
		TopicName: "a/b/c",
		Payload:   []byte("hello"),
		PacketID:  12,
	})
	require.NoError(t, err)
	require.Equal(t, []byte{
		byte(packets.Pubrec << 4), 2, // Fixed header
		0, 12, // Packet ID - LSB+MSB
	}, c1.p.W.(*quietWriter).f[0])

}

func TestServerProcessPublishQOS2WriteError(t *testing.T) {
	s, _, _, cl := setupClient("zen")
	cl.p.W = &quietWriter{errAfter: -1}
	err := s.processPacket(cl, &packets.PublishPacket{
		FixedHeader: packets.FixedHeader{
			Type: packets.Publish,
			Qos:  2,
		},
		TopicName: "a/b/c",
		Payload:   []byte("hello"),
		PacketID:  12,
	})

	require.Error(t, err)
}

func TestServerProcessPublishDisallowACL(t *testing.T) {
	s, _, _, cl := setupClient("c1")
	cl.p.W = new(quietWriter)
	cl.ac = new(auth.Disallow)

	err := s.processPacket(cl, &packets.PublishPacket{
		FixedHeader: packets.FixedHeader{
			Type: packets.Publish,
		},
		TopicName: "a/b/c",
		Payload:   []byte("hello"),
	})

	require.Error(t, err)
	require.Equal(t, ErrACLNotAuthorized, err)
}

func TestServerProcessPuback(t *testing.T) {
	s, _, _, cl := setupClient("zen")

	cl.inFlight.set(11, &inFlightMessage{packet: &packets.PublishPacket{PacketID: 11}, sent: 0})
	require.NotNil(t, cl.inFlight.internal[11])

	err := s.processPacket(cl, &packets.PubackPacket{
		FixedHeader: packets.FixedHeader{
			Type:      packets.Puback,
			Remaining: 2,
		},
		PacketID: 11,
	})
	require.NoError(t, err)
	require.Nil(t, cl.inFlight.internal[11])
}

func TestServerProcessPubrec(t *testing.T) {
	s, _, _, cl := setupClient("zen")
	cl.p.W = new(quietWriter)

	cl.inFlight.set(12, &inFlightMessage{packet: &packets.PublishPacket{PacketID: 12}, sent: 0})

	err := s.processPacket(cl, &packets.PubrecPacket{
		FixedHeader: packets.FixedHeader{
			Type: packets.Pubrec,
		},
		PacketID: 12,
	})
	require.NoError(t, err)
	require.Equal(t, []byte{
		byte(packets.Pubrel<<4) | 2, 2, // Fixed header
		0, 12, // Packet ID - LSB+MSB
	}, cl.p.W.(*quietWriter).f[0])
}

func BenchmarkServerProcessPubrec(b *testing.B) {
	s, _, _, cl := setupClient("zen")
	cl.p.W = new(quietWriter)

	pk := &packets.PubrecPacket{
		FixedHeader: packets.FixedHeader{
			Type: packets.Pubrec,
		},
		PacketID: 12,
	}
	for n := 0; n < b.N; n++ {
		err := s.processPubrec(cl, pk)
		if err != nil {
			panic(err)
		}
	}
}

func TestServerProcessPubrecWriteError(t *testing.T) {
	s, _, _, cl := setupClient("zen")
	cl.p.W = &quietWriter{errAfter: -1}

	cl.inFlight.set(12, &inFlightMessage{packet: &packets.PublishPacket{PacketID: 12}, sent: 0})

	err := s.processPacket(cl, &packets.PubrecPacket{
		FixedHeader: packets.FixedHeader{
			Type: packets.Pubrec,
		},
		PacketID: 12,
	})
	require.Error(t, err)
}

func TestServerProcessPubrel(t *testing.T) {
	s, _, _, cl := setupClient("zen")
	cl.p.W = new(quietWriter)

	cl.inFlight.set(10, &inFlightMessage{packet: &packets.PublishPacket{PacketID: 10}, sent: 0})

	err := s.processPacket(cl, &packets.PubrelPacket{
		FixedHeader: packets.FixedHeader{
			Type: packets.Pubrel,
		},
		PacketID: 10,
	})
	require.NoError(t, err)
	require.Equal(t, []byte{
		byte(packets.Pubcomp << 4), 2, // Fixed header
		0, 10, // Packet ID - LSB+MSB
	}, cl.p.W.(*quietWriter).f[0])
}

func BenchmarkServerProcessPubrel(b *testing.B) {
	s, _, _, cl := setupClient("zen")
	cl.p.W = new(quietWriter)

	cl.inFlight.set(10, &inFlightMessage{packet: &packets.PublishPacket{PacketID: 10}, sent: 0})

	pk := &packets.PubrelPacket{
		FixedHeader: packets.FixedHeader{
			Type: packets.Pubrel,
		},
		PacketID: 10,
	}

	for n := 0; n < b.N; n++ {
		err := s.processPubrel(cl, pk)
		if err != nil {
			panic(err)
		}
	}
}

func TestServerProcessPubrelWriteError(t *testing.T) {
	s, _, _, cl := setupClient("zen")
	cl.p.W = &quietWriter{errAfter: -1}

	cl.inFlight.set(10, &inFlightMessage{packet: &packets.PublishPacket{PacketID: 10}, sent: 0})

	err := s.processPacket(cl, &packets.PubrelPacket{
		FixedHeader: packets.FixedHeader{
			Type: packets.Pubrel,
		},
		PacketID: 10,
	})
	require.Error(t, err)
}

func TestServerProcessPubcomp(t *testing.T) {
	s, _, _, cl := setupClient("zen")
	cl.p.W = new(quietWriter)

	cl.inFlight.set(11, &inFlightMessage{packet: &packets.PublishPacket{PacketID: 11}, sent: 0})
	require.NotNil(t, cl.inFlight.internal[11])

	err := s.processPacket(cl, &packets.PubcompPacket{
		FixedHeader: packets.FixedHeader{
			Type:      packets.Pubcomp,
			Remaining: 2,
		},
		PacketID: 11,
	})
	require.NoError(t, err)
	require.Nil(t, cl.inFlight.internal[11])

}

func BenchmarkServerProcessPubcomp(b *testing.B) {
	s, _, _, cl := setupClient("zen")
	cl.p.W = new(quietWriter)

	cl.inFlight.set(11, &inFlightMessage{packet: &packets.PublishPacket{PacketID: 11}, sent: 0})

	pk := &packets.PubcompPacket{
		FixedHeader: packets.FixedHeader{
			Type:      packets.Pubcomp,
			Remaining: 2,
		},
		PacketID: 11,
	}
	for n := 0; n < b.N; n++ {
		err := s.processPubcomp(cl, pk)
		if err != nil {
			panic(err)
		}
	}
}

func TestServerProcessSubscribe(t *testing.T) {
	s, _, _, cl := setupClient("zen")
	cl.p.W = new(quietWriter)

	s.topics.RetainMessage(&packets.PublishPacket{
		FixedHeader: packets.FixedHeader{
			Type:   packets.Publish,
			Retain: true,
		},
		TopicName: "a/b/c",
		Payload:   []byte("hello"),
	})
	require.Equal(t, 1, len(s.topics.Messages("a/b/c")))

	err := s.processPacket(cl, &packets.SubscribePacket{
		FixedHeader: packets.FixedHeader{
			Type: packets.Subscribe,
		},
		PacketID: 10,
		Topics:   []string{"a/b/c", "d/e/f"},
		Qoss:     []byte{0, 1},
	})
	require.NoError(t, err)
	require.Equal(t, []byte{
		byte(packets.Suback << 4), 4, // Fixed header
		0, 10, // Packet ID - LSB+MSB
		0, // Return Code QoS 0
		1, // Return Code QoS 1
	}, cl.p.W.(*quietWriter).f[0])

	require.Equal(t, []byte{
		byte(packets.Publish<<4 | 1), 12, // Fixed header
		0, 5, // Topic Name - LSB+MSB
		'a', '/', 'b', '/', 'c', // Topic Name
		'h', 'e', 'l', 'l', 'o', // Payload
	}, cl.p.W.(*quietWriter).f[1])

	require.Contains(t, cl.subscriptions, "a/b/c")
	require.Contains(t, cl.subscriptions, "d/e/f")
	require.Equal(t, byte(0), cl.subscriptions["a/b/c"])
	require.Equal(t, byte(1), cl.subscriptions["d/e/f"])
	require.Equal(t, topics.Subscriptions{cl.id: 0}, s.topics.Subscribers("a/b/c"))
	require.Equal(t, topics.Subscriptions{cl.id: 1}, s.topics.Subscribers("d/e/f"))
}

func BenchmarkServerProcessSubscribe(b *testing.B) {
	s, _, _, cl := setupClient("zen")
	cl.p.W = new(quietWriter)

	pk := &packets.SubscribePacket{
		FixedHeader: packets.FixedHeader{
			Type: packets.Subscribe,
		},
		PacketID: 10,
		Topics:   []string{"a/b/c"},
		Qoss:     []byte{0},
	}
	for n := 0; n < b.N; n++ {
		err := s.processSubscribe(cl, pk)
		if err != nil {
			panic(err)
		}
	}
}

func TestServerProcessSubscribeFailACL(t *testing.T) {
	s, _, _, cl := setupClient("zen")
	cl.p.W = new(quietWriter)
	cl.ac = new(auth.Disallow)

	err := s.processPacket(cl, &packets.SubscribePacket{
		FixedHeader: packets.FixedHeader{
			Type: packets.Subscribe,
		},
		PacketID: 10,
		Topics:   []string{"a/b/c", "d/e/f"},
		Qoss:     []byte{0, 1},
	})
	require.NoError(t, err)
	require.Equal(t, []byte{
		byte(packets.Suback << 4), 4, // Fixed header
		0, 10, // Packet ID - LSB+MSB
		packets.ErrSubAckNetworkError,
		packets.ErrSubAckNetworkError,
	}, cl.p.W.(*quietWriter).f[0])

	require.Empty(t, s.topics.Subscribers("a/b/c"))
	require.Empty(t, s.topics.Subscribers("d/e/f"))
}

func TestServerProcessSubscribeWriteError(t *testing.T) {
	s, _, _, cl := setupClient("zen")
	cl.p.W = &quietWriter{errAfter: -1}

	err := s.processPacket(cl, &packets.SubscribePacket{
		FixedHeader: packets.FixedHeader{
			Type: packets.Subscribe,
		},
		PacketID: 10,
		Topics:   []string{"a/b/c", "d/e/f"},
		Qoss:     []byte{0, 1},
	})
	require.Error(t, err)
}

func TestServerProcessSubscribeWriteRetainedError(t *testing.T) {
	s, _, _, cl := setupClient("zen")
	cl.p.W = &quietWriter{errAfter: 1}

	s.topics.RetainMessage(&packets.PublishPacket{
		FixedHeader: packets.FixedHeader{
			Type:   packets.Publish,
			Retain: true,
		},
		TopicName: "a/b/c",
		Payload:   []byte("hello"),
	})
	require.Equal(t, 1, len(s.topics.Messages("a/b/c")))

	err := s.processPacket(cl, &packets.SubscribePacket{
		FixedHeader: packets.FixedHeader{
			Type: packets.Subscribe,
		},
		PacketID: 10,
		Topics:   []string{"a/b/c", "d/e/f"},
		Qoss:     []byte{0, 1},
	})
	require.Error(t, err)
}

func TestServerProcessUnsubscribe(t *testing.T) {
	s, _, _, cl := setupClient("zen")
	cl.p.W = new(quietWriter)

	s.clients.add(cl)
	s.topics.Subscribe("a/b/c", cl.id, 0)
	s.topics.Subscribe("d/e/f", cl.id, 1)
	cl.noteSubscription("a/b/c", 0)
	cl.noteSubscription("d/e/f", 1)

	err := s.processPacket(cl, &packets.UnsubscribePacket{
		FixedHeader: packets.FixedHeader{
			Type: packets.Unsubscribe,
		},
		PacketID: 12,
		Topics:   []string{"a/b/c", "d/e/f"},
	})
	require.NoError(t, err)
	require.Equal(t, []byte{
		byte(packets.Unsuback << 4), 2, // Fixed header
		0, 12, // Packet ID - LSB+MSB
	}, cl.p.W.(*quietWriter).f[0])

	require.Empty(t, s.topics.Subscribers("a/b/c"))
	require.Empty(t, s.topics.Subscribers("d/e/f"))
	require.NotContains(t, cl.subscriptions, "a/b/c")
	require.NotContains(t, cl.subscriptions, "d/e/f")
}

func BenchmarkServerProcessUnsubscribe(b *testing.B) {
	s, _, _, cl := setupClient("zen")
	cl.p.W = new(quietWriter)

	pk := &packets.UnsubscribePacket{
		FixedHeader: packets.FixedHeader{
			Type: packets.Unsubscribe,
		},
		PacketID: 12,
		Topics:   []string{"a/b/c"},
	}
	for n := 0; n < b.N; n++ {
		err := s.processUnsubscribe(cl, pk)
		if err != nil {
			panic(err)
		}
	}
}

func TestServerProcessUnsubscribeWriteError(t *testing.T) {
	s, _, _, cl := setupClient("zen")
	cl.p.W = &quietWriter{errAfter: -1}
	err := s.processPacket(cl, &packets.UnsubscribePacket{
		FixedHeader: packets.FixedHeader{
			Type: packets.Unsubscribe,
		},
	})
	require.Error(t, err)
}
