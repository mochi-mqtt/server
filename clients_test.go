package mqtt

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mochi-co/mqtt/packets"
)

func TestNewClient(t *testing.T) {
	r, _ := net.Pipe()
	p := packets.NewParser(r)
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
	require.Equal(t, defaultKeepalive, cl.keepalive)
}

func BenchmarkNewClient(b *testing.B) {
	r, _ := net.Pipe()
	p := packets.NewParser(r)
	r.Close()
	pk := new(packets.ConnectPacket)

	for n := 0; n < b.N; n++ {
		newClient(p, pk)
	}
}
