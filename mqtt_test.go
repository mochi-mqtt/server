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
	require.NotNil(t, s.readers)
	require.NotNil(t, s.writers)
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

	o := make(chan error)
	go func() {
		o <- s.EstablishConnection(r, new(auth.Allow))
	}()
	time.Sleep(5 * time.Millisecond)
	require.NotEmpty(t, s.clients.internal)
	require.NotNil(t, s.clients.internal["zen"])

	s.clients.internal["zen"].close()
	require.NoError(t, <-o)

	// Check Conanck response.
	w.Close()
	r.Close()

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
	client := newClient(p, &packets.ConnectPacket{
		ClientIdentifier: "zen",
	})

	err := s.processPacket(client, &packets.PingPacket{
		FixedHeader: packets.FixedHeader{
			Type: packets.Ping,
		},
	})

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
