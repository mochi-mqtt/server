// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

// Package chaos tests a cluster of nodes running in docker under real network faults:
// partitions (iptables), delay and packet loss (tc netem), crashes (docker kill), and
// process pauses (docker pause), while a workload of clients moves sessions between
// nodes. After each fault heals, it checks that no client was ever connected to two
// nodes at once, that messages are delivered between every pair of nodes, that retained
// messages converge, and that every session exists on only one node.
//
// It needs docker, and is excluded from normal test runs:
//
//	cd cluster/chaos && go test -tags chaos -v -timeout 30m
//
// Set CHAOS_KEEP=1 to leave the cluster running afterwards, and CHAOS_RUN to choose
// scenarios with a regular expression, as with -run.
package chaos
