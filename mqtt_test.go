package mqtt

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mochi-co/mqtt/listeners"
)

func TestNew(t *testing.T) {
	s := New()
	require.NotNil(t, s)
	require.NotNil(t, s.listeners)
}

func BenchmarkNew(b *testing.B) {
	for n := 0; n < b.N; n++ {
		New()
	}
}

func TestAddTCPListener(t *testing.T) {
	s := New()
	require.NotNil(t, s)

	err := s.AddListener(listeners.NewTCP("t1", ":1882"))
	require.NoError(t, err)
	require.NotNil(t, s.listeners["t1"])

	// Add listener on existing id
	err = s.AddListener(listeners.NewTCP("t1", ":1882"))
	require.Error(t, err)
	require.Equal(t, ErrListenerIDExists, err)
	s.listeners["t1"].Close(listeners.MockCloser)
}

func BenchmarkAddTCPListener(b *testing.B) {
	s := New()

	l := listeners.NewTCP("t1", ":1882")
	for n := 0; n < b.N; n++ {
		err := s.AddListener(l)
		if err != nil {
			panic(err)
		}
		s.listeners[l.ID()].Close(listeners.MockCloser)
		delete(s.listeners, l.ID())
	}
}

func TestListenAndServe(t *testing.T) {
	s := New()
	require.NotNil(t, s)

	err := s.AddListener(listeners.NewTCP("t1", ":1882"))
	require.NoError(t, err)

	err = s.ListenAndServe()
	require.NoError(t, err)

}
