package listeners

import (
	"errors"
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
	o := make(chan bool)
	go func(o chan bool) {
		l.Serve(MockEstablisher)
		o <- true
	}(o)
	time.Sleep(time.Millisecond) // easy non-channel wait for start of serving
	var closed bool
	l.Close(func(id string) {
		closed = true
	})
	require.Equal(t, true, closed)
	<-o

	// Close broken/closed listener.
	l, err = NewTCP("t1", ":1883")
	require.NoError(t, err)
	o = make(chan bool)
	go func(o chan bool) {
		l.Serve(MockEstablisher)
		o <- true
	}(o)

	time.Sleep(time.Millisecond)
	l.listen.Close()
	l.Close(MockCloser)
	<-o

	// Accept/Establish.
	l, err = NewTCP("t1", ":1883")
	require.NoError(t, err)
	o = make(chan bool)
	ok := make(chan bool)
	go func(o chan bool, ok chan bool) {
		l.Serve(func(c net.Conn) error {
			ok <- true
			return errors.New("testing")
		})
		o <- true
	}(o, ok)

	time.Sleep(time.Millisecond)
	net.Dial(l.protocol, l.listen.Addr().String())
	require.Equal(t, true, <-ok)
	l.Close(MockCloser)
	<-o

}
