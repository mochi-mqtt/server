// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

//go:build chaos

package chaos

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	sessionClients = 12 // persistent clients moving between nodes throughout each scenario
	retainedKeys   = 5  // topics with retained messages written throughout each scenario
	deliveryRound  = 50 // messages each node publishes when checking delivery
	dialTimeout    = 15 // seconds; a takeover from an unreachable node waits out its lease
	overlapSlack   = 50 // milliseconds of client-observed overlap allowed for scheduling
)

// workload moves persistent sessions between random nodes, and writes retained messages
// to random nodes, until its context is cancelled.
type workload struct {
	scenario string
	ids      []string
	wg       sync.WaitGroup

	mu       sync.Mutex
	conns    map[string][]*conn
	retained map[string]map[string]bool // key -> values published

	dials, connected, refused, failed, kicked, retainedWrites atomic.Int64
}

func startWorkload(ctx context.Context, scenario string) *workload {
	w := &workload{
		scenario: scenario,
		conns:    map[string][]*conn{},
		retained: map[string]map[string]bool{},
	}

	for i := 0; i < sessionClients; i++ {
		id := fmt.Sprintf("session-%d", i)
		w.ids = append(w.ids, id)
		w.wg.Add(1)
		go w.sessionLoop(ctx, id, rand.New(rand.NewSource(time.Now().UnixNano()+int64(i))))
	}

	w.wg.Add(1)
	go w.retainedLoop(ctx, rand.New(rand.NewSource(time.Now().UnixNano())))
	return w
}

// sessionLoop repeatedly connects a client to a random node, holds the connection for a
// while, and then either closes it or leaves it open so the next connection must take the
// session over from it.
func (w *workload) sessionLoop(ctx context.Context, id string, rnd *rand.Rand) {
	defer w.wg.Done()
	for ctx.Err() == nil {
		n := nodes[rnd.Intn(len(nodes))]
		w.dials.Add(1)
		c, code, err := dial(n, id, true, time.Second*dialTimeout)
		if err != nil || code != 0 {
			if err != nil {
				w.failed.Add(1)
			} else {
				w.refused.Add(1)
			}
			sleep(ctx, time.Millisecond*100)
			continue
		}

		w.connected.Add(1)
		w.mu.Lock()
		w.conns[id] = append(w.conns[id], c)
		w.mu.Unlock()

		// The client receives its own heartbeats, so any activity on a superseded
		// connection shows up as a message received after its successor connected.
		_ = c.subscribe("chaos/heartbeat/"+id, 0)

		hold := time.After(time.Millisecond * time.Duration(300+rnd.Intn(1200)))
		beat := time.NewTicker(time.Millisecond * 200)
	held:
		for {
			select {
			case <-ctx.Done():
				break held
			case <-c.done:
				w.kicked.Add(1)
				break held
			case <-hold:
				break held
			case <-beat.C:
				_ = c.publish("chaos/heartbeat/"+id, []byte(n.name), 0, false)
			}
		}
		beat.Stop()

		if rnd.Intn(2) == 0 {
			c.close()
		}
	}
}

// retainedLoop writes retained messages to random nodes.
func (w *workload) retainedLoop(ctx context.Context, rnd *rand.Rand) {
	defer w.wg.Done()
	writers := map[string]*conn{}
	defer func() {
		for _, c := range writers {
			c.close()
		}
	}()

	for i := 0; ctx.Err() == nil; i++ {
		n := nodes[rnd.Intn(len(nodes))]
		c := writers[n.name]
		if c == nil || isClosed(c) {
			var code byte
			var err error
			c, code, err = dial(n, "retained-writer-"+n.name, false, time.Second*dialTimeout)
			if err != nil || code != 0 {
				sleep(ctx, time.Millisecond*100)
				continue
			}
			writers[n.name] = c
		}

		key := fmt.Sprintf("chaos/retained/%d", rnd.Intn(retainedKeys))
		value := fmt.Sprintf("%s-%s-%d", w.scenario, n.name, i)
		w.mu.Lock()
		if w.retained[key] == nil {
			w.retained[key] = map[string]bool{}
		}
		w.retained[key][value] = true // recorded before publishing, as it may apply without an ack
		w.mu.Unlock()

		if c.publish(key, []byte(value), 1, true) == nil {
			w.retainedWrites.Add(1)
		}
		sleep(ctx, time.Millisecond*150)
	}
}

// stop stops the workload and closes every connection.
func (w *workload) stop(cancel context.CancelFunc) {
	cancel()
	w.wg.Wait()
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, cs := range w.conns {
		for _, c := range cs {
			c.close()
		}
	}
}

func isClosed(c *conn) bool {
	_, closed := c.closedTime()
	return closed
}

func sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

// checkNoOverlap checks that no client was ever served by two nodes at once. A superseded
// connection must never receive a message after its successor connected and, unless a
// node was frozen (when its connections stay open but can't be served), it must have
// been closed by then too.
func checkNoOverlap(t *testing.T, w *workload, slack time.Duration, frozen bool) {
	t.Helper()
	w.mu.Lock()
	defer w.mu.Unlock()

	var worstOpen, worstActive time.Duration
	for id, cs := range w.conns {
		sort.Slice(cs, func(i, j int) bool { return cs[i].connacked.Before(cs[j].connacked) })
		for i := range cs {
			end, closed := cs[i].closedTime()
			require.True(t, closed, "%s connection to %s still open", id, cs[i].node)
			for _, next := range cs[i+1:] {
				if rx := cs[i].lastRx.Load(); rx > 0 {
					active := time.Unix(0, rx).Sub(next.connacked)
					worstActive = max(worstActive, active)
					require.LessOrEqual(t, active, slack,
						"%s received a message on %s %s after connecting to %s", id, cs[i].node, active, next.node)
				}

				open := end.Sub(next.connacked)
				worstOpen = max(worstOpen, open)
				if !frozen {
					require.LessOrEqual(t, open, slack,
						"%s was connected to %s and %s at once for %s (connected %s and %s, first closed %s with reason %#x)",
						id, cs[i].node, next.node, open, cs[i].connacked.Format(time.StampMilli),
						next.connacked.Format(time.StampMilli), end.Format(time.StampMilli), cs[i].reason)
				}
			}
		}
	}
	t.Logf("sessions: dials=%d connected=%d refused=%d failed=%d taken-over-or-fenced=%d worst-open-overlap=%s worst-active-overlap=%s",
		w.dials.Load(), w.connected.Load(), w.refused.Load(), w.failed.Load(), w.kicked.Load(), worstOpen, worstActive)
}

// checkDelivery checks that messages published on every node reach subscribers on every
// node, once routes have converged.
func checkDelivery(t *testing.T, scenario string) {
	t.Helper()
	prefix := "chaos/check/" + scenario
	subs := map[string]*conn{}
	pubs := map[string]*conn{}
	for _, n := range nodes {
		sub, code, err := dial(n, "check-sub-"+n.name, false, time.Second*dialTimeout)
		require.NoError(t, err)
		require.Zero(t, code, "subscriber refused by %s", n.name)
		require.NoError(t, sub.subscribe(prefix+"/#", 1))
		subs[n.name] = sub

		pub, code, err := dial(n, "check-pub-"+n.name, false, time.Second*dialTimeout)
		require.NoError(t, err)
		require.Zero(t, code, "publisher refused by %s", n.name)
		pubs[n.name] = pub
	}
	defer func() {
		for _, c := range subs {
			c.close()
		}
		for _, c := range pubs {
			c.close()
		}
	}()

	// Routes are eventually consistent, so wait until probes from every node reach every node.
	require.Eventually(t, func() bool {
		round := fmt.Sprint(time.Now().UnixNano())
		for name, pub := range pubs {
			if pub.publish(prefix+"/"+name+"/probe", []byte(round), 1, false) != nil {
				return false
			}
		}
		for _, sub := range subs {
			if !receiveFrom(sub, round, len(pubs), time.Second*2) {
				return false
			}
		}
		return true
	}, time.Second*60, time.Millisecond*100, "routes did not converge")

	for name, pub := range pubs {
		for i := 0; i < deliveryRound; i++ {
			require.NoError(t, pub.publish(prefix+"/"+name+"/m", []byte(fmt.Sprintf("m:%s:%d", name, i)), 1, false))
		}
	}

	want := len(pubs) * deliveryRound
	for name, sub := range subs {
		seen := map[string]int{}
		deadline := time.After(time.Second * 20)
	collect:
		for len(seen) < want {
			select {
			case pk := <-sub.messages:
				if strings.HasPrefix(string(pk.Payload), "m:") {
					seen[string(pk.Payload)]++
				}
			case <-deadline:
				break collect
			}
		}
		require.Len(t, seen, want, "subscriber on %s received %d of %d messages", name, len(seen), want)
		for m, count := range seen {
			require.Equal(t, 1, count, "subscriber on %s received %s %d times", name, m, count)
		}
	}
	t.Logf("delivery: %d messages from every node reached every node exactly once", deliveryRound)
}

// receiveFrom waits until a subscriber receives payload from n publishers.
func receiveFrom(c *conn, payload string, n int, timeout time.Duration) bool {
	got := map[string]bool{}
	deadline := time.After(timeout)
	for len(got) < n {
		select {
		case pk := <-c.messages:
			if string(pk.Payload) == payload {
				got[pk.TopicName] = true
			}
		case <-deadline:
			return false
		}
	}
	return true
}

// retainedOn returns the retained messages a node holds under chaos/retained.
func retainedOn(n node) (map[string]string, error) {
	c, code, err := dial(n, "check-retained-"+n.name, false, time.Second*dialTimeout)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, fmt.Errorf("refused: %#x", code)
	}
	defer c.close()

	if err := c.subscribe("chaos/retained/#", 0); err != nil {
		return nil, err
	}

	out := map[string]string{}
	for {
		select {
		case pk := <-c.messages:
			out[pk.TopicName] = string(pk.Payload)
		case <-time.After(time.Millisecond * 500):
			return out, nil
		}
	}
}

// checkRetained checks that every node converges on the same retained messages, each one
// of the values written.
func checkRetained(t *testing.T, w *workload) {
	t.Helper()
	var views map[string]map[string]string
	converged := assert.Eventually(t, func() bool {
		views = map[string]map[string]string{}
		for _, n := range nodes {
			v, err := retainedOn(n)
			if err != nil {
				return false
			}
			views[n.name] = v
		}
		for _, n := range nodes[1:] {
			if fmt.Sprint(views[n.name]) != fmt.Sprint(views[nodes[0].name]) {
				return false
			}
		}
		return true
	}, time.Second*30, time.Millisecond*200)
	if !converged {
		t.Fatalf("retained messages did not converge: %v", views)
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	for key, value := range views[nodes[0].name] {
		if strings.HasPrefix(value, w.scenario+"-") {
			require.True(t, w.retained[key][value], "%s has a value that was never written: %s", key, value)
		}
	}
	t.Logf("retained: %d writes, all nodes agree on %v", w.retainedWrites.Load(), views[nodes[0].name])
}

// checkSessions checks that every workload session exists on exactly one node, and that
// no stale copies are left behind.
func checkSessions(t *testing.T, w *workload) {
	t.Helper()
	var where map[string][]string
	converged := assert.Eventually(t, func() bool {
		where = map[string][]string{}
		for _, n := range nodes {
			cs, err := clients(n)
			if err != nil {
				return false
			}
			for _, c := range cs {
				if strings.HasPrefix(c.ID, "session-") {
					where[c.ID] = append(where[c.ID], n.name)
				}
			}
		}
		for _, id := range w.ids {
			if len(where[id]) > 1 {
				return false
			}
		}
		return true
	}, time.Second*30, time.Millisecond*250)
	if !converged {
		t.Fatalf("sessions on more than one node: %v", where)
	}

	n := 0
	for _, id := range w.ids {
		n += len(where[id])
	}
	t.Logf("sessions: %d of %d persistent sessions held, each by one node", n, len(w.ids))
}

// checkRunning checks that no node has crashed.
func checkRunning(t *testing.T) {
	t.Helper()
	for _, n := range nodes {
		out, err := run("docker", "inspect", "-f", "{{.State.Running}} {{.RestartCount}}", n.container())
		require.NoError(t, err)
		require.True(t, strings.HasPrefix(out, "true"), "%s is not running: %s", n.name, out)
	}
}

func logStats(t *testing.T) {
	t.Helper()
	for _, n := range nodes {
		s, err := stats(n)
		if err != nil {
			t.Logf("%s: %v", n.name, err)
			continue
		}
		t.Logf("%s: leader=%v fenced=%d unreachable-waits=%d superseded=%d taken-over=%d handed-over=%d refused=%d forwarded=%d dropped=%d link-connects=%d resyncs=%d",
			n.name, s.IsLeader, s.ClientsFenced, s.OwnerUnreachable, s.SessionsSuperseded, s.SessionsTakenOver,
			s.SessionsHandedOver, s.ConnectionsRefused, s.Forwarded, s.Dropped, s.LinkConnects, s.LinkResyncs)
	}
}

// scenario runs the workload, injects a fault, heals it, and checks the cluster.
func scenario(t *testing.T, name string, slack time.Duration, frozen bool, fault func(t *testing.T)) {
	if pattern := os.Getenv("CHAOS_RUN"); pattern != "" && !regexp.MustCompile(pattern).MatchString(name) {
		return
	}

	t.Run(name, func(t *testing.T) {
		require.NoError(t, waitHealthy(time.Minute*2, nodes...))
		ctx, cancel := context.WithCancel(context.Background())
		w := startWorkload(ctx, name)
		defer w.stop(cancel)

		time.Sleep(time.Second * 2)
		start := time.Now()
		fault(t)
		heal(t)
		require.NoError(t, waitHealthy(time.Minute*2, nodes...))
		t.Logf("fault and recovery took %s", time.Since(start).Round(time.Millisecond))

		time.Sleep(time.Second * 3) // keep moving sessions on the healed cluster
		w.stop(cancel)

		checkRunning(t)
		checkNoOverlap(t, w, slack, frozen)
		checkDelivery(t, name)
		checkRetained(t, w)
		checkSessions(t, w)
		logStats(t)
	})
}

func TestChaos(t *testing.T) {
	slack := time.Millisecond * overlapSlack

	scenario(t, "baseline", slack, false, func(t *testing.T) {
		time.Sleep(time.Second * 8)
	})

	scenario(t, "isolate-one-node", slack, false, func(t *testing.T) {
		partition(t, nodes[:1], nodes[1:])
		time.Sleep(time.Second * 10)
	})

	scenario(t, "split-two-three", slack, false, func(t *testing.T) {
		partition(t, nodes[:2], nodes[2:])
		time.Sleep(time.Second * 10)
	})

	scenario(t, "isolate-leader", slack, false, func(t *testing.T) {
		l, err := leader()
		require.NoError(t, err)
		partition(t, []node{l}, others(l))
		time.Sleep(time.Second * 10)
	})

	scenario(t, "bridge-partition", slack, false, func(t *testing.T) {
		// n1 and n2 can't reach each other, but both can reach everyone else.
		partition(t, nodes[:1], nodes[1:2])
		time.Sleep(time.Second * 10)
	})

	scenario(t, "kill-leader", slack, false, func(t *testing.T) {
		l, err := leader()
		require.NoError(t, err)
		docker(t, "kill", l.container())
		t.Logf("killed leader %s", l.name)
		time.Sleep(time.Second * 8)
		docker(t, "start", l.container())
	})

	scenario(t, "pause-node", slack, true, func(t *testing.T) {
		l, err := leader()
		require.NoError(t, err)
		n := others(l)[0]
		docker(t, "pause", n.container())
		t.Logf("paused %s", n.name)
		time.Sleep(time.Second * 6) // longer than the lease
		docker(t, "unpause", n.container())
	})

	// Lost packets are retransmitted after at least 200ms, so a disconnect can arrive after
	// the connack of its successor; the activity check still applies.
	scenario(t, "slow-lossy-network", time.Second, false, func(t *testing.T) {
		for _, n := range nodes {
			netem(t, n, "delay", "40ms", "20ms", "loss", "3%")
		}
		time.Sleep(time.Second * 15)
	})

	scenario(t, "rolling-restart", slack, false, func(t *testing.T) {
		rollingRestart(t)
	})
}

// rollingRestart drains and restarts every node in turn, checking that an offline
// session and the messages queued for it survive.
func rollingRestart(t *testing.T) {
	durable, code, err := dial(nodes[2], "durable", true, time.Second*dialTimeout)
	require.NoError(t, err)
	require.Zero(t, code)
	require.NoError(t, durable.subscribe("chaos/durable", 1))
	durable.close()

	publish := func(n node, i int) {
		require.Eventually(t, func() bool {
			c, code, err := dial(n, "durable-pub", false, time.Second*dialTimeout)
			if err != nil || code != 0 {
				return false
			}
			defer c.close()
			return c.publish("chaos/durable", []byte(fmt.Sprintf("queued-%d", i)), 1, false) == nil
		}, time.Second*30, time.Millisecond*200)
	}

	publish(nodes[0], 0)
	for i, n := range nodes {
		res, err := drain(n)
		require.NoError(t, err)
		t.Logf("drained %s: %+v", n.name, res)
		docker(t, "restart", "-t", "30", n.container())
		require.NoError(t, waitHealthy(time.Minute*2, nodes...))
		publish(others(n)[0], i+1)
	}

	c, code, err := dial(nodes[4], "durable", true, time.Second*dialTimeout)
	require.NoError(t, err)
	require.Zero(t, code)
	defer c.close()
	require.True(t, c.present, "durable session lost during the rolling restart")

	got := map[string]bool{}
	deadline := time.After(time.Second * 10)
	for len(got) < len(nodes)+1 {
		select {
		case pk := <-c.messages:
			got[string(pk.Payload)] = true
		case <-deadline:
			t.Fatalf("durable session received %d of %d queued messages: %v", len(got), len(nodes)+1, got)
		}
	}
	t.Logf("durable session survived the rolling restart with all %d queued messages", len(got))
}
