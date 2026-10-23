#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
# SPDX-FileContributor: mochi-co
#
# Smoke tests the cluster image and helm chart on a local kind cluster: installs three
# nodes, checks they form one cluster, sends messages between nodes with an authenticated
# client, checks an unauthenticated client is refused, then does a rolling restart and
# checks again. Needs docker, kind, kubectl, helm and go.
#
#   ./deploy/kind/smoke.sh
#
# Set KEEP=1 to leave the kind cluster running (its kubeconfig is deploy/kind/.kubeconfig),
# and SKIP_BUILD=1 to use an existing mochi-cluster:ci image.
set -Eeuo pipefail

cd "$(dirname "$0")/../.."
# kind, kubectl and helm use a kubeconfig of their own, leaving ~/.kube/config untouched.
export KUBECONFIG=${SMOKE_KUBECONFIG:-$PWD/deploy/kind/.kubeconfig}
cluster=${KIND_CLUSTER:-mochi-smoke}
ctx="kind-$cluster"
ns=mqtt
release=smoke
sts="$release-mochi-cluster"
replicas=3
k() { kubectl --context "$ctx" -n "$ns" "$@"; }

bin=$(mktemp -d)
pids=()
cleanup() {
	for p in ${pids[@]+"${pids[@]}"}; do kill "$p" 2>/dev/null || true; done
	if [[ "${KEEP:-}" != 1 ]]; then
		kind delete cluster --name "$cluster" >/dev/null 2>&1 || true
	fi
	rm -rf "$bin"
}
trap cleanup EXIT

dump() {
	echo "--- smoke test failed; cluster state:"
	k get pods -o wide || true
	for i in $(seq 0 $((replicas - 1))); do
		echo "--- logs $sts-$i"
		k logs "$sts-$i" --tail 80 || true
	done
}
trap 'dump' ERR

forward() { # pod local-port remote-port
	k port-forward "pod/$1" "$2:$3" >/dev/null 2>&1 &
	pids+=($!)
	for _ in $(seq 50); do
		if nc -z 127.0.0.1 "$2" 2>/dev/null; then return 0; fi
		sleep 0.2
	done
	echo "port-forward to $1:$3 did not open" >&2
	return 1
}

stop_forwards() {
	for p in ${pids[@]+"${pids[@]}"}; do kill "$p" 2>/dev/null || true; done
	pids=()
	sleep 1
}

round=0
check() { # label
	# each round uses new local ports, as an earlier port-forward may still hold its port
	round=$((round + 1))
	local admin=$((28000 + round * 10)) a=$((21800 + round * 10)) b=$((21801 + round * 10))
	echo "=== $1: membership"
	for i in $(seq 0 $((replicas - 1))); do
		forward "$sts-$i" $((admin + i)) 8080
		stats=$(curl -fsS "127.0.0.1:$((admin + i))/stats")
		members=$(grep -o '"Members":[0-9]*' <<<"$stats" | cut -d: -f2)
		leader=$(grep -o '"Leader":"[^"]*"' <<<"$stats" | cut -d'"' -f4)
		echo "$sts-$i: members=$members leader=$leader"
		if [[ "$members" != "$replicas" ]]; then
			echo "expected $replicas members on $sts-$i" >&2
			return 1
		fi
	done

	echo "=== $1: messages between nodes"
	forward "$sts-0" "$a" 1883
	forward "$sts-$((replicas - 1))" "$b" 1883
	"$bin/loadtest" -label "$1 cross node qos1" -pub "127.0.0.1:$a" -sub "127.0.0.1:$b" \
		-qos 1 -messages 2000 -publishers 2 -subscribers 2 -username smoke -password smoke-password
	"$bin/loadtest" -label "$1 connect" -pub "127.0.0.1:$a,127.0.0.1:$b" -connections 200 \
		-username smoke -password smoke-password

	echo "=== $1: unauthenticated client is refused"
	if "$bin/loadtest" -pub "127.0.0.1:$a" -connections 1 -username smoke -password wrong >/dev/null 2>&1; then
		echo "a client with the wrong password connected" >&2
		return 1
	fi
	stop_forwards
}

go build -o "$bin/loadtest" ./examples/benchmark/loadtest

if [[ "${SKIP_BUILD:-}" != 1 ]]; then
	docker build -f deploy/docker/Dockerfile.cluster --build-arg VERSION=ci -t mochi-cluster:ci .
fi

if ! kind get clusters | grep -qx "$cluster"; then
	kind create cluster --name "$cluster" --wait 2m
fi
kind load docker-image mochi-cluster:ci --name "$cluster"

helm upgrade --install "$release" deploy/helm/mochi-cluster --kube-context "$ctx" -n "$ns" --create-namespace \
	-f deploy/helm/mochi-cluster/ci/kind-values.yaml --set replicaCount=$replicas --wait --timeout 5m
k rollout status "statefulset/$sts" --timeout 5m

check "install"

echo "=== rolling restart"
k rollout restart "statefulset/$sts"
k rollout status "statefulset/$sts" --timeout 5m
check "after rolling restart"

echo "=== smoke test passed"
