package mqtt

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"

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
	client.inFlight.set(1, new(packets.PublishPacket))
	require.NotNil(t, client.inFlight.internal[1])
	require.NotEqual(t, 0, client.inFlight.internal[1].sent)
}

func BenchmarkInFlightSet(b *testing.B) {
	client := newClient(nil, new(packets.ConnectPacket))
	for n := 0; n < b.N; n++ {
		client.inFlight.set(1, new(packets.PublishPacket))
	}
}

func TestInFlightGet(t *testing.T) {
	client := newClient(nil, new(packets.ConnectPacket))
	client.inFlight.set(2, new(packets.PublishPacket))

	msg, ok := client.inFlight.get(2)
	require.Equal(t, true, ok)
	require.NotEqual(t, 0, msg.sent)
}

func BenchmarkInFlightGet(b *testing.B) {
	client := newClient(nil, new(packets.ConnectPacket))
	client.inFlight.set(2, new(packets.PublishPacket))
	for n := 0; n < b.N; n++ {
		client.inFlight.get(2)
	}
}

func TestInFlightDelete(t *testing.T) {
	client := newClient(nil, new(packets.ConnectPacket))
	client.inFlight.set(3, new(packets.PublishPacket))
	require.NotNil(t, client.inFlight.internal[3])

	client.inFlight.delete(3)
	require.Nil(t, client.inFlight.internal[3])

	_, ok := client.inFlight.get(3)
	require.Equal(t, false, ok)
}

func BenchmarInFlightDelete(b *testing.B) {
	client := newClient(nil, new(packets.ConnectPacket))
	for n := 0; n < b.N; n++ {
		client.inFlight.set(4, new(packets.PublishPacket))
		client.inFlight.delete(4)
	}
}
