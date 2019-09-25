package listeners

import (
	//	"net"
	"errors"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewMockListener(t *testing.T) {
	mocked := NewMockListener("t1", ":1882")
	require.Equal(t, "t1", mocked.id)
	require.Equal(t, ":1882", mocked.address)
}

func TestNewMockListenerServe(t *testing.T) {
	mocked := NewMockListener("t1", ":1882")
	o := make(chan error)
	go func(o chan error) {
		o <- mocked.Serve(MockEstablisher)
	}(o)
	time.Sleep(time.Millisecond) // easy non-channel wait for start of serving
	var closed bool
	err := mocked.Close(func() {
		closed = true
	})
	require.NoError(t, err)
	require.Equal(t, true, closed)
	require.NoError(t, <-o)

	// Fail with serve error.
	mocked = NewMockListener("t1", ":1882")
	o = make(chan error)
	go func(o chan error) {
		o <- mocked.Serve(MockEstablisher)
	}(o)

	mocked.accept <- errors.New("fail")
	require.Error(t, <-o)
}

func TestNewMockListenerClose(t *testing.T) {
	mocked := NewMockListener("t1", ":1882")
	err := mocked.Close(MockCloser)
	require.NoError(t, err)
}

func TestNewListeners(t *testing.T) {
	l := NewListeners()
	require.NotNil(t, l.internal)
}

func BenchmarkNewListeners(b *testing.B) {
	for n := 0; n < b.N; n++ {
		NewListeners()
	}
}

func TestAddListener(t *testing.T) {
	l := NewListeners()
	mocked := NewMockListener("t1", ":1882")
	l.Add(mocked)
	require.NotNil(t, l.internal["t1"])
}

func BenchmarkAddListener(b *testing.B) {
	l := NewListeners()
	mocked := NewMockListener("t1", ":1882")
	for n := 0; n < b.N; n++ {
		l.Add(mocked)
	}
}

func TestGetListener(t *testing.T) {
	l := NewListeners()
	mocked := NewMockListener("t1", ":1882")
	l.Add(mocked)

	mocked = NewMockListener("t2", ":1882")
	l.Add(mocked)

	require.NotNil(t, l.internal["t1"])
	require.NotNil(t, l.internal["t2"])

	g, ok := l.Get("t1")
	require.Equal(t, true, ok)
	require.Equal(t, g.ID(), "t1")
}

func BenchmarkGetListener(b *testing.B) {
	l := NewListeners()
	mocked := NewMockListener("t1", ":1882")
	l.Add(mocked)
	for n := 0; n < b.N; n++ {
		l.Get("t1")
	}
}

func TestDeleteListener(t *testing.T) {
	l := NewListeners()
	mocked := NewMockListener("t1", ":1882")
	l.Add(mocked)
	require.NotNil(t, l.internal["t1"])

	l.Delete("t1")
	_, ok := l.Get("t1")
	require.Equal(t, false, ok)
	require.Nil(t, l.internal["t1"])
}

func BenchmarkDeleteListener(b *testing.B) {
	l := NewListeners()
	mocked := NewMockListener("t1", ":1882")
	l.Add(mocked)
	for n := 0; n < b.N; n++ {
		l.Delete("t1")
	}
}

func TestServeListener(t *testing.T) {
	l := NewListeners()
	mocked := NewMockListener("t1", ":1882")
	l.Add(mocked)
	l.Serve("t1", MockEstablisher)
	time.Sleep(time.Millisecond)
	require.Equal(t, true, l.internal["t1"].(*MockListener).serving)
	l.Close("t1", MockCloser)
	require.Equal(t, false, l.internal["t1"].(*MockListener).serving)

	log.Println("B")

	// Serve Error
}

/*
func TestCloseListener(t *testing.T) {
	l := NewListeners()
	tcp, err := NewTCP("t1", ":1882")
	tcp.listen.Close()
	require.NoError(t, err)
	l.Add(tcp)
	require.NotNil(t, l.internal["t1"])

	l.Delete("t1")
	_, ok := l.Get("t1")
	require.Equal(t, false, ok)
	require.Nil(t, l.internal["t1"])
}


func TestServeListener(t *testing.T) {
	l := NewListeners()
	l.Add(NewTCP("t1", ":1882"))

	o := make(chan bool)
	l.Serve("t1", MockEstablisher)
	time.Sleep(time.Millisecond)

	err = l.internal["t1"].(*TCP).Close(MockCloser)
	l.wg.Wait()
	require.NoError(t, err)

}
*/
