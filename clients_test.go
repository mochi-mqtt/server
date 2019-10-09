package mqtt

import (
	"log"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mochi-co/mqtt/auth"
	"github.com/mochi-co/mqtt/packets"
)

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
	require.Contains(t, cl.internal, "t1")
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
	require.Contains(t, cl.internal, "t1")
	require.Contains(t, cl.internal, "t2")

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
	require.Contains(t, cl.internal, "t1")
	require.Contains(t, cl.internal, "t2")
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
	require.Contains(t, cl.internal, "t1")

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

func TestClientsGetByListener(t *testing.T) {
	cl := newClients()
	cl.add(&client{id: "t1", listener: "tcp1"})
	cl.add(&client{id: "t2", listener: "ws1"})
	require.Contains(t, cl.internal, "t1")
	require.Contains(t, cl.internal, "t2")

	clients := cl.getByListener("tcp1")
	log.Println(clients)
	require.NotEmpty(t, clients)
	require.Equal(t, 1, len(clients))
	require.Equal(t, "tcp1", clients[0].listener)
}

func BenchmarkClientsGetByListener(b *testing.B) {
	cl := newClients()
	cl.add(&client{id: "t1", listener: "tcp1"})
	cl.add(&client{id: "t2", listener: "ws1"})
	for n := 0; n < b.N; n++ {
		cl.getByListener("tcp1")
	}
}

func TestNewClient(t *testing.T) {
	r, _ := net.Pipe()
	p := NewParser(r, newBufioReader(r), newBufioWriter(r))
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

	cl := newClient(p, pk, new(auth.Allow))
	require.NotNil(t, cl)
	require.NotNil(t, cl.inFlight.internal)
	require.NotNil(t, cl.subscriptions)
	require.Equal(t, pk.Keepalive, cl.keepalive)
	require.Equal(t, pk.CleanSession, cl.cleanSession)
	require.Equal(t, pk.ClientIdentifier, cl.id)

	// Autogenerate id.
	pk = new(packets.ConnectPacket)
	cl = newClient(p, pk, new(auth.Allow))
	require.NotNil(t, cl)
	require.NotEmpty(t, cl.id)

	// Autoset keepalive
	pk = new(packets.ConnectPacket)
	cl = newClient(p, pk, new(auth.Allow))
	require.NotNil(t, cl)
	require.Equal(t, defaultClientKeepalive, cl.keepalive)
}

func TestNewClientLWT(t *testing.T) {
	r, _ := net.Pipe()
	p := NewParser(r, newBufioReader(r), newBufioWriter(r))
	r.Close()
	pk := &packets.ConnectPacket{
		FixedHeader: packets.FixedHeader{
			Type:      packets.Connect,
			Remaining: 29,
		},
		ProtocolName:     "MQTT",
		ProtocolVersion:  4,
		CleanSession:     true,
		Keepalive:        60,
		ClientIdentifier: "zen",
		WillFlag:         true,
		WillTopic:        "lwt",
		WillMessage:      []byte("lol gg"),
		WillQos:          1,
		WillRetain:       false,
	}

	cl := newClient(p, pk, new(auth.Allow))
	require.Equal(t, pk.WillTopic, cl.lwt.topic)
	require.Equal(t, pk.WillMessage, cl.lwt.message)
	require.Equal(t, pk.WillQos, cl.lwt.qos)
	require.Equal(t, pk.WillRetain, cl.lwt.retain)
}

func BenchmarkNewClient(b *testing.B) {
	r, _ := net.Pipe()
	p := NewParser(r, newBufioReader(r), newBufioWriter(r))
	r.Close()
	pk := new(packets.ConnectPacket)

	for n := 0; n < b.N; n++ {
		newClient(p, pk, new(auth.Allow))
	}
}

func TestNextPacketID(t *testing.T) {
	_, _, _, cl := setupClient("zen")

	require.Equal(t, uint32(1), cl.nextPacketID())
	require.Equal(t, uint32(2), cl.nextPacketID())

	cl.packetID = uint32(65534)
	require.Equal(t, uint32(65535), cl.nextPacketID())
	require.Equal(t, uint32(1), cl.nextPacketID())
}

func BenchmarkNextPacketID(b *testing.B) {
	_, _, _, cl := setupClient("zen")

	for n := 0; n < b.N; n++ {
		cl.nextPacketID()
	}
}

func TestClientNoteSubscription(t *testing.T) {
	client := newClient(nil, new(packets.ConnectPacket), new(auth.Allow))
	require.NotNil(t, client)
	client.noteSubscription("a/b/c", 0)
	require.Contains(t, client.subscriptions, "a/b/c")
	require.Equal(t, byte(0), client.subscriptions["a/b/c"])
}

func BenchmarkClientNoteSubscription(b *testing.B) {
	client := newClient(nil, new(packets.ConnectPacket), new(auth.Allow))
	for n := 0; n < b.N; n++ {
		client.noteSubscription("a/b/c", 0)
	}
}

func TestClientForgetSubscription(t *testing.T) {
	client := newClient(nil, new(packets.ConnectPacket), new(auth.Allow))
	require.NotNil(t, client)
	client.subscriptions = map[string]byte{
		"a/b/c/": 1,
	}
	client.forgetSubscription("a/b/c/")
	require.Empty(t, client.subscriptions["a/b/c"])
}

func BenchmarkClientForgetSubscription(b *testing.B) {
	client := newClient(nil, new(packets.ConnectPacket), new(auth.Allow))
	for n := 0; n < b.N; n++ {
		client.noteSubscription("a/b/c", 0)
		client.forgetSubscription("a/b/c/")
	}
}

func TestClientClose(t *testing.T) {
	r, w := net.Pipe()
	p := NewParser(r, newBufioReader(r), newBufioWriter(w))
	pk := &packets.ConnectPacket{
		ClientIdentifier: "zen3",
	}

	client := newClient(p, pk, new(auth.Allow))
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
	client := newClient(nil, new(packets.ConnectPacket), new(auth.Allow))
	client.inFlight.set(1, &inFlightMessage{packet: new(packets.PublishPacket), sent: 0})
	require.NotNil(t, client.inFlight.internal[1])
	require.NotEqual(t, 0, client.inFlight.internal[1].sent)
}

func BenchmarkInFlightSet(b *testing.B) {
	client := newClient(nil, new(packets.ConnectPacket), new(auth.Allow))
	in := &inFlightMessage{packet: new(packets.PublishPacket), sent: 0}
	for n := 0; n < b.N; n++ {
		client.inFlight.set(1, in)
	}
}

func TestInFlightGet(t *testing.T) {
	client := newClient(nil, new(packets.ConnectPacket), new(auth.Allow))
	client.inFlight.set(2, &inFlightMessage{packet: new(packets.PublishPacket), sent: 0})

	msg, ok := client.inFlight.get(2)
	require.Equal(t, true, ok)
	require.NotEqual(t, 0, msg.sent)
}

func BenchmarkInFlightGet(b *testing.B) {
	client := newClient(nil, new(packets.ConnectPacket), new(auth.Allow))
	client.inFlight.set(2, &inFlightMessage{packet: new(packets.PublishPacket), sent: 0})
	for n := 0; n < b.N; n++ {
		client.inFlight.get(2)
	}
}

func TestInFlightDelete(t *testing.T) {
	client := newClient(nil, new(packets.ConnectPacket), new(auth.Allow))
	client.inFlight.set(3, &inFlightMessage{packet: new(packets.PublishPacket), sent: 0})
	require.NotNil(t, client.inFlight.internal[3])

	client.inFlight.delete(3)
	require.Nil(t, client.inFlight.internal[3])

	_, ok := client.inFlight.get(3)
	require.Equal(t, false, ok)
}

func BenchmarInFlightDelete(b *testing.B) {
	client := newClient(nil, new(packets.ConnectPacket), new(auth.Allow))
	for n := 0; n < b.N; n++ {
		client.inFlight.set(4, &inFlightMessage{packet: new(packets.PublishPacket), sent: 0})
		client.inFlight.delete(4)
	}
}
