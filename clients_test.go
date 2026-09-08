// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package mqtt

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mochi-mqtt/server/v2/packets"
	"github.com/mochi-mqtt/server/v2/system"

	"github.com/stretchr/testify/require"
)

const pkInfo = "packet type %v, %s"

var errClientStop = errors.New("test stop")

func newTestClient() (cl *Client, r net.Conn, w net.Conn) {
	r, w = net.Pipe()

	cl = newClient(w, &ops{
		info:  new(system.Info),
		hooks: new(Hooks),
		log:   logger,
		options: &Options{
			Capabilities: &Capabilities{
				ReceiveMaximum:             10,
				MaximumInflight:            5,
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
	cl.Add(&Client{ID: "t1", State: ClientState{open: context.Background()}, Net: ClientConnection{Listener: "tcp1"}})
	cl.Add(&Client{ID: "t2", State: ClientState{open: context.Background()}, Net: ClientConnection{Listener: "ws1"}})
	require.Contains(t, cl.internal, "t1")
	require.Contains(t, cl.internal, "t2")

	clients := cl.GetByListener("tcp1")
	require.NotEmpty(t, clients)
	require.Equal(t, 1, len(clients))
	require.Equal(t, "tcp1", clients[0].Net.Listener)
}

// GetByListener must not hold the read lock across Len, which takes it
// again. sync.RWMutex is not reentrant for readers — "if a goroutine holds
// a RWMutex for reading and another goroutine might call Lock, no goroutine
// should expect to be able to acquire a read lock until the initial read
// lock is released" — so a writer arriving between the two acquisitions
// blocks the second one, and is itself waiting on the first.
//
// It is reached on every shutdown: Server.Close calls CloseAll, which calls
// closeListenerClients, which calls this, while attachClient calls Delete
// for any client still connecting. A server that deadlocks there never
// finishes closing, so whatever the application does after Close never
// runs.
//
// Against the recursive version this deadlocks and the test times out; a
// deadlock cannot be failed politely from inside the goroutines it stops,
// so the assertion is that the workers finish at all.
func TestGetByListenerDoesNotDeadlockAgainstAWriter(t *testing.T) {
	cl := NewClients()
	for i := 0; i < 8; i++ {
		cl.Add(&Client{
			ID:    fmt.Sprintf("t%d", i),
			State: ClientState{open: context.Background()},
			Net:   ClientConnection{Listener: "tcp1"},
		})
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for n := 0; n < 4; n++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					cl.GetByListener("tcp1")
				}
			}
		}()
		go func(n int) {
			defer wg.Done()
			id := fmt.Sprintf("w%d", n)
			for {
				select {
				case <-stop:
					return
				default:
					cl.Add(&Client{
						ID:    id,
						State: ClientState{open: context.Background()},
						Net:   ClientConnection{Listener: "tcp1"},
					})
					cl.Delete(id)
				}
			}
		}(n)
	}

	time.Sleep(time.Second)
	close(stop)

	done := make(chan struct{})
	go func() { defer close(done); wg.Wait() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("GetByListener deadlocked against a writer")
	}
}

func TestNewClient(t *testing.T) {
	cl, _, _ := newTestClient()

	require.NotNil(t, cl)
	require.NotNil(t, cl.State.Inflight.internal)
	require.NotNil(t, cl.State.Subscriptions)
	require.NotNil(t, cl.State.TopicAliases)
	require.Equal(t, defaultKeepalive, cl.State.Keepalive)
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
	require.Equal(t, pk.Connect.Keepalive, cl.State.Keepalive)
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

func TestClientParseConnectReceiveMaxExceedMaxInflight(t *testing.T) {
	const MaxInflight uint16 = 1
	cl, _, _ := newTestClient()
	cl.ops.options.Capabilities.MaximumInflight = MaxInflight

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
	require.Equal(t, pk.Connect.Keepalive, cl.State.Keepalive)
	require.Equal(t, pk.Connect.Clean, cl.Properties.Clean)
	require.Equal(t, pk.Connect.ClientIdentifier, cl.ID)
	require.Equal(t, pk.Connect.WillTopic, cl.Properties.Will.TopicName)
	require.Equal(t, pk.Connect.WillPayload, cl.Properties.Will.Payload)
	require.Equal(t, pk.Connect.WillQos, cl.Properties.Will.Qos)
	require.Equal(t, pk.Connect.WillRetain, cl.Properties.Will.Retain)
	require.Equal(t, uint32(1), cl.Properties.Will.Flag)
	require.Equal(t, int32(cl.ops.options.Capabilities.ReceiveMaximum), cl.State.Inflight.receiveQuota)
	require.Equal(t, int32(cl.ops.options.Capabilities.ReceiveMaximum), cl.State.Inflight.maximumReceiveQuota)
	require.Equal(t, int32(MaxInflight), cl.State.Inflight.sendQuota)
	require.Equal(t, int32(MaxInflight), cl.State.Inflight.maximumSendQuota)
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

func TestClientParseConnectBelowMinimumKeepalive(t *testing.T) {
	cl, _, _ := newTestClient()
	var b bytes.Buffer
	x := bufio.NewWriter(&b)
	cl.ops.log = slog.New(slog.NewTextHandler(x, nil))

	pk := packets.Packet{
		ProtocolVersion: 4,
		Connect: packets.ConnectParams{
			ProtocolName:     []byte{'M', 'Q', 'T', 'T'},
			Keepalive:        minimumKeepalive - 1,
			ClientIdentifier: "mochi",
		},
	}
	cl.ParseConnect("tcp1", pk)
	err := x.Flush()
	require.NoError(t, err)
	require.True(t, strings.Contains(b.String(), ErrMinimumKeepalive.Error()))
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

	cl.State.packetID = cl.ops.options.Capabilities.maximumPacketID - 1
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

	cl.State.Inflight.Set(packets.Packet{ProtocolVersion: 5, PacketID: 1, Expiry: n - 1})
	cl.State.Inflight.Set(packets.Packet{ProtocolVersion: 5, PacketID: 2, Expiry: n - 2})
	cl.State.Inflight.Set(packets.Packet{ProtocolVersion: 5, PacketID: 3, Created: n - 3}) // within bounds
	cl.State.Inflight.Set(packets.Packet{ProtocolVersion: 5, PacketID: 5, Created: n - 5}) // over max server expiry limit
	cl.State.Inflight.Set(packets.Packet{ProtocolVersion: 5, PacketID: 7, Created: n})

	require.Equal(t, 5, cl.State.Inflight.Len())
	cl.ClearInflights()
	require.Equal(t, 0, cl.State.Inflight.Len())
}

func TestClientClearExpiredInflights(t *testing.T) {
	cl, _, _ := newTestClient()

	n := time.Now().Unix()
	cl.State.Inflight.Set(packets.Packet{ProtocolVersion: 5, PacketID: 1, Expiry: n - 1})
	cl.State.Inflight.Set(packets.Packet{ProtocolVersion: 5, PacketID: 2, Expiry: n - 2})
	cl.State.Inflight.Set(packets.Packet{ProtocolVersion: 5, PacketID: 3, Created: n - 3}) // within bounds
	cl.State.Inflight.Set(packets.Packet{ProtocolVersion: 5, PacketID: 5, Created: n - 5}) // over max server expiry limit
	cl.State.Inflight.Set(packets.Packet{ProtocolVersion: 5, PacketID: 7, Created: n})
	require.Equal(t, 5, cl.State.Inflight.Len())

	deleted := cl.ClearExpiredInflights(n, 4)
	require.Len(t, deleted, 3)
	require.ElementsMatch(t, []uint16{1, 2, 5}, deleted)
	require.Equal(t, 2, cl.State.Inflight.Len())

	cl.State.Inflight.Set(packets.Packet{PacketID: 11, Expiry: n - 1})
	cl.State.Inflight.Set(packets.Packet{PacketID: 12, Expiry: n - 2})  // expiry is ineffective for v3.
	cl.State.Inflight.Set(packets.Packet{PacketID: 13, Created: n - 3}) // within bounds for v3
	cl.State.Inflight.Set(packets.Packet{PacketID: 15, Created: n - 5}) // over max server expiry limit
	require.Equal(t, 6, cl.State.Inflight.Len())

	deleted = cl.ClearExpiredInflights(n, 4)
	require.Len(t, deleted, 3)
	require.ElementsMatch(t, []uint16{11, 12, 15}, deleted)
	require.Equal(t, 3, cl.State.Inflight.Len())

	cl.State.Inflight.Set(packets.Packet{PacketID: 17, Created: n - 1})
	deleted = cl.ClearExpiredInflights(n, 0) // maximumExpiry = 0 do not process abandon messages
	require.Len(t, deleted, 0)
	require.Equal(t, 4, cl.State.Inflight.Len())

	cl.State.Inflight.Set(packets.Packet{ProtocolVersion: 5, PacketID: 18, Expiry: n - 1})
	deleted = cl.ClearExpiredInflights(n, 0)        // maximumExpiry = 0 do not abandon messages
	require.ElementsMatch(t, []uint16{18}, deleted) // expiry is still effective for v5.
	require.Len(t, deleted, 1)
	require.Equal(t, 4, cl.State.Inflight.Len())
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
		_ = w.Close()
	}()

	buf, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, 0, cl.State.Inflight.Len())
	require.Equal(t, pk1.RawBytes, buf)
}

func TestClientResendInflightMessagesWriteFailure(t *testing.T) {
	pk1 := packets.TPacketData[packets.Publish].Get(packets.TPublishQos1Dup)
	cl, r, _ := newTestClient()
	_ = r.Close()

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
		_, _ = r.Write([]byte{packets.Connect << 4, 0x00})
		_ = r.Close()
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
		_, _ = r.Write([]byte{packets.Connect<<4 | 1<<1, 0x00, 0x00})
		_ = r.Close()
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
		_, _ = r.Write(packets.TPacketData[packets.Publish].Get(packets.TPublishQos1Dup).RawBytes)
		_ = r.Close()
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
		_ = r.Close()
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
		_, _ = r.Write([]byte{packets.Connect << 4, 0xd5, 0x86, 0xf9, 0x9e, 0x01})
		_ = r.Close()
	}()

	fh := new(packets.FixedHeader)
	err := cl.ReadFixedHeader(fh)
	require.Error(t, err)
}

func TestClientReadOK(t *testing.T) {
	cl, r, _ := newTestClient()
	defer cl.Stop(errClientStop)
	go func() {
		_, _ = r.Write([]byte{
			packets.Publish << 4, 18, // Fixed header
			0, 5, // Topic Name - LSB+MSB
			'a', '/', 'b', '/', 'c', // Topic Name
			'h', 'e', 'l', 'l', 'o', ' ', 'm', 'o', 'c', 'h', 'i', // Payload,
			packets.Publish << 4, 11, // Fixed header
			0, 5, // Topic Name - LSB+MSB
			'd', '/', 'e', '/', 'f', // Topic Name
			'y', 'e', 'a', 'h', // Payload
		})
		_ = r.Close()
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
	cl.State.cancelOpen()

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
	require.Equal(t, int64(0), cl.StopTime())
	cl.Stop(nil)
	require.Equal(t, nil, cl.State.stopCause.Load())
	require.InDelta(t, time.Now().Unix(), cl.State.disconnected, 1.0)
	require.Equal(t, cl.State.disconnected, cl.StopTime())
	require.True(t, cl.Closed())
	require.Equal(t, nil, cl.StopCause())
}

func TestClientClosed(t *testing.T) {
	cl, _, _ := newTestClient()
	require.False(t, cl.Closed())
	cl.Stop(nil)
	require.True(t, cl.Closed())
}

func TestClientIsTakenOver(t *testing.T) {
	cl, _, _ := newTestClient()
	require.False(t, cl.IsTakenOver())
	cl.State.isTakenOver.Store(true)
	require.True(t, cl.IsTakenOver())
}

func TestClientReadFixedHeaderError(t *testing.T) {
	cl, r, _ := newTestClient()
	defer cl.Stop(errClientStop)
	go func() {
		_, _ = r.Write([]byte{
			packets.Publish << 4, 11, // Fixed header
		})
		_ = r.Close()
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
		_, _ = r.Write([]byte{
			packets.Publish << 4, 11, // Fixed header
			0, 5, // Topic Name - LSB+MSB
			'd', '/', 'e', '/', 'f', // Topic Name
			'y', 'e', 'a', 'h', // Payload
		})
		_ = r.Close()
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
		_, _ = r.Write([]byte{
			packets.Publish << 4, 11, // Fixed header
			0, 5,
			'd', '/', 'e', '/', 'f',
			'y', 'e', 'a', 'h',
		})
		_ = r.Close()
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
				_, _ = r.Write(tt.RawBytes)
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
	_ = cl.Net.Conn.Close()
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
		_ = cl.Net.Conn.Close()

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

func TestClientWritePacketBuffer(t *testing.T) {
	r, w := net.Pipe()

	cl := newClient(w, &ops{
		info:  new(system.Info),
		hooks: new(Hooks),
		log:   logger,
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

	cl.ops.options.ClientNetWriteBufferSize = 10
	defer cl.Stop(errClientStop)

	small := packets.TPacketData[packets.Publish].Get(packets.TPublishNoPayload).Packet
	large := packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).Packet

	cl.State.outbound <- small

	tt := []struct {
		pks  []*packets.Packet
		size int
	}{
		{
			pks:  []*packets.Packet{small, small},
			size: 18,
		},
		{
			pks:  []*packets.Packet{large},
			size: 20,
		},
		{
			pks:  []*packets.Packet{small},
			size: 0,
		},
	}

	go func() {
		for i, tx := range tt {
			for _, pk := range tx.pks {
				cl.Properties.ProtocolVersion = pk.ProtocolVersion
				err := cl.WritePacket(*pk)
				require.NoError(t, err, "index: %d", i)
				if i == len(tt)-1 {
					cl.Net.Conn.Close()
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()

	var n int
	var err error
	for i, tx := range tt {
		buf := make([]byte, 100)
		if i == len(tt)-1 {
			buf, err = io.ReadAll(r)
			n = len(buf)
		} else {
			n, err = io.ReadAtLeast(r, buf, 1)
		}
		require.NoError(t, err, "index: %d", i)
		require.Equal(t, tx.size, n, "index: %d", i)
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
		_, _ = r.Write([]byte{
			0, 11, // Fixed header
			0, 5,
			'd', '/', 'e', '/', 'f',
			'y', 'e', 'a', 'h',
		})
		_ = r.Close()
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
		_, _ = r.Write([]byte{
			0, 11, // Fixed header
			0, 5,
			'd', '/', 'e', '/', 'f',
			'y', 'e', 'a', 'h',
		})
		_ = r.Close()
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
	_ = cl.Net.Conn.Close()

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

// [MQTT-3.1.2-29] names PUBLISH among the packets a Reason String or User
// Properties may still be sent on when a client sets Request Problem
// Information to 0. A PUBLISH's User Properties are application data
// forwarded from the publisher, not problem information about a failure,
// so suppressing them silently drops data the subscriber was sent.
func TestClientWritePacketRequestProblemInfoExemptsPublish(t *testing.T) {
	tt := []struct {
		name string
		pk   packets.Packet
		want bool
	}{
		{
			name: "publish keeps its user properties",
			pk: packets.Packet{
				FixedHeader: packets.FixedHeader{Type: packets.Publish},
				TopicName:   "a/b/c",
				Payload:     []byte("hello"),
				Properties: packets.Properties{
					User: []packets.UserProperty{{Key: "prop-key", Val: "prop-val"}},
				},
			},
			want: true,
		},
		{
			name: "puback does not",
			pk: packets.Packet{
				FixedHeader: packets.FixedHeader{Type: packets.Puback},
				PacketID:    1,
				Properties: packets.Properties{
					User: []packets.UserProperty{{Key: "prop-key", Val: "prop-val"}},
				},
			},
			want: false,
		},
	}

	for _, tx := range tt {
		t.Run(tx.name, func(t *testing.T) {
			cl, r, _ := newTestClient()
			defer cl.Stop(errClientStop)
			cl.Properties.ProtocolVersion = 5
			cl.Properties.Props.RequestProblemInfoFlag = true
			cl.Properties.Props.RequestProblemInfo = 0x0

			o := make(chan []byte)
			go func() {
				buf, err := io.ReadAll(r)
				require.NoError(t, err)
				o <- buf
			}()

			tx.pk.ProtocolVersion = 5
			require.NoError(t, cl.WritePacket(tx.pk))

			time.Sleep(2 * time.Millisecond)
			_ = cl.Net.Conn.Close()

			require.Equal(t, tx.want, bytes.Contains(<-o, []byte("prop-key")))
		})
	}
}

// A client that never reads its socket must not hold the client lock for
// ever, because every write happens under it — including the one
// publishToClient needs a packet identifier for before it can reach the
// outbound queue that would have shed that client.
//
// net.Pipe is unbuffered, so a write to it blocks until the other side
// reads. Nothing reads here, which is a client that has stopped reading
// exactly as a full socket buffer is.
func TestClientWritePacketTimesOutRatherThanHoldingTheLock(t *testing.T) {
	cl, _, _ := newTestClient()
	defer cl.Stop(errClientStop)
	cl.ops.options.ClientNetWriteTimeout = 50 * time.Millisecond

	done := make(chan error, 1)
	go func() {
		done <- cl.WritePacket(*pkTable[1].Packet)
	}()

	select {
	case err := <-done:
		require.Error(t, err)
		require.True(t, isTimeout(err), "want a network timeout, got %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("WritePacket to a client that never reads did not return: it is holding " +
			"the client lock, and every other goroutine that needs it — publishToClient " +
			"taking a packet identifier for a QoS 1 delivery — waits behind it")
	}

	// And the lock is free again, which is the whole point of bounding it.
	locked := make(chan struct{})
	go func() {
		cl.Lock()
		cl.Unlock()
		close(locked)
	}()
	select {
	case <-locked:
	case <-time.After(2 * time.Second):
		t.Fatal("the client lock was still held after the write timed out")
	}
}

// Zero is the default and must leave writes exactly as they were, so that
// taking this patch changes nothing for anyone who does not set it.
func TestClientWritePacketIsUnboundedByDefault(t *testing.T) {
	cl, r, _ := newTestClient()
	defer cl.Stop(errClientStop)
	require.Zero(t, cl.ops.options.ClientNetWriteTimeout)

	done := make(chan error, 1)
	go func() { done <- cl.WritePacket(*pkTable[1].Packet) }()

	// It is still blocked a moment later, because nothing has read yet.
	select {
	case err := <-done:
		t.Fatalf("the write returned early with %v: without a timeout it waits for a reader", err)
	case <-time.After(100 * time.Millisecond):
	}

	// Read, and it completes normally.
	buf := make([]byte, len(pkTable[1].RawBytes))
	go func() { _, _ = io.ReadFull(r, buf) }()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("the write never completed even once its reader arrived")
	}
}

// A write that ran out of time has put part of a packet on the wire, so
// the next packet written would land in the middle of the last one. It is
// also the moment the loop would otherwise take the client lock again for
// every packet still queued, a timeout at a time, while publishToClient
// waits for that lock to take a packet identifier.
func TestClientWriteLoopStopsTheClientWhenAWriteTimesOut(t *testing.T) {
	cl, _, _ := newTestClient()
	cl.ops.options.ClientNetWriteTimeout = 50 * time.Millisecond

	go cl.WriteLoop()

	pk := *pkTable[1].Packet
	cl.State.outbound <- &pk
	atomic.AddInt32(&cl.State.outboundQty, 1)

	// Nothing reads the pipe, so the write times out and the loop stops the
	// client rather than trying the next packet into a broken stream.
	require.Eventually(t, cl.Closed, 2*time.Second, 10*time.Millisecond,
		"the client was not stopped after its write ran out of time")
	require.True(t, isTimeout(cl.StopCause()), "want a network timeout, got %v", cl.StopCause())
}

func TestClientWritePacketDeadlineSurvivesThePacketsTheClientSends(t *testing.T) {
	cl, r, _ := newTestClient()
	defer cl.Stop(errClientStop)
	cl.ops.options.ClientNetWriteTimeout = 50 * time.Millisecond
	cl.State.Keepalive = 0 // the worst case: the refresh's expiry is the zero time

	go func() { _ = cl.Read(func(*Client, packets.Packet) error { return nil }) }()

	// The client keeps sending packets that need no reply, which is what
	// takes the read loop round again — and round again is where the
	// keepalive deadline is refreshed.
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		for {
			select {
			case <-stop:
				return
			case <-time.After(10 * time.Millisecond):
			}
			if _, err := r.Write([]byte{packets.Pingreq << 4, 0}); err != nil {
				return
			}
		}
	}()

	done := make(chan error, 1)
	go func() { done <- cl.WritePacket(*pkTable[1].Packet) }()

	select {
	case err := <-done:
		require.True(t, isTimeout(err), "want a network timeout, got %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("a client that keeps sending switched off the write deadline armed for " +
			"it: the keepalive refresh runs at the top of every pass of the read loop, " +
			"so it has to bound reads and not both")
	}
}

// A write to a client that has stopped reading is bounded by the keepalive
// even when ClientNetWriteTimeout is unset.
//
// This is a regression test for the default configuration rather than for the
// option. refreshDeadline arms only the read deadline, so the keepalive no
// longer bounds a write the way SetDeadline used to - and with the option at
// its zero value that would leave a write able to block forever while holding
// the client's lock, which is the deadlock the option exists to prevent. An
// operator who upgrades and sets nothing must not end up worse off.
func TestWriteIsBoundedByKeepaliveWithNoWriteTimeout(t *testing.T) {
	r, w := net.Pipe()
	defer r.Close()
	defer w.Close()

	cl, _, _ := newTestClient()
	cl.Net.Conn = r
	cl.State.Keepalive = 1
	cl.ops.options.ClientNetWriteTimeout = 0 // the default, and the point

	done := make(chan error, 1)
	go func() {
		// Nothing reads w, so this write can only end at a deadline.
		done <- cl.WritePacket(*packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).Packet)
	}()

	select {
	case err := <-done:
		require.Error(t, err)
		require.True(t, isTimeout(err), "want a timeout, got %v", err)
	case <-time.After(4 * time.Second):
		t.Fatal("a write to a client that stopped reading was never bounded, with the keepalive set and the write timeout unset")
	}
}

// And the option still overrides the keepalive when it is set, which is what
// makes it useful: a deployment wanting a tighter bound than 1.5x keepalive
// can have one.
func TestWriteTimeoutOverridesTheKeepaliveBound(t *testing.T) {
	r, w := net.Pipe()
	defer r.Close()
	defer w.Close()

	cl, _, _ := newTestClient()
	cl.Net.Conn = r
	cl.State.Keepalive = 60 // 90s if the keepalive decided it
	cl.ops.options.ClientNetWriteTimeout = 200 * time.Millisecond

	done := make(chan error, 1)
	go func() {
		done <- cl.WritePacket(*packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).Packet)
	}()

	select {
	case err := <-done:
		require.Error(t, err)
		require.True(t, isTimeout(err), "want a timeout, got %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("the write timeout did not override the longer keepalive bound")
	}
}

// recordingConn notes which deadline a caller armed. Which one matters more
// than it looks: a websocket connection overrides SetWriteDeadline ONLY, and
// passes SetDeadline and SetReadDeadline through to the socket underneath -
// where gorilla overwrites the write deadline before every frame it sends. So
// a bound armed with SetDeadline reaches a TCP client and silently does not
// reach a websocket one.
type recordingConn struct {
	net.Conn
	write, read, both time.Time
	cleared           bool
}

// The armed deadline, not the last one: WritePacket clears it again on the
// way out, so recording the most recent call records the clearing and reads
// as though nothing was ever armed.
func (c *recordingConn) SetWriteDeadline(t time.Time) error {
	if t.IsZero() {
		c.cleared = true
	} else if c.write.IsZero() {
		c.write = t
	}
	return nil
}
func (c *recordingConn) SetReadDeadline(t time.Time) error { c.read = t; return nil }
func (c *recordingConn) SetDeadline(t time.Time) error     { c.both = t; return nil }
func (c *recordingConn) Write(p []byte) (int, error)       { return len(p), nil }

// The keepalive bound is armed as a WRITE deadline, which is the only one a
// websocket connection routes to the thing that actually bounds the write.
//
// Armed as SetDeadline instead, this would still pass for a TCP client and
// bound nothing at all for a websocket one - so the assertion is on which
// method was called, not merely on the write ending.
func TestTheKeepaliveWriteBoundIsArmedWhereAWebsocketCanSeeIt(t *testing.T) {
	r, w := net.Pipe()
	defer r.Close()
	defer w.Close()

	rec := &recordingConn{Conn: r}
	cl, _, _ := newTestClient()
	cl.Net.Conn = rec
	cl.State.Keepalive = 10
	cl.ops.options.ClientNetWriteTimeout = 0

	require.NoError(t, cl.WritePacket(*packets.TPacketData[packets.Publish].Get(packets.TPublishBasic).Packet))

	require.False(t, rec.write.IsZero(), "no write deadline was armed, so a websocket write is unbounded")
	require.True(t, rec.both.IsZero(), "armed with SetDeadline, which a websocket connection does not route to the write")

	// 10s keepalive means the 15s expiry refreshDeadline uses.
	require.WithinDuration(t, time.Now().Add(15*time.Second), rec.write, 2*time.Second)

	// And it is released with the lock, so it never outlives the write it
	// was armed for.
	require.True(t, rec.cleared, "the deadline was left armed on the connection")
}
