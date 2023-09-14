// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package main

import (
	"crypto/tls"
	"log"
	"os"
	"os/signal"
	"syscall"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
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
)

func main() {
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		done <- true
	}()

	cert, err := tls.X509KeyPair(testCertificate, testPrivateKey)
	if err != nil {
		log.Fatal(err)
	}

	// Basic TLS Config
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	// Optionally, if you want clients to authenticate only with certs issued by your CA,
	// you might want to use something like this:
	// certPool := x509.NewCertPool()
	// _ = certPool.AppendCertsFromPEM(caCertPem)
	// tlsConfig := &tls.Config{
	//	ClientCAs: certPool,
	// 	ClientAuth: tls.RequireAndVerifyClientCert,
	// }

	server := mqtt.New(nil)
	_ = server.AddHook(new(auth.AllowHook), nil)

	tcp := listeners.NewTCP("t1", ":1883", &listeners.Config{
		TLSConfig: tlsConfig,
	})
	err = server.AddListener(tcp)
	if err != nil {
		log.Fatal(err)
	}

	ws := listeners.NewWebsocket("ws1", ":1882", &listeners.Config{
		TLSConfig: tlsConfig,
	})
	err = server.AddListener(ws)
	if err != nil {
		log.Fatal(err)
	}

	stats := listeners.NewHTTPStats("stats", ":8080", &listeners.Config{
		TLSConfig: tlsConfig,
	}, server.Info)
	err = server.AddListener(stats)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		err := server.Serve()
		if err != nil {
			log.Fatal(err)
		}
	}()

	<-done
	server.Log.Warn("caught signal, stopping...")
	_ = server.Close()
	server.Log.Info("main.go finished")
}
