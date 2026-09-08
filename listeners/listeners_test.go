// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package listeners

import (
	"bytes"
	"crypto/tls"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"log/slog"

	"github.com/stretchr/testify/require"
)

const testAddr = ":22222"

var (
	basicConfig = Config{ID: "t1", Address: testAddr}
	tlsConfig   = Config{ID: "t1", Address: testAddr, TLSConfig: tlsConfigBasic}

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
	tlsConfig.TLSConfig = tlsConfigBasic
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

// syncBuffer collects log output from the serving goroutine, which writes it
// while the test reads it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// TestListenerLogLineNamesTheListenerOnce serves a listener the way the
// server does — through a logger already carrying the listener id, which is
// what AddListener builds — and requires that the line says it once.
//
// The id is worth having on every line and it is not in question here. What
// is, is where it comes from: a call site that adds it again produces
// `listener=t1 ... listener=t1`, which reads past as ordinary and files
// twice in a log pipeline keyed on the field.
func TestListenerLogLineNamesTheListenerOnce(t *testing.T) {
	var out syncBuffer
	l := NewTCP(basicConfig)
	require.NoError(t, l.Init(slog.New(slog.NewTextHandler(&out, nil)).With("listener", basicConfig.ID)))

	o := make(chan bool)
	established := make(chan bool)
	go func() {
		l.Serve(func(id string, c net.Conn) error {
			established <- true
			return errors.New("ending")
		})
		o <- true
	}()

	time.Sleep(time.Millisecond)
	_, _ = net.Dial("tcp", l.listen.Addr().String())
	require.Equal(t, true, <-established)

	// Wait for the line itself, not for Serve to return: the Warn is written
	// from the per-connection goroutine, which nothing here joins. Waiting on
	// Serve instead reads an empty buffer often enough to matter, and an
	// assertion about a line that has not been written yet passes or fails on
	// the scheduler rather than on the code.
	var line string
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
		if line = out.String(); strings.Contains(line, "connection ended with an error") {
			break
		}
		time.Sleep(time.Millisecond)
	}
	l.Close(MockCloser)
	<-o

	require.Contains(t, line, "connection ended with an error",
		"the listener logged nothing for a connection that ended badly")
	require.Contains(t, line, "listener="+basicConfig.ID)
	require.Equal(t, 1, strings.Count(line, "listener="),
		"the listener id is repeated in the line: %s", line)
}

// TestNoListenerLogCallPassesTheListenerKey asks the question of the whole
// package rather than of the lines somebody has looked at. Every listener's
// logger is built by AddListener with `listener` already on it, so passing
// that key at a call site can only ever duplicate it — and the line above
// was written six times before anyone read one of them on a terminal.
//
// The syntax tree rather than the text, so that a call gofmt has wrapped
// across lines is not invisible, and a count of what was read, so that a
// walk finding no files cannot pass as a walk finding nothing wrong.
func TestNoListenerLogCallPassesTheListenerKey(t *testing.T) {
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)

	scanned := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		scanned++

		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, name, nil, 0)
		require.NoError(t, err)

		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			switch sel.Sel.Name {
			case "Debug", "Info", "Warn", "Error":
			default:
				return true
			}
			for _, arg := range call.Args {
				lit, ok := arg.(*ast.BasicLit)
				if ok && lit.Kind == token.STRING && lit.Value == `"listener"` {
					t.Errorf("%s:%d passes the key \"listener\" to a log call, and the "+
						"logger AddListener built already carries it", name,
						fset.Position(lit.Pos()).Line)
				}
			}
			return true
		})
	}

	require.Greater(t, scanned, 4, "scanned %d files, which is too few to be the package", scanned)
}
