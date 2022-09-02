package listeners

import (
	"crypto/tls"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var (
	testCertificate = []byte(`-----BEGIN CERTIFICATE-----
MIIB/zCCAWgCCQDm3jV+lSF1AzANBgkqhkiG9w0BAQsFADBEMQswCQYDVQQGEwJB
VTETMBEGA1UECAwKU29tZS1TdGF0ZTERMA8GA1UECgwITW9jaGkgQ28xDTALBgNV
BAsMBE1RVFQwHhcNMjAwMTA0MjAzMzQyWhcNMjEwMTAzMjAzMzQyWjBEMQswCQYD
VQQGEwJBVTETMBEGA1UECAwKU29tZS1TdGF0ZTERMA8GA1UECgwITW9jaGkgQ28x
DTALBgNVBAsMBE1RVFQwgZ8wDQYJKoZIhvcNAQEBBQADgY0AMIGJAoGBAKz2bUz3
AOymssVLuvSOEbQ/sF8C/Ill8nRTd7sX9WBIxHJZf+gVn8lQ4BTQ0NchLDRIlpbi
OuZgktpd6ba8sIfVM4jbVprctky5tGsyHRFwL/GAycCtKwvuXkvcwSwLvB8b29EI
MLQ/3vNnYuC3eZ4qqxlODJgRsfQ7mUNB8zkLAgMBAAEwDQYJKoZIhvcNAQELBQAD
gYEAiMoKnQaD0F/J332arGvcmtbHmF2XZp/rGy3dooPug8+OPUSAJY9vTfxJwOsQ
qN1EcI+kIgrGxzA3VRfVYV8gr7IX+fUYfVCaPGcDCfPvo/Ihu757afJRVvpafWgy
zSpDZYu6C62h3KSzMJxffDjy7/2t8oYbTzkLSamsHJJjLZw=
-----END CERTIFICATE-----`)

	testPrivateKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIICXAIBAAKBgQCs9m1M9wDsprLFS7r0jhG0P7BfAvyJZfJ0U3e7F/VgSMRyWX/o
FZ/JUOAU0NDXISw0SJaW4jrmYJLaXem2vLCH1TOI21aa3LZMubRrMh0RcC/xgMnA
rSsL7l5L3MEsC7wfG9vRCDC0P97zZ2Lgt3meKqsZTgyYEbH0O5lDQfM5CwIDAQAB
AoGBAKlmVVirFqmw/qhDaqD4wBg0xI3Zw/Lh+Vu7ICoK5hVeT6DbTW3GOBAY+M8K
UXBSGhQ+/9ZZTmyyK0JZ9nw2RAG3lONU6wS41pZhB7F4siatZfP/JJfU6p+ohe8m
n22hTw4brY/8E/tjuki9T5e2GeiUPBhjbdECkkVXMYBPKDZhAkEA5h/b/HBcsIZZ
mL2d3dyWkXR/IxngQa4NH3124M8MfBqCYXPLgD7RDI+3oT/uVe+N0vu6+7CSMVx6
INM67CuE0QJBAMBpKW54cfMsMya3CM1BfdPEBzDT5kTMqxJ7ez164PHv9CJCnL0Z
AuWgM/p2WNbAF1yHNxw1eEfNbUWwVX2yhxsCQEtnMQvcPWLSAtWbe/jQaL2scGQt
/F9JCp/A2oz7Cto3TXVlHc8dxh3ZkY/ShOO/pLb3KOODjcOCy7mpvOrZr6ECQH32
WoFPqImhrfryaHi3H0C7XFnC30S7GGOJIy0kfI7mn9St9x50eUkKj/yv7YjpSGHy
w0lcV9npyleNEOqxLXECQBL3VRGCfZfhfFpL8z+5+HPKXw6FxWr+p5h8o3CZ6Yi3
OJVN3Mfo6mbz34wswrEdMXn25MzAwbhFQvCVpPZrFwc=
-----END RSA PRIVATE KEY-----`)

	tlsConfigBasic *tls.Config
)

func init() {
	cert, err := tls.X509KeyPair(testCertificate, testPrivateKey)
	if err != nil {
		log.Fatal(err)
	}

	// Basic TLS Config
	tlsConfigBasic = &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
}

func TestNew(t *testing.T) {
	l := New(nil)
	require.NotNil(t, l.internal)
}

func BenchmarkNewListeners(b *testing.B) {
	for n := 0; n < b.N; n++ {
		New(nil)
	}
}

func TestAddListener(t *testing.T) {
	l := New(nil)
	l.Add(NewMockListener("t1", ":1882"))
	require.Contains(t, l.internal, "t1")
}

func BenchmarkAddListener(b *testing.B) {
	l := New(nil)
	mocked := NewMockListener("t1", ":1882")
	for n := 0; n < b.N; n++ {
		l.Add(mocked)
	}
}

func TestGetListener(t *testing.T) {
	l := New(nil)
	l.Add(NewMockListener("t1", ":1882"))
	l.Add(NewMockListener("t2", ":1882"))
	require.Contains(t, l.internal, "t1")
	require.Contains(t, l.internal, "t2")

	g, ok := l.Get("t1")
	require.Equal(t, true, ok)
	require.Equal(t, g.ID(), "t1")
}

func BenchmarkGetListener(b *testing.B) {
	l := New(nil)
	l.Add(NewMockListener("t1", ":1882"))
	for n := 0; n < b.N; n++ {
		l.Get("t1")
	}
}

func TestLenListener(t *testing.T) {
	l := New(nil)
	l.Add(NewMockListener("t1", ":1882"))
	l.Add(NewMockListener("t2", ":1882"))
	require.Contains(t, l.internal, "t1")
	require.Contains(t, l.internal, "t2")
	require.Equal(t, 2, l.Len())
}

func BenchmarkLenListener(b *testing.B) {
	l := New(nil)
	l.Add(NewMockListener("t1", ":1882"))
	for n := 0; n < b.N; n++ {
		l.Len()
	}
}

func TestDeleteListener(t *testing.T) {
	l := New(nil)
	l.Add(NewMockListener("t1", ":1882"))
	require.Contains(t, l.internal, "t1")

	l.Delete("t1")
	_, ok := l.Get("t1")
	require.Equal(t, false, ok)
	require.Nil(t, l.internal["t1"])
}

func BenchmarkDeleteListener(b *testing.B) {
	l := New(nil)
	l.Add(NewMockListener("t1", ":1882"))
	for n := 0; n < b.N; n++ {
		l.Delete("t1")
	}
}

func TestServeListener(t *testing.T) {
	l := New(nil)
	l.Add(NewMockListener("t1", ":1882"))
	l.Serve("t1", MockEstablisher)
	time.Sleep(time.Millisecond)
	require.Equal(t, true, l.internal["t1"].(*MockListener).IsServing())

	l.Close("t1", MockCloser)
	require.Equal(t, false, l.internal["t1"].(*MockListener).IsServing())
}

func BenchmarkServeListener(b *testing.B) {
	l := New(nil)
	l.Add(NewMockListener("t1", ":1882"))
	for n := 0; n < b.N; n++ {
		l.Serve("t1", MockEstablisher)
	}
}

func TestServeAllListeners(t *testing.T) {
	l := New(nil)
	l.Add(NewMockListener("t1", ":1882"))
	l.Add(NewMockListener("t2", ":1882"))
	l.Add(NewMockListener("t3", ":1882"))
	l.ServeAll(MockEstablisher)
	time.Sleep(time.Millisecond)

	require.Equal(t, true, l.internal["t1"].(*MockListener).IsServing())
	require.Equal(t, true, l.internal["t2"].(*MockListener).IsServing())
	require.Equal(t, true, l.internal["t3"].(*MockListener).IsServing())

	l.Close("t1", MockCloser)
	l.Close("t2", MockCloser)
	l.Close("t3", MockCloser)

	require.Equal(t, false, l.internal["t1"].(*MockListener).IsServing())
	require.Equal(t, false, l.internal["t2"].(*MockListener).IsServing())
	require.Equal(t, false, l.internal["t3"].(*MockListener).IsServing())
}

func BenchmarkServeAllListeners(b *testing.B) {
	l := New(nil)
	l.Add(NewMockListener("t1", ":1882"))
	l.Add(NewMockListener("t2", ":1883"))
	l.Add(NewMockListener("t3", ":1884"))
	for n := 0; n < b.N; n++ {
		l.ServeAll(MockEstablisher)
	}
}

func TestCloseListener(t *testing.T) {
	l := New(nil)
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
	l := New(nil)
	mocked := NewMockListener("t1", ":1882")
	l.Add(mocked)
	l.Serve("t1", MockEstablisher)
	for n := 0; n < b.N; n++ {
		l.internal["t1"].(*MockListener).done = make(chan bool)
		l.Close("t1", MockCloser)
	}
}

func TestCloseAllListeners(t *testing.T) {
	l := New(nil)
	l.Add(NewMockListener("t1", ":1882"))
	l.Add(NewMockListener("t2", ":1882"))
	l.Add(NewMockListener("t3", ":1882"))
	l.ServeAll(MockEstablisher)
	time.Sleep(time.Millisecond)
	require.Equal(t, true, l.internal["t1"].(*MockListener).IsServing())
	require.Equal(t, true, l.internal["t2"].(*MockListener).IsServing())
	require.Equal(t, true, l.internal["t3"].(*MockListener).IsServing())

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
	l := New(nil)
	mocked := NewMockListener("t1", ":1882")
	l.Add(mocked)
	l.Serve("t1", MockEstablisher)
	for n := 0; n < b.N; n++ {
		l.internal["t1"].(*MockListener).done = make(chan bool)
		l.Close("t1", MockCloser)
	}
}
