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
