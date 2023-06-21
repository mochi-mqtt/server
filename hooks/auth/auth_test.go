// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-co
// SPDX-FileContributor: mochi-co

package auth

import (
	"os"
	"testing"

	"github.com/mochi-co/mqtt/v2"
	"github.com/mochi-co/mqtt/v2/packets"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slog"
)

var logger = zerolog.New(os.Stderr).With().Timestamp().Logger().Level(zerolog.Disabled)
var slogger = slog.New(slog.NewTextHandler(os.Stdout, nil))

// func teardown(t *testing.T, path string, h *Hook) {
// 	h.Stop()
// }

func TestBasicID(t *testing.T) {
	h := new(Hook)
	require.Equal(t, "auth-ledger", h.ID())
}

func TestBasicProvides(t *testing.T) {
	h := new(Hook)
	require.True(t, h.Provides(mqtt.OnACLCheck))
	require.True(t, h.Provides(mqtt.OnConnectAuthenticate))
	require.False(t, h.Provides(mqtt.OnPublish))
}

func TestBasicInitBadConfig(t *testing.T) {
	h := new(Hook)
	h.SetOpts(&logger, slogger, nil)

	err := h.Init(map[string]any{})
	require.Error(t, err)
}

func TestBasicInitDefaultConfig(t *testing.T) {
	h := new(Hook)
	h.SetOpts(&logger, slogger, nil)

	err := h.Init(nil)
	require.NoError(t, err)
}

func TestBasicInitWithLedgerPointer(t *testing.T) {
	h := new(Hook)
	h.SetOpts(&logger, slogger, nil)

	ln := &Ledger{
		Auth: []AuthRule{
			{
				Remote: "127.0.0.1",
				Allow:  true,
			},
		},
		ACL: []ACLRule{
			{
				Remote: "127.0.0.1",
				Filters: Filters{
					"#": ReadWrite,
				},
			},
		},
	}

	err := h.Init(&Options{
		Ledger: ln,
	})

	require.NoError(t, err)
	require.Same(t, ln, h.ledger)
}

func TestBasicInitWithLedgerJSON(t *testing.T) {
	h := new(Hook)
	h.SetOpts(&logger, slogger, nil)

	require.Nil(t, h.ledger)
	err := h.Init(&Options{
		Data: ledgerJSON,
	})

	require.NoError(t, err)
	require.Equal(t, ledgerStruct.Auth[0].Username, h.ledger.Auth[0].Username)
	require.Equal(t, ledgerStruct.ACL[0].Client, h.ledger.ACL[0].Client)
}

func TestBasicInitWithLedgerYAML(t *testing.T) {
	h := new(Hook)
	h.SetOpts(&logger, slogger, nil)

	require.Nil(t, h.ledger)
	err := h.Init(&Options{
		Data: ledgerYAML,
	})

	require.NoError(t, err)
	require.Equal(t, ledgerStruct.Auth[0].Username, h.ledger.Auth[0].Username)
	require.Equal(t, ledgerStruct.ACL[0].Client, h.ledger.ACL[0].Client)
}

func TestBasicInitWithLedgerBadDAta(t *testing.T) {
	h := new(Hook)
	h.SetOpts(&logger, slogger, nil)

	require.Nil(t, h.ledger)
	err := h.Init(&Options{
		Data: []byte("fdsfdsafasd"),
	})

	require.Error(t, err)
}

func TestOnConnectAuthenticate(t *testing.T) {
	h := new(Hook)
	h.SetOpts(&logger, slogger, nil)

	ln := new(Ledger)
	ln.Auth = checkLedger.Auth
	ln.ACL = checkLedger.ACL
	err := h.Init(
		&Options{
			Ledger: ln,
		},
	)

	require.NoError(t, err)

	require.True(t, h.OnConnectAuthenticate(
		&mqtt.Client{
			Properties: mqtt.ClientProperties{
				Username: []byte("mochi"),
			},
		},
		packets.Packet{Connect: packets.ConnectParams{Password: []byte("melon")}},
	))

	require.False(t, h.OnConnectAuthenticate(
		&mqtt.Client{
			Properties: mqtt.ClientProperties{
				Username: []byte("mochi"),
			},
		},
		packets.Packet{Connect: packets.ConnectParams{Password: []byte("bad-pass")}},
	))

	require.False(t, h.OnConnectAuthenticate(
		&mqtt.Client{},
		packets.Packet{},
	))
}

func TestOnACL(t *testing.T) {
	h := new(Hook)
	h.SetOpts(&logger, slogger, nil)

	ln := new(Ledger)
	ln.Auth = checkLedger.Auth
	ln.ACL = checkLedger.ACL
	err := h.Init(
		&Options{
			Ledger: ln,
		},
	)

	require.NoError(t, err)

	require.True(t, h.OnACLCheck(
		&mqtt.Client{
			Properties: mqtt.ClientProperties{
				Username: []byte("mochi"),
			},
		},
		"mochi/info",
		true,
	))

	require.False(t, h.OnACLCheck(
		&mqtt.Client{
			Properties: mqtt.ClientProperties{
				Username: []byte("mochi"),
			},
		},
		"d/j/f",
		true,
	))

	require.True(t, h.OnACLCheck(
		&mqtt.Client{
			Properties: mqtt.ClientProperties{
				Username: []byte("mochi"),
			},
		},
		"readonly",
		false,
	))

	require.False(t, h.OnACLCheck(
		&mqtt.Client{
			Properties: mqtt.ClientProperties{
				Username: []byte("mochi"),
			},
		},
		"readonly",
		true,
	))
}
