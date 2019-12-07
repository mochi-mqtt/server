package listeners

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/mochi-co/mqtt/internal/auth"
)

func TestMockEstablisher(t *testing.T) {
	_, w := net.Pipe()
	err := MockEstablisher("t1", w, new(auth.Allow))
	require.NoError(t, err)
	w.Close()
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

	require.Equal(t, false, mocked.IsListening)
	mocked.Listen()
	require.Equal(t, true, mocked.IsListening)
}

func TestMockListenerServe(t *testing.T) {
	mocked := NewMockListener("t1", ":1882")
	require.Equal(t, false, mocked.IsServing)

	o := make(chan bool)
	go func(o chan bool) {
		mocked.Serve(MockEstablisher)
		o <- true
	}(o)

	time.Sleep(time.Millisecond) // easy non-channel wait for start of serving
	require.Equal(t, true, mocked.IsServing)

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
	require.NotNil(t, mocked.Config)
}

func TestMockListenerClose(t *testing.T) {
	mocked := NewMockListener("t1", ":1882")
	var closed bool
	mocked.Close(func(id string) {
		closed = true
	})
	require.Equal(t, true, closed)
}

func TestNew(t *testing.T) {
	l := New()
	require.NotNil(t, l.internal)
}

func BenchmarkNewListeners(b *testing.B) {
	for n := 0; n < b.N; n++ {
		New()
	}
}

func TestAddListener(t *testing.T) {
	l := New()
	l.Add(NewMockListener("t1", ":1882"))
	require.Contains(t, l.internal, "t1")
}

func BenchmarkAddListener(b *testing.B) {
	l := New()
	mocked := NewMockListener("t1", ":1882")
	for n := 0; n < b.N; n++ {
		l.Add(mocked)
	}
}

func TestGetListener(t *testing.T) {
	l := New()
	l.Add(NewMockListener("t1", ":1882"))
	l.Add(NewMockListener("t2", ":1882"))
	require.Contains(t, l.internal, "t1")
	require.Contains(t, l.internal, "t2")

	g, ok := l.Get("t1")
	require.Equal(t, true, ok)
	require.Equal(t, g.ID(), "t1")
}

func BenchmarkGetListener(b *testing.B) {
	l := New()
	l.Add(NewMockListener("t1", ":1882"))
	for n := 0; n < b.N; n++ {
		l.Get("t1")
	}
}

func TestLenListener(t *testing.T) {
	l := New()
	l.Add(NewMockListener("t1", ":1882"))
	l.Add(NewMockListener("t2", ":1882"))
	require.Contains(t, l.internal, "t1")
	require.Contains(t, l.internal, "t2")
	require.Equal(t, 2, l.Len())
}

func BenchmarkLenListener(b *testing.B) {
	l := New()
	l.Add(NewMockListener("t1", ":1882"))
	for n := 0; n < b.N; n++ {
		l.Len()
	}
}

func TestDeleteListener(t *testing.T) {
	l := New()
	l.Add(NewMockListener("t1", ":1882"))
	require.Contains(t, l.internal, "t1")

	l.Delete("t1")
	_, ok := l.Get("t1")
	require.Equal(t, false, ok)
	require.Nil(t, l.internal["t1"])
}

func BenchmarkDeleteListener(b *testing.B) {
	l := New()
	l.Add(NewMockListener("t1", ":1882"))
	for n := 0; n < b.N; n++ {
		l.Delete("t1")
	}
}

func TestServeListener(t *testing.T) {
	l := New()
	l.Add(NewMockListener("t1", ":1882"))
	l.Serve("t1", MockEstablisher)
	time.Sleep(time.Millisecond)
	require.Equal(t, true, l.internal["t1"].(*MockListener).IsServing)

	l.Close("t1", MockCloser)
	require.Equal(t, false, l.internal["t1"].(*MockListener).IsServing)
}

func TestServeListenerFailure(t *testing.T) {
	l := New()
	m := NewMockListener("t1", ":1882")
	m.errListen = true
	l.Add(m)
	err := l.Serve("t1", MockEstablisher)
	require.Error(t, err)
}

func BenchmarkServeListener(b *testing.B) {
	l := New()
	l.Add(NewMockListener("t1", ":1882"))
	for n := 0; n < b.N; n++ {
		l.Serve("t1", MockEstablisher)
	}
}

func TestServeAllListeners(t *testing.T) {
	l := New()
	l.Add(NewMockListener("t1", ":1882"))
	l.Add(NewMockListener("t2", ":1882"))
	l.Add(NewMockListener("t3", ":1882"))
	err := l.ServeAll(MockEstablisher)
	require.NoError(t, err)
	time.Sleep(time.Millisecond)

	require.Equal(t, true, l.internal["t1"].(*MockListener).IsServing)
	require.Equal(t, true, l.internal["t2"].(*MockListener).IsServing)
	require.Equal(t, true, l.internal["t3"].(*MockListener).IsServing)

	l.Close("t1", MockCloser)
	l.Close("t2", MockCloser)
	l.Close("t3", MockCloser)

	require.Equal(t, false, l.internal["t1"].(*MockListener).IsServing)
	require.Equal(t, false, l.internal["t2"].(*MockListener).IsServing)
	require.Equal(t, false, l.internal["t3"].(*MockListener).IsServing)
}

func TestServeAllListenersFailure(t *testing.T) {
	l := New()
	m := NewMockListener("t1", ":1882")
	m.errListen = true
	l.Add(m)
	err := l.ServeAll(MockEstablisher)
	require.Error(t, err)
}

func BenchmarkServeAllListeners(b *testing.B) {
	l := New()
	l.Add(NewMockListener("t1", ":1882"))
	l.Add(NewMockListener("t2", ":1883"))
	l.Add(NewMockListener("t3", ":1884"))
	for n := 0; n < b.N; n++ {
		l.ServeAll(MockEstablisher)
	}
}

func TestCloseListener(t *testing.T) {
	l := New()
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
	l := New()
	mocked := NewMockListener("t1", ":1882")
	l.Add(mocked)
	l.Serve("t1", MockEstablisher)
	for n := 0; n < b.N; n++ {
		l.internal["t1"].(*MockListener).done = make(chan bool)
		l.Close("t1", MockCloser)
	}
}

func TestCloseAllListeners(t *testing.T) {
	l := New()
	l.Add(NewMockListener("t1", ":1882"))
	l.Add(NewMockListener("t2", ":1882"))
	l.Add(NewMockListener("t3", ":1882"))
	l.ServeAll(MockEstablisher)
	time.Sleep(time.Millisecond)
	require.Equal(t, true, l.internal["t1"].(*MockListener).IsServing)
	require.Equal(t, true, l.internal["t2"].(*MockListener).IsServing)
	require.Equal(t, true, l.internal["t3"].(*MockListener).IsServing)

	closed := make(map[string]bool)
	l.CloseAll(func(id string) {
		closed[id] = true
	})
	require.Contains(t, closed, "t1")
	require.Contains(t, closed, "t2")
	require.Contains(t, closed, "t3")
	require.Equal(t, true, closed["t1"])
	require.Equal(t, true, closed["t2"])
	require.Equal(t, true, closed["t3"])
}

func BenchmarkCloseAllListeners(b *testing.B) {
	l := New()
	mocked := NewMockListener("t1", ":1882")
	l.Add(mocked)
	l.Serve("t1", MockEstablisher)
	for n := 0; n < b.N; n++ {
		l.internal["t1"].(*MockListener).done = make(chan bool)
		l.Close("t1", MockCloser)
	}
}
