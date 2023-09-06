// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package listeners

import (
	"crypto/tls"
	"log"
	"os"
	"testing"
	"time"

	"log/slog"

	"github.com/stretchr/testify/require"
)

const testAddr = ":22222"

var (
	logger = slog.New(slog.NewTextHandler(os.Stdout, nil))

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
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cert},
	}
}

func TestNew(t *testing.T) {
	l := New()
	require.NotNil(t, l.internal)
}

func TestAddListener(t *testing.T) {
	l := New()
	l.Add(NewMockListener("t1", testAddr))
	require.Contains(t, l.internal, "t1")
}

func TestGetListener(t *testing.T) {
	l := New()
	l.Add(NewMockListener("t1", testAddr))
	l.Add(NewMockListener("t2", testAddr))
	require.Contains(t, l.internal, "t1")
	require.Contains(t, l.internal, "t2")

	g, ok := l.Get("t1")
	require.True(t, ok)
	require.Equal(t, g.ID(), "t1")
}

func TestLenListener(t *testing.T) {
	l := New()
	l.Add(NewMockListener("t1", testAddr))
	l.Add(NewMockListener("t2", testAddr))
	require.Contains(t, l.internal, "t1")
	require.Contains(t, l.internal, "t2")
	require.Equal(t, 2, l.Len())
}

func TestDeleteListener(t *testing.T) {
	l := New()
	l.Add(NewMockListener("t1", testAddr))
	require.Contains(t, l.internal, "t1")
	l.Delete("t1")
	_, ok := l.Get("t1")
	require.False(t, ok)
	require.Nil(t, l.internal["t1"])
}

func TestServeListener(t *testing.T) {
	l := New()
	l.Add(NewMockListener("t1", testAddr))
	l.Serve("t1", MockEstablisher)
	time.Sleep(time.Millisecond)
	require.True(t, l.internal["t1"].(*MockListener).IsServing())

	l.Close("t1", MockCloser)
	require.False(t, l.internal["t1"].(*MockListener).IsServing())
}

func TestServeAllListeners(t *testing.T) {
	l := New()
	l.Add(NewMockListener("t1", testAddr))
	l.Add(NewMockListener("t2", testAddr))
	l.Add(NewMockListener("t3", testAddr))
	l.ServeAll(MockEstablisher)
	time.Sleep(time.Millisecond)

	require.True(t, l.internal["t1"].(*MockListener).IsServing())
	require.True(t, l.internal["t2"].(*MockListener).IsServing())
	require.True(t, l.internal["t3"].(*MockListener).IsServing())

	l.Close("t1", MockCloser)
	l.Close("t2", MockCloser)
	l.Close("t3", MockCloser)

	require.False(t, l.internal["t1"].(*MockListener).IsServing())
	require.False(t, l.internal["t2"].(*MockListener).IsServing())
	require.False(t, l.internal["t3"].(*MockListener).IsServing())
}

func TestCloseListener(t *testing.T) {
	l := New()
	mocked := NewMockListener("t1", testAddr)
	l.Add(mocked)
	l.Serve("t1", MockEstablisher)
	time.Sleep(time.Millisecond)
	var closed bool
	l.Close("t1", func(id string) {
		closed = true
	})
	require.True(t, closed)
}

func TestCloseAllListeners(t *testing.T) {
	l := New()
	l.Add(NewMockListener("t1", testAddr))
	l.Add(NewMockListener("t2", testAddr))
	l.Add(NewMockListener("t3", testAddr))
	l.ServeAll(MockEstablisher)
	time.Sleep(time.Millisecond)
	require.True(t, l.internal["t1"].(*MockListener).IsServing())
	require.True(t, l.internal["t2"].(*MockListener).IsServing())
	require.True(t, l.internal["t3"].(*MockListener).IsServing())

	closed := make(map[string]bool)
	l.CloseAll(func(id string) {
		closed[id] = true
	})
	require.Contains(t, closed, "t1")
	require.Contains(t, closed, "t2")
	require.Contains(t, closed, "t3")
	require.True(t, closed["t1"])
	require.True(t, closed["t2"])
	require.True(t, closed["t3"])
}
