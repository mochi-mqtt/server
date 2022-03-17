package redis

import (
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/mochi-co/mqtt/server/persistence"
	"github.com/mochi-co/mqtt/server/system"
	"github.com/stretchr/testify/require"
	"testing"
)

var opts = &redis.Options{
	Addr:     "localhost:6379",
	Password: "", // no password set
	DB:       0,  // use default DB
}

func teardown(s *Store, t *testing.T) {
	s.Close()
}

func TestNew(t *testing.T) {
	s := New(opts)
	require.NotNil(t, s)
}

func TestNewNoOpts(t *testing.T) {
	s := New(nil)
	require.NotNil(t, s)
	require.Equal(t, "", s.opts.Addr)
}

func TestOpen(t *testing.T) {
	mr, _ := miniredis.Run()
	opts.Addr = mr.Addr()
	defer mr.Close()
	s := New(opts)
	err := s.Open()
	require.NoError(t, err)
	require.Equal(t, opts.Addr, s.opts.Addr)
	defer teardown(s, t)
	require.NotNil(t, s.db)
}

func TestWriteAndRetrieveServerInfo(t *testing.T) {
	mr, _ := miniredis.Run()
	opts.Addr = mr.Addr()
	defer mr.Close()
	s := New(opts)
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
	s := New(nil)
	err := s.WriteServerInfo(persistence.ServerInfo{})
	require.Error(t, err)
}

func TestWriteServerInfoFail(t *testing.T) {
	mr, _ := miniredis.Run()
	opts.Addr = mr.Addr()
	defer mr.Close()
	s := New(opts)
	err := s.Open()
	require.NoError(t, err)
	err = s.WriteServerInfo(persistence.ServerInfo{})
	require.Error(t, err)
}

func TestReadServerInfoNoDB(t *testing.T) {
	mr, _ := miniredis.Run()
	opts.Addr = mr.Addr()
	defer mr.Close()
	s := New(opts)
	_, err := s.ReadServerInfo()
	require.Error(t, err)
}

func TestReadServerInfoFail(t *testing.T) {
	mr, _ := miniredis.Run()
	opts.Addr = mr.Addr()
	defer mr.Close()
	s := New(opts)
	err := s.Open()
	require.NoError(t, err)
	s.Close()
	_, err = s.ReadServerInfo()
	require.Error(t, err)
}

func TestWriteRetrieveDeleteSubscription(t *testing.T) {
	mr, _ := miniredis.Run()
	opts.Addr = mr.Addr()
	defer mr.Close()
	s := New(opts)
	err := s.Open()
	require.NoError(t, err)
	defer teardown(s, t)

	v := persistence.Subscription{
		ID:     "a/b/c",
		Client: "test",
		Filter: "a/b/c",
		QoS:    1,
		T:      persistence.KSubscription,
	}
	err = s.WriteSubscription(v)
	require.NoError(t, err)

	v2 := persistence.Subscription{
		ID:     "d/e/f",
		Client: "test",
		Filter: "d/e/f",
		QoS:    2,
		T:      persistence.KSubscription,
	}
	err = s.WriteSubscription(v2)
	require.NoError(t, err)

	subs, err := s.ReadSubscriptionsByCid("test")
	require.NoError(t, err)
	require.Equal(t, persistence.KSubscription, subs[0].T)
	require.Equal(t, 2, len(subs))

	err = s.DeleteSubscription("test", "d/e/f")
	require.NoError(t, err)

	subs, err = s.ReadSubscriptionsByCid("test")
	require.NoError(t, err)
	require.Equal(t, 1, len(subs))
}

func TestWriteSubscriptionNoDB(t *testing.T) {
	mr, _ := miniredis.Run()
	opts.Addr = mr.Addr()
	defer mr.Close()
	s := New(opts)
	err := s.WriteSubscription(persistence.Subscription{})
	require.Error(t, err)
}

func TestWriteSubscriptionFail(t *testing.T) {
	mr, _ := miniredis.Run()
	opts.Addr = mr.Addr()
	defer mr.Close()
	s := New(opts)
	err := s.Open()
	require.NoError(t, err)
	s.Close()
	err = s.WriteSubscription(persistence.Subscription{})
	require.Error(t, err)
}

func TestReadSubscriptionNoDB(t *testing.T) {
	mr, _ := miniredis.Run()
	opts.Addr = mr.Addr()
	defer mr.Close()
	s := New(opts)
	_, err := s.ReadSubscriptionsByCid("test")
	require.Error(t, err)
}

func TestReadSubscriptionFail(t *testing.T) {
	mr, _ := miniredis.Run()
	opts.Addr = mr.Addr()
	defer mr.Close()
	s := New(opts)
	err := s.Open()
	require.NoError(t, err)
	s.Close()
	_, err = s.ReadSubscriptionsByCid("test")
	require.Error(t, err)
}

func TestWriteRetrieveDeleteInflight(t *testing.T) {
	mr, _ := miniredis.Run()
	opts.Addr = mr.Addr()
	defer mr.Close()
	s := New(opts)
	err := s.Open()
	require.NoError(t, err)
	defer teardown(s, t)

	v := persistence.Message{
		ID:        "0",
		T:         persistence.KInflight,
		PacketID:  0,
		Client: "client1",
		TopicName: "a/b/c",
		Payload:   []byte{'h', 'e', 'l', 'l', 'o'},
		Sent:      100,
		Resends:   0,
	}
	err = s.WriteInflight(v)
	require.NoError(t, err)

	v2 := persistence.Message{
		ID:        "100",
		T:         persistence.KInflight,
		PacketID:  100,
		Client: "client1",
		TopicName: "d/e/f",
		Payload:   []byte{'y', 'e', 's'},
		Sent:      200,
		Resends:   1,
	}
	err = s.WriteInflight(v2)
	require.NoError(t, err)

	msgs, err := s.ReadInflightByCid("client1")
	require.NoError(t, err)
	require.Equal(t, persistence.KInflight, msgs[0].T)
	require.Equal(t, 2, len(msgs))

	err = s.DeleteInflight("client1", 100)
	require.NoError(t, err)

	msgs, err = s.ReadInflightByCid("client1")
	require.NoError(t, err)
	require.Equal(t, 1, len(msgs))

}

func TestWriteInflightNoDB(t *testing.T) {
	mr, _ := miniredis.Run()
	opts.Addr = mr.Addr()
	defer mr.Close()
	s := New(opts)
	err := s.WriteInflight(persistence.Message{})
	require.Error(t, err)
}

func TestWriteInflightFail(t *testing.T) {
	mr, _ := miniredis.Run()
	opts.Addr = mr.Addr()
	defer mr.Close()
	s := New(opts)
	err := s.Open()
	require.NoError(t, err)
	s.Close()
	err = s.WriteInflight(persistence.Message{})
	require.Error(t, err)
}

func TestReadInflightNoDB(t *testing.T) {
	mr, _ := miniredis.Run()
	opts.Addr = mr.Addr()
	defer mr.Close()
	s := New(opts)
	_, err := s.ReadInflightByCid("test")
	require.Error(t, err)
}

func TestReadInflightFail(t *testing.T) {
	mr, _ := miniredis.Run()
	opts.Addr = mr.Addr()
	defer mr.Close()
	s := New(opts)
	err := s.Open()
	require.NoError(t, err)
	s.Close()
	_, err = s.ReadInflightByCid("test")
	require.Error(t, err)
}

func TestWriteRetrieveDeleteRetained(t *testing.T) {
	mr, _ := miniredis.Run()
	opts.Addr = mr.Addr()
	defer mr.Close()
	s := New(opts)
	err := s.Open()
	require.NoError(t, err)
	defer teardown(s, t)

	v := persistence.Message{
		ID: s.GenRetainedId("a/b/c"),
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
		ID: s.GenRetainedId("d/e/f"),
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

	msg, err := s.ReadRetainedByTopic("d/e/f")
	require.NoError(t, err)
	require.Equal(t, persistence.KRetained, msg.T)
	require.Equal(t, true, msg.FixedHeader.Retain)

	err = s.DeleteRetained("d/e/f")
	require.NoError(t, err)
}

func TestWriteRetainedNoDB(t *testing.T) {
	mr, _ := miniredis.Run()
	opts.Addr = mr.Addr()
	defer mr.Close()
	s := New(opts)
	err := s.WriteRetained(persistence.Message{})
	require.Error(t, err)
}

func TestWriteRetainedFail(t *testing.T) {
	mr, _ := miniredis.Run()
	opts.Addr = mr.Addr()
	defer mr.Close()
	s := New(opts)
	err := s.Open()
	require.NoError(t, err)

	err = s.WriteRetained(persistence.Message{})
	require.Error(t, err)
}

func TestReadRetainedNoDB(t *testing.T) {
	mr, _ := miniredis.Run()
	opts.Addr = mr.Addr()
	defer mr.Close()
	s := New(opts)
	_, err := s.ReadRetainedByTopic("a/b/c")
	require.Error(t, err)
}

func TestReadRetainedFail(t *testing.T) {
	mr, _ := miniredis.Run()
	opts.Addr = mr.Addr()
	defer mr.Close()
	s := New(opts)
	err := s.Open()
	require.NoError(t, err)
	s.Close()
	_, err = s.ReadRetainedByTopic("a/b/c")
	require.Error(t, err)
}

func TestWriteRetrieveDeleteClients(t *testing.T) {
	mr, _ := miniredis.Run()
	opts.Addr = mr.Addr()
	defer mr.Close()
	s := New(opts)
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

	client, err := s.ReadClientByCid("client1")
	require.NoError(t, err)

	require.Equal(t, []byte{'m', 'o', 'c', 'h', 'i'}, client.Username)
	require.Equal(t, "a/b/c", client.LWT.Topic)

	v2 := persistence.Client{
		ID:       "cl_client2",
		ClientID: "client2",
		T:        persistence.KClient,
		Listener: "tcp1",
	}
	err = s.WriteClient(v2)
	require.NoError(t, err)

	client, err = s.ReadClientByCid("client2")
	require.NoError(t, err)
	require.Equal(t, persistence.KClient, client.T)

	err = s.DeleteClient("client2")
	require.NoError(t, err)
}

func TestWriteClientNoDB(t *testing.T) {
	mr, _ := miniredis.Run()
	opts.Addr = mr.Addr()
	defer mr.Close()
	s := New(opts)
	err := s.WriteClient(persistence.Client{})
	require.Error(t, err)
}

func TestWriteClientFail(t *testing.T) {
	mr, _ := miniredis.Run()
	opts.Addr = mr.Addr()
	defer mr.Close()
	s := New(opts)
	err := s.Open()
	require.NoError(t, err)
	s.Close()
	err = s.WriteClient(persistence.Client{})
	require.Error(t, err)
}

func TestReadClientNoDB(t *testing.T) {
	mr, _ := miniredis.Run()
	opts.Addr = mr.Addr()
	defer mr.Close()
	s := New(opts)
	_, err := s.ReadClientByCid("test")
	require.Error(t, err)
}

func TestReadClientFail(t *testing.T) {
	mr, _ := miniredis.Run()
	opts.Addr = mr.Addr()
	defer mr.Close()
	s := New(opts)
	err := s.Open()
	require.NoError(t, err)
	s.Close()
	_, err = s.ReadClientByCid("test")
	require.Error(t, err)
}

func TestDeleteSubscriptionNoDB(t *testing.T) {
	mr, _ := miniredis.Run()
	opts.Addr = mr.Addr()
	defer mr.Close()
	s := New(opts)
	err := s.DeleteSubscription("test","a")
	require.Error(t, err)
}

func TestDeleteSubscriptionFail(t *testing.T) {
	mr, _ := miniredis.Run()
	opts.Addr = mr.Addr()
	defer mr.Close()
	s := New(opts)
	err := s.Open()
	require.NoError(t, err)
	s.Close()
	err = s.DeleteSubscription("test","a")
	require.Error(t, err)
}

func TestDeleteClientNoDB(t *testing.T) {
	mr, _ := miniredis.Run()
	opts.Addr = mr.Addr()
	defer mr.Close()
	s := New(opts)
	err := s.DeleteClient("test")
	require.Error(t, err)
}

func TestDeleteClientFail(t *testing.T) {
	mr, _ := miniredis.Run()
	opts.Addr = mr.Addr()
	defer mr.Close()
	s := New(opts)
	err := s.Open()
	require.NoError(t, err)
	s.Close()
	err = s.DeleteClient("a")
	require.Error(t, err)
}

func TestDeleteInflightNoDB(t *testing.T) {
	mr, _ := miniredis.Run()
	opts.Addr = mr.Addr()
	defer mr.Close()
	s := New(opts)
	err := s.DeleteInflight("test",1)
	require.Error(t, err)
}

func TestDeleteInflightFail(t *testing.T) {
	mr, _ := miniredis.Run()
	opts.Addr = mr.Addr()
	defer mr.Close()
	s := New(opts)
	err := s.Open()
	require.NoError(t, err)
	s.Close()
	err = s.DeleteInflight("test",1)
	require.Error(t, err)
}

func TestDeleteRetainedNoDB(t *testing.T) {
	mr, _ := miniredis.Run()
	opts.Addr = mr.Addr()
	defer mr.Close()
	s := New(opts)
	err := s.DeleteRetained("a")
	require.Error(t, err)
}

func TestDeleteRetainedFail(t *testing.T) {
	mr, _ := miniredis.Run()
	opts.Addr = mr.Addr()
	defer mr.Close()
	s := New(opts)
	err := s.Open()
	require.NoError(t, err)
	s.Close()
	err = s.DeleteRetained("a")
	require.Error(t, err)
}
