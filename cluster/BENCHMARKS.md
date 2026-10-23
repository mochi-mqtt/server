# Cluster benchmarks

Measured on an Apple M4 Pro (14 cores, 48GB), macOS 26.7, Go 1.27, with every broker and
client on the same machine over loopback. They show the relative cost of clustering; absolute
numbers depend heavily on the machine and network. Reproduce them with:

```bash
MQTT_STRESSER=$(go env GOPATH)/bin/mqtt-stresser ./examples/benchmark/cluster-bench.sh
```

Each configuration runs [examples/benchmark](../examples/benchmark) with the same settings:
standalone, as a one node cluster, and as a three node cluster. `same node` publishes and
subscribes on one node of the three; `cross node` publishes on one node and subscribes on
another, so every message is forwarded between nodes; `fan-out` subscribes on both other nodes.

## Latency at a steady rate

[loadtest](../examples/benchmark/loadtest), 4 publishers at 5,000 msg/s each, 2 subscribers each
receiving every message (40,000 deliveries a second), 64 byte payloads. The load tool flushes
writes every millisecond, which accounts for about half a millisecond of these latencies.

| Configuration | QoS | Delivered | p50 | p95 | p99 |
|---|---|---|---|---|---|
| standalone | 0 | 100% | 585µs | 1.04ms | 1.15ms |
| one node cluster | 0 | 100% | 564µs | 1.02ms | 1.11ms |
| three nodes, same node | 0 | 100% | 578µs | 1.05ms | 1.14ms |
| three nodes, cross node | 0 | 100% | 701µs | 1.19ms | 1.27ms |
| three nodes, fan-out to two nodes | 0 | 100% | 660µs | 1.16ms | 1.24ms |
| standalone | 1 | 100% | 702µs | 1.15ms | 5.25ms |
| one node cluster | 1 | 100% | 702µs | 1.19ms | 4.11ms |
| three nodes, same node | 1 | 100% | 615µs | 1.16ms | 5.11ms |
| three nodes, cross node | 1 | 100% | 655µs | 1.15ms | 4.75ms |

Forwarding a message to another node adds no measurable latency on loopback; across a real
network, add one network hop.

## Throughput

loadtest as fast as possible: 4 publishers sending 25,000 messages each (10,000 at QoS 1), 2
subscribers. At QoS 0 the publishers outrun the subscribers, so the broker drops messages for
them (as QoS 0 allows) and latency is mostly queueing; QoS 1 delivers everything.

| Configuration | QoS | Publish msg/s | Receive msg/s | Delivered |
|---|---|---|---|---|
| standalone | 0 | 1,008,263 | 863,190 | 75% |
| one node cluster | 0 | 2,865,706 | 851,091 | 79% |
| three nodes, same node | 0 | 940,360 | 822,378 | 77% |
| three nodes, cross node | 0 | 2,215,412 | 791,847 | 100% |
| three nodes, fan-out | 0 | 1,991,321 | 909,968 | 100% |
| three nodes, all-to-all (3 publishers, 3 subscribers on every node) | 0 | 1,198,909 | 834,303 | 96% |
| standalone | 1 | 45,133 | 90,321 | 100% |
| one node cluster | 1 | 45,344 | 90,749 | 100% |
| three nodes, same node | 1 | 51,215 | 102,480 | 100% |
| three nodes, cross node | 1 | 45,494 | 90,409 | 100% |

Receive throughput is about the same in every configuration, so clustering costs nothing
measurable here. Cross node, the subscribers' node delivers forwarded messages on its own
goroutines, which spreads the work, so fewer messages are dropped. QoS 1 throughput is
limited by the load tool's window of 256 unacknowledged publishes per publisher.

### mqtt-stresser

As in the main README: `mqtt-stresser -broker tcp://localhost:1883 -num-messages=10000`, median
msg/s. Its clients publish and subscribe on the same broker, so for the three node cluster this
measures one node of a cluster, not forwarding.

| Configuration | 2 clients publish | 2 clients receive | 10 clients publish | 10 clients receive |
|---|---|---|---|---|
| standalone | 31,188 | 15,078 | 7,448 | 122,410 |
| one node cluster | 31,246 | 16,209 | 7,692 | 84,632 |
| three node cluster | 33,618 | 17,633 | 13,004 | 48,316 |

mqtt-stresser's results vary a lot between runs (see the note in the main README), so treat
these as showing the cluster hook costs nothing measurable on a node's local traffic.

## QoS 1 delivery in the server core

Benchmarking the cluster showed that QoS 1 delivery in the server itself was slow, and could
stall, so it was fixed (standalone server, 4 publishers, 2 subscribers unless noted):

| Scenario | Before | After |
|---|---|---|
| QoS 1 as fast as possible | 28k msg/s received, p50 75ms | 90k msg/s received, p50 20ms |
| QoS 1 at 20k msg/s | couldn't keep up (14k msg/s), p50 79ms | 20k msg/s, p50 0.7ms |
| 8 publishers, 1 subscriber, default 8,192 inflight | stalled after 12% | 100%, 172k msg/s |
| Finding the next waiting message, 8,192 inflight | 204µs | 45ns |

- A QoS 1 message sent after waiting for send quota was removed from the client's inflight
  messages, so its acknowledgement never returned the quota: each one leaked a unit, until the
  client stalled for good, and the message wasn't resent if the client reconnected.
- Finding the next waiting message copied and sorted every inflight message, after every
  packet from the client.
- Waiting messages lost their expiry, and messages were dropped (and rolled back without
  telling storage hooks) when a client's write queue was full. They now wait, in order.
- A recursive read lock in `Inflight.NextImmediate` could deadlock the client.
- Every acknowledgement was written with its own system call. Replies are now held while more
  packets from the client are already buffered, and written together.

## Connections

loadtest connecting 5,000 clients, 16 at a time. Every connection in a cluster claims its
session in the raft registry; through a follower, the claim is forwarded to the leader.

| Configuration | Connects/s | p50 | p95 | p99 |
|---|---|---|---|---|
| standalone | 1,917 | 5.2ms | 14.6ms | 103ms |
| one node cluster | 2,004 | 4.9ms | 13.6ms | 108ms |
| three nodes, on the leader | 2,181 | 5.4ms | 15.0ms | 53.4ms |
| three nodes, on followers | 1,811 | 8.1ms | 14.2ms | 18.8ms |

The registry claim adds about 2ms per connection on a follower, and doesn't limit the
connection rate on this machine.

## Micro-benchmarks

`go test ./cluster -run '^$' -bench . -benchmem`

| Benchmark | ns/op | B/op | allocs/op |
|---|---|---|---|
| EncodePublish (256 byte payload, into a frame) | 694 | 768 | 3 |
| DecodePublish | 126 | 144 | 4 |
| RouteMatch (3,000 remote filters) | 1,032 | 2,464 | 14 |
| Forward (route and queue for three peers) | 2,547 | 3,233 | 17 |
| LocalRoutes (add and remove) | 133 | 258 | 2 |
| RetainedApply | 237 | 22 | 1 |
