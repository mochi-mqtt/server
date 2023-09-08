// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package auth

import (
	"testing"

	"github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
	"github.com/stretchr/testify/require"
)

var (
	checkLedger = Ledger{
		Users: Users{ // users are allowed by default
			"mochi-co": {
				Password: "melon",
				ACL: Filters{
					"d/+/f":      Deny,
					"mochi-co/#": ReadWrite,
					"readonly":   ReadOnly,
				},
			},
			"suspended-username": {
				Password: "any",
				Disallow: true,
			},
			"mochi": { // ACL only, will defer to AuthRules for authentication
				ACL: Filters{
					"special/mochi": ReadOnly,
					"secret/mochi":  Deny,
					"ignored":       ReadWrite,
				},
			},
		},
		Auth: AuthRules{
			{Username: "banned-user"},                               // never allow specific username
			{Remote: "127.0.0.1", Allow: true},                      // always allow localhost
			{Remote: "123.123.123.123"},                             // disallow any from specific address
			{Username: "not-mochi", Remote: "111.144.155.166"},      // disallow specific username and address
			{Remote: "111.*", Allow: true},                          // allow any in wildcard (that isn't the above username)
			{Username: "mochi", Password: "melon", Allow: true},     // allow matching user/pass
			{Username: "mochi-co", Password: "melon", Allow: false}, // allow matching user/pass (should never trigger due to Users map)
		},
		ACL: ACLRules{
			{
				Username: "mochi", // allow matching user/pass
				Filters: Filters{
					"a/b/c":     Deny,
					"d/+/f":     Deny,
					"mochi/#":   ReadWrite,
					"updates/#": WriteOnly,
					"readonly":  ReadOnly,
					"ignored":   Deny,
				},
			},
			{Remote: "localhost", Filters: Filters{"$SYS/#": ReadOnly}}, // allow $SYS access to localhost
			{Username: "admin", Filters: Filters{"$SYS/#": ReadOnly}},   // allow $SYS access to admin
			{Remote: "001.002.003.004"},                                 // Allow all with no filter
			{Filters: Filters{"$SYS/#": Deny}},                          // Deny $SYS access to all others
		},
	}
)

func TestRStringMatches(t *testing.T) {
	require.True(t, RString("*").Matches("any"))
	require.True(t, RString("*").Matches(""))
	require.True(t, RString("").Matches("any"))
	require.True(t, RString("").Matches(""))
	require.False(t, RString("no").Matches("any"))
	require.False(t, RString("no").Matches(""))
}

func TestCanAuthenticate(t *testing.T) {
	tt := []struct {
		desc   string
		client *mqtt.Client
		pk     packets.Packet
		n      int
		ok     bool
	}{
		{
			desc: "allow all local 127.0.0.1",
			client: &mqtt.Client{
				Properties: mqtt.ClientProperties{
					Username: []byte("mochi"),
				},
				Net: mqtt.ClientConnection{
					Remote: "127.0.0.1",
				},
			},
			pk: packets.Packet{Connect: packets.ConnectParams{}},
			ok: true,
			n:  1,
		},
		{
			desc: "allow username/password",
			client: &mqtt.Client{
				Properties: mqtt.ClientProperties{
					Username: []byte("mochi"),
				},
			},
			pk: packets.Packet{Connect: packets.ConnectParams{Password: []byte("melon")}},
			ok: true,
			n:  5,
		},
		{
			desc: "deny username/password",
			client: &mqtt.Client{
				Properties: mqtt.ClientProperties{
					Username: []byte("mochi"),
				},
			},
			pk: packets.Packet{Connect: packets.ConnectParams{Password: []byte("bad-pass")}},
			ok: false,
			n:  0,
		},
		{
			desc: "allow all local 127.0.0.1",
			client: &mqtt.Client{
				Properties: mqtt.ClientProperties{
					Username: []byte("mochi"),
				},
				Net: mqtt.ClientConnection{
					Remote: "127.0.0.1",
				},
			},
			pk: packets.Packet{Connect: packets.ConnectParams{Password: []byte("bad-pass")}},
			ok: true,
			n:  1,
		},
		{
			desc: "allow username/password",
			client: &mqtt.Client{
				Properties: mqtt.ClientProperties{
					Username: []byte("mochi"),
				},
			},
			pk: packets.Packet{Connect: packets.ConnectParams{Password: []byte("melon")}},
			ok: true,
			n:  5,
		},
		{
			desc: "deny username/password",
			client: &mqtt.Client{
				Properties: mqtt.ClientProperties{
					Username: []byte("mochi"),
				},
			},
			pk: packets.Packet{Connect: packets.ConnectParams{Password: []byte("bad-pass")}},
			ok: false,
			n:  0,
		},
		{
			desc: "deny client from address",
			client: &mqtt.Client{
				Properties: mqtt.ClientProperties{
					Username: []byte("not-mochi"),
				},
				Net: mqtt.ClientConnection{
					Remote: "111.144.155.166",
				},
			},
			pk: packets.Packet{},
			ok: false,
			n:  3,
		},
		{
			desc: "allow remote wildcard",
			client: &mqtt.Client{
				Properties: mqtt.ClientProperties{
					Username: []byte("mochi"),
				},
				Net: mqtt.ClientConnection{
					Remote: "111.0.0.1",
				},
			},
			pk: packets.Packet{},
			ok: true,
			n:  4,
		},
		{
			desc: "never allow username",
			client: &mqtt.Client{
				Properties: mqtt.ClientProperties{
					Username: []byte("banned-user"),
				},
				Net: mqtt.ClientConnection{
					Remote: "127.0.0.1",
				},
			},
			pk: packets.Packet{},
			ok: false,
			n:  0,
		},
		{
			desc: "matching user in users",
			client: &mqtt.Client{
				Properties: mqtt.ClientProperties{
					Username: []byte("mochi-co"),
				},
			},
			pk: packets.Packet{Connect: packets.ConnectParams{Password: []byte("melon")}},
			ok: true,
			n:  0,
		},
		{
			desc: "never user in users",
			client: &mqtt.Client{
				Properties: mqtt.ClientProperties{
					Username: []byte("suspended-user"),
				},
			},
			pk: packets.Packet{Connect: packets.ConnectParams{Password: []byte("any")}},
			ok: false,
			n:  0,
		},
	}

	for _, d := range tt {
		t.Run(d.desc, func(t *testing.T) {
			n, ok := checkLedger.AuthOk(d.client, d.pk)
			require.Equal(t, d.n, n)
			require.Equal(t, d.ok, ok)
		})
	}
}

func TestCanACL(t *testing.T) {
	tt := []struct {
		client *mqtt.Client
		desc   string
		topic  string
		n      int
		write  bool
		ok     bool
	}{
		{
			desc:   "allow normal write on any other filter",
			client: &mqtt.Client{},
			topic:  "default/acl/write/access",
			write:  true,
			ok:     true,
		},
		{
			desc:   "allow normal read on any other filter",
			client: &mqtt.Client{},
			topic:  "default/acl/read/access",
			write:  false,
			ok:     true,
		},
		{
			desc: "deny user on literal filter",
			client: &mqtt.Client{
				Properties: mqtt.ClientProperties{
					Username: []byte("mochi"),
				},
			},
			topic: "a/b/c",
		},
		{
			desc: "deny user on partial filter",
			client: &mqtt.Client{
				Properties: mqtt.ClientProperties{
					Username: []byte("mochi"),
				},
			},
			topic: "d/j/f",
		},
		{
			desc: "allow read/write to user path",
			client: &mqtt.Client{
				Properties: mqtt.ClientProperties{
					Username: []byte("mochi"),
				},
			},
			topic: "mochi/read/write",
			write: true,
			ok:    true,
		},
		{
			desc: "deny read on write-only path",
			client: &mqtt.Client{
				Properties: mqtt.ClientProperties{
					Username: []byte("mochi"),
				},
			},
			topic: "updates/no/reading",
			write: false,
			ok:    false,
		},
		{
			desc: "deny read on write-only path ext",
			client: &mqtt.Client{
				Properties: mqtt.ClientProperties{
					Username: []byte("mochi"),
				},
			},
			topic: "updates/mochi",
			write: false,
			ok:    false,
		},
		{
			desc: "allow read on not-acl path (no #)",
			client: &mqtt.Client{
				Properties: mqtt.ClientProperties{
					Username: []byte("mochi"),
				},
			},
			topic: "updates",
			write: false,
			ok:    true,
		},
		{
			desc: "allow write on write-only path",
			client: &mqtt.Client{
				Properties: mqtt.ClientProperties{
					Username: []byte("mochi"),
				},
			},
			topic: "updates/mochi",
			write: true,
			ok:    true,
		},
		{
			desc: "deny write on read-only path",
			client: &mqtt.Client{
				Properties: mqtt.ClientProperties{
					Username: []byte("mochi"),
				},
			},
			topic: "readonly",
			write: true,
			ok:    false,
		},
		{
			desc: "allow read on read-only path",
			client: &mqtt.Client{
				Properties: mqtt.ClientProperties{
					Username: []byte("mochi"),
				},
			},
			topic: "readonly",
			write: false,
			ok:    true,
		},
		{
			desc: "allow $sys access to localhost",
			client: &mqtt.Client{
				Net: mqtt.ClientConnection{
					Remote: "localhost",
				},
			},
			topic: "$SYS/test",
			write: false,
			ok:    true,
			n:     1,
		},
		{
			desc: "allow $sys access to admin",
			client: &mqtt.Client{
				Properties: mqtt.ClientProperties{
					Username: []byte("admin"),
				},
			},
			topic: "$SYS/test",
			write: false,
			ok:    true,
			n:     2,
		},
		{
			desc: "deny $sys access to all others",
			client: &mqtt.Client{
				Properties: mqtt.ClientProperties{
					Username: []byte("mochi"),
				},
			},
			topic: "$SYS/test",
			write: false,
			ok:    false,
			n:     4,
		},
		{
			desc: "allow all with no filter",
			client: &mqtt.Client{
				Net: mqtt.ClientConnection{
					Remote: "001.002.003.004",
				},
			},
			topic: "any/path",
			write: true,
			ok:    true,
			n:     3,
		},
		{
			desc: "use users embedded acl deny",
			client: &mqtt.Client{
				Properties: mqtt.ClientProperties{
					Username: []byte("mochi"),
				},
			},
			topic: "secret/mochi",
			write: true,
			ok:    false,
		},
		{
			desc: "use users embedded acl any",
			client: &mqtt.Client{
				Properties: mqtt.ClientProperties{
					Username: []byte("mochi"),
				},
			},
			topic: "any/mochi",
			write: true,
			ok:    true,
		},
		{
			desc: "use users embedded acl write on read-only",
			client: &mqtt.Client{
				Properties: mqtt.ClientProperties{
					Username: []byte("mochi"),
				},
			},
			topic: "special/mochi",
			write: true,
			ok:    false,
		},
		{
			desc: "use users embedded acl read on read-only",
			client: &mqtt.Client{
				Properties: mqtt.ClientProperties{
					Username: []byte("mochi"),
				},
			},
			topic: "special/mochi",
			write: false,
			ok:    true,
		},
		{
			desc: "preference users embedded acl",
			client: &mqtt.Client{
				Properties: mqtt.ClientProperties{
					Username: []byte("mochi"),
				},
			},
			topic: "ignored",
			write: true,
			ok:    true,
		},
	}

	for _, d := range tt {
		t.Run(d.desc, func(t *testing.T) {
			n, ok := checkLedger.ACLOk(d.client, d.topic, d.write)
			require.Equal(t, d.n, n)
			require.Equal(t, d.ok, ok)
		})
	}
}

func TestMatchTopic(t *testing.T) {
	el, matched := MatchTopic("a/+/c/+", "a/b/c/d")
	require.True(t, matched)
	require.Equal(t, []string{"b", "d"}, el)

	el, matched = MatchTopic("a/+/+/+", "a/b/c/d")
	require.True(t, matched)
	require.Equal(t, []string{"b", "c", "d"}, el)

	el, matched = MatchTopic("stuff/#", "stuff/things/yeah")
	require.True(t, matched)
	require.Equal(t, []string{"things/yeah"}, el)

	el, matched = MatchTopic("a/+/#/+", "a/b/c/d/as/dds")
	require.True(t, matched)
	require.Equal(t, []string{"b", "c/d/as/dds"}, el)

	el, matched = MatchTopic("test", "test")
	require.True(t, matched)
	require.Equal(t, make([]string, 0), el)

	el, matched = MatchTopic("things/stuff//", "things/stuff/")
	require.False(t, matched)
	require.Equal(t, make([]string, 0), el)

	el, matched = MatchTopic("t", "t2")
	require.False(t, matched)
	require.Equal(t, make([]string, 0), el)

	el, matched = MatchTopic(" ", "  ")
	require.False(t, matched)
	require.Equal(t, make([]string, 0), el)
}

var (
	ledgerStruct = Ledger{
		Users: Users{
			"mochi": {
				Password: "peach",
				ACL: Filters{
					"readonly": ReadOnly,
					"deny":     Deny,
				},
			},
		},
		Auth: AuthRules{
			{
				Client:   "*",
				Username: "mochi-co",
				Password: "melon",
				Remote:   "192.168.1.*",
				Allow:    true,
			},
		},
		ACL: ACLRules{
			{
				Client:   "*",
				Username: "mochi-co",
				Remote:   "127.*",
				Filters: Filters{
					"readonly":  ReadOnly,
					"writeonly": WriteOnly,
					"readwrite": ReadWrite,
					"deny":      Deny,
				},
			},
		},
	}

	ledgerJSON = []byte(`{"users":{"mochi":{"password":"peach","acl":{"deny":0,"readonly":1}}},"auth":[{"client":"*","username":"mochi-co","remote":"192.168.1.*","password":"melon","allow":true}],"acl":[{"client":"*","username":"mochi-co","remote":"127.*","filters":{"deny":0,"readonly":1,"readwrite":3,"writeonly":2}}]}`)
	ledgerYAML = []byte(`users:
    mochi:
        password: peach
        acl:
            deny: 0
            readonly: 1
auth:
    - client: '*'
      username: mochi-co
      remote: 192.168.1.*
      password: melon
      allow: true
acl:
    - client: '*'
      username: mochi-co
      remote: 127.*
      filters:
        deny: 0
        readonly: 1
        readwrite: 3
        writeonly: 2
`)
)

func TestLedgerUpdate(t *testing.T) {
	old := &Ledger{
		Auth: AuthRules{
			{Remote: "127.0.0.1", Allow: true},
		},
	}

	n := &Ledger{
		Auth: AuthRules{
			{Remote: "127.0.0.1", Allow: true},
			{Remote: "192.168.*", Allow: true},
		},
	}

	old.Update(n)
	require.Len(t, old.Auth, 2)
	require.Equal(t, RString("192.168.*"), old.Auth[1].Remote)
	require.NotSame(t, n, old)
}

func TestLedgerToJSON(t *testing.T) {
	data, err := ledgerStruct.ToJSON()
	require.NoError(t, err)
	require.Equal(t, ledgerJSON, data)
}

func TestLedgerToYAML(t *testing.T) {
	data, err := ledgerStruct.ToYAML()
	require.NoError(t, err)
	require.Equal(t, ledgerYAML, data)
}

func TestLedgerUnmarshalFromYAML(t *testing.T) {
	l := new(Ledger)
	err := l.Unmarshal(ledgerYAML)
	require.NoError(t, err)
	require.Equal(t, &ledgerStruct, l)
	require.NotSame(t, l, &ledgerStruct)
}

func TestLedgerUnmarshalFromJSON(t *testing.T) {
	l := new(Ledger)
	err := l.Unmarshal(ledgerJSON)
	require.NoError(t, err)
	require.Equal(t, &ledgerStruct, l)
	require.NotSame(t, l, &ledgerStruct)
}

func TestLedgerUnmarshalNil(t *testing.T) {
	l := new(Ledger)
	err := l.Unmarshal([]byte{})
	require.NoError(t, err)
	require.Equal(t, new(Ledger), l)
}
