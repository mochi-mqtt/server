package redis

import (
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
	s := New(nil)
	err := s.Open()
	require.NoError(t, err)
	require.Equal(t, opts.Addr, s.opts.Addr)
	defer teardown(s, t)
	require.NotNil(t, s.db)
}

func TestWriteAndRetrieveServerInfo(t *testing.T) {
	s := New(nil)
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
	s := New(nil)
	err := s.Open()
	require.NoError(t, err)
	err = s.WriteServerInfo(persistence.ServerInfo{})
	require.Error(t, err)
}

func TestReadServerInfoNoDB(t *testing.T) {
	s := New(nil)
	_, err := s.ReadServerInfo()
	require.Error(t, err)
}

func TestReadServerInfoFail(t *testing.T) {
	s := New(nil)
	err := s.Open()
	require.NoError(t, err)
	s.Close()
	_, err = s.ReadServerInfo()
	require.Error(t, err)
}

func TestWriteRetrieveDeleteSubscription(t *testing.T) {
	s := New(nil)
	err := s.Open()
	require.NoError(t, err)
	defer teardown(s, t)

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

	subs, err := s.ReadSubscriptions(v2.Client)
	require.NoError(t, err)
	require.Equal(t, persistence.KSubscription, subs[0].T)
	require.Equal(t, 2, len(subs))

	err = s.DeleteSubscription(v2.Client,"test:d/e/f")
	require.NoError(t, err)

	subs, err = s.ReadSubscriptions(v2.Client)
	require.NoError(t, err)
	require.Equal(t, 1, len(subs))
}

func TestWriteSubscriptionNoDB(t *testing.T) {
	s := New(nil)
	err := s.WriteSubscription(persistence.Subscription{
		ID:     "test:g/h/k",
		Client: "test",
		Filter: "g/h/k",
		QoS:    1,
		T:      persistence.KSubscription,
	})
	require.Error(t, err)
}

func TestWriteSubscriptionFail(t *testing.T) {
	s := New(nil)
	err := s.Open()
	require.NoError(t, err)
	s.Close()
	err = s.WriteSubscription(persistence.Subscription{
		ID:     "test:g/h/k",
		Client: "test",
		Filter: "g/h/k",
		QoS:    1,
		T:      persistence.KSubscription,
	})
	require.Error(t, err)
}

func TestReadSubscriptionNoDB(t *testing.T) {
	s := New(nil)
	cid := "test"
	_, err := s.ReadSubscriptions(cid)
	require.Error(t, err)
}

func TestReadSubscriptionFail(t *testing.T) {
	s := New(nil)
	err := s.Open()
	require.NoError(t, err)
	s.Close()
	cid := "test"
	_, err = s.ReadSubscriptions(cid)
	require.Error(t, err)
}

func TestWriteRetrieveDeleteInflight(t *testing.T) {
	s := New(nil)
	err := s.Open()
	require.NoError(t, err)
	defer teardown(s, t)

	v := persistence.Message{
		ID:        "client1_if_0",
		T:         persistence.KInflight,
		Client:    "test",
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
		Client: "test",
		PacketID:  100,
		TopicName: "d/e/f",
		Payload:   []byte{'y', 'e', 's'},
		Sent:      200,
		Resends:   1,
	}
	err = s.WriteInflight(v2)
	require.NoError(t, err)

	msgs, err := s.ReadInflight(v.Client)
	require.NoError(t, err)
	require.Equal(t, persistence.KInflight, msgs[0].T)
	require.Equal(t, 2, len(msgs))

	err = s.DeleteInflight(v2.Client, v2.ID)
	require.NoError(t, err)

	msgs, err = s.ReadInflight(v.Client)
	require.NoError(t, err)
	require.Equal(t, 1, len(msgs))

}

func TestWriteInflightNoDB(t *testing.T) {
	s := New(nil)
	v := persistence.Message{
		ID:        "client1_if_1",
		T:         persistence.KInflight,
		Client:    "test",
		PacketID:  0,
		TopicName: "a/b/c",
		Payload:   []byte{'w', 'o', 'r', 'l', 'd'},
		Sent:      100,
		Resends:   0,
	}
	err := s.WriteInflight(v)
	require.Error(t, err)
}

func TestWriteInflightFail(t *testing.T) {
	s := New(nil)
	err := s.Open()
	require.NoError(t, err)
	s.Close()
	v := persistence.Message{
		ID:        "client1_if_2",
		T:         persistence.KInflight,
		Client:    "test",
		PacketID:  0,
		TopicName: "a/b/c",
		Payload:   []byte{'w', 'o', 'r', 'l', 'd'},
		Sent:      100,
		Resends:   0,
	}
	err = s.WriteInflight(v)
	require.Error(t, err)
}

func TestReadInflightNoDB(t *testing.T) {
	s := New(nil)
	cid := "test"
	_, err := s.ReadInflight(cid)
	require.Error(t, err)
}

func TestReadInflightFail(t *testing.T) {
	s := New(nil)
	err := s.Open()
	require.NoError(t, err)
	s.Close()
	cid := "test"
	_, err = s.ReadInflight(cid)
	require.Error(t, err)
}

func TestWriteRetrieveDeleteRetained(t *testing.T) {
	s := New(nil)
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
	err = s.WriteRetained(v2)
	require.NoError(t, err)

	msg, err := s.ReadRetained(v2.TopicName)
	require.NoError(t, err)
	require.Equal(t, persistence.KRetained, msg.T)
	require.Equal(t, true, msg.FixedHeader.Retain)

	err = s.DeleteRetained("client1_ret_300")
	require.NoError(t, err)

	msg, err = s.ReadRetained(v2.TopicName)
	require.NoError(t, err)
}

func TestWriteRetainedNoDB(t *testing.T) {
	s := New(nil)
	err := s.WriteRetained(persistence.Message{})
	require.Error(t, err)
}

func TestWriteRetainedFail(t *testing.T) {
	s := New(nil)
	err := s.Open()
	require.NoError(t, err)

	err = s.WriteRetained(persistence.Message{})
	require.Error(t, err)
}

func TestReadRetainedNoDB(t *testing.T) {
	s := New(nil)
	topic := "d/e/f"
	_, err := s.ReadRetained(topic)
	require.Error(t, err)
}

func TestReadRetainedFail(t *testing.T) {
	s := New(nil)
	err := s.Open()
	require.NoError(t, err)
	s.Close()
	topic := "d/e/f"
	_, err = s.ReadRetained(topic)
	require.Error(t, err)
}

func TestWriteRetrieveDeleteClients(t *testing.T) {
	s := New(nil)
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

	client, err := s.ReadClient(v.ClientID)
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

	client, err = s.ReadClient(v2.ClientID)
	require.NoError(t, err)
	require.Equal(t, persistence.KClient, client.T)

	err = s.DeleteClient("cl_client2")
	require.NoError(t, err)

	client, err = s.ReadClient(v2.ClientID)
	require.NoError(t, err)
}

func TestWriteClientNoDB(t *testing.T) {
	s := New(nil)
	err := s.WriteClient(persistence.Client{})
	require.Error(t, err)
}

func TestWriteClientFail(t *testing.T) {
	s := New(nil)
	err := s.Open()
	require.NoError(t, err)
	s.Close()
	err = s.WriteClient(persistence.Client{})
	require.Error(t, err)
}

func TestReadClientNoDB(t *testing.T) {
	s := New(nil)
	cid := "test"
	_, err := s.ReadClient(cid)
	require.Error(t, err)
}

func TestReadClientFail(t *testing.T) {
	s := New(nil)
	err := s.Open()
	require.NoError(t, err)
	s.Close()
	cid := "test"
	_, err = s.ReadClient(cid)
	require.Error(t, err)
}

func TestDeleteSubscriptionNoDB(t *testing.T) {
	s := New(nil)
	filter := "test:g/h/k"
	cid := "test"
	err := s.DeleteSubscription(cid, filter)
	require.Error(t, err)
}

func TestDeleteSubscriptionFail(t *testing.T) {
	s := New(nil)
	err := s.Open()
	require.NoError(t, err)
	s.Close()
	filter := "test:g/h/k"
	cid := "test"
	err = s.DeleteSubscription(cid, filter)
	require.Error(t, err)
}

func TestDeleteClientNoDB(t *testing.T) {
	s := New(nil)
	err := s.DeleteClient("a")
	require.Error(t, err)
}

func TestDeleteClientFail(t *testing.T) {
	s := New(nil)
	err := s.Open()
	require.NoError(t, err)
	s.Close()
	err = s.DeleteClient("a")
	require.Error(t, err)
}

func TestDeleteInflightNoDB(t *testing.T) {
	s := New(nil)
	cid := "test"
	fid := "client1_if_2"
	err := s.DeleteInflight(cid, fid)
	require.Error(t, err)
}

func TestDeleteInflightFail(t *testing.T) {
	s := New(nil)
	err := s.Open()
	require.NoError(t, err)
	s.Close()
	cid := "test"
	fid := "client1_if_2"
	err = s.DeleteInflight(cid, fid)
	require.Error(t, err)
}

func TestDeleteRetainedNoDB(t *testing.T) {
	s := New(nil)
	err := s.DeleteRetained("a")
	require.Error(t, err)
}

func TestDeleteRetainedFail(t *testing.T) {
	s := New(nil)
	err := s.Open()
	require.NoError(t, err)
	s.Close()
	err = s.DeleteRetained("a")
	require.Error(t, err)
}
