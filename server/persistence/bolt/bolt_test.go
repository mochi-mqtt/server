package bolt

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.etcd.io/bbolt"

	"github.com/mochi-co/mqtt/server/persistence"
	"github.com/mochi-co/mqtt/server/system"
)

const tmpPath = "testbolt.db"

func teardown(t *testing.T) {
	err := os.Remove(tmpPath)
	require.NoError(t, err)
}

func TestNew(t *testing.T) {
	s := New(tmpPath, &bbolt.Options{
		Timeout: 500 * time.Millisecond,
	})
	require.NotNil(t, s)
	require.Equal(t, tmpPath, s.path)
	require.Equal(t, 500*time.Millisecond, s.opts.Timeout)
}

func TestNewNoPath(t *testing.T) {
	s := New("", nil)
	require.NotNil(t, s)
	require.Equal(t, defaultPath, s.path)
}

func TestNewNoOpts(t *testing.T) {
	s := New("", nil)
	require.NotNil(t, s)
	require.Equal(t, defaultTimeout, s.opts.Timeout)
}

func TestOpen(t *testing.T) {
	s := New(tmpPath, nil)
	err := s.Open()
	require.NoError(t, err)
	defer teardown(t)
	require.NotNil(t, s.db)
}

func TestStoreAndRetrieveServerInfo(t *testing.T) {
	s := New(tmpPath, nil)
	err := s.Open()
	require.NoError(t, err)
	defer teardown(t)

	v := system.Info{
		Version: "test",
		Started: 100,
	}
	err = s.WriteServerInfo(persistence.ServerInfo{v, persistence.KServerInfo})
	require.NoError(t, err)

	r, err := s.ReadServerInfo()
	require.NoError(t, err)
	require.NotNil(t, r)
	require.Equal(t, v.Version, r.Version)
	require.Equal(t, v.Started, r.Started)
}

func TestWriteAndRetrieveSubscription(t *testing.T) {
	s := New(tmpPath, nil)
	err := s.Open()
	require.NoError(t, err)
	defer teardown(t)

	v := persistence.Subscription{
		ID:     "test:a/b/c",
		Client: "test",
		Filter: "a/b/c",
		QoS:    1,
		T:      persistence.KSubscription,
	}
	err = s.WriteSubscription(v)
	require.NoError(t, err)

	v2 := persistence.Subscription{
		ID:     "test:d/e/f",
		Client: "test",
		Filter: "d/e/f",
		QoS:    2,
		T:      persistence.KSubscription,
	}
	err = s.WriteSubscription(v2)
	require.NoError(t, err)

	subs, err := s.ReadSubscriptions()
	require.NoError(t, err)
	require.Equal(t, persistence.KSubscription, subs[0].T)
	require.Equal(t, 2, len(subs))
}

func TestWriteAndRetrieveInflight(t *testing.T) {
	s := New(tmpPath, nil)
	err := s.Open()
	require.NoError(t, err)
	defer teardown(t)

	v := persistence.Message{
		ID:        "client1_if_0",
		T:         persistence.KInflight,
		PacketID:  0,
		TopicName: "a/b/c",
		Payload:   []byte{'h', 'e', 'l', 'l', 'o'},
		Sent:      100,
		Resends:   0,
	}
	err = s.WriteInflight(v)
	require.NoError(t, err)

	v2 := persistence.Message{
		ID:        "client1_if_100",
		T:         persistence.KInflight,
		PacketID:  100,
		TopicName: "d/e/f",
		Payload:   []byte{'y', 'e', 's'},
		Sent:      200,
		Resends:   1,
	}
	err = s.WriteInflight(v2)
	require.NoError(t, err)

	msgs, err := s.ReadInflight()
	require.NoError(t, err)
	require.Equal(t, persistence.KInflight, msgs[0].T)
	require.Equal(t, 2, len(msgs))
}

func TestWriteAndRetrievePersistent(t *testing.T) {
	s := New(tmpPath, nil)
	err := s.Open()
	require.NoError(t, err)
	defer teardown(t)

	v := persistence.Message{
		ID: "client1_ret_200",
		T:  persistence.KRetained,
		FixedHeader: persistence.FixedHeader{
			Retain: true,
		},
		PacketID:  200,
		TopicName: "a/b/c",
		Payload:   []byte{'h', 'e', 'l', 'l', 'o'},
		Sent:      100,
		Resends:   0,
	}
	err = s.WriteInflight(v)
	require.NoError(t, err)

	v2 := persistence.Message{
		ID: "client1_ret_300",
		T:  persistence.KRetained,
		FixedHeader: persistence.FixedHeader{
			Retain: true,
		},
		PacketID:  100,
		TopicName: "d/e/f",
		Payload:   []byte{'y', 'e', 's'},
		Sent:      200,
		Resends:   1,
	}
	err = s.WriteInflight(v2)
	require.NoError(t, err)

	msgs, err := s.ReadRetained()
	require.NoError(t, err)
	require.Equal(t, persistence.KRetained, msgs[0].T)
	require.Equal(t, true, msgs[0].FixedHeader.Retain)
	require.Equal(t, 2, len(msgs))
}
