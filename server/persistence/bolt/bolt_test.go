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
	err = s.StoreServerInfo(persistence.ServerInfo{v, "server_info"})
	require.NoError(t, err)

	r, err := s.LoadServerInfo()
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
		T:      "subscription",
	}
	err = s.WriteSubscription(v)
	require.NoError(t, err)

	v2 := persistence.Subscription{
		ID:     "test:d/e/f",
		Client: "test",
		Filter: "d/e/f",
		QoS:    2,
		T:      "subscription",
	}
	err = s.WriteSubscription(v2)
	require.NoError(t, err)

	subs, err := s.LoadSubscriptions()
	require.NoError(t, err)
	require.Equal(t, 2, len(subs))

}
