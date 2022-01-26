package clients

import (
	"errors"
	"io"
	"io/ioutil"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mochi-co/mqtt/server/internal/circ"
	"github.com/mochi-co/mqtt/server/internal/packets"
	"github.com/mochi-co/mqtt/server/listeners/auth"
	"github.com/mochi-co/mqtt/server/system"
	"github.com/stretchr/testify/require"
)

func genClient() *Client {
	c, _ := net.Pipe()
	return NewClient(c, circ.NewReader(128, 8), circ.NewWriter(128, 8), new(system.Info))
}

func TestNewClients(t *testing.T) {
	cl := New()
	require.NotNil(t, cl.internal)
}

func BenchmarkNewClients(b *testing.B) {
	for n := 0; n < b.N; n++ {
		New()
	}
}

func TestClientsAdd(t *testing.T) {
	cl := New()
	cl.Add(&Client{ID: "t1"})
	require.Contains(t, cl.internal, "t1")
}

func BenchmarkClientsAdd(b *testing.B) {
	cl := New()
	client := &Client{ID: "t1"}
	for n := 0; n < b.N; n++ {
		cl.Add(client)
	}
}

func TestClientsGet(t *testing.T) {
	cl := New()
	cl.Add(&Client{ID: "t1"})
	cl.Add(&Client{ID: "t2"})
	require.Contains(t, cl.internal, "t1")
	require.Contains(t, cl.internal, "t2")

	client, ok := cl.Get("t1")
	require.Equal(t, true, ok)
	require.Equal(t, "t1", client.ID)
}

func BenchmarkClientsGet(b *testing.B) {
	cl := New()
	cl.Add(&Client{ID: "t1"})
	for n := 0; n < b.N; n++ {
		cl.Get("t1")
	}
}

func TestClientsLen(t *testing.T) {
	cl := New()
	cl.Add(&Client{ID: "t1"})
	cl.Add(&Client{ID: "t2"})
	require.Contains(t, cl.internal, "t1")
	require.Contains(t, cl.internal, "t2")
	require.Equal(t, 2, cl.Len())
}

func BenchmarkClientsLen(b *testing.B) {
	cl := New()
	cl.Add(&Client{ID: "t1"})
	for n := 0; n < b.N; n++ {
		cl.Len()
	}
}

func TestClientsDelete(t *testing.T) {
	cl := New()
	cl.Add(&Client{ID: "t1"})
	require.Contains(t, cl.internal, "t1")

	cl.Delete("t1")
	_, ok := cl.Get("t1")
	require.Equal(t, false, ok)
	require.Nil(t, cl.internal["t1"])
}

func BenchmarkClientsDelete(b *testing.B) {
	cl := New()
	cl.Add(&Client{ID: "t1"})
	for n := 0; n < b.N; n++ {
		cl.Delete("t1")
	}
}

func TestClientsGetByListener(t *testing.T) {
	cl := New()
	cl.Add(&Client{ID: "t1", Listener: "tcp1"})
	cl.Add(&Client{ID: "t2", Listener: "ws1"})
	require.Contains(t, cl.internal, "t1")
	require.Contains(t, cl.internal, "t2")

	clients := cl.GetByListener("tcp1")
	require.NotEmpty(t, clients)
	require.Equal(t, 1, len(clients))
	require.Equal(t, "tcp1", clients[0].Listener)
}

func BenchmarkClientsGetByListener(b *testing.B) {
	cl := New()
	cl.Add(&Client{ID: "t1", Listener: "tcp1"})
	cl.Add(&Client{ID: "t2", Listener: "ws1"})
	for n := 0; n < b.N; n++ {
		cl.GetByListener("tcp1")
	}
}

func TestNewClient(t *testing.T) {
	cl := genClient()

	require.NotNil(t, cl)
	require.NotNil(t, cl.Inflight.internal)
	require.NotNil(t, cl.Subscriptions)
	require.NotNil(t, cl.r)
	require.NotNil(t, cl.w)
	require.NotNil(t, cl.State.started)
	require.NotNil(t, cl.State.endedW)
	require.NotNil(t, cl.State.endedR)
}

func BenchmarkNewClient(b *testing.B) {
	c, _ := net.Pipe()
	for n := 0; n < b.N; n++ {
		NewClient(c, circ.NewReader(16, 4), circ.NewWriter(16, 4), nil)
	}
}

func TestNewClientStub(t *testing.T) {
	cl := NewClientStub(nil)

	require.NotNil(t, cl)
	require.NotNil(t, cl.Inflight.internal)
	require.NotNil(t, cl.Subscriptions)
}

func BenchmarkNewClientStub(b *testing.B) {
	for n := 0; n < b.N; n++ {
		NewClientStub(nil)
	}
}

func TestClientIdentify(t *testing.T) {
	cl := genClient()

	pk := packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type:      packets.Connect,
			Remaining: 16,
		},
		ProtocolName:     []byte{'M', 'Q', 'T', 'T'},
		ProtocolVersion:  4,
		CleanSession:     true,
		Keepalive:        60,
		ClientIdentifier: "mochi",
	}

	cl.Identify("tcp1", pk, new(auth.Allow))
	require.Equal(t, pk.Keepalive, cl.keepalive)
	require.Equal(t, pk.CleanSession, cl.cleanSession)
	require.Equal(t, pk.ClientIdentifier, cl.ID)
}

func BenchmarkClientIdentify(b *testing.B) {
	cl := genClient()

	pk := packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type:      packets.Connect,
			Remaining: 16,
		},
		ProtocolName:     []byte{'M', 'Q', 'T', 'T'},
		ProtocolVersion:  4,
		CleanSession:     true,
		Keepalive:        60,
		ClientIdentifier: "mochi",
	}

	for n := 0; n < b.N; n++ {
		cl.Identify("tcp1", pk, new(auth.Allow))
	}
}

func TestClientIdentifyNoID(t *testing.T) {
	cl := genClient()

	pk := packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type:      packets.Connect,
			Remaining: 16,
		},
		ProtocolName:    []byte{'M', 'Q', 'T', 'T'},
		ProtocolVersion: 4,
		CleanSession:    true,
		Keepalive:       60,
	}

	cl.Identify("tcp1", pk, new(auth.Allow))
	require.NotEmpty(t, cl.ID)
}

func TestClientIdentifyLWT(t *testing.T) {
	cl := genClient()

	pk := packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type:      packets.Connect,
			Remaining: 16,
		},
		ProtocolName:     []byte{'M', 'Q', 'T', 'T'},
		ProtocolVersion:  4,
		CleanSession:     true,
		Keepalive:        60,
		ClientIdentifier: "mochi",
		WillFlag:         true,
		WillTopic:        "lwt",
		WillMessage:      []byte("lol gg"),
		WillQos:          1,
		WillRetain:       false,
	}

	cl.Identify("tcp1", pk, new(auth.Allow))
	require.Equal(t, pk.WillTopic, cl.LWT.Topic)
	require.Equal(t, pk.WillMessage, cl.LWT.Message)
	require.Equal(t, pk.WillQos, cl.LWT.Qos)
	require.Equal(t, pk.WillRetain, cl.LWT.Retain)
}

func TestClientNextPacketID(t *testing.T) {
	cl := genClient()

	require.Equal(t, uint32(1), cl.NextPacketID())
	require.Equal(t, uint32(2), cl.NextPacketID())

	cl.packetID = uint32(65534)
	require.Equal(t, uint32(65535), cl.NextPacketID())
	require.Equal(t, uint32(1), cl.NextPacketID())
}

func BenchmarkClientNextPacketID(b *testing.B) {
	cl := genClient()

	for n := 0; n < b.N; n++ {
		cl.NextPacketID()
	}
}

func TestClientNoteSubscription(t *testing.T) {
	cl := genClient()

	cl.NoteSubscription("a/b/c", 0)
	require.Contains(t, cl.Subscriptions, "a/b/c")
	require.Equal(t, byte(0), cl.Subscriptions["a/b/c"])
}

func BenchmarkClientNoteSubscription(b *testing.B) {
	cl := genClient()
	for n := 0; n < b.N; n++ {
		cl.NoteSubscription("a/b/c", 0)
	}
}

func TestClientForgetSubscription(t *testing.T) {
	cl := genClient()
	require.NotNil(t, cl)
	cl.Subscriptions = map[string]byte{
		"a/b/c/": 1,
	}
	cl.ForgetSubscription("a/b/c/")
	require.Empty(t, cl.Subscriptions["a/b/c"])
}

func BenchmarkClientForgetSubscription(b *testing.B) {
	cl := genClient()
	for n := 0; n < b.N; n++ {
		cl.NoteSubscription("a/b/c", 0)
		cl.ForgetSubscription("a/b/c/")
	}
}

func TestClientRefreshDeadline(t *testing.T) {
	cl := genClient()
	cl.refreshDeadline(10)

	// How do we check net.Conn deadline?
	require.NotNil(t, cl.conn)
}

func BenchmarkClientRefreshDeadline(b *testing.B) {
	cl := genClient()
	for n := 0; n < b.N; n++ {
		cl.refreshDeadline(10)
	}
}

func TestClientStart(t *testing.T) {
	cl := genClient()
	cl.Start()
	defer cl.Stop()
	time.Sleep(time.Millisecond)
	require.Equal(t, uint32(1), atomic.LoadUint32(&cl.r.State))
	require.Equal(t, uint32(2), atomic.LoadUint32(&cl.w.State))
}

func BenchmarkClientStart(b *testing.B) {
	cl := genClient()
	defer cl.Stop()

	for n := 0; n < b.N; n++ {
		cl.Start()
	}
}

func TestClientReadFixedHeader(t *testing.T) {
	cl := genClient()
	cl.Start()
	defer cl.Stop()

	cl.r.Set([]byte{packets.Connect << 4, 0x00}, 0, 2)
	cl.r.SetPos(0, 2)

	fh := new(packets.FixedHeader)
	err := cl.ReadFixedHeader(fh)
	require.NoError(t, err)
	require.Equal(t, int64(2), atomic.LoadInt64(&cl.systemInfo.BytesRecv))

	tail, head := cl.r.GetPos()
	require.Equal(t, int64(2), tail)
	require.Equal(t, int64(2), head)

}

func TestClientReadFixedHeaderDecodeError(t *testing.T) {
	cl := genClient()
	cl.Start()
	defer cl.Stop()

	o := make(chan error)
	go func() {
		fh := new(packets.FixedHeader)
		cl.r.Set([]byte{packets.Connect<<4 | 1<<1, 0x00, 0x00}, 0, 2)
		cl.r.SetPos(0, 2)
		o <- cl.ReadFixedHeader(fh)
	}()
	time.Sleep(time.Millisecond)
	require.Error(t, <-o)
}

func TestClientReadFixedHeaderReadEOF(t *testing.T) {
	cl := genClient()
	cl.Start()
	defer cl.Stop()

	o := make(chan error)
	go func() {
		fh := new(packets.FixedHeader)
		cl.r.Set([]byte{packets.Connect << 4, 0x00}, 0, 2)
		cl.r.SetPos(0, 1)
		o <- cl.ReadFixedHeader(fh)
	}()
	time.Sleep(time.Millisecond)
	cl.r.Stop()
	err := <-o
	require.Error(t, err)
	require.Equal(t, io.EOF, err)
}

func TestClientReadFixedHeaderNoLengthTerminator(t *testing.T) {
	cl := genClient()
	cl.Start()
	defer cl.Stop()

	o := make(chan error)
	go func() {
		fh := new(packets.FixedHeader)
		err := cl.r.Set([]byte{packets.Connect << 4, 0xd5, 0x86, 0xf9, 0x9e, 0x01}, 0, 5)
		require.NoError(t, err)
		cl.r.SetPos(0, 5)
		o <- cl.ReadFixedHeader(fh)
	}()
	time.Sleep(time.Millisecond)
	require.Error(t, <-o)
}

func TestClientReadOK(t *testing.T) {
	cl := genClient()
	cl.Start()
	defer cl.Stop()

	// Two packets in a row...
	b := []byte{
		byte(packets.Publish << 4), 18, // Fixed header
		0, 5, // Topic Name - LSB+MSB
		'a', '/', 'b', '/', 'c', // Topic Name
		'h', 'e', 'l', 'l', 'o', ' ', 'm', 'o', 'c', 'h', 'i', // Payload,
		byte(packets.Publish << 4), 11, // Fixed header
		0, 5, // Topic Name - LSB+MSB
		'd', '/', 'e', '/', 'f', // Topic Name
		'y', 'e', 'a', 'h', // Payload
	}

	err := cl.r.Set(b, 0, len(b))
	require.NoError(t, err)
	cl.r.SetPos(0, int64(len(b)))

	o := make(chan error)
	var pks []packets.Packet
	go func() {
		o <- cl.Read(func(cl *Client, pk packets.Packet) error {
			pks = append(pks, pk)
			return nil
		})
	}()

	time.Sleep(time.Millisecond)
	cl.r.Stop()

	err = <-o
	require.Error(t, err)
	require.Equal(t, io.EOF, err)
	require.Equal(t, 2, len(pks))
	require.Equal(t, pks, []packets.Packet{
		{
			FixedHeader: packets.FixedHeader{
				Type:      packets.Publish,
				Remaining: 18,
			},
			TopicName: "a/b/c",
			Payload:   []byte("hello mochi"),
		},
		{
			FixedHeader: packets.FixedHeader{
				Type:      packets.Publish,
				Remaining: 11,
			},
			TopicName: "d/e/f",
			Payload:   []byte("yeah"),
		},
	})

	require.Equal(t, int64(len(b)), atomic.LoadInt64(&cl.systemInfo.BytesRecv))
	require.Equal(t, int64(2), atomic.LoadInt64(&cl.systemInfo.MessagesRecv))

}

func TestClientReadDone(t *testing.T) {
	cl := genClient()
	cl.Start()
	defer cl.Stop()
	cl.State.Done = 1

	err := cl.Read(func(cl *Client, pk packets.Packet) error {
		return nil
	})

	require.NoError(t, err)
}

func TestClientReadPacketError(t *testing.T) {
	cl := genClient()
	cl.Start()
	defer cl.Stop()

	b := []byte{
		0, 18,
		0, 5,
		'a', '/', 'b', '/', 'c',
		'h', 'e', 'l', 'l', 'o', ' ', 'm', 'o', 'c', 'h', 'i',
	}
	err := cl.r.Set(b, 0, len(b))
	require.NoError(t, err)
	cl.r.SetPos(0, int64(len(b)))

	o := make(chan error)
	go func() {
		o <- cl.Read(func(cl *Client, pk packets.Packet) error {
			return nil
		})
	}()

	require.Error(t, <-o)
}

func TestClientReadHandlerErr(t *testing.T) {
	cl := genClient()
	cl.Start()
	defer cl.Stop()

	b := []byte{
		byte(packets.Publish << 4), 11, // Fixed header
		0, 5, // Topic Name - LSB+MSB
		'd', '/', 'e', '/', 'f', // Topic Name
		'y', 'e', 'a', 'h', // Payload
	}

	err := cl.r.Set(b, 0, len(b))
	require.NoError(t, err)
	cl.r.SetPos(0, int64(len(b)))

	err = cl.Read(func(cl *Client, pk packets.Packet) error {
		return errors.New("test")
	})

	require.Error(t, err)
}

func TestClientReadPacketOK(t *testing.T) {
	cl := genClient()
	cl.Start()
	defer cl.Stop()

	err := cl.r.Set([]byte{
		byte(packets.Publish << 4), 11, // Fixed header
		0, 5,
		'd', '/', 'e', '/', 'f',
		'y', 'e', 'a', 'h',
	}, 0, 13)
	require.NoError(t, err)
	cl.r.SetPos(0, 13)

	fh := new(packets.FixedHeader)
	err = cl.ReadFixedHeader(fh)
	require.NoError(t, err)

	pk, err := cl.ReadPacket(fh)
	require.NoError(t, err)
	require.NotNil(t, pk)

	require.Equal(t, packets.Packet{
		FixedHeader: packets.FixedHeader{
			Type:      packets.Publish,
			Remaining: 11,
		},
		TopicName: "d/e/f",
		Payload:   []byte("yeah"),
	}, pk)
}

func TestClientReadPacket(t *testing.T) {
	cl := genClient()
	cl.Start()
	defer cl.Stop()

	for i, tt := range pkTable {
		err := cl.r.Set(tt.bytes, 0, len(tt.bytes))
		require.NoError(t, err)
		cl.r.SetPos(0, int64(len(tt.bytes)))

		fh := new(packets.FixedHeader)
		err = cl.ReadFixedHeader(fh)
		require.NoError(t, err)

		pk, err := cl.ReadPacket(fh)
		require.NoError(t, err)
		require.NotNil(t, pk)

		require.Equal(t, tt.packet, pk, "Mismatched packet: [i:%d] %d", i, tt.bytes[0])
		if tt.packet.FixedHeader.Type == packets.Publish {
			require.Equal(t, int64(1), atomic.LoadInt64(&cl.systemInfo.PublishRecv))
		}
	}
}

func TestClientReadPacketReadingError(t *testing.T) {
	cl := genClient()
	cl.Start()
	defer cl.Stop()

	err := cl.r.Set([]byte{
		0, 11, // Fixed header
		0, 5,
		'd', '/', 'e', '/', 'f',
		'y', 'e', 'a', 'h',
	}, 0, 13)
	require.NoError(t, err)
	cl.r.SetPos(2, 13)

	_, err = cl.ReadPacket(&packets.FixedHeader{
		Type:      0,
		Remaining: 11,
	})
	require.Error(t, err)
}

func TestClientReadPacketReadError(t *testing.T) {
	cl := genClient()
	cl.Start()
	defer cl.Stop()
	cl.r.Stop()

	_, err := cl.ReadPacket(&packets.FixedHeader{
		Remaining: 1,
	})
	require.Error(t, err)
	require.Equal(t, io.EOF, err)
}

func TestClientReadPacketReadUnknown(t *testing.T) {
	cl := genClient()
	cl.Start()
	defer cl.Stop()
	cl.r.Stop()

	_, err := cl.ReadPacket(&packets.FixedHeader{
		Remaining: 1,
	})
	require.Error(t, err)
}

func TestClientWritePacket(t *testing.T) {
	for i, tt := range pkTable {
		r, w := net.Pipe()
		cl := NewClient(r, circ.NewReader(128, 8), circ.NewWriter(128, 8), new(system.Info))
		cl.Start()

		o := make(chan []byte)
		go func() {
			buf, err := ioutil.ReadAll(w)
			require.NoError(t, err)
			o <- buf
		}()

		n, err := cl.WritePacket(tt.packet)
		require.NoError(t, err, "Error [i:%d] %d", i, tt.packet)
		require.Equal(t, len(tt.bytes), n, "Mismatched written [i:%d] %d", i, tt.packet)

		time.Sleep(2 * time.Millisecond)
		r.Close()

		require.Equal(t, tt.bytes, <-o, "Mismatched packet: [i:%d] %d", i, tt.bytes[0])
		cl.Stop()
		require.Equal(t, int64(n), atomic.LoadInt64(&cl.systemInfo.BytesSent))
		require.Equal(t, int64(1), atomic.LoadInt64(&cl.systemInfo.MessagesSent))
		if tt.packet.FixedHeader.Type == packets.Publish {
			require.Equal(t, int64(1), atomic.LoadInt64(&cl.systemInfo.PublishSent))
		}
	}
}

func TestClientWritePacketWriteNoConn(t *testing.T) {
	c, _ := net.Pipe()
	cl := NewClient(c, circ.NewReader(16, 4), circ.NewWriter(16, 4), new(system.Info))
	cl.w.SetPos(0, 16)
	cl.Stop()

	_, err := cl.WritePacket(pkTable[1].packet)
	require.Error(t, err)
	require.Equal(t, ErrConnectionClosed, err)
}

func TestClientWritePacketWriteError(t *testing.T) {
	c, _ := net.Pipe()
	cl := NewClient(c, circ.NewReader(16, 4), circ.NewWriter(16, 4), new(system.Info))
	cl.w.SetPos(0, 16)
	cl.w.Stop()

	_, err := cl.WritePacket(pkTable[1].packet)
	require.Error(t, err)
}

func TestClientWritePacketInvalidPacket(t *testing.T) {
	c, _ := net.Pipe()
	cl := NewClient(c, circ.NewReader(16, 4), circ.NewWriter(16, 4), new(system.Info))
	cl.Start()

	_, err := cl.WritePacket(packets.Packet{})
	require.Error(t, err)
}

/////

func TestInflightSet(t *testing.T) {
	cl := genClient()
	q := cl.Inflight.Set(1, InflightMessage{Packet: packets.Packet{}, Sent: 0})
	require.Equal(t, true, q)
	require.NotNil(t, cl.Inflight.internal[1])
	require.NotEqual(t, 0, cl.Inflight.internal[1].Sent)

	q = cl.Inflight.Set(1, InflightMessage{Packet: packets.Packet{}, Sent: 0})
	require.Equal(t, false, q)
}

func BenchmarkInflightSet(b *testing.B) {
	cl := genClient()
	in := InflightMessage{Packet: packets.Packet{}, Sent: 0}
	for n := 0; n < b.N; n++ {
		cl.Inflight.Set(1, in)
	}
}

func TestInflightGet(t *testing.T) {
	cl := genClient()
	cl.Inflight.Set(2, InflightMessage{Packet: packets.Packet{}, Sent: 0})

	msg, ok := cl.Inflight.Get(2)
	require.Equal(t, true, ok)
	require.NotEqual(t, 0, msg.Sent)
}

func BenchmarkInflightGet(b *testing.B) {
	cl := genClient()
	cl.Inflight.Set(2, InflightMessage{Packet: packets.Packet{}, Sent: 0})
	for n := 0; n < b.N; n++ {
		cl.Inflight.Get(2)
	}
}

func TestInflightGetAll(t *testing.T) {
	cl := genClient()
	cl.Inflight.Set(2, InflightMessage{})

	m := cl.Inflight.GetAll()
	o := map[uint16]InflightMessage{
		2: InflightMessage{},
	}
	require.Equal(t, o, m)
}

func BenchmarkInflightGetAll(b *testing.B) {
	cl := genClient()
	cl.Inflight.Set(2, InflightMessage{Packet: packets.Packet{}, Sent: 0})
	for n := 0; n < b.N; n++ {
		cl.Inflight.Get(2)
	}
}

func TestInflightLen(t *testing.T) {
	cl := genClient()
	cl.Inflight.Set(2, InflightMessage{Packet: packets.Packet{}, Sent: 0})
	require.Equal(t, 1, cl.Inflight.Len())
}

func BenchmarkInflightLen(b *testing.B) {
	cl := genClient()
	cl.Inflight.Set(2, InflightMessage{Packet: packets.Packet{}, Sent: 0})
	for n := 0; n < b.N; n++ {
		cl.Inflight.Len()
	}
}

func TestInflightDelete(t *testing.T) {
	cl := genClient()
	cl.Inflight.Set(3, InflightMessage{Packet: packets.Packet{}, Sent: 0})
	require.NotNil(t, cl.Inflight.internal[3])

	q := cl.Inflight.Delete(3)
	require.Equal(t, true, q)
	require.Equal(t, int64(0), cl.Inflight.internal[3].Sent)

	_, ok := cl.Inflight.Get(3)
	require.Equal(t, false, ok)

	q = cl.Inflight.Delete(3)
	require.Equal(t, false, q)
}

func BenchmarkInflightDelete(b *testing.B) {
	cl := genClient()
	for n := 0; n < b.N; n++ {
		cl.Inflight.Set(4, InflightMessage{Packet: packets.Packet{}, Sent: 0})
		cl.Inflight.Delete(4)
	}
}

var (
	pkTable = []struct {
		bytes  []byte
		packet packets.Packet
	}{
		{
			bytes: []byte{
				byte(packets.Connect << 4), 16, // Fixed header
				0, 4, // Protocol Name - MSB+LSB
				'M', 'Q', 'T', 'T', // Protocol Name
				4,     // Protocol Version
				0,     // Packet Flags
				0, 60, // Keepalive
				0, 4, // Client ID - MSB+LSB
				'z', 'e', 'n', '3',
			},
			packet: packets.Packet{
				FixedHeader: packets.FixedHeader{
					Type:      packets.Connect,
					Remaining: 16,
				},
				ProtocolName:     []byte("MQTT"),
				ProtocolVersion:  4,
				CleanSession:     false,
				Keepalive:        60,
				ClientIdentifier: "zen3",
			},
		},
		{
			bytes: []byte{
				byte(packets.Connack << 4), 2,
				0,
				packets.Accepted,
			},
			packet: packets.Packet{
				FixedHeader: packets.FixedHeader{
					Type:      packets.Connack,
					Remaining: 2,
				},
				SessionPresent: false,
				ReturnCode:     packets.Accepted,
			},
		},
		{
			bytes: []byte{
				byte(packets.Publish << 4), 18,
				0, 5,
				'a', '/', 'b', '/', 'c',
				'h', 'e', 'l', 'l', 'o', ' ', 'm', 'o', 'c', 'h', 'i',
			},
			packet: packets.Packet{
				FixedHeader: packets.FixedHeader{
					Type:      packets.Publish,
					Remaining: 18,
				},
				TopicName: "a/b/c",
				Payload:   []byte("hello mochi"),
			},
		},
		{
			bytes: []byte{
				byte(packets.Puback << 4), 2, // Fixed header
				0, 11, // Packet ID - LSB+MSB
			},
			packet: packets.Packet{
				FixedHeader: packets.FixedHeader{
					Type:      packets.Puback,
					Remaining: 2,
				},
				PacketID: 11,
			},
		},
		{
			bytes: []byte{
				byte(packets.Pubrec << 4), 2, // Fixed header
				0, 12, // Packet ID - LSB+MSB
			},
			packet: packets.Packet{
				FixedHeader: packets.FixedHeader{
					Type:      packets.Pubrec,
					Remaining: 2,
				},
				PacketID: 12,
			},
		},
		{
			bytes: []byte{
				byte(packets.Pubrel<<4) | 2, 2, // Fixed header
				0, 12, // Packet ID - LSB+MSB
			},
			packet: packets.Packet{
				FixedHeader: packets.FixedHeader{
					Type:      packets.Pubrel,
					Remaining: 2,
					Qos:       1,
				},
				PacketID: 12,
			},
		},
		{
			bytes: []byte{
				byte(packets.Pubcomp << 4), 2, // Fixed header
				0, 14, // Packet ID - LSB+MSB
			},
			packet: packets.Packet{
				FixedHeader: packets.FixedHeader{
					Type:      packets.Pubcomp,
					Remaining: 2,
				},
				PacketID: 14,
			},
		},
		{
			bytes: []byte{
				byte(packets.Subscribe << 4), 30, // Fixed header
				0, 15, // Packet ID - LSB+MSB

				0, 3, // Topic Name - LSB+MSB
				'a', '/', 'b', // Topic Name
				0, // QoS

				0, 11, // Topic Name - LSB+MSB
				'd', '/', 'e', '/', 'f', '/', 'g', '/', 'h', '/', 'i', // Topic Name
				1, // QoS

				0, 5, // Topic Name - LSB+MSB
				'x', '/', 'y', '/', 'z', // Topic Name
				2, // QoS
			},
			packet: packets.Packet{
				FixedHeader: packets.FixedHeader{
					Type:      packets.Subscribe,
					Remaining: 30,
				},
				PacketID: 15,
				Topics: []string{
					"a/b",
					"d/e/f/g/h/i",
					"x/y/z",
				},
				Qoss: []byte{0, 1, 2},
			},
		},
		{
			bytes: []byte{
				byte(packets.Suback << 4), 6, // Fixed header
				0, 17, // Packet ID - LSB+MSB
				0,    // Return Code QoS 0
				1,    // Return Code QoS 1
				2,    // Return Code QoS 2
				0x80, // Return Code fail
			},
			packet: packets.Packet{
				FixedHeader: packets.FixedHeader{
					Type:      packets.Suback,
					Remaining: 6,
				},
				PacketID:    17,
				ReturnCodes: []byte{0, 1, 2, 0x80},
			},
		},
		{
			bytes: []byte{
				byte(packets.Unsubscribe << 4), 27, // Fixed header
				0, 35, // Packet ID - LSB+MSB

				0, 3, // Topic Name - LSB+MSB
				'a', '/', 'b', // Topic Name

				0, 11, // Topic Name - LSB+MSB
				'd', '/', 'e', '/', 'f', '/', 'g', '/', 'h', '/', 'i', // Topic Name

				0, 5, // Topic Name - LSB+MSB
				'x', '/', 'y', '/', 'z', // Topic Name
			},
			packet: packets.Packet{
				FixedHeader: packets.FixedHeader{
					Type:      packets.Unsubscribe,
					Remaining: 27,
				},
				PacketID: 35,
				Topics: []string{
					"a/b",
					"d/e/f/g/h/i",
					"x/y/z",
				},
			},
		},
		{
			bytes: []byte{
				byte(packets.Unsuback << 4), 2, // Fixed header
				0, 37, // Packet ID - LSB+MSB

			},
			packet: packets.Packet{
				FixedHeader: packets.FixedHeader{
					Type:      packets.Unsuback,
					Remaining: 2,
				},
				PacketID: 37,
			},
		},
		{
			bytes: []byte{
				byte(packets.Pingreq << 4), 0, // fixed header
			},
			packet: packets.Packet{
				FixedHeader: packets.FixedHeader{
					Type:      packets.Pingreq,
					Remaining: 0,
				},
			},
		},
		{
			bytes: []byte{
				byte(packets.Pingresp << 4), 0, // fixed header
			},
			packet: packets.Packet{
				FixedHeader: packets.FixedHeader{
					Type:      packets.Pingresp,
					Remaining: 0,
				},
			},
		},
		{
			bytes: []byte{
				byte(packets.Disconnect << 4), 0, // fixed header
			},
			packet: packets.Packet{
				FixedHeader: packets.FixedHeader{
					Type:      packets.Disconnect,
					Remaining: 0,
				},
			},
		},
	}
)
