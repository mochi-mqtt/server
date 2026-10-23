// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package main

import (
	"context"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// config is the configuration of a cluster node. Every setting can be given as a flag, or
// as the environment variable named in its usage, so that one image runs anywhere.
type config struct {
	NodeName string

	MQTTAddr    string // plain tcp listener, empty to disable
	TLSAddr     string // tls listener, used if TLSCertFile and TLSKeyFile are set
	TLSCertFile string
	TLSKeyFile  string
	WSAddr      string // websocket listener, empty to disable
	AdminAddr   string // metrics, health and draining, empty to disable

	BindAddr    string
	Advertise   string
	GossipPort  int
	ForwardAddr string
	RaftAddr    string
	Seeds       []string
	Bootstrap   string // "true", "false", or "auto" (true if the node name ends in -0)
	GossipKey   []byte
	JoinTimeout time.Duration

	LeaseTimeout time.Duration
	ClaimTimeout time.Duration

	AuthFile string
	AllowAll bool

	DrainOnStop  bool
	DrainRate    int
	DrainTimeout time.Duration

	LogLevel  slog.Level
	LogFormat string // "json" or "text"
}

var (
	errNoAuth       = errors.New("an auth file (MQTT_AUTH_FILE) is required, or set MQTT_ALLOW_ALL=true to allow every client")
	errTLSPair      = errors.New("MQTT_TLS_CERT_FILE and MQTT_TLS_KEY_FILE must be set together")
	errNoListener   = errors.New("no mqtt listener is enabled")
	errBadBootstrap = errors.New("MQTT_BOOTSTRAP must be true, false or auto")
	errBadGossipKey = errors.New("MQTT_GOSSIP_KEY must be base64 of 16, 24 or 32 bytes")
	ordinalZero     = regexp.MustCompile(`-0$`)
)

// parseConfig reads the configuration from args, using getenv for the defaults.
func parseConfig(args []string, getenv func(string) string) (*config, error) {
	fs := flag.NewFlagSet("mochi-cluster", flag.ContinueOnError)
	env := envDefaults{getenv: getenv}

	hostname, _ := os.Hostname()
	c := new(config)
	var seeds, gossipKey, gossipKeyFile, logLevel string

	fs.StringVar(&c.NodeName, "name", env.str("MQTT_NODE_NAME", hostname), "unique node name [MQTT_NODE_NAME] (default: hostname)")
	fs.StringVar(&c.MQTTAddr, "mqtt", env.str("MQTT_LISTEN", ":1883"), "mqtt tcp listener address, empty to disable [MQTT_LISTEN]")
	fs.StringVar(&c.TLSAddr, "tls", env.str("MQTT_TLS_LISTEN", ":8883"), "mqtt tls listener address [MQTT_TLS_LISTEN]")
	fs.StringVar(&c.TLSCertFile, "tls-cert", env.str("MQTT_TLS_CERT_FILE", ""), "tls certificate file, enables the tls listener [MQTT_TLS_CERT_FILE]")
	fs.StringVar(&c.TLSKeyFile, "tls-key", env.str("MQTT_TLS_KEY_FILE", ""), "tls key file [MQTT_TLS_KEY_FILE]")
	fs.StringVar(&c.WSAddr, "ws", env.str("MQTT_WS_LISTEN", ""), "mqtt websocket listener address, empty to disable [MQTT_WS_LISTEN]")
	fs.StringVar(&c.AdminAddr, "admin", env.str("MQTT_ADMIN_LISTEN", ":8080"), "admin http address for /metrics, /stats, /healthz, /livez and /drain [MQTT_ADMIN_LISTEN]")

	fs.StringVar(&c.BindAddr, "bind", env.str("MQTT_BIND", "0.0.0.0"), "gossip bind address [MQTT_BIND]")
	fs.StringVar(&c.Advertise, "advertise", env.str("MQTT_ADVERTISE", ""), "address announced to other nodes [MQTT_ADVERTISE]")
	fs.IntVar(&c.GossipPort, "gossip", env.int("MQTT_GOSSIP_PORT", 7946), "gossip port, tcp and udp [MQTT_GOSSIP_PORT]")
	fs.StringVar(&c.ForwardAddr, "forward", env.str("MQTT_FORWARD_LISTEN", ":7947"), "peer link listener address [MQTT_FORWARD_LISTEN]")
	fs.StringVar(&c.RaftAddr, "raft", env.str("MQTT_RAFT_LISTEN", ":7948"), "session registry (raft) listener address [MQTT_RAFT_LISTEN]")
	fs.StringVar(&seeds, "seeds", env.str("MQTT_SEEDS", ""), "comma-separated gossip addresses or dns names of other nodes [MQTT_SEEDS]")
	fs.StringVar(&c.Bootstrap, "bootstrap", env.str("MQTT_BOOTSTRAP", "auto"), "create a new cluster if no seed is found: true, false, or auto for nodes named *-0 [MQTT_BOOTSTRAP]")
	fs.StringVar(&gossipKey, "gossip-key", env.str("MQTT_GOSSIP_KEY", ""), "base64 key encrypting gossip (16, 24 or 32 bytes) [MQTT_GOSSIP_KEY]")
	fs.StringVar(&gossipKeyFile, "gossip-key-file", env.str("MQTT_GOSSIP_KEY_FILE", ""), "file containing the base64 gossip key [MQTT_GOSSIP_KEY_FILE]")
	fs.DurationVar(&c.JoinTimeout, "join-timeout", env.duration("MQTT_JOIN_TIMEOUT", 2*time.Minute), "how long a non-bootstrap node waits for a seed to resolve [MQTT_JOIN_TIMEOUT]")

	fs.DurationVar(&c.LeaseTimeout, "lease", env.duration("MQTT_LEASE_TIMEOUT", 0), "registry lease timeout (default 10s) [MQTT_LEASE_TIMEOUT]")
	fs.DurationVar(&c.ClaimTimeout, "claim", env.duration("MQTT_CLAIM_TIMEOUT", 0), "session claim timeout (default 5s) [MQTT_CLAIM_TIMEOUT]")

	fs.StringVar(&c.AuthFile, "auth-file", env.str("MQTT_AUTH_FILE", ""), "auth ledger file, yaml or json [MQTT_AUTH_FILE]")
	fs.BoolVar(&c.AllowAll, "allow-all", env.bool("MQTT_ALLOW_ALL", false), "allow every client without an auth file [MQTT_ALLOW_ALL]")

	fs.BoolVar(&c.DrainOnStop, "drain-on-stop", env.bool("MQTT_DRAIN_ON_STOP", true), "drain the node before stopping [MQTT_DRAIN_ON_STOP]")
	fs.IntVar(&c.DrainRate, "drain-rate", env.int("MQTT_DRAIN_RATE", 0), "clients disconnected per second when draining, 0 for all at once [MQTT_DRAIN_RATE]")
	fs.DurationVar(&c.DrainTimeout, "drain-timeout", env.duration("MQTT_DRAIN_TIMEOUT", time.Minute), "longest a drain on stop may take [MQTT_DRAIN_TIMEOUT]")

	fs.StringVar(&logLevel, "log-level", env.str("MQTT_LOG_LEVEL", "info"), "debug, info, warn or error [MQTT_LOG_LEVEL]")
	fs.StringVar(&c.LogFormat, "log-format", env.str("MQTT_LOG_FORMAT", "json"), "json or text [MQTT_LOG_FORMAT]")

	if env.err != nil {
		return nil, env.err
	}

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	for _, s := range strings.Split(seeds, ",") {
		if s = strings.TrimSpace(s); s != "" {
			c.Seeds = append(c.Seeds, s)
		}
	}

	if err := c.LogLevel.UnmarshalText([]byte(logLevel)); err != nil {
		return nil, fmt.Errorf("log level: %w", err)
	}

	if gossipKeyFile != "" {
		b, err := os.ReadFile(gossipKeyFile)
		if err != nil {
			return nil, fmt.Errorf("gossip key file: %w", err)
		}
		gossipKey = string(b)
	}

	if gossipKey = strings.TrimSpace(gossipKey); gossipKey != "" {
		key, err := base64.StdEncoding.DecodeString(gossipKey)
		if err != nil || (len(key) != 16 && len(key) != 24 && len(key) != 32) {
			return nil, errBadGossipKey
		}
		c.GossipKey = key
	}

	return c, c.validate()
}

func (c *config) validate() error {
	switch {
	case c.NodeName == "":
		return errors.New("a node name (MQTT_NODE_NAME) is required")
	case c.AuthFile == "" && !c.AllowAll:
		return errNoAuth
	case (c.TLSCertFile == "") != (c.TLSKeyFile == ""):
		return errTLSPair
	case c.MQTTAddr == "" && c.TLSCertFile == "" && c.WSAddr == "":
		return errNoListener
	case c.LogFormat != "json" && c.LogFormat != "text":
		return errors.New("MQTT_LOG_FORMAT must be json or text")
	}

	switch c.Bootstrap {
	case "true", "false", "auto":
	default:
		return errBadBootstrap
	}

	return nil
}

// bootstrap reports whether this node may create a new cluster.
func (c *config) bootstrap() bool {
	return c.Bootstrap == "true" || (c.Bootstrap == "auto" && ordinalZero.MatchString(c.NodeName))
}

// lookupFn resolves a host to its addresses, as net.Resolver.LookupHost.
type lookupFn func(ctx context.Context, host string) ([]string, error)

// resolveSeeds resolves the seeds to addresses, leaving out this node's own addresses. A
// dns name such as a kubernetes headless service resolves to every node, including this
// one; the cluster bootstraps a new cluster when given no seeds, and joining only itself
// would leave the node waiting for a quorum which never forms. Seeds which don't resolve
// are skipped, as nodes which haven't started yet may not have dns records.
func resolveSeeds(ctx context.Context, seeds []string, self map[string]bool, lookup lookupFn) []string {
	var out []string
	seen := map[string]bool{}
	for _, seed := range seeds {
		host, port, err := net.SplitHostPort(seed)
		if err != nil {
			continue
		}

		addrs, err := lookup(ctx, host)
		if err != nil {
			continue
		}

		for _, a := range addrs {
			addr := net.JoinHostPort(a, port)
			if self[a] || seen[addr] {
				continue
			}
			seen[addr] = true
			out = append(out, addr)
		}
	}
	return out
}

// selfAddrs returns the addresses which identify this node: the advertised address, if
// set, and the addresses of every local interface.
func selfAddrs(advertise string) map[string]bool {
	self := map[string]bool{}
	if advertise != "" {
		self[advertise] = true
	}

	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		if ipn, ok := a.(*net.IPNet); ok {
			self[ipn.IP.String()] = true
		}
	}
	return self
}

// envDefaults reads flag defaults from the environment, keeping the first parse error.
type envDefaults struct {
	getenv func(string) string
	err    error
}

func (e *envDefaults) str(name, def string) string {
	if v, ok := e.lookup(name); ok {
		return v
	}
	return def
}

func (e *envDefaults) int(name string, def int) int {
	v, ok := e.lookup(name)
	if !ok {
		return def
	}

	n, err := strconv.Atoi(v)
	if err != nil {
		e.fail(name, err)
		return def
	}
	return n
}

func (e *envDefaults) bool(name string, def bool) bool {
	v, ok := e.lookup(name)
	if !ok {
		return def
	}

	b, err := strconv.ParseBool(v)
	if err != nil {
		e.fail(name, err)
		return def
	}
	return b
}

func (e *envDefaults) duration(name string, def time.Duration) time.Duration {
	v, ok := e.lookup(name)
	if !ok {
		return def
	}

	d, err := time.ParseDuration(v)
	if err != nil {
		e.fail(name, err)
		return def
	}
	return d
}

func (e *envDefaults) lookup(name string) (string, bool) {
	v := strings.TrimSpace(e.getenv(name))
	return v, v != ""
}

func (e *envDefaults) fail(name string, err error) {
	if e.err == nil {
		e.err = fmt.Errorf("%s: %w", name, err)
	}
}
