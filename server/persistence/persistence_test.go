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

func TestMockStoreWriteClient(t *testing.T) {
	s := new(MockStore)
	err := s.WriteClient(Client{})
	require.NoError(t, err)
}

func TestMockStoreWriteInflight(t *testing.T) {
	s := new(MockStore)
	err := s.WriteInflight(Message{})
	require.NoError(t, err)
}

func TestMockStoreWriteRetained(t *testing.T) {
	s := new(MockStore)
	err := s.WriteRetained(Message{})
	require.NoError(t, err)
}

func TestMockStoreWriteServerInfo(t *testing.T) {
	s := new(MockStore)
	err := s.WriteServerInfo(ServerInfo{})
	require.NoError(t, err)
}

func TestMockStorReadServerInfo(t *testing.T) {
	s := new(MockStore)
	_, err := s.ReadServerInfo()
	require.NoError(t, err)
}

func TestMockStoreReadSubscriptions(t *testing.T) {
	s := new(MockStore)
	_, err := s.ReadSubscriptions()
	require.NoError(t, err)
}

func TestMockStoreReadClients(t *testing.T) {
	s := new(MockStore)
	_, err := s.ReadClients()
	require.NoError(t, err)
}

func TestMockStoreReadInflight(t *testing.T) {
	s := new(MockStore)
	_, err := s.ReadInflight()
	require.NoError(t, err)
}

func TestMockStoreReadRetained(t *testing.T) {
	s := new(MockStore)
	_, err := s.ReadRetained()
	require.NoError(t, err)
}
