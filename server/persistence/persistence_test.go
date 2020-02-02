package persistence

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMockStoreOpen(t *testing.T) {
	s := new(MockStore)
	err := s.Open()
	require.NoError(t, err)
	require.Equal(t, true, s.Opened)
}

func TestMockStoreOpenFail(t *testing.T) {
	s := new(MockStore)
	s.FailOpen = true
	err := s.Open()
	require.Error(t, err)
}

func TestMockStoreClose(t *testing.T) {
	s := new(MockStore)
	s.Close()
	require.Equal(t, true, s.Closed)
}

func TestMockStoreWriteSubscription(t *testing.T) {
	s := new(MockStore)
	err := s.WriteSubscription(Subscription{})
	require.NoError(t, err)
}

func TestMockStoreWriteSubscriptionFail(t *testing.T) {
	s := &MockStore{
		Fail: map[string]bool{
			"write_subs": true,
		},
	}
	err := s.WriteSubscription(Subscription{})
	require.Error(t, err)
}

func TestMockStoreWriteClient(t *testing.T) {
	s := new(MockStore)
	err := s.WriteClient(Client{})
	require.NoError(t, err)
}

func TestMockStoreWriteClientFail(t *testing.T) {
	s := &MockStore{
		Fail: map[string]bool{
			"write_clients": true,
		},
	}
	err := s.WriteClient(Client{})
	require.Error(t, err)
}

func TestMockStoreWriteInflight(t *testing.T) {
	s := new(MockStore)
	err := s.WriteInflight(Message{})
	require.NoError(t, err)
}

func TestMockStoreWriteInflightFail(t *testing.T) {
	s := &MockStore{
		Fail: map[string]bool{
			"write_inflight": true,
		},
	}
	err := s.WriteInflight(Message{})
	require.Error(t, err)
}

func TestMockStoreWriteRetained(t *testing.T) {
	s := new(MockStore)
	err := s.WriteRetained(Message{})
	require.NoError(t, err)
}

func TestMockStoreWriteRetainedFail(t *testing.T) {
	s := &MockStore{
		Fail: map[string]bool{
			"write_retained": true,
		},
	}
	err := s.WriteRetained(Message{})
	require.Error(t, err)
}

func TestMockStoreWriteServerInfo(t *testing.T) {
	s := new(MockStore)
	err := s.WriteServerInfo(ServerInfo{})
	require.NoError(t, err)
}

func TestMockStoreWriteServerInfoFail(t *testing.T) {
	s := &MockStore{
		Fail: map[string]bool{
			"write_info": true,
		},
	}
	err := s.WriteServerInfo(ServerInfo{})
	require.Error(t, err)
}

func TestMockStoreDeleteSubscription(t *testing.T) {
	s := new(MockStore)
	err := s.DeleteSubscription("client1:d/e/f")
	require.NoError(t, err)
}

func TestMockStoreDeleteSubscriptionFail(t *testing.T) {
	s := &MockStore{
		Fail: map[string]bool{
			"delete_subs": true,
		},
	}
	err := s.DeleteSubscription("client1:a/b/c")
	require.Error(t, err)
}

func TestMockStoreDeleteClient(t *testing.T) {
	s := new(MockStore)
	err := s.DeleteClient("client1")
	require.NoError(t, err)
}

func TestMockStoreDeleteClientFail(t *testing.T) {
	s := &MockStore{
		Fail: map[string]bool{
			"delete_clients": true,
		},
	}
	err := s.DeleteClient("client1")
	require.Error(t, err)
}

func TestMockStoreDeleteInflight(t *testing.T) {
	s := new(MockStore)
	err := s.DeleteInflight("client1-if-100")
	require.NoError(t, err)
}

func TestMockStoreDeleteInflightFail(t *testing.T) {
	s := &MockStore{
		Fail: map[string]bool{
			"delete_inflight": true,
		},
	}
	err := s.DeleteInflight("client1-if-100")
	require.Error(t, err)
}

func TestMockStoreDeleteRetained(t *testing.T) {
	s := new(MockStore)
	err := s.DeleteRetained("client1-ret-100")
	require.NoError(t, err)
}

func TestMockStoreDeleteRetainedFail(t *testing.T) {
	s := &MockStore{
		Fail: map[string]bool{
			"delete_retained": true,
		},
	}
	err := s.DeleteRetained("client1-ret-100")
	require.Error(t, err)
}

func TestMockStorReadServerInfo(t *testing.T) {
	s := new(MockStore)
	_, err := s.ReadServerInfo()
	require.NoError(t, err)
}

func TestMockStorReadServerInfoFail(t *testing.T) {
	s := &MockStore{
		Fail: map[string]bool{
			"read_info": true,
		},
	}
	_, err := s.ReadServerInfo()
	require.Error(t, err)
}

func TestMockStoreReadSubscriptions(t *testing.T) {
	s := new(MockStore)
	_, err := s.ReadSubscriptions()
	require.NoError(t, err)
}

func TestMockStoreReadSubscriptionsFail(t *testing.T) {
	s := &MockStore{
		Fail: map[string]bool{
			"read_subs": true,
		},
	}
	_, err := s.ReadSubscriptions()
	require.Error(t, err)
}

func TestMockStoreReadClients(t *testing.T) {
	s := new(MockStore)
	_, err := s.ReadClients()
	require.NoError(t, err)
}

func TestMockStoreReadClientsFail(t *testing.T) {
	s := &MockStore{
		Fail: map[string]bool{
			"read_clients": true,
		},
	}
	_, err := s.ReadClients()
	require.Error(t, err)
}

func TestMockStoreReadInflight(t *testing.T) {
	s := new(MockStore)
	_, err := s.ReadInflight()
	require.NoError(t, err)
}

func TestMockStoreReadInflightFail(t *testing.T) {
	s := &MockStore{
		Fail: map[string]bool{
			"read_inflight": true,
		},
	}
	_, err := s.ReadInflight()
	require.Error(t, err)
}

func TestMockStoreReadRetained(t *testing.T) {
	s := new(MockStore)
	_, err := s.ReadRetained()
	require.NoError(t, err)
}

func TestMockStoreReadRetainedFail(t *testing.T) {
	s := &MockStore{
		Fail: map[string]bool{
			"read_retained": true,
		},
	}
	_, err := s.ReadRetained()
	require.Error(t, err)
}
