package listeners

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mochi-co/mqtt/server/listeners/auth"
	"github.com/mochi-co/mqtt/server/system"
	"github.com/stretchr/testify/require"
)

func TestNewHTTPStats(t *testing.T) {
	l := NewHTTPStats("t1", testPort)
	require.Equal(t, "t1", l.id)
	require.Equal(t, testPort, l.address)
}

func BenchmarkNewHTTPStats(b *testing.B) {
	for n := 0; n < b.N; n++ {
		NewHTTPStats("t1", testPort)
	}
}

func TestHTTPStatsSetConfig(t *testing.T) {
	l := NewHTTPStats("t1", testPort)

	l.SetConfig(&Config{
		Auth: new(auth.Allow),
	})
	require.NotNil(t, l.config)
	require.NotNil(t, l.config.Auth)
	require.Equal(t, new(auth.Allow), l.config.Auth)

	// Switch to disallow on bad config set.
	l.SetConfig(new(Config))
	require.NotNil(t, l.config)
	require.NotNil(t, l.config.Auth)
	require.Equal(t, new(auth.Disallow), l.config.Auth)
}

func BenchmarkHTTPStatsSetConfig(b *testing.B) {
	l := NewHTTPStats("t1", testPort)
	for n := 0; n < b.N; n++ {
		l.SetConfig(new(Config))
	}
}

func TestHTTPStatsID(t *testing.T) {
	l := NewHTTPStats("t1", testPort)
	require.Equal(t, "t1", l.ID())
}

func BenchmarkHTTPStatsID(b *testing.B) {
	l := NewHTTPStats("t1", testPort)
	for n := 0; n < b.N; n++ {
		l.ID()
	}
}

func TestHTTPStatsListen(t *testing.T) {
	l := NewHTTPStats("t1", testPort)
	err := l.Listen(new(system.Info))
	require.NoError(t, err)

	require.NotNil(t, l.system)
	require.NotNil(t, l.listen)
	require.Equal(t, testPort, l.listen.Addr)

	l.listen.Close()
}

func TestHTTPStatsListenTLS(t *testing.T) {
	l := NewHTTPStats("t1", testPort)
	l.SetConfig(&Config{
		Auth: new(auth.Allow),
		TLS: &TLS{
			Certificate: testCertificate,
			PrivateKey:  testPrivateKey,
		},
	})
	err := l.Listen(new(system.Info))
	require.NoError(t, err)
	require.NotNil(t, l.listen.TLSConfig)
	l.listen.Close()
}

func TestHTTPStatsListenTLSInvalid(t *testing.T) {
	l := NewHTTPStats("t1", testPort)
	l.SetConfig(&Config{
		Auth: new(auth.Allow),
		TLS: &TLS{
			Certificate: []byte("abcde"),
			PrivateKey:  testPrivateKey,
		},
	})
	err := l.Listen(new(system.Info))
	require.Error(t, err)
}

func TestHTTPStatsServeAndClose(t *testing.T) {
	l := NewHTTPStats("t1", testPort)
	err := l.Listen(&system.Info{
		Version: "test",
	})
	require.NoError(t, err)

	o := make(chan bool)
	go func(o chan bool) {
		l.Serve(MockEstablisher)
		o <- true
	}(o)
	time.Sleep(time.Millisecond)

	resp, err := http.Get("http://localhost" + testPort)
	require.NoError(t, err)
	require.NotNil(t, resp)

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)

	v := new(system.Info)
	err = json.Unmarshal(body, v)
	require.NoError(t, err)
	require.Equal(t, "test", v.Version)

	var closed bool
	l.Close(func(id string) {
		closed = true
	})
	require.Equal(t, true, closed)

	_, err = http.Get("http://localhost" + testPort)
	require.Error(t, err)

	<-o
}

func TestHTTPStatsServeTLSAndClose(t *testing.T) {
	l := NewHTTPStats("t1", testPort)
	l.SetConfig(&Config{
		Auth: new(auth.Allow),
		TLS: &TLS{
			Certificate: testCertificate,
			PrivateKey:  testPrivateKey,
		},
	})
	err := l.Listen(&system.Info{
		Version: "test",
	})
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
	require.Equal(t, true, closed)
}

func TestHTTPStatsJSONHandler(t *testing.T) {
	l := NewHTTPStats("t1", testPort)
	err := l.Listen(&system.Info{
		Version: "test",
	})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	l.jsonHandler(w, nil)
	resp := w.Result()
	body, _ := ioutil.ReadAll(resp.Body)

	v := new(system.Info)
	err = json.Unmarshal(body, v)
	require.NoError(t, err)
	require.Equal(t, "test", v.Version)
}
