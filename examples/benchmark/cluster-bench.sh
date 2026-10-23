#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
# SPDX-FileContributor: mochi-co
#
# Benchmarks the benchmark server standalone, as a one node cluster, and as a three node
# cluster on this machine, using loadtest and (if MQTT_STRESSER is set to its path)
# mqtt-stresser, as in the README.
#
#   ./examples/benchmark/cluster-bench.sh
set -euo pipefail

cd "$(dirname "$0")/../.."
bin=$(mktemp -d)
trap 'kill $(jobs -p) 2>/dev/null || true; rm -rf "$bin"' EXIT

go build -o "$bin/server" ./examples/benchmark
go build -o "$bin/loadtest" ./examples/benchmark/loadtest

stop() {
	kill -INT $(jobs -p) 2>/dev/null || true
	wait 2>/dev/null || true
	sleep 1
}

node() { # name mqtt-port base-port [seed]
	local args=(-tcp ":$2" -cluster-name "$1" -cluster-gossip "$3" -cluster-forward ":$(($3 + 1))" -cluster-raft ":$(($3 + 2))")
	[[ -n "${4:-}" ]] && args+=(-cluster-seeds "$4")
	"$bin/server" "${args[@]}" >"$bin/$1.log" 2>&1 &
}

load() { # label args...
	local label=$1
	shift
	"$bin/loadtest" -label "$label" "$@"
}

stress() { # label broker clients
	if [[ -n "${MQTT_STRESSER:-}" ]]; then
		echo "--- mqtt-stresser $1 (-num-clients=$3 -num-messages=10000)"
		"$MQTT_STRESSER" -broker "tcp://$2" -num-clients="$3" -num-messages=10000 2>&1 | grep -E "(Publishing|Receiving) Throughput|Fastest|Slowest|Median" || true
	fi
}

suite() { # label publish-broker subscribe-broker
	load "$1 qos0" -pub "$2" -sub "$3" -qos 0
	load "$1 qos1" -pub "$2" -sub "$3" -qos 1 -messages 10000
	load "$1 qos0 @20k/s" -pub "$2" -sub "$3" -qos 0 -messages 25000 -rate 5000
	load "$1 qos1 @20k/s" -pub "$2" -sub "$3" -qos 1 -messages 10000 -rate 5000
}

a=127.0.0.1:1883
b=127.0.0.1:1884
c=127.0.0.1:1885

echo "=== standalone"
"$bin/server" -tcp :1883 >"$bin/standalone.log" 2>&1 &
sleep 1
stress standalone $a 2
stress standalone $a 10
suite "standalone" $a $a
load "standalone connect" -pub $a -connections 5000
stop

echo "=== one node cluster"
node n1 1883 7946
sleep 3
stress "one node" $a 2
stress "one node" $a 10
suite "1-node" $a $a
load "1-node connect" -pub $a -connections 5000
stop

echo "=== three node cluster"
node n1 1883 7946
sleep 3
node n2 1884 7956 127.0.0.1:7946
node n3 1885 7966 127.0.0.1:7946
sleep 4
stress "three node, one node" $a 2
stress "three node, one node" $a 10
suite "3-node same node" $a $a
suite "3-node cross node" $a $b
load "3-node fan-out qos0" -pub $a -sub "$b,$c" -subscribers 2
load "3-node fan-out qos0 @20k/s" -pub $a -sub "$b,$c" -subscribers 2 -messages 25000 -rate 5000
load "3-node all-to-all qos0" -pub "$a,$b,$c" -sub "$a,$b,$c" -publishers 3 -subscribers 3
load "3-node connect leader" -pub $a -connections 5000
load "3-node connect followers" -pub "$b,$c" -connections 5000
stop
