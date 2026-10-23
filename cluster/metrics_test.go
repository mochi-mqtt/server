// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package cluster

import (
	"bufio"
	"bytes"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStats(t *testing.T) {
	nodes := newCluster(t, "n1", "n2")

	sub := connect(t, nodes[1], "sub", nil)
	sub.subscribe("m/#", 0)
	waitRoutes(t, nodes[0], "n2", "m/#")
	pub := connect(t, nodes[0], "pub", nil)
	pub.publish("m/x", []byte("x"), 0)
	pub.publishRetained("m/r", []byte("r"))
	_, ok := sub.receive(waitFor)
	require.True(t, ok)
	waitRetained(t, nodes, "m/r", []byte("r"))

	s1, s2 := nodes[0].cluster.Stats(), nodes[1].cluster.Stats()
	require.EqualValues(t, 2, s1.Forwarded) // the publish, and the retained publish
	require.Positive(t, s1.BytesForwarded)
	require.EqualValues(t, 2, s2.Received)
	require.EqualValues(t, 1, s1.RetainedSent)
	require.EqualValues(t, 1, s2.RetainedApplied)
	require.EqualValues(t, 1, s2.LocalRoutes)
	require.EqualValues(t, 1, s1.RemoteRoutes)
	require.Positive(t, s2.RouteUpdatesSent)
	require.Positive(t, s1.LinkConnects)
	require.EqualValues(t, 2, s1.Members)
	require.EqualValues(t, 1, s1.Claims)
	require.EqualValues(t, 1, s1.OwnedSessions)
	require.True(t, s1.InContact)
	require.NotEmpty(t, s1.Leader)
	require.NotEqual(t, s1.IsLeader, s2.IsLeader)
	require.Positive(t, s1.RaftAppliedIndex)
	require.Equal(t, []PeerStats{{Name: "n2", Connected: true, Routes: 1}}, s1.Peers)
}

func TestWritePrometheus(t *testing.T) {
	nodes := newCluster(t, "n1", "n2")
	connect(t, nodes[0], "c", nil)

	rec := httptest.NewRecorder()
	nodes[0].cluster.MetricsHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Header().Get("Content-Type"), "text/plain")

	body := rec.Body.String()
	require.Contains(t, body, "# TYPE mochi_cluster_publishes_forwarded_total counter\n")
	require.Contains(t, body, "mochi_cluster_session_claims_total 1\n")
	require.Contains(t, body, "mochi_cluster_members 2\n")
	require.Contains(t, body, "mochi_cluster_registry_in_contact 1\n")
	require.Contains(t, body, `mochi_cluster_peer_connected{peer="n2"} 1`+"\n")

	// Every sample line is a valid exposition line, and every metric has help and a type.
	sample := regexp.MustCompile(`^mochi_cluster_[a-z_]+(\{peer="[^"]*"\})? [0-9.e+-]+$`)
	types := map[string]bool{}
	sc := bufio.NewScanner(strings.NewReader(body))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "# TYPE ") {
			types[strings.Fields(line)[2]] = true
			continue
		}
		if strings.HasPrefix(line, "# HELP ") {
			continue
		}
		require.Regexp(t, sample, line)
		name := line[:strings.IndexAny(line, "{ ")]
		require.True(t, types[name], "metric %s has no type", name)
	}
}

func TestWritePrometheusError(t *testing.T) {
	nodes := newCluster(t, "n1")
	require.Error(t, nodes[0].cluster.WritePrometheus(failingWriter{}))
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, http.ErrHandlerTimeout }

func TestEscapeLabel(t *testing.T) {
	require.Equal(t, `a\"b\\c\nd`, escapeLabel("a\"b\\c\nd"))
	var buf bytes.Buffer
	pw := &promWriter{w: &buf}
	pw.sample("m", peerLabel(`x"y`), "1")
	require.Equal(t, `m{peer="x\"y"} 1`+"\n", buf.String())
}
