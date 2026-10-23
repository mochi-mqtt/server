// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"io"
	"log/slog"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func envOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestParseConfigDefaults(t *testing.T) {
	c, err := parseConfig(nil, envOf(map[string]string{"MQTT_NODE_NAME": "mqtt-0", "MQTT_ALLOW_ALL": "true"}))
	require.NoError(t, err)
	require.Equal(t, "mqtt-0", c.NodeName)
	require.Equal(t, ":1883", c.MQTTAddr)
	require.Equal(t, ":8080", c.AdminAddr)
	require.Equal(t, "0.0.0.0", c.BindAddr)
	require.Equal(t, 7946, c.GossipPort)
	require.Equal(t, ":7947", c.ForwardAddr)
	require.Equal(t, ":7948", c.RaftAddr)
	require.Equal(t, "auto", c.Bootstrap)
	require.True(t, c.DrainOnStop)
	require.Equal(t, time.Minute, c.DrainTimeout)
	require.Equal(t, slog.LevelInfo, c.LogLevel)
	require.Equal(t, "json", c.LogFormat)
	require.Empty(t, c.Seeds)
	require.Nil(t, c.GossipKey)
}

func TestParseConfigEnvAndFlags(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(make([]byte, 32))
	c, err := parseConfig(
		[]string{"-gossip", "8000", "-log-level", "debug"},
		envOf(map[string]string{
			"MQTT_NODE_NAME":     "n1",
			"MQTT_AUTH_FILE":     "/etc/mochi/auth.yaml",
			"MQTT_SEEDS":         " mqtt-peers:7946, 10.0.0.1:7946 ,",
			"MQTT_GOSSIP_PORT":   "7000", // overridden by the flag
			"MQTT_GOSSIP_KEY":    key,
			"MQTT_DRAIN_ON_STOP": "false",
			"MQTT_LEASE_TIMEOUT": "3s",
			"MQTT_BOOTSTRAP":     "false",
		}),
	)
	require.NoError(t, err)
	require.Equal(t, []string{"mqtt-peers:7946", "10.0.0.1:7946"}, c.Seeds)
	require.Equal(t, 8000, c.GossipPort)
	require.Len(t, c.GossipKey, 32)
	require.False(t, c.DrainOnStop)
	require.Equal(t, 3*time.Second, c.LeaseTimeout)
	require.Equal(t, slog.LevelDebug, c.LogLevel)
	require.False(t, c.bootstrap())
}

func TestParseConfigGossipKeyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key")
	require.NoError(t, os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(make([]byte, 16))+"\n"), 0o600))

	c, err := parseConfig([]string{"-gossip-key-file", path}, envOf(map[string]string{"MQTT_NODE_NAME": "n1", "MQTT_ALLOW_ALL": "1"}))
	require.NoError(t, err)
	require.Len(t, c.GossipKey, 16)
}

func TestParseConfigErrors(t *testing.T) {
	base := map[string]string{"MQTT_NODE_NAME": "n1", "MQTT_ALLOW_ALL": "true"}
	with := func(k, v string) map[string]string {
		m := map[string]string{}
		for a, b := range base {
			m[a] = b
		}
		m[k] = v
		return m
	}

	tests := []struct {
		name string
		args []string
		env  map[string]string
		err  error
	}{
		{"no auth", nil, with("MQTT_ALLOW_ALL", "false"), errNoAuth},
		{"tls cert without key", nil, with("MQTT_TLS_CERT_FILE", "cert.pem"), errTLSPair},
		{"no listener", []string{"-mqtt", ""}, base, errNoListener}, // an empty env var means unset
		{"bad bootstrap", nil, with("MQTT_BOOTSTRAP", "maybe"), errBadBootstrap},
		{"bad gossip key", nil, with("MQTT_GOSSIP_KEY", base64.StdEncoding.EncodeToString([]byte("short"))), errBadGossipKey},
		{"bad int", nil, with("MQTT_GOSSIP_PORT", "seven"), nil},
		{"bad bool", nil, with("MQTT_ALLOW_ALL", "sure"), nil},
		{"bad duration", nil, with("MQTT_CLAIM_TIMEOUT", "5"), nil},
		{"bad log level", nil, with("MQTT_LOG_LEVEL", "loud"), nil},
		{"bad log format", nil, with("MQTT_LOG_FORMAT", "xml"), nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseConfig(tt.args, envOf(tt.env))
			require.Error(t, err)
			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
			}
		})
	}
}

func TestBootstrap(t *testing.T) {
	tests := []struct {
		name, bootstrap string
		want            bool
	}{
		{"mqtt-0", "auto", true},
		{"mqtt-10", "auto", false},
		{"mqtt-1", "auto", false},
		{"mqtt-1", "true", true},
		{"mqtt-0", "false", false},
	}

	for _, tt := range tests {
		c := &config{NodeName: tt.name, Bootstrap: tt.bootstrap}
		require.Equal(t, tt.want, c.bootstrap(), "%s %s", tt.name, tt.bootstrap)
	}
}

func TestResolveSeeds(t *testing.T) {
	lookup := func(_ context.Context, host string) ([]string, error) {
		switch host {
		case "mqtt-peers":
			return []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}, nil
		case "10.0.0.2":
			return []string{"10.0.0.2"}, nil
		}
		return nil, errors.New("no such host")
	}

	self := map[string]bool{"10.0.0.1": true}
	got := resolveSeeds(context.Background(), []string{"mqtt-peers:7946", "10.0.0.2:7946", "missing:7946", "no-port"}, self, lookup)
	require.Equal(t, []string{"10.0.0.2:7946", "10.0.0.3:7946"}, got)

	got = resolveSeeds(context.Background(), []string{"mqtt-peers:7946"}, map[string]bool{"10.0.0.1": true, "10.0.0.2": true, "10.0.0.3": true}, lookup)
	require.Empty(t, got)
}

func TestClusterOptions(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	// a node which may not bootstrap needs seeds
	_, err := clusterOptions(context.Background(), &config{NodeName: "mqtt-1", Bootstrap: "auto"}, log)
	require.Error(t, err)

	// the first node bootstraps when no seed resolves to another node
	opts, err := clusterOptions(context.Background(), &config{NodeName: "mqtt-0", Bootstrap: "auto", Seeds: []string{"mqtt-peers.invalid:7946"}}, log)
	require.NoError(t, err)
	require.True(t, opts.Bootstrap)
	require.Empty(t, opts.Seeds)

	// any other node waits for a seed, then gives up
	_, err = clusterOptions(context.Background(), &config{NodeName: "mqtt-1", Bootstrap: "auto", Seeds: []string{"mqtt-peers.invalid:7946"}, JoinTimeout: time.Millisecond}, log)
	require.ErrorContains(t, err, "no seed resolved")

	opts, err = clusterOptions(context.Background(), &config{NodeName: "mqtt-1", Bootstrap: "auto", Seeds: []string{"192.0.2.10:7946"}, GossipKey: make([]byte, 16)}, log)
	require.NoError(t, err)
	require.Equal(t, []string{"192.0.2.10:7946"}, opts.Seeds)
	require.Len(t, opts.Memberlist.SecretKey, 16)
}

func TestCertReloader(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := filepath.Join(dir, "tls.crt"), filepath.Join(dir, "tls.key")

	_, err := newCertReloader(certFile, keyFile)
	require.Error(t, err)

	writeCert(t, certFile, keyFile, "first")
	r, err := newCertReloader(certFile, keyFile)
	require.NoError(t, err)
	require.Equal(t, "first", leafCN(t, r))

	writeCert(t, certFile, keyFile, "second")
	future := time.Now().Add(time.Minute)
	require.NoError(t, os.Chtimes(certFile, future, future))
	require.Equal(t, "second", leafCN(t, r))

	// a broken update keeps the last good certificate
	require.NoError(t, os.WriteFile(certFile, []byte("garbage"), 0o600))
	later := future.Add(time.Minute)
	require.NoError(t, os.Chtimes(certFile, later, later))
	require.Equal(t, "second", leafCN(t, r))
}

func leafCN(t *testing.T, r *certReloader) string {
	cert, err := r.get(nil)
	require.NoError(t, err)
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	require.NoError(t, err)
	return leaf.Subject.CommonName
}

func writeCert(t *testing.T, certFile, keyFile, cn string) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)

	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))
	require.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600))
}
