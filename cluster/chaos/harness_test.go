// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

//go:build chaos

package chaos

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/mochi-mqtt/server/v2/cluster"
)

const project = "mochi-chaos"

// node is a cluster node running in docker.
type node struct {
	name  string
	ip    string // address on the cluster network
	mqtt  string // mqtt address on the host
	admin string // admin http address on the host
}

func (n node) container() string {
	return "chaos-" + n.name
}

var nodes = func() []node {
	ns := make([]node, 5)
	for i := range ns {
		ns[i] = node{
			name:  fmt.Sprintf("n%d", i+1),
			ip:    fmt.Sprintf("172.30.0.%d", 11+i),
			mqtt:  fmt.Sprintf("127.0.0.1:%d", 18831+i),
			admin: fmt.Sprintf("http://127.0.0.1:%d", 18081+i),
		}
	}
	return ns
}()

func nodeNamed(name string) node {
	for _, n := range nodes {
		if n.name == name {
			return n
		}
	}
	panic("unknown node " + name)
}

func names(ns []node) []string {
	s := make([]string, len(ns))
	for i, n := range ns {
		s[i] = n.name
	}
	return s
}

func run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err != nil {
		return out.String(), fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, out.String())
	}
	return out.String(), nil
}

func compose(args ...string) (string, error) {
	return run("docker", append([]string{"compose", "-p", project, "-f", "docker-compose.yml"}, args...)...)
}

// dexec runs a command in a node's container.
func dexec(n node, args ...string) (string, error) {
	return run("docker", append([]string{"exec", n.container()}, args...)...)
}

func TestMain(m *testing.M) {
	if err := setup(); err != nil {
		fmt.Fprintln(os.Stderr, "chaos setup failed:", err)
		dumpLogs("setup")
		teardown()
		os.Exit(1)
	}

	code := m.Run()
	if code != 0 {
		dumpLogs("failed")
	}

	if os.Getenv("CHAOS_KEEP") == "" {
		teardown()
	}
	os.Exit(code)
}

// setup builds the node binary and image, and starts the cluster: n1 first, as it
// bootstraps the cluster, then the others.
func setup() error {
	arch, err := run("docker", "version", "-f", "{{.Server.Arch}}")
	if err != nil {
		return err
	}

	build := exec.Command("go", "build", "-o", "bin/mochi-cluster", "../../examples/cluster")
	build.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+strings.TrimSpace(arch), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		return fmt.Errorf("build node: %w\n%s", err, out)
	}

	teardown()
	if _, err := compose("build", "n1"); err != nil {
		return err
	}

	if _, err := compose("up", "-d", "--no-build", "n1"); err != nil {
		return err
	}
	if err := waitHealthy(time.Minute, nodes[:1]...); err != nil {
		return err
	}

	if _, err := compose(append([]string{"up", "-d", "--no-build"}, names(nodes[1:])...)...); err != nil {
		return err
	}
	return waitHealthy(time.Minute, nodes...)
}

func teardown() {
	_, _ = compose("down", "-v", "--remove-orphans", "-t", "5")
}

// dumpLogs writes every node's log to a file, for investigating failures.
func dumpLogs(reason string) {
	out, _ := compose("logs", "--no-color", "-t")
	name := fmt.Sprintf("chaos-logs-%s-%d.txt", reason, time.Now().Unix())
	if err := os.WriteFile(name, []byte(out), 0o644); err == nil {
		fmt.Fprintln(os.Stderr, "node logs written to", name)
	}
}

var httpClient = &http.Client{Timeout: time.Second * 2}

// stats returns a node's cluster stats.
func stats(n node) (cluster.Stats, error) {
	var s cluster.Stats
	resp, err := httpClient.Get(n.admin + "/stats")
	if err != nil {
		return s, err
	}
	defer resp.Body.Close()
	return s, json.NewDecoder(resp.Body).Decode(&s)
}

type clientInfo struct {
	ID        string `json:"id"`
	Connected bool   `json:"connected"`
}

// clients returns a node's mqtt clients.
func clients(n node) ([]clientInfo, error) {
	var cs []clientInfo
	resp, err := httpClient.Get(n.admin + "/clients")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return cs, json.NewDecoder(resp.Body).Decode(&cs)
}

// drain drains a node.
func drain(n node) (cluster.DrainResult, error) {
	var out struct {
		Result cluster.DrainResult `json:"result"`
		Error  string              `json:"error"`
	}
	c := &http.Client{Timeout: time.Minute}
	resp, err := c.Post(n.admin+"/drain", "", nil)
	if err != nil {
		return out.Result, err
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out.Result, err
	}
	if out.Error != "" {
		return out.Result, fmt.Errorf("drain %s: %s", n.name, out.Error)
	}
	return out.Result, nil
}

// waitHealthy waits until every given node is in contact with the registry and sees
// every running node as a member.
func waitHealthy(timeout time.Duration, ns ...node) error {
	deadline := time.Now().Add(timeout)
	var last error
	for time.Now().Before(deadline) {
		last = nil
		for _, n := range ns {
			s, err := stats(n)
			switch {
			case err != nil:
				last = fmt.Errorf("%s: %w", n.name, err)
			case !s.InContact:
				last = fmt.Errorf("%s: not in contact with the registry", n.name)
			case s.Members < len(ns):
				last = fmt.Errorf("%s: %d members, want %d", n.name, s.Members, len(ns))
			case s.Draining:
				last = fmt.Errorf("%s: draining", n.name)
			}
			if last != nil {
				break
			}
		}

		if last == nil {
			return nil
		}
		time.Sleep(time.Millisecond * 250)
	}
	return fmt.Errorf("cluster not healthy after %s: %w", timeout, last)
}

// leader returns the current registry leader.
func leader() (node, error) {
	for _, n := range nodes {
		if s, err := stats(n); err == nil && s.IsLeader {
			return n, nil
		}
	}
	return node{}, fmt.Errorf("no registry leader")
}

// partition cuts all traffic between every node in a and every node in b.
func partition(t *testing.T, a, b []node) {
	t.Helper()
	for _, x := range a {
		for _, y := range b {
			for _, pair := range [][2]node{{x, y}, {y, x}} {
				from, to := pair[0], pair[1]
				mustExec(t, from, "iptables", "-A", "INPUT", "-s", to.ip, "-j", "DROP")
				mustExec(t, from, "iptables", "-A", "OUTPUT", "-d", to.ip, "-j", "DROP")
			}
		}
	}
	t.Logf("partitioned %v from %v", names(a), names(b))
}

// netem degrades a node's network.
func netem(t *testing.T, n node, args ...string) {
	t.Helper()
	mustExec(t, n, append([]string{"tc", "qdisc", "replace", "dev", "eth0", "root", "netem"}, args...)...)
}

// heal removes every network fault.
func heal(t *testing.T) {
	t.Helper()
	for _, n := range nodes {
		_, _ = dexec(n, "iptables", "-F")
		_, _ = dexec(n, "tc", "qdisc", "del", "dev", "eth0", "root")
	}
	t.Log("healed network")
}

func mustExec(t *testing.T, n node, args ...string) {
	t.Helper()
	if _, err := dexec(n, args...); err != nil {
		t.Fatal(err)
	}
}

func docker(t *testing.T, args ...string) {
	t.Helper()
	if _, err := run("docker", args...); err != nil {
		t.Fatal(err)
	}
}

// others returns every node except the given ones.
func others(except ...node) []node {
	var out []node
	for _, n := range nodes {
		skip := false
		for _, e := range except {
			skip = skip || e.name == n.name
		}
		if !skip {
			out = append(out, n)
		}
	}
	return out
}
