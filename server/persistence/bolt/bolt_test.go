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

func teardown(s *Store, t *testing.T) {
	s.Close()
	err := os.Remove(tmpPath)
	require.NoError(t, err)
}

func TestSatsifies(t *testing.T) {
	var x persistence.Store
	x = New(tmpPath, &bbolt.Options{
		Timeout: 500 * time.Millisecond,
	})
	require.NotNil(t, x)
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
	defer teardown(s, t)
	require.NotNil(t, s.db)
}

func TestOpenFailure(t *testing.T) {
	s := New("..", nil)
	err := s.Open()
	require.Error(t, err)
}

func TestWriteAndRetrieveServerInfo(t *testing.T) {
	s := New(tmpPath, nil)
	err := s.Open()
	require.NoError(t, err)
	defer teardown(s, t)

	v := system.Info{
		Version: "test",
		Started: 100,
	}
	err = s.WriteServerInfo(persistence.ServerInfo{
		Info: v,
		ID:   persistence.KServerInfo,
	})
	require.NoError(t, err)

	r, err := s.ReadServerInfo()
	require.NoError(t, err)
	require.NotNil(t, r)
	require.Equal(t, v.Version, r.Version)
	require.Equal(t, v.Started, r.Started)
}

func TestWriteServerInfoNoDB(t *testing.T) {
	s := New(tmpPath, nil)
	err := s.WriteServerInfo(persistence.ServerInfo{})
	require.Error(t, err)
}

func TestWriteServerInfoFail(t *testing.T) {
	s := New(tmpPath, nil)
	err := s.Open()
	require.NoError(t, err)

	err = os.Remove(tmpPath)
	require.NoError(t, err)

	err = s.WriteServerInfo(persistence.ServerInfo{})
	require.Error(t, err)
}

func TestReadServerInfoNoDB(t *testing.T) {
	s := New(tmpPath, nil)
	_, err := s.ReadServerInfo()
	require.Error(t, err)
}

func TestReadServerInfoFail(t *testing.T) {
	s := New(tmpPath, nil)
	err := s.Open()
	require.NoError(t, err)
	s.Close()
	_, err = s.ReadServerInfo()
	require.Error(t, err)
}

func TestWriteRetrieveDeleteSubscription(t *testing.T) {
	s := New(tmpPath, nil)
	err := s.Open()
	require.NoError(t, err)
	defer teardown(s, t)

	v := persistence.Subscription{
		ID:     "sub_test:a/b/c",
		Client: "test",
		Filter: "a/b/c",
		QoS:    1,
		T:      persistence.KSubscription,
	}
	err = s.WriteSubscription(v)
	require.NoError(t, err)

	v2 := persistence.Subscription{
		ID:     "sub_test:d/e/f",
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

	err = s.DeleteSubscription(v2.Client, "d/e/f")
	require.NoError(t, err)

	subs, err = s.ReadSubscriptions()
	require.NoError(t, err)
	require.Equal(t, 1, len(subs))
}

func TestWriteSubscriptionNoDB(t *testing.T) {
	s := New(tmpPath, nil)
	err := s.WriteSubscription(persistence.Subscription{})
	require.Error(t, err)
}

func TestWriteSubscriptionFail(t *testing.T) {
	s := New(tmpPath, nil)
	err := s.Open()
	require.NoError(t, err)
	s.Close()
	err = s.WriteSubscription(persistence.Subscription{})
	require.Error(t, err)
}

func TestReadSubscriptionNoDB(t *testing.T) {
	s := New(tmpPath, nil)
	_, err := s.ReadSubscriptions()
	require.Error(t, err)
}

func TestReadSubscriptionFail(t *testing.T) {
	s := New(tmpPath, nil)
	err := s.Open()
	require.NoError(t, err)
	s.Close()
	_, err = s.ReadSubscriptions()
	require.Error(t, err)
}

func TestWriteRetrieveDeleteInflight(t *testing.T) {
	s := New(tmpPath, nil)
	err := s.Open()
	require.NoError(t, err)
	defer teardown(s, t)

	v := persistence.Message{
		ID:        "ifm_client1_0",
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
		ID:        "ifm_client1_100",
		T:         persistence.KInflight,
		PacketID:  100,
		Client:    "client1",
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

	err = s.DeleteInflight(v2.Client, v2.PacketID)
	require.NoError(t, err)

	msgs, err = s.ReadInflight()
	require.NoError(t, err)
	require.Equal(t, 1, len(msgs))

}

func TestWriteInflightNoDB(t *testing.T) {
	s := New(tmpPath, nil)
	err := s.WriteInflight(persistence.Message{})
	require.Error(t, err)
}

func TestWriteInflightFail(t *testing.T) {
	s := New(tmpPath, nil)
	err := s.Open()
	require.NoError(t, err)
	s.Close()
	err = s.WriteInflight(persistence.Message{})
	require.Error(t, err)
}

func TestReadInflightNoDB(t *testing.T) {
	s := New(tmpPath, nil)
	_, err := s.ReadInflight()
	require.Error(t, err)
}

func TestReadInflightFail(t *testing.T) {
	s := New(tmpPath, nil)
	err := s.Open()
	require.NoError(t, err)
	s.Close()
	_, err = s.ReadInflight()
	require.Error(t, err)
}

func TestWriteRetrieveDeleteRetained(t *testing.T) {
	s := New(tmpPath, nil)
	err := s.Open()
	require.NoError(t, err)
	defer teardown(s, t)

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
	err = s.WriteRetained(v)
	require.NoError(t, err)

	v2 := persistence.Message{
		ID: "ret_d/e/f",
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
	err = s.WriteRetained(v2)
	require.NoError(t, err)

	msgs, err := s.ReadRetained()
	require.NoError(t, err)
	require.Equal(t, persistence.KRetained, msgs[0].T)
	require.Equal(t, true, msgs[0].FixedHeader.Retain)
	require.Equal(t, 2, len(msgs))

	err = s.DeleteRetained(v2.TopicName)
	require.NoError(t, err)

	msgs, err = s.ReadRetained()
	require.NoError(t, err)
	require.Equal(t, 1, len(msgs))
}

func TestWriteRetainedNoDB(t *testing.T) {
	s := New(tmpPath, nil)
	err := s.WriteRetained(persistence.Message{})
	require.Error(t, err)
}

func TestWriteRetainedFail(t *testing.T) {
	s := New(tmpPath, nil)
	err := s.Open()
	require.NoError(t, err)

	err = os.Remove(tmpPath)
	require.NoError(t, err)

	err = s.WriteRetained(persistence.Message{})
	require.Error(t, err)
}

func TestReadRetainedNoDB(t *testing.T) {
	s := New(tmpPath, nil)
	_, err := s.ReadRetained()
	require.Error(t, err)
}

func TestReadRetainedFail(t *testing.T) {
	s := New(tmpPath, nil)
	err := s.Open()
	require.NoError(t, err)
	s.Close()
	_, err = s.ReadRetained()
	require.Error(t, err)
}

func TestWriteRetrieveDeleteClients(t *testing.T) {
	s := New(tmpPath, nil)
	err := s.Open()
	require.NoError(t, err)
	defer teardown(s, t)

	v := persistence.Client{
		ID:       "cl_client1",
		ClientID: "client1",
		T:        persistence.KClient,
		Listener: "tcp1",
		Username: []byte{'m', 'o', 'c', 'h', 'i'},
		LWT: persistence.LWT{
			Topic:   "a/b/c",
			Message: []byte{'h', 'e', 'l', 'l', 'o'},
			Qos:     1,
			Retain:  true,
		},
	}
	err = s.WriteClient(v)
	require.NoError(t, err)

	clients, err := s.ReadClients()
	require.NoError(t, err)

	require.Equal(t, []byte{'m', 'o', 'c', 'h', 'i'}, clients[0].Username)
	require.Equal(t, "a/b/c", clients[0].LWT.Topic)

	v2 := persistence.Client{
		ID:       "cl_client2",
		ClientID: "client2",
		T:        persistence.KClient,
		Listener: "tcp1",
	}
	err = s.WriteClient(v2)
	require.NoError(t, err)

	clients, err = s.ReadClients()
	require.NoError(t, err)
	require.Equal(t, persistence.KClient, clients[0].T)
	require.Equal(t, 2, len(clients))

	err = s.DeleteClient("cl_client2")
	require.NoError(t, err)

	clients, err = s.ReadClients()
	require.NoError(t, err)
	require.Equal(t, 1, len(clients))
}

func TestWriteClientNoDB(t *testing.T) {
	s := New(tmpPath, nil)
	err := s.WriteClient(persistence.Client{})
	require.Error(t, err)
}

func TestWriteClientFail(t *testing.T) {
	s := New(tmpPath, nil)
	err := s.Open()
	require.NoError(t, err)
	s.Close()
	err = s.WriteClient(persistence.Client{})
	require.Error(t, err)
}

func TestReadClientNoDB(t *testing.T) {
	s := New(tmpPath, nil)
	_, err := s.ReadClients()
	require.Error(t, err)
}

func TestReadClientFail(t *testing.T) {
	s := New(tmpPath, nil)
	err := s.Open()
	require.NoError(t, err)
	s.Close()
	_, err = s.ReadClients()
	require.Error(t, err)
}

func TestDeleteSubscriptionNoDB(t *testing.T) {
	s := New(tmpPath, nil)
	err := s.DeleteSubscription("test","a")
	require.Error(t, err)
}

func TestDeleteSubscriptionFail(t *testing.T) {
	s := New(tmpPath, nil)
	err := s.Open()
	require.NoError(t, err)
	s.Close()
	err = s.DeleteSubscription("test", "a")
	require.Error(t, err)
}

func TestDeleteClientNoDB(t *testing.T) {
	s := New(tmpPath, nil)
	err := s.DeleteClient("a")
	require.Error(t, err)
}

func TestDeleteClientFail(t *testing.T) {
	s := New(tmpPath, nil)
	err := s.Open()
	require.NoError(t, err)
	s.Close()
	err = s.DeleteClient("a")
	require.Error(t, err)
}

func TestDeleteInflightNoDB(t *testing.T) {
	s := New(tmpPath, nil)
	err := s.DeleteInflight("test", 1)
	require.Error(t, err)
}

func TestDeleteInflightFail(t *testing.T) {
	s := New(tmpPath, nil)
	err := s.Open()
	require.NoError(t, err)
	s.Close()
	err = s.DeleteInflight("test", 1)
	require.Error(t, err)
}

func TestDeleteRetainedNoDB(t *testing.T) {
	s := New(tmpPath, nil)
	err := s.DeleteRetained("a")
	require.Error(t, err)
}

func TestDeleteRetainedFail(t *testing.T) {
	s := New(tmpPath, nil)
	err := s.Open()
	require.NoError(t, err)
	s.Close()
	err = s.DeleteRetained("a")
	require.Error(t, err)
}
