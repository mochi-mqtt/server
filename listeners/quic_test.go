// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package listeners

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"math/big"
	"net"
	"testing"
	"time"

	"log/slog"

	"github.com/stretchr/testify/require"
)

func generateTestTLSConfig() *tls.Config {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, priv.Public(), priv)
	if err != nil {
		panic(err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{certDER},
			PrivateKey:  priv,
		}},
		NextProtos: []string{"mqtt"},
	}
}

func TestNewQUIC(t *testing.T) {
	config := Config{
		ID:        "quic1",
		Address:   ":1883",
		TLSConfig: generateTestTLSConfig(),
	}
	l := NewQUIC(config, nil)
	require.Equal(t, "quic1", l.ID())
	require.Equal(t, ":1883", l.Address())
	require.Equal(t, "mqtt-quic", l.Protocol())
}

func TestQUICID(t *testing.T) {
	config := Config{
		ID:        "quic1",
		Address:   ":1883",
		TLSConfig: generateTestTLSConfig(),
	}
	l := NewQUIC(config, nil)
	require.Equal(t, "quic1", l.ID())
}

func TestQUICAddress(t *testing.T) {
	config := Config{
		ID:        "quic1",
		Address:   ":1883",
		TLSConfig: generateTestTLSConfig(),
	}
	l := NewQUIC(config, nil)
	require.Equal(t, ":1883", l.Address())
}

func TestQUICProtocol(t *testing.T) {
	config := Config{
		ID:        "quic1",
		Address:   ":1883",
		TLSConfig: generateTestTLSConfig(),
	}
	l := NewQUIC(config, nil)
	require.Equal(t, "mqtt-quic", l.Protocol())
}

func TestQUICTypeConstant(t *testing.T) {
	require.Equal(t, "mqtt-quic", TypeQUIC)
}

func TestQUICInitWithoutTLS(t *testing.T) {
	config := Config{
		ID:      "quic1",
		Address: ":0",
	}
	l := NewQUIC(config, nil)
	err := l.Init(slog.Default())
	require.Error(t, err)
	require.Equal(t, ErrTLSRequired, err)
}

func TestQUICInit(t *testing.T) {
	config := Config{
		ID:        "quic1",
		Address:   ":0",
		TLSConfig: generateTestTLSConfig(),
	}
	l := NewQUIC(config, nil)
	err := l.Init(slog.Default())
	require.NoError(t, err)
	require.NotNil(t, l.listener)
	l.Close(MockCloser)
}

func TestQUICClose(t *testing.T) {
	config := Config{
		ID:        "quic1",
		Address:   ":0",
		TLSConfig: generateTestTLSConfig(),
	}
	l := NewQUIC(config, nil)
	err := l.Init(slog.Default())
	require.NoError(t, err)

	closerCalled := false
	l.Close(func(id string) {
		closerCalled = true
	})
	require.True(t, closerCalled)
}

func TestQUICStreamConn(t *testing.T) {
	// Test that QUICStreamConn implements net.Conn
	var _ net.Conn = (*QUICStreamConn)(nil)
}

func TestQUICDefaultConfig(t *testing.T) {
	// Verify default configuration constants are reasonable values
	require.Equal(t, 300*time.Second, DefaultQUICMaxIdleTimeout)
	require.Equal(t, 30*time.Second, DefaultQUICKeepAlivePeriod)
	require.True(t, DefaultQUICMaxIncomingStreams >= 100)
	require.True(t, DefaultQUICMaxIncomingUniStreams >= 100)
	require.True(t, DefaultQUICInitialStreamReceiveWindow >= 64*1024)
	require.True(t, DefaultQUICMaxStreamReceiveWindow >= 1024*1024)
	require.True(t, DefaultQUICInitialConnectionReceiveWindow >= 64*1024)
	require.True(t, DefaultQUICMaxConnectionReceiveWindow >= 1024*1024)
}
