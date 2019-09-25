package listeners

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewTCP(t *testing.T) {
	l, err := NewTCP("t1", ":1883")
	require.NoError(t, err)
	require.Equal(t, "t1", l.id)
	require.Equal(t, ":1883", l.address)
	require.NotNil(t, l.end)
	require.NotNil(t, l.done)

	// Existing bind address.
	_, err = NewTCP("t1", ":1883")
	require.Error(t, err)
	l.listen.Close()
}

func BenchmarkNewTCP(b *testing.B) {
	for n := 0; n < b.N; n++ {
		NewTCP("t1", ":1883")
	}
}

func TestTCPID(t *testing.T) {
	l, err := NewTCP("t1", ":1883")
	l.listen.Close()
	require.NoError(t, err)
	require.Equal(t, "t1", l.ID())
}

func BenchmarkTCPID(b *testing.B) {
	l, err := NewTCP("t1", ":1883")
	if err != nil {
		panic(err)
	}
	for n := 0; n < b.N; n++ {
		l.ID()
	}
}

func TestTCPServe(t *testing.T) {

	// Close Connection.
	l, err := NewTCP("t1", ":1883")
	require.NoError(t, err)
	o := make(chan error)
	go func(o chan error) {
		o <- l.Serve(MockEstablisher)
	}(o)
	time.Sleep(time.Millisecond) // easy non-channel wait for start of serving
	var closed bool
	err = l.Close(func() {
		closed = true
	})
	require.NoError(t, err)
	require.Equal(t, true, closed)
	require.NoError(t, <-o)

	// Close broken/closed listener.
	l, err = NewTCP("t1", ":1883")
	require.NoError(t, err)
	o = make(chan error)
	go func(o chan error) {
		o <- l.Serve(MockEstablisher)
	}(o)

	time.Sleep(time.Millisecond)
	l.listen.Close()
	err = l.Close(MockCloser)
	require.Error(t, err)
	require.NoError(t, <-o)

	// Accept/Establish.
	l, err = NewTCP("t1", ":1883")
	require.NoError(t, err)
	o = make(chan error)
	ok := make(chan bool)
	go func(o chan error, ok chan bool) {
		o <- l.Serve(func(c net.Conn) {
			ok <- true
		})
	}(o, ok)

	time.Sleep(time.Millisecond)
	net.Dial(l.protocol, l.listen.Addr().String())
	require.Equal(t, true, <-ok)
	l.Close(MockCloser)
	require.NoError(t, <-o)

}
