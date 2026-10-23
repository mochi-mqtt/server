// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package cluster

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/hashicorp/raft"
)

// metrics are the counters a node keeps about its cluster activity.
type metrics struct {
	publishesForwarded atomic.Int64
	publishesDropped   atomic.Int64
	publishesReceived  atomic.Int64
	bytesForwarded     atomic.Int64

	routeUpdatesSent atomic.Int64
	linkConnects     atomic.Int64
	linkResyncs      atomic.Int64

	retainedSent    atomic.Int64
	retainedApplied atomic.Int64
	retainedIgnored atomic.Int64

	claims              atomic.Int64
	claimFailures       atomic.Int64
	connectionsRefused  atomic.Int64
	sessionsTakenOver   atomic.Int64 // sessions moved to this node from another
	sessionsHandedOver  atomic.Int64 // sessions moved from this node to another
	ownerUnreachable    atomic.Int64 // takeovers which waited out an unreachable node's lease
	sessionsSuperseded  atomic.Int64 // sessions dropped because another node claimed them
	clientsFenced       atomic.Int64 // clients disconnected after losing contact with the registry
	drainDisconnected   atomic.Int64
	drainMigrated       atomic.Int64
	drainMigrateFailure atomic.Int64
}

// PeerStats describes this node's link to another node.
type PeerStats struct {
	Name      string
	Connected bool // the outbound link is connected
	Queued    int  // frames waiting to be sent
	Draining  bool // the peer is draining
	Routes    int  // route filters announced by the peer
}

// Stats is a snapshot of a node's cluster counters and state.
type Stats struct {
	Forwarded int64 // publishes sent to peers
	Dropped   int64 // publishes not sent because a peer was unknown or its queue was full
	Received  int64 // publishes received from peers

	BytesForwarded   int64 // bytes of publish frames sent to peers
	RouteUpdatesSent int64 // route changes queued for peers
	LinkConnects     int64 // outbound peer links established, each sending a full snapshot
	LinkResyncs      int64 // peer links restarted because a queue was full

	RetainedSent    int64 // retained message changes sent to peers
	RetainedApplied int64 // retained message changes from peers applied
	RetainedIgnored int64 // retained message changes from peers ignored as older

	Claims             int64 // sessions claimed in the registry
	ClaimFailures      int64 // claims which failed, refusing the connection
	ConnectionsRefused int64 // connections refused for any cluster reason
	SessionsTakenOver  int64 // sessions moved to this node from another node
	SessionsHandedOver int64 // sessions moved from this node to another node
	OwnerUnreachable   int64 // takeovers which waited for an unreachable node's lease to expire
	SessionsSuperseded int64 // sessions dropped because another node claimed them
	ClientsFenced      int64 // clients disconnected because the node lost contact with the registry

	DrainDisconnected    int64 // clients disconnected while draining
	DrainMigrated        int64 // offline sessions moved to other nodes while draining
	DrainMigrateFailures int64 // offline sessions which could not be moved while draining
	Draining             bool
	Members              int // gossip members, including this node
	Leader               string
	IsLeader             bool
	InContact            bool          // in contact with the registry leader
	LastContact          time.Duration // since the node last heard from a registry quorum
	RaftAppliedIndex     uint64
	RaftLastIndex        uint64
	OwnedSessions        int // sessions this node holds registry claims for
	LocalRoutes          int // filters this node announces
	RemoteRoutes         int // filters announced by other nodes
	RetainedMessages     int
	RetainedTombstones   int
	Peers                []PeerStats
}

// Stats returns a snapshot of the node's cluster counters and state.
func (c *Cluster) Stats() Stats {
	m := &c.metrics
	s := Stats{
		Forwarded:            m.publishesForwarded.Load(),
		Dropped:              m.publishesDropped.Load(),
		Received:             m.publishesReceived.Load(),
		BytesForwarded:       m.bytesForwarded.Load(),
		RouteUpdatesSent:     m.routeUpdatesSent.Load(),
		LinkConnects:         m.linkConnects.Load(),
		LinkResyncs:          m.linkResyncs.Load(),
		RetainedSent:         m.retainedSent.Load(),
		RetainedApplied:      m.retainedApplied.Load(),
		RetainedIgnored:      m.retainedIgnored.Load(),
		Claims:               m.claims.Load(),
		ClaimFailures:        m.claimFailures.Load(),
		ConnectionsRefused:   m.connectionsRefused.Load(),
		SessionsTakenOver:    m.sessionsTakenOver.Load(),
		SessionsHandedOver:   m.sessionsHandedOver.Load(),
		OwnerUnreachable:     m.ownerUnreachable.Load(),
		SessionsSuperseded:   m.sessionsSuperseded.Load(),
		ClientsFenced:        m.clientsFenced.Load(),
		DrainDisconnected:    m.drainDisconnected.Load(),
		DrainMigrated:        m.drainMigrated.Load(),
		DrainMigrateFailures: m.drainMigrateFailure.Load(),
		Draining:             c.draining.Load(),
		Members:              len(c.Members()),
		Leader:               c.Leader(),
		InContact:            c.inContact(),
		OwnedSessions:        c.sessions.len(),
		RetainedMessages:     c.server.Topics.Retained.Len(),
	}

	if c.raft != nil {
		s.IsLeader = c.raft.State() == raft.Leader
		if last := c.lastContact(); !last.IsZero() && last.Year() > 1970 {
			s.LastContact = time.Since(last)
		}
		s.RaftAppliedIndex = c.raft.AppliedIndex()
		s.RaftLastIndex = c.raft.LastIndex()
	}

	c.localMu.Lock()
	s.LocalRoutes = len(c.local)
	c.localMu.Unlock()

	c.retainMu.Lock()
	for _, e := range c.retained {
		if e.deleted {
			s.RetainedTombstones++
		}
	}
	c.retainMu.Unlock()

	c.peersMu.RLock()
	for name, p := range c.peers {
		routes := len(c.remote.filters(name))
		s.RemoteRoutes += routes
		s.Peers = append(s.Peers, PeerStats{
			Name:      name,
			Connected: p.connected(),
			Queued:    len(p.queue),
			Draining:  p.draining.Load(),
			Routes:    routes,
		})
	}
	c.peersMu.RUnlock()
	sort.Slice(s.Peers, func(i, j int) bool { return s.Peers[i].Name < s.Peers[j].Name })

	return s
}

// WritePrometheus writes the node's stats in the Prometheus text exposition format.
func (c *Cluster) WritePrometheus(w io.Writer) error {
	s := c.Stats()
	pw := &promWriter{w: w}

	counter := func(name, help string, v int64) {
		pw.metric("mochi_cluster_"+name, "counter", help, "", strconv.FormatInt(v, 10))
	}
	gauge := func(name, help string, v float64) {
		pw.metric("mochi_cluster_"+name, "gauge", help, "", strconv.FormatFloat(v, 'g', -1, 64))
	}

	counter("publishes_forwarded_total", "Publishes sent to peers.", s.Forwarded)
	counter("publishes_dropped_total", "Publishes not sent because a peer was unknown or its queue was full.", s.Dropped)
	counter("publishes_received_total", "Publishes received from peers.", s.Received)
	counter("forwarded_bytes_total", "Bytes of publish frames sent to peers.", s.BytesForwarded)
	counter("route_updates_sent_total", "Route changes queued for peers.", s.RouteUpdatesSent)
	counter("link_connects_total", "Outbound peer links established.", s.LinkConnects)
	counter("link_resyncs_total", "Peer links restarted because a queue was full.", s.LinkResyncs)
	counter("retained_sent_total", "Retained message changes sent to peers.", s.RetainedSent)
	counter("retained_applied_total", "Retained message changes from peers applied.", s.RetainedApplied)
	counter("retained_ignored_total", "Retained message changes from peers ignored as older.", s.RetainedIgnored)
	counter("session_claims_total", "Sessions claimed in the registry.", s.Claims)
	counter("session_claim_failures_total", "Session claims which failed.", s.ClaimFailures)
	counter("connections_refused_total", "Connections refused by the cluster.", s.ConnectionsRefused)
	counter("sessions_taken_over_total", "Sessions moved to this node from another node.", s.SessionsTakenOver)
	counter("sessions_handed_over_total", "Sessions moved from this node to another node.", s.SessionsHandedOver)
	counter("session_owner_unreachable_total", "Takeovers which waited for an unreachable node's lease to expire.", s.OwnerUnreachable)
	counter("sessions_superseded_total", "Sessions dropped because another node claimed them.", s.SessionsSuperseded)
	counter("clients_fenced_total", "Clients disconnected because the node lost contact with the registry.", s.ClientsFenced)
	counter("drain_disconnected_total", "Clients disconnected while draining.", s.DrainDisconnected)
	counter("drain_migrated_total", "Offline sessions moved to other nodes while draining.", s.DrainMigrated)
	counter("drain_migrate_failures_total", "Offline sessions which could not be moved while draining.", s.DrainMigrateFailures)

	gauge("draining", "1 if the node is draining.", boolFloat(s.Draining))
	gauge("members", "Gossip members, including this node.", float64(s.Members))
	gauge("registry_leader", "1 if this node is the registry leader.", boolFloat(s.IsLeader))
	gauge("registry_in_contact", "1 if this node is in contact with the registry leader.", boolFloat(s.InContact))
	gauge("registry_last_contact_seconds", "Seconds since this node last heard from a registry quorum.", s.LastContact.Seconds())
	gauge("registry_applied_index", "The last registry log index applied.", float64(s.RaftAppliedIndex))
	gauge("registry_last_index", "The last registry log index stored.", float64(s.RaftLastIndex))
	gauge("sessions_owned", "Sessions this node holds registry claims for.", float64(s.OwnedSessions))
	gauge("local_routes", "Route filters this node announces.", float64(s.LocalRoutes))
	gauge("remote_routes", "Route filters announced by other nodes.", float64(s.RemoteRoutes))
	gauge("retained_messages", "Retained messages held by this node.", float64(s.RetainedMessages))
	gauge("retained_tombstones", "Deleted retained messages remembered by this node.", float64(s.RetainedTombstones))

	pw.header("mochi_cluster_peer_connected", "gauge", "1 if the outbound link to a peer is connected.")
	for _, p := range s.Peers {
		pw.sample("mochi_cluster_peer_connected", peerLabel(p.Name), strconv.FormatFloat(boolFloat(p.Connected), 'g', -1, 64))
	}
	pw.header("mochi_cluster_peer_queued_frames", "gauge", "Frames waiting to be sent to a peer.")
	for _, p := range s.Peers {
		pw.sample("mochi_cluster_peer_queued_frames", peerLabel(p.Name), strconv.Itoa(p.Queued))
	}
	pw.header("mochi_cluster_peer_routes", "gauge", "Route filters announced by a peer.")
	for _, p := range s.Peers {
		pw.sample("mochi_cluster_peer_routes", peerLabel(p.Name), strconv.Itoa(p.Routes))
	}

	return pw.err
}

// MetricsHandler returns an http handler which serves the node's stats in the Prometheus
// text exposition format.
func (c *Cluster) MetricsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_ = c.WritePrometheus(w)
	})
}

func boolFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func peerLabel(name string) string {
	return `peer="` + escapeLabel(name) + `"`
}

// escapeLabel escapes a Prometheus label value.
func escapeLabel(v string) string {
	b := make([]byte, 0, len(v))
	for i := 0; i < len(v); i++ {
		switch v[i] {
		case '\\':
			b = append(b, '\\', '\\')
		case '"':
			b = append(b, '\\', '"')
		case '\n':
			b = append(b, '\\', 'n')
		default:
			b = append(b, v[i])
		}
	}
	return string(b)
}

type promWriter struct {
	w   io.Writer
	err error
}

func (p *promWriter) printf(format string, a ...any) {
	if p.err == nil {
		_, p.err = fmt.Fprintf(p.w, format, a...)
	}
}

func (p *promWriter) header(name, kind, help string) {
	p.printf("# HELP %s %s\n# TYPE %s %s\n", name, help, name, kind)
}

func (p *promWriter) sample(name, labels, value string) {
	if labels != "" {
		p.printf("%s{%s} %s\n", name, labels, value)
		return
	}
	p.printf("%s %s\n", name, value)
}

func (p *promWriter) metric(name, kind, help, labels, value string) {
	p.header(name, kind, help)
	p.sample(name, labels, value)
}
