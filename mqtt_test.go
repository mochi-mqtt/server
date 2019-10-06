package mqtt

import (
	"bufio"
	"io/ioutil"
	"log"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/mochi-co/mqtt/auth"
	"github.com/mochi-co/mqtt/listeners"
	"github.com/mochi-co/mqtt/packets"
)

func newBufioReader(c net.Conn) *bufio.Reader {
	return bufio.NewReaderSize(c, 512)
}

func newBufioWriter(c net.Conn) *bufio.Writer {
	return bufio.NewWriterSize(c, 512)
}

func TestNew(t *testing.T) {
	s := New()
	require.NotNil(t, s)
	require.NotNil(t, s.listeners)
	require.NotNil(t, s.clients)
	require.NotNil(t, s.buffers)
	log.Println(s)
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
func TestServerEstablishConnectionOK(t *testing.T) {
	s := New()
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
func TestServerEstablishConnectionBadFixedHeader(t *testing.T) {
	s := New()
	r, w := net.Pipe()
	go func() {
		w.Write([]byte{99})
		w.Close()
	}()
	err := s.EstablishConnection(r, new(auth.Allow))
	r.Close()
	require.Error(t, err)
	require.Equal(t, ErrReadConnectFixedHeader, err)
}

func TestServerEstablishConnectionBadConnectPacket(t *testing.T) {
	s := New()
	r, w := net.Pipe()
	go func() {
		w.Write([]byte{byte(packets.Connect << 4), 17})
		w.Close()
	}()
	err := s.EstablishConnection(r, new(auth.Allow))
	r.Close()
	require.Error(t, err)
	require.Equal(t, ErrReadConnectPacket, err)
}

func TestServerEstablishConnectionNotConnectPacket(t *testing.T) {
	s := New()
	r, w := net.Pipe()
	go func() {
		w.Write([]byte{
			byte(packets.Connack << 4), 2, // fixed header
			0, // No existing session
			packets.Accepted,
		})
		w.Close()
	}()
	err := s.EstablishConnection(r, new(auth.Allow))
	r.Close()
	require.Error(t, err)
	require.Equal(t, ErrFirstPacketInvalid, err)
}

func TestServerEstablishConnectionInvalidConnectPacket(t *testing.T) {
	s := New()
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
	err := s.EstablishConnection(r, new(auth.Allow))
	r.Close()
	require.Error(t, err)
	require.Equal(t, ErrReadConnectInvalid, err)
}

func TestServerEstablishConnectionBadAuth(t *testing.T) {
	s := New()
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
	err := s.EstablishConnection(r, new(auth.Disallow))
	r.Close()
	require.Error(t, err)
	require.Equal(t, ErrConnectNotAuthorized, err)
}

func TestServerEstablishConnectionReadClientError(t *testing.T) {
	s := New()
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
		o <- s.EstablishConnection(r, new(auth.Allow))
	}()
	time.Sleep(5 * time.Millisecond)
	w.Close()
	//	s.clients.internal["zen"].close()
	require.Error(t, <-o)
	r.Close()

}

func TestServerReadClientOK(t *testing.T) {
	s := New()
	r, w := net.Pipe()
	p := packets.NewParser(r, newBufioReader(r), newBufioWriter(w))
	cl := newClient(p, new(packets.ConnectPacket))

	go func() {
		b := []byte{
			byte(packets.Publish << 4), 18, // Fixed header
			0, 5, // Topic Name - LSB+MSB
			'a', '/', 'b', '/', 'c', // Topic Name
			'h', 'e', 'l', 'l', 'o', ' ', 'm', 'o', 'c', 'h', 'i', // Payload
		}
		w.Write(b)
		cl.close()

	}()
	time.Sleep(time.Millisecond)
	o := make(chan error)
	go func() {
		o <- s.readClient(cl)
	}()
	time.Sleep(5 * time.Millisecond)
	require.NoError(t, <-o)
	w.Close()
	r.Close()
}

func TestServerReadClientNoConn(t *testing.T) {
	s := New()
	r, _ := net.Pipe()
	p := packets.NewParser(r, newBufioReader(r), newBufioWriter(r))
	p.Conn.Close()
	p.Conn = nil
	r.Close()
	cl := newClient(p, new(packets.ConnectPacket))

	err := s.readClient(cl)
	require.Error(t, err)
	require.Equal(t, ErrConnectionClosed, err)
}

func TestServerReadClientBadFixedHeader(t *testing.T) {
	s := New()
	r, w := net.Pipe()
	go func() {
		w.Write([]byte{99})
		w.Close()
	}()
	p := packets.NewParser(r, newBufioReader(r), newBufioWriter(w))
	cl := newClient(p, new(packets.ConnectPacket))
	err := s.readClient(cl)
	r.Close()
	require.Error(t, err)
	require.Equal(t, ErrReadFixedHeader, err)
}

func TestServerReadClientDisconnect(t *testing.T) {
	s := New()
	r, w := net.Pipe()
	go func() {
		w.Write([]byte{packets.Disconnect << 4, 0})
		w.Close()
	}()
	p := packets.NewParser(r, newBufioReader(r), newBufioWriter(w))
	cl := newClient(p, new(packets.ConnectPacket))
	err := s.readClient(cl)
	r.Close()
	require.NoError(t, err)
}

func TestServerReadClientBadPacketPayload(t *testing.T) {
	s := New()
	r, w := net.Pipe()
	go func() {
		w.Write([]byte{
			byte(packets.Publish << 4), 7, // Fixed header
			0, 5, // Topic Name - LSB+MSB
			'a', '/',
			0, 11, // Packet ID - LSB+MSB, // malformed packet id.
		})
		w.Close()
	}()
	p := packets.NewParser(r, newBufioReader(r), newBufioWriter(w))
	cl := newClient(p, new(packets.ConnectPacket))
	err := s.readClient(cl)
	r.Close()
	require.Error(t, err)
	require.Equal(t, ErrReadPacketPayload, err)
}

func TestServerReadClientBadPacketValidation(t *testing.T) {
	s := New()
	r, w := net.Pipe()
	go func() {
		w.Write([]byte{
			byte(packets.Unsubscribe<<4) | 1<<1, 9, // Fixed header
			0, 0, // Packet ID - LSB+MSB
			0, 5, // Topic Name - LSB+MSB
			'a', '/', 'b', '/', 'c', // Topic Name
		})
		w.Close()
	}()
	p := packets.NewParser(r, newBufioReader(r), newBufioWriter(w))
	cl := newClient(p, new(packets.ConnectPacket))
	err := s.readClient(cl)
	r.Close()
	require.Error(t, err)
	require.Equal(t, ErrReadPacketValidation, err)
}

func TestServerWriteClient(t *testing.T) {
	s := New()
	r, w := net.Pipe()
	p := packets.NewParser(r, newBufioReader(r), newBufioWriter(w))
	pk := new(packets.ConnectPacket)
	cl := newClient(p, pk)

	go func() {
		err := s.writeClient(cl, &packets.PublishPacket{
			FixedHeader: packets.FixedHeader{
				Type:      packets.Publish,
				Remaining: 18,
			},
			TopicName: "a/b/c",
			Payload:   []byte("hello mochi"),
		})
		if err != nil {
			panic(err)
		}
		w.Close()
	}()
	time.Sleep(10 * time.Millisecond)

	buf, err := ioutil.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, []byte{byte(packets.Publish << 4), 18, // Fixed header
		0, 5, // Topic Name - LSB+MSB
		'a', '/', 'b', '/', 'c', // Topic Name
		'h', 'e', 'l', 'l', 'o', ' ', 'm', 'o', 'c', 'h', 'i',
	}, buf)
	r.Close()
}

func TestServerWriteClientBadEncode(t *testing.T) {
	s := New()
	r, w := net.Pipe()
	p := packets.NewParser(r, newBufioReader(r), newBufioWriter(w))
	pk := new(packets.ConnectPacket)
	cl := newClient(p, pk)
	err := s.writeClient(cl, &packets.PublishPacket{
		FixedHeader: packets.FixedHeader{
			Qos: 1,
		},
		PacketID: 0,
	})
	require.Error(t, err)
	r.Close()
	w.Close()
}

func TestServerWriteClientNilFlush(t *testing.T) {
	s := New()
	r, w := net.Pipe()
	p := packets.NewParser(r, newBufioReader(r), newBufioWriter(w))
	pk := new(packets.ConnectPacket)
	cl := newClient(p, pk)
	r.Close()
	w.Close()
	err := s.writeClient(cl, &packets.PublishPacket{
		FixedHeader: packets.FixedHeader{
			Qos: 1,
		},
		PacketID: 0,
	})
	require.Error(t, err)
}

func TestServerWriteClientNilWriter(t *testing.T) {
	s := New()
	r, w := net.Pipe()
	p := packets.NewParser(r, newBufioReader(r), newBufioWriter(w))
	p.W = nil
	pk := new(packets.ConnectPacket)
	cl := newClient(p, pk)
	err := s.writeClient(cl, &packets.PublishPacket{})
	require.Error(t, err)
	require.Equal(t, ErrConnectionClosed, err)
	r.Close()
	w.Close()
}

func TestServerCloseClient(t *testing.T) { // as opposed to client.close
	s := New()
	r, w := net.Pipe()
	p := packets.NewParser(r, newBufioReader(r), newBufioWriter(w))
	pk := &packets.ConnectPacket{
		ClientIdentifier: "zen",
	}

	s.clients.add(newClient(p, pk))
	require.NotNil(t, s.clients.internal["zen"])

	// close the client connection.
	s.closeClient(s.clients.internal["zen"], true)
	var ok bool
	select {
	case _, ok = <-s.clients.internal["zen"].end:
	}
	require.Equal(t, false, ok)
	require.Nil(t, s.clients.internal["zen"].p.Conn)
}

func TestServerProcessPacketCONNECT(t *testing.T) {
	s := New()
	r, w := net.Pipe()
	p := packets.NewParser(r, newBufioReader(r), newBufioWriter(w))
	client := newClient(p, &packets.ConnectPacket{
		ClientIdentifier: "zen",
	})

	err := s.processPacket(client, &packets.ConnectPacket{
		FixedHeader: packets.FixedHeader{
			Type: packets.Connect,
		},
	})

	var ok bool
	select {
	case _, ok = <-client.end:
	}
	require.Equal(t, false, ok)
	require.Nil(t, client.p.Conn)

	require.NoError(t, err)
}

func TestServerProcessPacketDISCONNECT(t *testing.T) {
	s := New()
	r, w := net.Pipe()
	p := packets.NewParser(r, newBufioReader(r), newBufioWriter(w))
	client := newClient(p, &packets.ConnectPacket{
		ClientIdentifier: "zen",
	})

	err := s.processPacket(client, &packets.DisconnectPacket{
		FixedHeader: packets.FixedHeader{
			Type: packets.Disconnect,
		},
	})

	var ok bool
	select {
	case _, ok = <-client.end:
	}
	require.Equal(t, false, ok)
	require.Nil(t, client.p.Conn)

	require.NoError(t, err)
}

func TestServerProcessPacketPINGOK(t *testing.T) {
	s := New()
	r, w := net.Pipe()
	p := packets.NewParser(r, newBufioReader(r), newBufioWriter(w))
	cl := newClient(p, &packets.ConnectPacket{
		ClientIdentifier: "zen",
	})

	o := make(chan error, 2) // race with r/w, so use buffered to not block
	go func() {
		o <- s.processPacket(cl, &packets.PingreqPacket{
			FixedHeader: packets.FixedHeader{
				Type: packets.Pingreq,
			},
		})
		w.Close()
	}()
	time.Sleep(10 * time.Millisecond)
	buf, err := ioutil.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, []byte{
		byte(packets.Pingresp << 4), 0,
	}, buf)
	require.NoError(t, <-o)
	close(o)
	r.Close()
}

func TestServerProcessPacketPINGClose(t *testing.T) {
	s := New()
	r, w := net.Pipe()
	p := packets.NewParser(r, newBufioReader(r), newBufioWriter(w))
	cl := newClient(p, &packets.ConnectPacket{
		ClientIdentifier: "zen",
	})

	o := make(chan error, 2) // race with r/w, so use buffered to not block
	go func() {
		r.Close()
		w.Close()
		o <- s.processPacket(cl, &packets.PingreqPacket{
			FixedHeader: packets.FixedHeader{
				Type: packets.Pingreq,
			},
		})

	}()
	require.NoError(t, <-o)
	require.Nil(t, p.Conn)
	close(o)
}

func TestServerProcessPacketPublishOK(t *testing.T) {
	s := New()

	// Sender
	r, w := net.Pipe()
	c1 := newClient(
		packets.NewParser(r, newBufioReader(r), newBufioWriter(w)),
		&packets.ConnectPacket{ClientIdentifier: "c1"},
	)
	s.clients.add(c1)

	// Subscriber
	r2, w2 := net.Pipe()
	c2 := newClient(
		packets.NewParser(r2, newBufioReader(r2), newBufioWriter(w2)),
		&packets.ConnectPacket{ClientIdentifier: "c2"},
	)
	s.clients.add(c2)
	s.topics.Subscribe("a/b/+", c2.id, 0)
	s.topics.Subscribe("a/+/c", c2.id, 1)

	o := make(chan error, 2)
	go func() {
		o <- s.processPacket(c1, &packets.PublishPacket{
			FixedHeader: packets.FixedHeader{
				Type: packets.Publish,
			},
			TopicName: "a/b/c",
			Payload:   []byte("hello"),
		})
		r.Close()
		w.Close()
		w2.Close()
	}()

	recv := make(chan []byte)
	go func() {
		buf, err := ioutil.ReadAll(r2)
		if err != nil {
			panic(err)
		}
		recv <- buf
	}()

	require.NoError(t, <-o)
	require.Equal(t,
		[]byte{
			byte(packets.Publish<<4 | 2), 14, // Fixed header
			0, 5, // Topic Name - LSB+MSB
			'a', '/', 'b', '/', 'c', // Topic Name
			0, 1, // packet id from qos=1
			'h', 'e', 'l', 'l', 'o', // Payload
		},
		<-recv,
	)
	close(o)
	close(recv)
	r2.Close()
}

func TestServerProcessPacketPublishRetain(t *testing.T) {
	s := New()
	r, w := net.Pipe()
	p := packets.NewParser(r, newBufioReader(r), newBufioWriter(w))
	client := newClient(p, &packets.ConnectPacket{
		ClientIdentifier: "zen",
	})

	pk := &packets.PublishPacket{
		FixedHeader: packets.FixedHeader{
			Type:   packets.Publish,
			Retain: true,
		},
		TopicName: "a/b/c",
		Payload:   []byte("hello"),
	}

	o := make(chan error, 2) // race with r/w, so use buffered to not block
	go func() {
		o <- s.processPacket(client, pk)
	}()

	time.Sleep(10 * time.Millisecond)
	require.NoError(t, <-o)

	require.Equal(t, pk, s.topics.Messages("a/b/c")[0])
	close(o)
	r.Close()
}

func TestNewClients(t *testing.T) {
	cl := newClients()
	require.NotNil(t, cl.internal)
}

func BenchmarkNewClients(b *testing.B) {
	for n := 0; n < b.N; n++ {
		newClients()
	}
}

func TestClientsAdd(t *testing.T) {
	cl := newClients()
	cl.add(&client{id: "t1"})
	require.NotNil(t, cl.internal["t1"])
}

func BenchmarkClientsAdd(b *testing.B) {
	cl := newClients()
	client := &client{id: "t1"}
	for n := 0; n < b.N; n++ {
		cl.add(client)
	}
}

func TestClientsGet(t *testing.T) {
	cl := newClients()
	cl.add(&client{id: "t1"})
	cl.add(&client{id: "t2"})
	require.NotNil(t, cl.internal["t1"])
	require.NotNil(t, cl.internal["t2"])

	client, ok := cl.get("t1")
	require.Equal(t, true, ok)
	require.Equal(t, "t1", client.id)
}

func BenchmarkClientsGet(b *testing.B) {
	cl := newClients()
	cl.add(&client{id: "t1"})
	for n := 0; n < b.N; n++ {
		cl.get("t1")
	}
}

func TestClientsLen(t *testing.T) {
	cl := newClients()
	cl.add(&client{id: "t1"})
	cl.add(&client{id: "t2"})
	require.NotNil(t, cl.internal["t1"])
	require.NotNil(t, cl.internal["t2"])
	require.Equal(t, 2, cl.len())
}

func BenchmarkClientsLen(b *testing.B) {
	cl := newClients()
	cl.add(&client{id: "t1"})
	for n := 0; n < b.N; n++ {
		cl.len()
	}
}

func TestClientsDelete(t *testing.T) {
	cl := newClients()
	cl.add(&client{id: "t1"})
	require.NotNil(t, cl.internal["t1"])

	cl.delete("t1")
	_, ok := cl.get("t1")
	require.Equal(t, false, ok)
	require.Nil(t, cl.internal["t1"])
}

func BenchmarkClientsDelete(b *testing.B) {
	cl := newClients()
	cl.add(&client{id: "t1"})
	for n := 0; n < b.N; n++ {
		cl.delete("t1")
	}
}

func TestNewClient(t *testing.T) {
	r, _ := net.Pipe()
	p := packets.NewParser(r, newBufioReader(r), newBufioWriter(r))
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
	require.NotNil(t, cl.inFlight.internal)
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
	require.Equal(t, clientKeepalive, cl.keepalive)
}

func BenchmarkNewClient(b *testing.B) {
	r, _ := net.Pipe()
	p := packets.NewParser(r, newBufioReader(r), newBufioWriter(r))
	r.Close()
	pk := new(packets.ConnectPacket)

	for n := 0; n < b.N; n++ {
		newClient(p, pk)
	}
}

func TestNextPacketID(t *testing.T) {
	r, w := net.Pipe()
	p := packets.NewParser(r, newBufioReader(r), newBufioWriter(w))
	client := newClient(p, new(packets.ConnectPacket))
	require.NotNil(t, client)

	require.Equal(t, uint32(1), client.nextPacketID())
	require.Equal(t, uint32(2), client.nextPacketID())

	client.packetID = uint32(65534)
	require.Equal(t, uint32(65535), client.nextPacketID())
	require.Equal(t, uint32(1), client.nextPacketID())
}

func BenchmarkNextPacketID(b *testing.B) {
	r, w := net.Pipe()
	p := packets.NewParser(r, newBufioReader(r), newBufioWriter(w))
	client := newClient(p, new(packets.ConnectPacket))

	for n := 0; n < b.N; n++ {
		client.nextPacketID()
	}
}

func TestClientClose(t *testing.T) {
	r, w := net.Pipe()
	p := packets.NewParser(r, newBufioReader(r), newBufioWriter(w))
	pk := &packets.ConnectPacket{
		ClientIdentifier: "zen3",
	}

	client := newClient(p, pk)
	require.NotNil(t, client)

	client.close()

	var ok bool
	select {
	case _, ok = <-client.end:
	}
	require.Equal(t, false, ok)
	require.Nil(t, client.p.Conn)
	r.Close()
	w.Close()
}

func TestInFlightSet(t *testing.T) {
	client := newClient(nil, new(packets.ConnectPacket))
	client.inFlight.set(1, &inFlightMessage{sent: 1})
	require.NotNil(t, client.inFlight.internal[1])
	require.NotEqual(t, 0, client.inFlight.internal[1].sent)
}

func BenchmarkInFlightSet(b *testing.B) {
	client := newClient(nil, new(packets.ConnectPacket))
	for n := 0; n < b.N; n++ {
		client.inFlight.set(1, &inFlightMessage{sent: 1})
	}
}

func TestInFlightGet(t *testing.T) {
	client := newClient(nil, new(packets.ConnectPacket))
	client.inFlight.set(2, &inFlightMessage{sent: 1})

	msg, ok := client.inFlight.get(2)
	require.Equal(t, true, ok)
	require.NotEqual(t, 0, msg.sent)
}

func BenchmarkInFlightGet(b *testing.B) {
	client := newClient(nil, new(packets.ConnectPacket))
	client.inFlight.set(2, &inFlightMessage{sent: 1})
	for n := 0; n < b.N; n++ {
		client.inFlight.get(2)
	}
}

func TestInFlightDelete(t *testing.T) {
	client := newClient(nil, new(packets.ConnectPacket))
	client.inFlight.set(3, &inFlightMessage{sent: 1})
	require.NotNil(t, client.inFlight.internal[3])

	client.inFlight.delete(3)
	require.Nil(t, client.inFlight.internal[3])

	_, ok := client.inFlight.get(3)
	require.Equal(t, false, ok)

}

func BenchmarInFlightDelete(b *testing.B) {
	client := newClient(nil, new(packets.ConnectPacket))
	for n := 0; n < b.N; n++ {
		client.inFlight.set(4, &inFlightMessage{sent: 1})
		client.inFlight.delete(4)
	}
}
