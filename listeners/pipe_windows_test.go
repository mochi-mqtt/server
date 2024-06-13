package listeners

import (
	"errors"
	"github.com/stretchr/testify/require"
	"github.com/xlango/npipe"
	"net"
	"testing"
	"time"
)

const testWindowsPipe = `\\.\pipe\mypipename`

var (
	windowsPipeConfig = Config{ID: "t1", Address: testWindowsPipe}
)

func TestNewWindowsPipe(t *testing.T) {
	l := NewWindowsPipe(windowsPipeConfig)
	require.Equal(t, "t1", l.id)
	require.Equal(t, testWindowsPipe, l.address)
}

func TestWindowsPipeID(t *testing.T) {
	l := NewWindowsPipe(windowsPipeConfig)
	require.Equal(t, "t1", l.ID())
}

func TestWindowsPipeAddress(t *testing.T) {
	l := NewWindowsPipe(windowsPipeConfig)
	require.Equal(t, testWindowsPipe, l.Address())
}

func TestWindowsPipeProtocol(t *testing.T) {
	l := NewWindowsPipe(windowsPipeConfig)
	require.Equal(t, "WindowsPipe", l.Protocol())
}

func TestWindowsPipeInit(t *testing.T) {
	l := NewWindowsPipe(windowsPipeConfig)
	err := l.Init(logger)
	l.Close(MockCloser)
	require.NoError(t, err)

	t2Config := windowsPipeConfig
	t2Config.ID = "t2"
	l2 := NewWindowsPipe(t2Config)
	err = l2.Init(logger)
	l2.Close(MockCloser)
	require.NoError(t, err)
}

func TestWindowsPipeServeAndClose(t *testing.T) {
	l := NewWindowsPipe(windowsPipeConfig)
	err := l.Init(logger)
	require.NoError(t, err)

	o := make(chan bool)
	go func(o chan bool) {
		l.Serve(MockEstablisher)
		o <- true
	}(o)

	time.Sleep(time.Millisecond)

	var closed bool
	l.Close(func(id string) {
		closed = true
	})

	require.True(t, closed)
	<-o

	l.Close(MockCloser)      // coverage: close closed
	l.Serve(MockEstablisher) // coverage: serve closed
}

func TestWindowsPipeEstablishThenEnd(t *testing.T) {
	l := NewWindowsPipe(windowsPipeConfig)
	err := l.Init(logger)
	require.NoError(t, err)

	o := make(chan bool)
	established := make(chan bool)
	go func() {
		l.Serve(func(id string, c net.Conn) error {
			established <- true
			return errors.New("ending") // return an error to exit immediately
		})
		o <- true
	}()

	time.Sleep(time.Millisecond)
	_, _ = npipe.Dial(l.listen.Addr().String())
	require.Equal(t, true, <-established)
	l.Close(MockCloser)
	<-o
}
