// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

// Package main demonstrates how to use MQTT over QUIC with mochi-mqtt.
//
// QUIC provides improved performance over TCP, especially in high-latency
// or lossy network conditions, with features like:
// - 0-RTT connection establishment
// - Multiplexed streams without head-of-line blocking
// - Connection migration
// - Built-in TLS 1.3 encryption
//
// Client URL formats:
// - NanoSDK (C): mqtt-quic://localhost:14567
// - Paho (Go):   quic://localhost:14567
//
// The server listens on UDP port and accepts both formats.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"log"
	"math/big"
	"os"
	"os/signal"
	"syscall"
	"time"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/quic-go/quic-go"
)

const (
	// Default ports
	quicPort = ":14567" // MQTT over QUIC port
	tcpPort  = ":1883"  // Standard MQTT TCP port
)

// generateTLSConfig creates a self-signed TLS configuration for testing.
// In production, use proper certificates from a trusted CA.
func generateTLSConfig() *tls.Config {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		log.Fatal(err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, priv.Public(), priv)
	if err != nil {
		log.Fatal(err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{certDER},
			PrivateKey:  priv,
		}},
		NextProtos: []string{"mqtt"}, // ALPN protocol for MQTT over QUIC
	}
}

func main() {
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		done <- true
	}()

	// Create TLS configuration (required for QUIC)
	tlsConfig := generateTLSConfig()

	// Optional: Configure QUIC-specific settings optimized for MQTT
	// If nil is passed to NewQUIC, default optimized settings will be used
	quicConfig := &quic.Config{
		MaxIdleTimeout:                 listeners.DefaultQUICMaxIdleTimeout,
		KeepAlivePeriod:                listeners.DefaultQUICKeepAlivePeriod,
		MaxIncomingStreams:             listeners.DefaultQUICMaxIncomingStreams,
		MaxIncomingUniStreams:          listeners.DefaultQUICMaxIncomingUniStreams,
		InitialStreamReceiveWindow:     listeners.DefaultQUICInitialStreamReceiveWindow,
		MaxStreamReceiveWindow:         listeners.DefaultQUICMaxStreamReceiveWindow,
		InitialConnectionReceiveWindow: listeners.DefaultQUICInitialConnectionReceiveWindow,
		MaxConnectionReceiveWindow:     listeners.DefaultQUICMaxConnectionReceiveWindow,
		Allow0RTT:                      false, // Disable 0-RTT for security by default
	}

	// Create MQTT server
	server := mqtt.New(nil)
	_ = server.AddHook(new(auth.AllowHook), nil)

	// Add QUIC listener (mqtt-quic:// protocol)
	// Compatible with NanoSDK clients using mqtt-quic://host:port format
	quicListener := listeners.NewQUIC(listeners.Config{
		ID:        "quic1",
		Address:   quicPort,
		TLSConfig: tlsConfig,
	}, quicConfig)

	err := server.AddListener(quicListener)
	if err != nil {
		log.Fatal(err)
	}

	// Optionally, also add TCP listener for backward compatibility
	tcpListener := listeners.NewTCP(listeners.Config{
		ID:        "tcp1",
		Address:   tcpPort,
		TLSConfig: tlsConfig,
	})
	err = server.AddListener(tcpListener)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Starting MQTT server...")
	log.Println("  QUIC listener (mqtt-quic) on", quicPort)
	log.Println("    - NanoSDK client: mqtt-quic://localhost" + quicPort)
	log.Println("    - Paho Go client: quic://localhost" + quicPort)
	log.Println("  TCP listener on", tcpPort)

	go func() {
		err := server.Serve()
		if err != nil {
			log.Fatal(err)
		}
	}()

	<-done
	server.Log.Warn("caught signal, stopping...")
	_ = server.Close()
	server.Log.Info("server stopped")
}
