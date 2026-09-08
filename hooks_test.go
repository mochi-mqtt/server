// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package mqtt

import (
	"bytes"
	"errors"
	"log/slog"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mochi-mqtt/server/v2/hooks/storage"
	"github.com/mochi-mqtt/server/v2/packets"
	"github.com/mochi-mqtt/server/v2/system"

	"github.com/stretchr/testify/require"
)

type modifiedHookBase struct {
	HookBase
	err    error
	fail   bool
	failAt int
}

var errTestHook = errors.New("error")

func (h *modifiedHookBase) ID() string {
	return "modified"
}

func (h *modifiedHookBase) Init(config any) error {
	if config != nil {
		return errTestHook
	}
	return nil
}

func (h *modifiedHookBase) Provides(b byte) bool {
	return true
}

func (h *modifiedHookBase) Stop() error {
	if h.fail {
		return errTestHook
	}

	return nil
}

func (h *modifiedHookBase) OnConnect(cl *Client, pk packets.Packet) error {
	if h.fail {
		return errTestHook
	}

	return nil
}

func (h *modifiedHookBase) OnConnectAuthenticate(cl *Client, pk packets.Packet) bool {
	return true
}

func (h *modifiedHookBase) OnACLCheck(cl *Client, topic string, write bool) bool {
	return true
}

func (h *modifiedHookBase) OnPublish(cl *Client, pk packets.Packet) (packets.Packet, error) {
	if h.fail {
		if h.err != nil {
			return pk, h.err
		}

		return pk, errTestHook
	}

	return pk, nil
}

func (h *modifiedHookBase) OnPacketRead(cl *Client, pk packets.Packet) (packets.Packet, error) {
	if h.fail {
		if h.err != nil {
			return pk, h.err
		}

		return pk, errTestHook
	}

	return pk, nil
}

func (h *modifiedHookBase) OnAuthPacket(cl *Client, pk packets.Packet) (packets.Packet, error) {
	if h.fail {
		if h.err != nil {
			return pk, h.err
		}

		return pk, errTestHook
	}

	return pk, nil
}

func (h *modifiedHookBase) OnWill(cl *Client, will Will) (Will, error) {
	if h.fail {
		return will, errTestHook
	}

	return will, nil
}

func (h *modifiedHookBase) StoredClients() (v []storage.Client, err error) {
	if h.fail || h.failAt == 1 {
		return v, errTestHook
	}

	return []storage.Client{
		{ID: "cl1"},
		{ID: "cl2"},
		{ID: "cl3"},
	}, nil
}

func (h *modifiedHookBase) StoredSubscriptions() (v []storage.Subscription, err error) {
	if h.fail || h.failAt == 2 {
		return v, errTestHook
	}

	return []storage.Subscription{
		{ID: "sub1"},
		{ID: "sub2"},
		{ID: "sub3"},
	}, nil
}

func (h *modifiedHookBase) StoredRetainedMessages() (v []storage.Message, err error) {
	if h.fail || h.failAt == 3 {
		return v, errTestHook
	}

	return []storage.Message{
		{ID: "r1"},
		{ID: "r2"},
		{ID: "r3"},
	}, nil
}

func (h *modifiedHookBase) StoredInflightMessages() (v []storage.Message, err error) {
	if h.fail || h.failAt == 4 {
		return v, errTestHook
	}

	return []storage.Message{
		{ID: "i1"},
		{ID: "i2"},
		{ID: "i3"},
	}, nil
}

func (h *modifiedHookBase) StoredSysInfo() (v storage.SystemInfo, err error) {
	if h.fail || h.failAt == 5 {
		return v, errTestHook
	}

	return storage.SystemInfo{
		Info: system.Info{
			Version: "2.0.0",
		},
	}, nil
}

type providesCheckHook struct {
	HookBase
}

func (h *providesCheckHook) Provides(b byte) bool {
	return b == OnConnect
}

func TestHooksProvides(t *testing.T) {
	h := new(Hooks)
	err := h.Add(new(providesCheckHook), nil)
	require.NoError(t, err)

	err = h.Add(new(HookBase), nil)
	require.NoError(t, err)

	require.True(t, h.Provides(OnConnect, OnDisconnect))
	require.False(t, h.Provides(OnDisconnect))
}

func TestHooksAddLenGetAll(t *testing.T) {
	h := new(Hooks)
	err := h.Add(new(HookBase), nil)
	require.NoError(t, err)

	err = h.Add(new(modifiedHookBase), nil)
	require.NoError(t, err)

	require.Equal(t, int64(2), atomic.LoadInt64(&h.qty))
	require.Equal(t, int64(2), h.Len())

	all := h.GetAll()
	require.Equal(t, "base", all[0].ID())
	require.Equal(t, "modified", all[1].ID())
}

func TestHooksAddInitFailure(t *testing.T) {
	h := new(Hooks)
	err := h.Add(new(modifiedHookBase), map[string]any{})
	require.Error(t, err)
	require.Equal(t, int64(0), atomic.LoadInt64(&h.qty))
}

func TestHooksStop(t *testing.T) {
	h := new(Hooks)
	h.Log = logger

	err := h.Add(new(HookBase), nil)
	require.NoError(t, err)
	require.Equal(t, int64(1), atomic.LoadInt64(&h.qty))
	require.Equal(t, int64(1), h.Len())

	h.Stop()
}

// coverage: also cover some empty functions
func TestHooksNonReturns(t *testing.T) {
	h := new(Hooks)
	cl := new(Client)

	for i := 0; i < 2; i++ {
		t.Run("step-"+strconv.Itoa(i), func(t *testing.T) {
			// on first iteration, check without hook methods
			h.OnStarted()
			h.OnStopped()
			h.OnSysInfoTick(new(system.Info))
			h.OnSessionEstablish(cl, packets.Packet{})
			h.OnSessionEstablished(cl, packets.Packet{})
			h.OnDisconnect(cl, nil, false)
			h.OnConnectRefused(cl, packets.Packet{}, packets.ErrServerBusy)
			h.OnPacketSent(cl, packets.Packet{}, []byte{})
			h.OnPacketProcessed(cl, packets.Packet{}, nil)
			h.OnSubscribed(cl, packets.Packet{}, []byte{1})
			h.OnUnsubscribed(cl, packets.Packet{})
			h.OnPublished(cl, packets.Packet{})
			h.OnPublishDropped(cl, packets.Packet{})
			h.OnRetainMessage(cl, packets.Packet{}, 0)
			h.OnRetainPublished(cl, packets.Packet{})
			h.OnQosPublish(cl, packets.Packet{}, time.Now().Unix(), 0)
			h.OnQosComplete(cl, packets.Packet{})
			h.OnQosDropped(cl, packets.Packet{})
			h.OnPacketIDExhausted(cl, packets.Packet{})
			h.OnWillSent(cl, packets.Packet{})
			h.OnClientExpired(cl)
			h.OnRetainedExpired("a/b/c")

			// on second iteration, check added hook methods
			err := h.Add(new(modifiedHookBase), nil)
			require.NoError(t, err)
		})
	}
}

func TestHooksOnConnectAuthenticate(t *testing.T) {
	h := new(Hooks)

	ok := h.OnConnectAuthenticate(new(Client), packets.Packet{})
	require.False(t, ok)

	err := h.Add(new(modifiedHookBase), nil)
	require.NoError(t, err)

	ok = h.OnConnectAuthenticate(new(Client), packets.Packet{})
	require.True(t, ok)
}

func TestHooksOnACLCheck(t *testing.T) {
	h := new(Hooks)

	ok := h.OnACLCheck(new(Client), "a/b/c", true)
	require.False(t, ok)

	err := h.Add(new(modifiedHookBase), nil)
	require.NoError(t, err)

	ok = h.OnACLCheck(new(Client), "a/b/c", true)
	require.True(t, ok)
}

func TestHooksOnSubscribe(t *testing.T) {
	h := new(Hooks)
	err := h.Add(new(modifiedHookBase), nil)
	require.NoError(t, err)

	pki := packets.Packet{
		Filters: packets.Subscriptions{
			{Filter: "a/b/c", Qos: 1},
		},
	}
	pk := h.OnSubscribe(new(Client), pki)
	require.EqualValues(t, pk, pki)
}

func TestHooksOnSelectSubscribers(t *testing.T) {
	h := new(Hooks)
	err := h.Add(new(modifiedHookBase), nil)
	require.NoError(t, err)

	subs := &Subscribers{
		Subscriptions: map[string]packets.Subscription{
			"cl1": {Filter: "a/b/c"},
		},
	}

	subs2 := h.OnSelectSubscribers(subs, packets.Packet{})
	require.EqualValues(t, subs, subs2)
}

func TestHooksOnUnsubscribe(t *testing.T) {
	h := new(Hooks)
	err := h.Add(new(modifiedHookBase), nil)
	require.NoError(t, err)

	pki := packets.Packet{
		Filters: packets.Subscriptions{
			{Filter: "a/b/c", Qos: 1},
		},
	}

	pk := h.OnUnsubscribe(new(Client), pki)
	require.EqualValues(t, pk, pki)
}

func TestHooksOnPublish(t *testing.T) {
	h := new(Hooks)
	h.Log = logger

	hook := new(modifiedHookBase)
	err := h.Add(hook, nil)
	require.NoError(t, err)

	pk, err := h.OnPublish(new(Client), packets.Packet{PacketID: 10})
	require.NoError(t, err)
	require.Equal(t, uint16(10), pk.PacketID)

	// coverage: failure
	hook.fail = true
	pk, err = h.OnPublish(new(Client), packets.Packet{PacketID: 10})
	require.Error(t, err)
	require.Equal(t, uint16(10), pk.PacketID)

	// coverage: reject packet
	hook.err = packets.ErrRejectPacket
	pk, err = h.OnPublish(new(Client), packets.Packet{PacketID: 10})
	require.Error(t, err)
	require.ErrorIs(t, err, packets.ErrRejectPacket)
	require.Equal(t, uint16(10), pk.PacketID)
}

func TestHooksOnPublishLogsAReasonCodeAtDebug(t *testing.T) {
	var buf bytes.Buffer
	h := new(Hooks)
	h.Log = slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	hook := new(modifiedHookBase)
	err := h.Add(hook, nil)
	require.NoError(t, err)

	// A reason code is a decision the server answers with, not a fault.
	hook.fail = true
	hook.err = packets.ErrQuotaExceeded
	_, err = h.OnPublish(new(Client), packets.Packet{PacketID: 10})
	require.ErrorIs(t, err, packets.ErrQuotaExceeded)
	require.NotContains(t, buf.String(), "level=ERROR")
	require.Contains(t, buf.String(), "level=DEBUG")
	require.Contains(t, buf.String(), "reason=\"quota exceeded\"")

	// Anything that is not a reason code the server can answer with is
	// still an error.
	buf.Reset()
	hook.err = nil
	_, err = h.OnPublish(new(Client), packets.Packet{PacketID: 10})
	require.ErrorIs(t, err, errTestHook)
	require.Contains(t, buf.String(), "level=ERROR")
}

func TestHooksOnPacketRead(t *testing.T) {
	h := new(Hooks)
	h.Log = logger

	hook := new(modifiedHookBase)
	err := h.Add(hook, nil)
	require.NoError(t, err)

	pk, err := h.OnPacketRead(new(Client), packets.Packet{PacketID: 10})
	require.NoError(t, err)
	require.Equal(t, uint16(10), pk.PacketID)

	// coverage: failure
	hook.fail = true
	pk, err = h.OnPacketRead(new(Client), packets.Packet{PacketID: 10})
	require.NoError(t, err)
	require.Equal(t, uint16(10), pk.PacketID)

	// coverage: reject packet
	hook.err = packets.ErrRejectPacket
	pk, err = h.OnPacketRead(new(Client), packets.Packet{PacketID: 10})
	require.Error(t, err)
	require.ErrorIs(t, err, packets.ErrRejectPacket)
	require.Equal(t, uint16(10), pk.PacketID)
}

func TestHooksOnAuthPacket(t *testing.T) {
	h := new(Hooks)
	h.Log = logger

	hook := new(modifiedHookBase)
	err := h.Add(hook, nil)
	require.NoError(t, err)

	pk, err := h.OnAuthPacket(new(Client), packets.Packet{PacketID: 10})
	require.NoError(t, err)
	require.Equal(t, uint16(10), pk.PacketID)

	hook.fail = true
	pk, err = h.OnAuthPacket(new(Client), packets.Packet{PacketID: 10})
	require.Error(t, err)
	require.Equal(t, uint16(10), pk.PacketID)
}

func TestHooksOnConnect(t *testing.T) {
	h := new(Hooks)
	h.Log = logger

	hook := new(modifiedHookBase)
	err := h.Add(hook, nil)
	require.NoError(t, err)

	err = h.OnConnect(new(Client), packets.Packet{PacketID: 10})
	require.NoError(t, err)

	hook.fail = true
	err = h.OnConnect(new(Client), packets.Packet{PacketID: 10})
	require.Error(t, err)
}

func TestHooksOnPacketEncode(t *testing.T) {
	h := new(Hooks)
	h.Log = logger

	hook := new(modifiedHookBase)
	err := h.Add(hook, nil)
	require.NoError(t, err)

	pk := h.OnPacketEncode(new(Client), packets.Packet{PacketID: 10})
	require.Equal(t, uint16(10), pk.PacketID)
}

func TestHooksOnLWT(t *testing.T) {
	h := new(Hooks)
	h.Log = logger

	hook := new(modifiedHookBase)
	err := h.Add(hook, nil)
	require.NoError(t, err)

	lwt := h.OnWill(new(Client), Will{TopicName: "a/b/c"})
	require.Equal(t, "a/b/c", lwt.TopicName)

	// coverage: fail lwt
	hook.fail = true
	lwt = h.OnWill(new(Client), Will{TopicName: "a/b/c"})
	require.Equal(t, "a/b/c", lwt.TopicName)
}

func TestHooksStoredClients(t *testing.T) {
	h := new(Hooks)
	h.Log = logger

	v, err := h.StoredClients()
	require.NoError(t, err)
	require.Len(t, v, 0)

	hook := new(modifiedHookBase)
	err = h.Add(hook, nil)
	require.NoError(t, err)

	v, err = h.StoredClients()
	require.NoError(t, err)
	require.Len(t, v, 3)

	hook.fail = true
	v, err = h.StoredClients()
	require.Error(t, err)
	require.Len(t, v, 0)
}

func TestHooksStoredSubscriptions(t *testing.T) {
	h := new(Hooks)
	h.Log = logger

	v, err := h.StoredSubscriptions()
	require.NoError(t, err)
	require.Len(t, v, 0)

	hook := new(modifiedHookBase)
	err = h.Add(hook, nil)
	require.NoError(t, err)

	v, err = h.StoredSubscriptions()
	require.NoError(t, err)
	require.Len(t, v, 3)

	hook.fail = true
	v, err = h.StoredSubscriptions()
	require.Error(t, err)
	require.Len(t, v, 0)
}

func TestHooksStoredRetainedMessages(t *testing.T) {
	h := new(Hooks)
	h.Log = logger

	v, err := h.StoredRetainedMessages()
	require.NoError(t, err)
	require.Len(t, v, 0)

	hook := new(modifiedHookBase)
	err = h.Add(hook, nil)
	require.NoError(t, err)

	v, err = h.StoredRetainedMessages()
	require.NoError(t, err)
	require.Len(t, v, 3)

	hook.fail = true
	v, err = h.StoredRetainedMessages()
	require.Error(t, err)
	require.Len(t, v, 0)
}

func TestHooksStoredInflightMessages(t *testing.T) {
	h := new(Hooks)
	h.Log = logger

	v, err := h.StoredInflightMessages()
	require.NoError(t, err)
	require.Len(t, v, 0)

	hook := new(modifiedHookBase)
	err = h.Add(hook, nil)
	require.NoError(t, err)

	v, err = h.StoredInflightMessages()
	require.NoError(t, err)
	require.Len(t, v, 3)

	hook.fail = true
	v, err = h.StoredInflightMessages()
	require.Error(t, err)
	require.Len(t, v, 0)
}

func TestHooksStoredSysInfo(t *testing.T) {
	h := new(Hooks)
	h.Log = logger

	v, err := h.StoredSysInfo()
	require.NoError(t, err)
	require.Equal(t, "", v.Info.Version)

	hook := new(modifiedHookBase)
	err = h.Add(hook, nil)
	require.NoError(t, err)

	v, err = h.StoredSysInfo()
	require.NoError(t, err)
	require.Equal(t, "2.0.0", v.Info.Version)

	hook.fail = true
	v, err = h.StoredSysInfo()
	require.Error(t, err)
	require.Equal(t, "", v.Info.Version)
}

func TestHookBaseID(t *testing.T) {
	h := new(HookBase)
	require.Equal(t, "base", h.ID())
}

func TestHookBaseProvidesNone(t *testing.T) {
	h := new(HookBase)
	require.False(t, h.Provides(OnConnect))
	require.False(t, h.Provides(OnDisconnect))
}

func TestHookBaseInit(t *testing.T) {
	h := new(HookBase)
	require.Nil(t, h.Init(nil))
}

func TestHookBaseSetOpts(t *testing.T) {
	h := new(HookBase)
	h.SetOpts(logger, new(HookOptions))
	require.NotNil(t, h.Log)
	require.NotNil(t, h.Opts)
}

func TestHookBaseClose(t *testing.T) {
	h := new(HookBase)
	require.Nil(t, h.Stop())
}

func TestHookBaseOnConnectAuthenticate(t *testing.T) {
	h := new(HookBase)
	v := h.OnConnectAuthenticate(new(Client), packets.Packet{})
	require.False(t, v)
}

func TestHookBaseOnACLCheck(t *testing.T) {
	h := new(HookBase)
	v := h.OnACLCheck(new(Client), "topic", true)
	require.False(t, v)
}

func TestHookBaseOnConnect(t *testing.T) {
	h := new(HookBase)
	err := h.OnConnect(new(Client), packets.Packet{})
	require.NoError(t, err)
}

func TestHookBaseOnPublish(t *testing.T) {
	h := new(HookBase)
	pk, err := h.OnPublish(new(Client), packets.Packet{PacketID: 10})
	require.NoError(t, err)
	require.Equal(t, uint16(10), pk.PacketID)
}

func TestHookBaseOnPacketRead(t *testing.T) {
	h := new(HookBase)
	pk, err := h.OnPacketRead(new(Client), packets.Packet{PacketID: 10})
	require.NoError(t, err)
	require.Equal(t, uint16(10), pk.PacketID)
}

func TestHookBaseOnAuthPacket(t *testing.T) {
	h := new(HookBase)
	pk, err := h.OnAuthPacket(new(Client), packets.Packet{PacketID: 10})
	require.NoError(t, err)
	require.Equal(t, uint16(10), pk.PacketID)
}

func TestHookBaseOnLWT(t *testing.T) {
	h := new(HookBase)
	lwt, err := h.OnWill(new(Client), Will{TopicName: "a/b/c"})
	require.NoError(t, err)
	require.Equal(t, "a/b/c", lwt.TopicName)
}

func TestHookBaseStoredClients(t *testing.T) {
	h := new(HookBase)
	v, err := h.StoredClients()
	require.NoError(t, err)
	require.Empty(t, v)
}

func TestHookBaseStoredSubscriptions(t *testing.T) {
	h := new(HookBase)
	v, err := h.StoredSubscriptions()
	require.NoError(t, err)
	require.Empty(t, v)
}

func TestHookBaseStoredInflightMessages(t *testing.T) {
	h := new(HookBase)
	v, err := h.StoredInflightMessages()
	require.NoError(t, err)
	require.Empty(t, v)
}

func TestHookBaseStoredRetainedMessages(t *testing.T) {
	h := new(HookBase)
	v, err := h.StoredRetainedMessages()
	require.NoError(t, err)
	require.Empty(t, v)
}

func TestHookBaseStoreSysInfo(t *testing.T) {
	h := new(HookBase)
	v, err := h.StoredSysInfo()
	require.NoError(t, err)
	require.Equal(t, "", v.Version)
}

// legacyHook implements the Hook interface the way an embedder did before
// this release: WITHOUT embedding HookBase, and knowing nothing of
// OnConnectRefused.
//
// Its method set is exactly the interface's. If a later change adds a method
// to Hook, this file stops compiling - which is the point. A minor release
// must not break an embedder's build, and nothing else in the tree would
// notice if one did.
type legacyHook struct{}

func (h *legacyHook) ID() string                                               { return "" }
func (h *legacyHook) Provides(b byte) bool                                     { return false }
func (h *legacyHook) Init(config any) error                                    { return nil }
func (h *legacyHook) Stop() error                                              { return nil }
func (h *legacyHook) SetOpts(l *slog.Logger, o *HookOptions)                   {}
func (h *legacyHook) OnStarted()                                               {}
func (h *legacyHook) OnStopped()                                               {}
func (h *legacyHook) OnConnectAuthenticate(cl *Client, pk packets.Packet) bool { return false }
func (h *legacyHook) OnACLCheck(cl *Client, topic string, write bool) bool     { return false }
func (h *legacyHook) OnSysInfoTick(*system.Info)                               {}
func (h *legacyHook) OnConnect(cl *Client, pk packets.Packet) error            { return nil }
func (h *legacyHook) OnSessionEstablish(cl *Client, pk packets.Packet)         {}
func (h *legacyHook) OnSessionEstablished(cl *Client, pk packets.Packet)       {}
func (h *legacyHook) OnDisconnect(cl *Client, err error, expire bool)          {}
func (h *legacyHook) OnAuthPacket(cl *Client, pk packets.Packet) (packets.Packet, error) {
	return packets.Packet{}, nil
}
func (h *legacyHook) OnPacketRead(cl *Client, pk packets.Packet) (packets.Packet, error) {
	return packets.Packet{}, nil
}
func (h *legacyHook) OnPacketEncode(cl *Client, pk packets.Packet) packets.Packet {
	return packets.Packet{}
}
func (h *legacyHook) OnPacketSent(cl *Client, pk packets.Packet, b []byte)       {}
func (h *legacyHook) OnPacketProcessed(cl *Client, pk packets.Packet, err error) {}
func (h *legacyHook) OnSubscribe(cl *Client, pk packets.Packet) packets.Packet {
	return packets.Packet{}
}
func (h *legacyHook) OnSubscribed(cl *Client, pk packets.Packet, reasonCodes []byte) {}
func (h *legacyHook) OnSelectSubscribers(subs *Subscribers, pk packets.Packet) *Subscribers {
	return nil
}
func (h *legacyHook) OnUnsubscribe(cl *Client, pk packets.Packet) packets.Packet {
	return packets.Packet{}
}
func (h *legacyHook) OnUnsubscribed(cl *Client, pk packets.Packet) {}
func (h *legacyHook) OnPublish(cl *Client, pk packets.Packet) (packets.Packet, error) {
	return packets.Packet{}, nil
}
func (h *legacyHook) OnPublished(cl *Client, pk packets.Packet)                           {}
func (h *legacyHook) OnPublishDropped(cl *Client, pk packets.Packet)                      {}
func (h *legacyHook) OnRetainMessage(cl *Client, pk packets.Packet, r int64)              {}
func (h *legacyHook) OnRetainPublished(cl *Client, pk packets.Packet)                     {}
func (h *legacyHook) OnQosPublish(cl *Client, pk packets.Packet, sent int64, resends int) {}
func (h *legacyHook) OnQosComplete(cl *Client, pk packets.Packet)                         {}
func (h *legacyHook) OnQosDropped(cl *Client, pk packets.Packet)                          {}
func (h *legacyHook) OnPacketIDExhausted(cl *Client, pk packets.Packet)                   {}
func (h *legacyHook) OnWill(cl *Client, will Will) (Will, error)                          { return Will{}, nil }
func (h *legacyHook) OnWillSent(cl *Client, pk packets.Packet)                            {}
func (h *legacyHook) OnClientExpired(cl *Client)                                          {}
func (h *legacyHook) OnRetainedExpired(filter string)                                     {}
func (h *legacyHook) StoredClients() ([]storage.Client, error)                            { return nil, nil }
func (h *legacyHook) StoredSubscriptions() ([]storage.Subscription, error)                { return nil, nil }
func (h *legacyHook) StoredInflightMessages() ([]storage.Message, error)                  { return nil, nil }
func (h *legacyHook) StoredRetainedMessages() ([]storage.Message, error)                  { return nil, nil }
func (h *legacyHook) StoredSysInfo() (storage.SystemInfo, error)                          { return storage.SystemInfo{}, nil }

// A hook written before OnConnectRefused existed still satisfies Hook, so an
// embedder who never embedded HookBase keeps compiling across this release.
func TestAHookWithoutHookBaseStillSatisfiesTheInterface(t *testing.T) {
	var h Hook = new(legacyHook)
	require.Equal(t, "", h.ID())

	// And the dispatcher steps over it rather than reaching for a method it
	// does not have.
	hooks := new(Hooks)
	require.NoError(t, hooks.Add(new(legacyHook), nil))
	hooks.OnConnectRefused(new(Client), packets.Packet{}, packets.ErrServerBusy)
}
