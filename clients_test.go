// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-co
// SPDX-FileContributor: mochi-co

package mqtt

import (
	"errors"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mochi-co/mqtt/v2/packets"
	"github.com/mochi-co/mqtt/v2/system"

	"github.com/stretchr/testify/require"
)

const pkInfo = "packet type %v, %s"

var errClientStop = errors.New("test stop")

func newTestClient() (cl *Client, r net.Conn, w net.Conn) {
	r, w = net.Pipe()

	cl = newClient(w, &ops{
		info:  new(system.Info),
		hooks: new(Hooks),
		log:   &logger,
		options: &Options{
			Capabilities: &Capabilities{
				ReceiveMaximum:             10,
				TopicAliasMaximum:          10000,
				MaximumClientWritesPending: 3,
				maximumPacketID:            10,
			},
		},
	})

	cl.ID = "mochi"
	cl.State.Inflight.maximumSendQuota = 5
	cl.State.Inflight.sendQuota = 5
	cl.State.Inflight.maximumReceiveQuota = 10
	cl.State.Inflight.receiveQuota = 10
	cl.Properties.Props.TopicAliasMaximum = 0
	cl.Properties.Props.RequestResponseInfo = 0x1

	go cl.WriteLoop()

	return
}

func TestNewInflights(t *testing.T) {
	require.NotNil(t, NewInflights().internal)
}

func TestNewClients(t *testing.T) {
	cl := NewClients()
	require.NotNil(t, cl.internal)
}

func TestClientsAdd(t *testing.T) {
	cl := NewClients()
	cl.Add(&Client{ID: "t1"})
	require.Contains(t, cl.internal, "t1")
}

func TestClientsGet(t *testing.T) {
	cl := NewClients()
	cl.Add(&Client{ID: "t1"})
	cl.Add(&Client{ID: "t2"})
	require.Contains(t, cl.internal, "t1")
	require.Contains(t, cl.internal, "t2")

	client, ok := cl.Get("t1")
	require.Equal(t, true, ok)
	require.Equal(t, "t1", client.ID)
}

func TestClientsGetAll(t *testing.T) {
	cl := NewClients()
	cl.Add(&Client{ID: "t1"})
	cl.Add(&Client{ID: "t2"})
	cl.Add(&Client{ID: "t3"})
	require.Contains(t, cl.internal, "t1")
	require.Contains(t, cl.internal, "t2")
	require.Contains(t, cl.internal, "t3")

	clients := cl.GetAll()
	require.Len(t, clients, 3)
}

func TestClientsLen(t *testing.T) {
	cl := NewClients()
	cl.Add(&Client{ID: "t1"})
	cl.Add(&Client{ID: "t2"})
	require.Contains(t, cl.internal, "t1")
	require.Contains(t, cl.internal, "t2")
	require.Equal(t, 2, cl.Len())
}

func TestClientsDelete(t *testing.T) {
	cl := NewClients()
	cl.Add(&Client{ID: "t1"})
	require.Contains(t, cl.internal, "t1")

	cl.Delete("t1")
	_, ok := cl.Get("t1")
	require.Equal(t, false, ok)
	require.Nil(t, cl.internal["t1"])
}

func TestClientsGetByListener(t *testing.T) {
	cl := NewClients()
	cl.Add(&Client{ID: "t1", Net: ClientConnection{Listener: "tcp1"}})
	cl.Add(&Client{ID: "t2", Net: ClientConnection{Listener: "ws1"}})
	require.Contains(t, cl.internal, "t1")
	require.Contains(t, cl.internal, "t2")

	clients := cl.GetByListener("tcp1")
	require.NotEmpty(t, clients)
	require.Equal(t, 1, len(clients))
	require.Equal(t, "tcp1", clients[0].Net.Listener)
}

func TestNewClient(t *testing.T) {
	cl, _, _ := newTestClient()

	require.NotNil(t, cl)
	require.NotNil(t, cl.State.Inflight.internal)
	require.NotNil(t, cl.State.Subscriptions)
	require.NotNil(t, cl.State.TopicAliases)
	require.Equal(t, defaultKeepalive, cl.State.keepalive)
	require.Equal(t, defaultClientProtocolVersion, cl.Properties.ProtocolVersion)
	require.NotNil(t, cl.Net.Conn)
	require.NotNil(t, cl.Net.bconn)
	require.NotNil(t, cl.ops)
	require.NotNil(t, cl.ops.options.Capabilities)
	require.False(t, cl.Net.Inline)
}

func TestClientParseConnect(t *testing.T) {
	cl, _, _ := newTestClient()

	pk := packets.Packet{
		ProtocolVersion: 4,
		Connect: packets.ConnectParams{
			ProtocolName:     []byte{'M', 'Q', 'T', 'T'},
			Clean:            true,
			Keepalive:        60,
			ClientIdentifier: "mochi",
			WillFlag:         true,
			WillTopic:        "lwt",
			WillPayload:      []byte("lol gg"),
			WillQos:          1,
			WillRetain:       false,
		},
		Properties: packets.Properties{
			ReceiveMaximum: uint16(5),
		},
	}

	cl.ParseConnect("tcp1", pk)
	require.Equal(t, pk.Connect.ClientIdentifier, cl.ID)
	require.Equal(t, pk.Connect.Keepalive, cl.State.keepalive)
	require.Equal(t, pk.Connect.Clean, cl.Properties.Clean)
	require.Equal(t, pk.Connect.ClientIdentifier, cl.ID)
	require.Equal(t, pk.Connect.WillTopic, cl.Properties.Will.TopicName)
	require.Equal(t, pk.Connect.WillPayload, cl.Properties.Will.Payload)
	require.Equal(t, pk.Connect.WillQos, cl.Properties.Will.Qos)
	require.Equal(t, pk.Connect.WillRetain, cl.Properties.Will.Retain)
	require.Equal(t, uint32(1), cl.Properties.Will.Flag)
	require.Equal(t, int32(cl.ops.options.Capabilities.ReceiveMaximum), cl.State.Inflight.receiveQuota)
	require.Equal(t, int32(cl.ops.options.Capabilities.ReceiveMaximum), cl.State.Inflight.maximumReceiveQuota)
	require.Equal(t, int32(pk.Properties.ReceiveMaximum), cl.State.Inflight.sendQuota)
	require.Equal(t, int32(pk.Properties.ReceiveMaximum), cl.State.Inflight.maximumSendQuota)
}

func TestClientParseConnectOverrideWillDelay(t *testing.T) {
	cl, _, _ := newTestClient()

	pk := packets.Packet{
		ProtocolVersion: 4,
		Connect: packets.ConnectParams{
			ProtocolName:     []byte{'M', 'Q', 'T', 'T'},
			Clean:            true,
			Keepalive:        60,
			ClientIdentifier: "mochi",
			WillFlag:         true,
			WillProperties: packets.Properties{
				WillDelayInterval: 200,
			},
		},
		Properties: packets.Properties{
			SessionExpiryInterval:     100,
			SessionExpiryIntervalFlag: true,
		},
	}

	cl.ParseConnect("tcp1", pk)
	require.Equal(t, pk.Properties.SessionExpiryInterval, cl.Properties.Will.WillDelayInterval)
}

func TestClientParseConnectNoID(t *testing.T) {
	cl, _, _ := newTestClient()
	cl.ParseConnect("tcp1", packets.Packet{})
	require.NotEmpty(t, cl.ID)
}

func TestClientNextPacketID(t *testing.T) {
	cl, _, _ := newTestClient()

	i, err := cl.NextPacketID()
	require.NoError(t, err)
	require.Equal(t, uint32(1), i)

	i, err = cl.NextPacketID()
	require.NoError(t, err)
	require.Equal(t, uint32(2), i)
}

func TestClientNextPacketIDInUse(t *testing.T) {
	cl, _, _ := newTestClient()

	// skip over 2
	cl.State.Inflight.Set(packets.Packet{PacketID: 2})

	i, err := cl.NextPacketID()
	require.NoError(t, err)
	require.Equal(t, uint32(1), i)

	i, err = cl.NextPacketID()
	require.NoError(t, err)
	require.Equal(t, uint32(3), i)

	// Skip over overflow
	cl.State.Inflight.Set(packets.Packet{PacketID: 65535})
	atomic.StoreUint32(&cl.State.packetID, 65534)

	i, err = cl.NextPacketID()
	require.NoError(t, err)
	require.Equal(t, uint32(1), i)
}

func TestClientNextPacketIDExhausted(t *testing.T) {
	cl, _, _ := newTestClient()
	for i := uint32(1); i <= cl.ops.options.Capabilities.maximumPacketID; i++ {
		cl.State.Inflight.internal[uint16(i)] = packets.Packet{PacketID: uint16(i)}
	}

	i, err := cl.NextPacketID()
	require.Error(t, err)
	require.ErrorIs(t, err, packets.ErrQuotaExceeded)
	require.Equal(t, uint32(0), i)
}

func TestClientNextPacketIDOverflow(t *testing.T) {
	cl, _, _ := newTestClient()
	for i := uint32(0); i < cl.ops.options.Capabilities.maximumPacketID; i++ {
		cl.State.Inflight.internal[uint16(i)] = packets.Packet{}
	}

	cl.State.packetID = uint32(cl.ops.options.Capabilities.maximumPacketID - 1)
	i, err := cl.NextPacketID()
	require.NoError(t, err)
	require.Equal(t, cl.ops.options.Capabilities.maximumPacketID, i)
	cl.State.Inflight.internal[uint16(cl.ops.options.Capabilities.maximumPacketID)] = packets.Packet{}

	cl.State.packetID = cl.ops.options.Capabilities.maximumPacketID
	_, err = cl.NextPacketID()
	require.Error(t, err)
	require.ErrorIs(t, err, packets.ErrQuotaExceeded)
}

func TestClientClearInflights(t *testing.T) {
	cl, _, _ := newTestClient()

	n := time.Now().Unix()
	cl.State.Inflight.Set(packets.Packet{PacketID: 1, Expiry: n - 1})
	cl.State.Inflight.Set(packets.Packet{PacketID: 2, Expiry: n - 2})
	cl.State.Inflight.Set(packets.Packet{PacketID: 3, Created: n - 3}) // within bounds
	cl.State.Inflight.Set(packets.Packet{PacketID: 5, Created: n - 5}) // over max server expiry limit
	cl.State.Inflight.Set(packets.Packet{PacketID: 7, Created: n})
	require.Equal(t, 5, cl.State.Inflight.Len())

	deleted := cl.ClearInflights(n, 4)
	require.Len(t, deleted, 3)
	require.ElementsMatch(t, []uint16{1, 2, 5}, deleted)
	require.Equal(t, 2, cl.State.Inflight.Len())
}

func TestClientResendInflightMessages(t *testing.T) {
	pk1 := packets.TPacketData[packets.Puback].Get(packets.TPuback)
	cl, r, w := newTestClient()

	cl.State.Inflight.Set(*pk1.Packet)
	require.Equal(t, 1, cl.State.Inflight.Len())

	go func() {
		err := cl.ResendInflightMessages(true)
		require.NoError(t, err)
		time.Sleep(time.Millisecond)
		w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, 0, cl.State.Inflight.Len())
	require.Equal(t, pk1.RawBytes, buf)
}

func TestClientResendInflightMessagesWriteFailure(t *testing.T) {
	pk1 := packets.TPacketData[packets.Publish].Get(packets.TPublishQos1Dup)
	cl, r, _ := newTestClient()
	r.Close()

	cl.State.Inflight.Set(*pk1.Packet)
	require.Equal(t, 1, cl.State.Inflight.Len())
	err := cl.ResendInflightMessages(true)
	require.Error(t, err)
	require.ErrorIs(t, err, io.ErrClosedPipe)
	require.Equal(t, 1, cl.State.Inflight.Len())
}

func TestClientResendInflightMessagesNoMessages(t *testing.T) {
	cl, _, _ := newTestClient()
	err := cl.ResendInflightMessages(true)
	require.NoError(t, err)
}

func TestClientRefreshDeadline(t *testing.T) {
	cl, _, _ := newTestClient()
	cl.refreshDeadline(10)
	require.NotNil(t, cl.Net.Conn) // how do we check net.Conn deadline?
}

func TestClientReadFixedHeader(t *testing.T) {
	cl, r, _ := newTestClient()

	defer cl.Stop(errClientStop)
	go func() {
		r.Write([]byte{packets.Connect << 4, 0x00})
		r.Close()
	}()

	fh := new(packets.FixedHeader)
	err := cl.ReadFixedHeader(fh)
	require.NoError(t, err)
	require.Equal(t, int64(2), atomic.LoadInt64(&cl.ops.info.BytesReceived))
}

func TestClientReadFixedHeaderDecodeError(t *testing.T) {
	cl, r, _ := newTestClient()
	defer cl.Stop(errClientStop)

	go func() {
		r.Write([]byte{packets.Connect<<4 | 1<<1, 0x00, 0x00})
		r.Close()
	}()

	fh := new(packets.FixedHeader)
	err := cl.ReadFixedHeader(fh)
	require.Error(t, err)
}

func TestClientReadFixedHeaderPacketOversized(t *testing.T) {
	cl, r, _ := newTestClient()
	cl.ops.options.Capabilities.MaximumPacketSize = 2
	defer cl.Stop(errClientStop)

	go func() {
		r.Write(packets.TPacketData[packets.Publish].Get(packets.TPublishQos1Dup).RawBytes)
		r.Close()
	}()

	fh := new(packets.FixedHeader)
	err := cl.ReadFixedHeader(fh)
	require.Error(t, err)
	require.ErrorIs(t, err, packets.ErrPacketTooLarge)
}

func TestClientReadFixedHeaderReadEOF(t *testing.T) {
	cl, r, _ := newTestClient()
	defer cl.Stop(errClientStop)

	go func() {
		r.Close()
	}()

	fh := new(packets.FixedHeader)
	err := cl.ReadFixedHeader(fh)
	require.Error(t, err)
	require.Equal(t, io.EOF, err)
}

func TestClientReadFixedHeaderNoLengthTerminator(t *testing.T) {
	cl, r, _ := newTestClient()
	defer cl.Stop(errClientStop)

	go func() {
		r.Write([]byte{packets.Connect << 4, 0xd5, 0x86, 0xf9, 0x9e, 0x01})
		r.Close()
	}()

	fh := new(packets.FixedHeader)
	err := cl.ReadFixedHeader(fh)
	require.Error(t, err)
}

func TestClientReadOK(t *testing.T) {
	cl, r, _ := newTestClient()
	defer cl.Stop(errClientStop)
	go func() {
		r.Write([]byte{
			packets.Publish << 4, 18, // Fixed header
			0, 5, // Topic Name - LSB+MSB
			'a', '/', 'b', '/', 'c', // Topic Name
			'h', 'e', 'l', 'l', 'o', ' ', 'm', 'o', 'c', 'h', 'i', // Payload,
			packets.Publish << 4, 11, // Fixed header
			0, 5, // Topic Name - LSB+MSB
			'd', '/', 'e', '/', 'f', // Topic Name
			'y', 'e', 'a', 'h', // Payload
		})
		r.Close()
	}()

	var pks []packets.Packet
	o := make(chan error)
	go func() {
		o <- cl.Read(func(cl *Client, pk packets.Packet) error {
			pks = append(pks, pk)
			return nil
		})
	}()

	err := <-o
	require.Error(t, err)
	require.ErrorIs(t, err, io.EOF)
	require.Equal(t, 2, len(pks))
	require.Equal(t, []packets.Packet{
		{
			ProtocolVersion: cl.Properties.ProtocolVersion,
			FixedHeader: packets.FixedHeader{
				Type:      packets.Publish,
				Remaining: 18,
			},
			TopicName: "a/b/c",
			Payload:   []byte("hello mochi"),
		},
		{
			ProtocolVersion: cl.Properties.ProtocolVersion,
			FixedHeader: packets.FixedHeader{
				Type:      packets.Publish,
				Remaining: 11,
			},
			TopicName: "d/e/f",
			Payload:   []byte("yeah"),
		},
	}, pks)

	require.Equal(t, int64(2), atomic.LoadInt64(&cl.ops.info.MessagesReceived))
}

func TestClientReadDone(t *testing.T) {
	cl, _, _ := newTestClient()
	defer cl.Stop(errClientStop)
	cl.State.done = 1

	o := make(chan error)
	go func() {
		o <- cl.Read(func(cl *Client, pk packets.Packet) error {
			return nil
		})
	}()

	require.NoError(t, <-o)
}

func TestClientStop(t *testing.T) {
	cl, _, _ := newTestClient()
	cl.Stop(nil)
	require.Equal(t, nil, cl.State.stopCause.Load())
	require.Equal(t, time.Now().Unix(), cl.State.disconnected)
	require.Equal(t, uint32(1), cl.State.done)
	require.Equal(t, nil, cl.StopCause())
}

func TestClientClosed(t *testing.T) {
	cl, _, _ := newTestClient()
	require.False(t, cl.Closed())
	cl.Stop(nil)
	require.True(t, cl.Closed())
}

func TestClientReadFixedHeaderError(t *testing.T) {
	cl, r, _ := newTestClient()
	defer cl.Stop(errClientStop)
	go func() {
		r.Write([]byte{
			packets.Publish << 4, 11, // Fixed header
		})
		r.Close()
	}()

	cl.Net.bconn = nil
	fh := new(packets.FixedHeader)
	err := cl.ReadFixedHeader(fh)
	require.Error(t, err)
	require.ErrorIs(t, ErrConnectionClosed, err)
}

func TestClientReadReadHandlerErr(t *testing.T) {
	cl, r, _ := newTestClient()
	defer cl.Stop(errClientStop)
	go func() {
		r.Write([]byte{
			packets.Publish << 4, 11, // Fixed header
			0, 5, // Topic Name - LSB+MSB
			'd', '/', 'e', '/', 'f', // Topic Name
			'y', 'e', 'a', 'h', // Payload
		})
		r.Close()
	}()

	err := cl.Read(func(cl *Client, pk packets.Packet) error {
		return errors.New("test")
	})

	require.Error(t, err)
}

func TestClientReadReadPacketOK(t *testing.T) {
	cl, r, _ := newTestClient()
	defer cl.Stop(errClientStop)
	go func() {
		r.Write([]byte{
			packets.Publish << 4, 11, // Fixed header
			0, 5,
			'd', '/', 'e', '/', 'f',
			'y', 'e', 'a', 'h',
		})
		r.Close()
	}()

	fh := new(packets.FixedHeader)
	err := cl.ReadFixedHeader(fh)
	require.NoError(t, err)

	pk, err := cl.ReadPacket(fh)
	require.NoError(t, err)
	require.NotNil(t, pk)

	require.Equal(t, packets.Packet{
		ProtocolVersion: cl.Properties.ProtocolVersion,
		FixedHeader: packets.FixedHeader{
			Type:      packets.Publish,
			Remaining: 11,
		},
		TopicName: "d/e/f",
		Payload:   []byte("yeah"),
	}, pk)
}

func TestClientReadPacket(t *testing.T) {
	cl, r, _ := newTestClient()
	defer cl.Stop(errClientStop)

	for _, tx := range pkTable {
		tt := tx // avoid data race
		t.Run(tt.Desc, func(t *testing.T) {
			atomic.StoreInt64(&cl.ops.info.PacketsReceived, 0)
			go func() {
				r.Write(tt.RawBytes)
			}()

			fh := new(packets.FixedHeader)
			err := cl.ReadFixedHeader(fh)
			require.NoError(t, err)

			if tt.Packet.ProtocolVersion == 5 {
				cl.Properties.ProtocolVersion = 5
			} else {
				cl.Properties.ProtocolVersion = 0
			}

			pk, err := cl.ReadPacket(fh)
			require.NoError(t, err, pkInfo, tt.Case, tt.Desc)
			require.NotNil(t, pk, pkInfo, tt.Case, tt.Desc)
			require.Equal(t, *tt.Packet, pk, pkInfo, tt.Case, tt.Desc)

			if tt.Packet.FixedHeader.Type == packets.Publish {
				require.Equal(t, int64(1), atomic.LoadInt64(&cl.ops.info.PacketsReceived), pkInfo, tt.Case, tt.Desc)
			}
		})
	}
}

func TestClientReadPacketInvalidTypeError(t *testing.T) {
	cl, _, _ := newTestClient()
	cl.Net.Conn.Close()
	_, err := cl.ReadPacket(&packets.FixedHeader{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid packet type")
}

func TestClientWritePacket(t *testing.T) {
	for _, tt := range pkTable {
		cl, r, _ := newTestClient()
		defer cl.Stop(errClientStop)

		cl.Properties.ProtocolVersion = tt.Packet.ProtocolVersion

		o := make(chan []byte)
		go func() {
			buf, err := io.ReadAll(r)
			require.NoError(t, err)
			o <- buf
		}()

		err := cl.WritePacket(*tt.Packet)
		require.NoError(t, err, pkInfo, tt.Case, tt.Desc)

		time.Sleep(2 * time.Millisecond)
		cl.Net.Conn.Close()

		require.Equal(t, tt.RawBytes, <-o, pkInfo, tt.Case, tt.Desc)

		cl.Stop(errClientStop)
		time.Sleep(time.Millisecond * 1)

		// The stop cause is either the test error, EOF, or a
		// closed pipe, depending on which goroutine runs first.
		err = cl.StopCause()
		require.True(t,
			errors.Is(err, errClientStop) ||
				errors.Is(err, io.EOF) ||
				errors.Is(err, io.ErrClosedPipe))

		require.Equal(t, int64(len(tt.RawBytes)), atomic.LoadInt64(&cl.ops.info.BytesSent))
		require.Equal(t, int64(1), atomic.LoadInt64(&cl.ops.info.PacketsSent))
		if tt.Packet.FixedHeader.Type == packets.Publish {
			require.Equal(t, int64(1), atomic.LoadInt64(&cl.ops.info.MessagesSent))
		}
	}
}

func TestWriteClientOversizePacket(t *testing.T) {
	cl, _, _ := newTestClient()
	cl.Properties.Props.MaximumPacketSize = 2
	pk := *packets.TPacketData[packets.Publish].Get(packets.TPublishDropOversize).Packet
	err := cl.WritePacket(pk)
	require.Error(t, err)
	require.ErrorIs(t, packets.ErrPacketTooLarge, err)
}

func TestClientReadPacketReadingError(t *testing.T) {
	cl, r, _ := newTestClient()
	defer cl.Stop(errClientStop)
	go func() {
		r.Write([]byte{
			0, 11, // Fixed header
			0, 5,
			'd', '/', 'e', '/', 'f',
			'y', 'e', 'a', 'h',
		})
		r.Close()
	}()

	_, err := cl.ReadPacket(&packets.FixedHeader{
		Type:      0,
		Remaining: 11,
	})
	require.Error(t, err)
}

func TestClientReadPacketReadUnknown(t *testing.T) {
	cl, r, _ := newTestClient()
	defer cl.Stop(errClientStop)
	go func() {
		r.Write([]byte{
			0, 11, // Fixed header
			0, 5,
			'd', '/', 'e', '/', 'f',
			'y', 'e', 'a', 'h',
		})
		r.Close()
	}()

	_, err := cl.ReadPacket(&packets.FixedHeader{
		Remaining: 1,
	})
	require.Error(t, err)
}

func TestClientWritePacketWriteNoConn(t *testing.T) {
	cl, _, _ := newTestClient()
	cl.Stop(errClientStop)

	err := cl.WritePacket(*pkTable[1].Packet)
	require.Error(t, err)
	require.Equal(t, ErrConnectionClosed, err)
}

func TestClientWritePacketWriteError(t *testing.T) {
	cl, _, _ := newTestClient()
	cl.Net.Conn.Close()

	err := cl.WritePacket(*pkTable[1].Packet)
	require.Error(t, err)
}

func TestClientWritePacketInvalidPacket(t *testing.T) {
	cl, _, _ := newTestClient()
	err := cl.WritePacket(packets.Packet{})
	require.Error(t, err)
}

var (
	pkTable = []packets.TPacketCase{
		packets.TPacketData[packets.Connect].Get(packets.TConnectMqtt311),
		packets.TPacketData[packets.Connack].Get(packets.TConnackAcceptedMqtt5),
		packets.TPacketData[packets.Connack].Get(packets.TConnackAcceptedNoSession),
		packets.TPacketData[packets.Publish].Get(packets.TPublishBasic),
		packets.TPacketData[packets.Publish].Get(packets.TPublishMqtt5),
		packets.TPacketData[packets.Puback].Get(packets.TPuback),
		packets.TPacketData[packets.Pubrec].Get(packets.TPubrec),
		packets.TPacketData[packets.Pubrel].Get(packets.TPubrel),
		packets.TPacketData[packets.Pubcomp].Get(packets.TPubcomp),
		packets.TPacketData[packets.Subscribe].Get(packets.TSubscribe),
		packets.TPacketData[packets.Subscribe].Get(packets.TSubscribeMqtt5),
		packets.TPacketData[packets.Suback].Get(packets.TSuback),
		packets.TPacketData[packets.Suback].Get(packets.TSubackMqtt5),
		packets.TPacketData[packets.Unsubscribe].Get(packets.TUnsubscribe),
		packets.TPacketData[packets.Unsubscribe].Get(packets.TUnsubscribeMqtt5),
		packets.TPacketData[packets.Unsuback].Get(packets.TUnsuback),
		packets.TPacketData[packets.Unsuback].Get(packets.TUnsubackMqtt5),
		packets.TPacketData[packets.Pingreq].Get(packets.TPingreq),
		packets.TPacketData[packets.Pingresp].Get(packets.TPingresp),
		packets.TPacketData[packets.Disconnect].Get(packets.TDisconnect),
		packets.TPacketData[packets.Disconnect].Get(packets.TDisconnectMqtt5),
		packets.TPacketData[packets.Auth].Get(packets.TAuth),
	}
)
