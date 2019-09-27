package listeners

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMockEstablisher(t *testing.T) {
	require.Nil(t, MockEstablisher(new(MockNetConn)))
}

func TestNewMockListener(t *testing.T) {
	mocked := NewMockListener("t1", ":1882")
	require.Equal(t, "t1", mocked.id)
	require.Equal(t, ":1882", mocked.address)
}

func TestNewMockListenerListen(t *testing.T) {
	mocked := NewMockListener("t1", ":1882")
	require.Equal(t, "t1", mocked.id)
	require.Equal(t, ":1882", mocked.address)

	require.Equal(t, false, mocked.Listening())
	mocked.Listen()
	require.Equal(t, true, mocked.Listening())

}

func TestMockListenerServe(t *testing.T) {
	mocked := NewMockListener("t1", ":1882")
	require.Equal(t, false, mocked.Serving())

	o := make(chan bool)
	go func(o chan bool) {
		mocked.Serve(MockEstablisher)
		o <- true
	}(o)

	time.Sleep(time.Millisecond) // easy non-channel wait for start of serving
	require.Equal(t, true, mocked.Serving())

	var closed bool
	mocked.Close(func(id string) {
		closed = true
	})
	require.Equal(t, true, closed)
	<-o

	mocked.Listen()
}

func TestMockListenerSetConfig(t *testing.T) {
	mocked := NewMockListener("t1", ":1883")
	mocked.SetConfig(new(Config))
	require.NotNil(t, mocked.config)
}

func TestMockListenerClose(t *testing.T) {
	mocked := NewMockListener("t1", ":1882")
	var closed bool
	mocked.Close(func(id string) {
		closed = true
	})
	require.Equal(t, true, closed)
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
	l.Add(NewMockListener("t1", ":1882"))
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
	l.Add(NewMockListener("t1", ":1882"))
	l.Add(NewMockListener("t2", ":1882"))
	require.NotNil(t, l.internal["t1"])
	require.NotNil(t, l.internal["t2"])

	g, ok := l.Get("t1")
	require.Equal(t, true, ok)
	require.Equal(t, g.ID(), "t1")
}

func BenchmarkGetListener(b *testing.B) {
	l := NewListeners()
	l.Add(NewMockListener("t1", ":1882"))
	for n := 0; n < b.N; n++ {
		l.Get("t1")
	}
}

func TestLenListener(t *testing.T) {
	l := NewListeners()
	l.Add(NewMockListener("t1", ":1882"))
	l.Add(NewMockListener("t2", ":1882"))
	require.NotNil(t, l.internal["t1"])
	require.NotNil(t, l.internal["t2"])
	require.Equal(t, 2, l.Len())
}

func BenchmarkLenListener(b *testing.B) {
	l := NewListeners()
	l.Add(NewMockListener("t1", ":1882"))
	for n := 0; n < b.N; n++ {
		l.Len()
	}
}

func TestDeleteListener(t *testing.T) {
	l := NewListeners()
	l.Add(NewMockListener("t1", ":1882"))
	require.NotNil(t, l.internal["t1"])

	l.Delete("t1")
	_, ok := l.Get("t1")
	require.Equal(t, false, ok)
	require.Nil(t, l.internal["t1"])
}

func BenchmarkDeleteListener(b *testing.B) {
	l := NewListeners()
	l.Add(NewMockListener("t1", ":1882"))
	for n := 0; n < b.N; n++ {
		l.Delete("t1")
	}
}

func TestServeListener(t *testing.T) {
	l := NewListeners()
	l.Add(NewMockListener("t1", ":1882"))
	l.Serve("t1", MockEstablisher)
	time.Sleep(time.Millisecond)
	require.Equal(t, true, l.internal["t1"].(*MockListener).Serving())

	l.Close("t1", MockCloser)
	require.Equal(t, false, l.internal["t1"].(*MockListener).Serving())
}

func BenchmarkServeListener(b *testing.B) {
	l := NewListeners()
	l.Add(NewMockListener("t1", ":1882"))
	for n := 0; n < b.N; n++ {
		l.Serve("t1", MockEstablisher)
	}
}

func TestServeAllListeners(t *testing.T) {
	l := NewListeners()
	l.Add(NewMockListener("t1", ":1882"))
	l.Add(NewMockListener("t2", ":1882"))
	l.Add(NewMockListener("t3", ":1882"))
	l.ServeAll(MockEstablisher)
	time.Sleep(time.Millisecond)

	require.Equal(t, true, l.internal["t1"].(*MockListener).Serving())
	require.Equal(t, true, l.internal["t2"].(*MockListener).Serving())
	require.Equal(t, true, l.internal["t3"].(*MockListener).Serving())

	l.Close("t1", MockCloser)
	l.Close("t2", MockCloser)
	l.Close("t3", MockCloser)

	require.Equal(t, false, l.internal["t1"].(*MockListener).Serving())
	require.Equal(t, false, l.internal["t2"].(*MockListener).Serving())
	require.Equal(t, false, l.internal["t3"].(*MockListener).Serving())
}

func BenchmarkServeAllListeners(b *testing.B) {
	l := NewListeners()
	l.Add(NewMockListener("t1", ":1882"))
	l.Add(NewMockListener("t2", ":1883"))
	l.Add(NewMockListener("t3", ":1884"))
	for n := 0; n < b.N; n++ {
		l.ServeAll(MockEstablisher)
	}
}

func TestCloseListener(t *testing.T) {
	l := NewListeners()
	mocked := NewMockListener("t1", ":1882")
	l.Add(mocked)
	l.Serve("t1", MockEstablisher)
	time.Sleep(time.Millisecond)
	var closed bool
	l.Close("t1", func(id string) {
		closed = true
	})
	require.Equal(t, true, closed)
}

func BenchmarkCloseListener(b *testing.B) {
	l := NewListeners()
	mocked := NewMockListener("t1", ":1882")
	l.Add(mocked)
	l.Serve("t1", MockEstablisher)
	for n := 0; n < b.N; n++ {
		l.internal["t1"].(*MockListener).done = make(chan bool)
		l.Close("t1", MockCloser)
	}
}

func TestCloseAllListeners(t *testing.T) {
	l := NewListeners()
	l.Add(NewMockListener("t1", ":1882"))
	l.Add(NewMockListener("t2", ":1882"))
	l.Add(NewMockListener("t3", ":1882"))
	l.ServeAll(MockEstablisher)
	time.Sleep(time.Millisecond)
	require.Equal(t, true, l.internal["t1"].(*MockListener).Serving())
	require.Equal(t, true, l.internal["t2"].(*MockListener).Serving())
	require.Equal(t, true, l.internal["t3"].(*MockListener).Serving())

	closed := make(map[string]bool)
	l.CloseAll(func(id string) {
		closed[id] = true
	})
	require.NotNil(t, closed["t1"])
	require.NotNil(t, closed["t2"])
	require.NotNil(t, closed["t3"])
	require.Equal(t, true, closed["t1"])
	require.Equal(t, true, closed["t2"])
	require.Equal(t, true, closed["t3"])
}

func BenchmarkCloseAllListeners(b *testing.B) {
	l := NewListeners()
	mocked := NewMockListener("t1", ":1882")
	l.Add(mocked)
	l.Serve("t1", MockEstablisher)
	for n := 0; n < b.N; n++ {
		l.internal["t1"].(*MockListener).done = make(chan bool)
		l.Close("t1", MockCloser)
	}
}

func TestMockNetConnRead(t *testing.T) {
	nc := new(MockNetConn)
	n, err := nc.Read([]byte{})
	require.Equal(t, 0, n)
	require.NoError(t, err)
}

func TestMockNetConnWrite(t *testing.T) {
	nc := new(MockNetConn)
	n, err := nc.Write([]byte{})
	require.Equal(t, 0, n)
	require.NoError(t, err)
}

func TestMockNetConnClose(t *testing.T) {
	nc := new(MockNetConn)
	err := nc.Close()
	require.NoError(t, err)
}

func TestMockNetConnLocalAddr(t *testing.T) {
	nc := new(MockNetConn)
	require.Equal(t, new(MockNetAddr), nc.LocalAddr())
}

func TestMockNetConnRemoteAddr(t *testing.T) {
	nc := new(MockNetConn)
	require.Equal(t, new(MockNetAddr), nc.RemoteAddr())
}

func TestMockNetConnSetDeadline(t *testing.T) {
	nc := new(MockNetConn)
	require.NoError(t, nc.SetDeadline(time.Now()))
}

func TestMockNetConnSetReadDeadline(t *testing.T) {
	nc := new(MockNetConn)
	require.NoError(t, nc.SetReadDeadline(time.Now()))
}

func TestMockNetConnSetWriteDeadline(t *testing.T) {
	nc := new(MockNetConn)
	require.NoError(t, nc.SetWriteDeadline(time.Now()))
}

func TestMockNetAddrNetwork(t *testing.T) {
	na := new(MockNetAddr)
	require.Equal(t, "tcp", na.Network())
}

func TestMockNetAddrString(t *testing.T) {
	na := new(MockNetAddr)
	require.Equal(t, "127.0.0.1", na.String())
}
