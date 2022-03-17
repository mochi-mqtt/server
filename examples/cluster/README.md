# Cluster Example

A basic cluster example is implemented using [hashicorp/memberlist](https://github.com/hashicorp/memberlist)

Cluster uses redis to store inflight and retain messages and subscriptions across nodes.

## Build

```shell
go build -o broker examples/cluster/main.go
```

## Usage

```shell
./broker -h
Usage of ./broker:
  -info string
        network address for web info dashboard listener (default ":8080")
  -members string
        seeds member list of cluster,such as 192.168.0.103:7946,192.168.0.104:7946
  -mode
        optional value true or false for cluster mode
  -port int
        listening port for cluster node,if this parameter is not set,then port is dynamically bound
  -redis string
        redis address for cluster mode (default "localhost:6379")
  -tcp string
        network address for TCP listener (default ":1883")
  -ws string
        network address for Websocket listener (default ":1882")
```

### Create Cluster

*Start two nodes on one laptop*

Start first node
```shell
./broker --mode=true --port=7946
```

You should see the output
```
Mochi MQTT Broker initializing...
TCP :1883
Websocket :1882
$SYS Dashboard :8080
Mqtt Server Started!  
A node has joined: narwaldeMacBook-Pro.local--0dcbdda9-cf28-4e11-a527-28179eb176ad
Local member 192.168.0.103:7946
Cluster Node Created! 
```

Start second node with first node as part of the member list
```shell
./broker --mode=true --port=7947 --members=192.168.0.103:7946 --tcp=:1885 --ws=:1886 --info=:1887
```

You should see the output
```
Mochi MQTT Broker initializing...
TCP :1885
Websocket :1886
$SYS Dashboard :1887
Mqtt Server Started!  
A node has joined: narwaldeMacBook-Pro.local--18d26675-d04f-4114-bea2-163e7a68a219
2022/03/17 00:20:44 [DEBUG] memberlist: Initiating push/pull sync with:  192.168.0.103:7946
A node has joined: narwaldeMacBook-Pro.local--0dcbdda9-cf28-4e11-a527-28179eb176ad
Local member 192.168.0.103:7947
Cluster Node Created! 
```

First node output will log the new connection
```shell
2022/03/17 00:20:44 [DEBUG] memberlist: Stream connection from=192.168.0.103:49756
2022/03/17 00:20:45 [DEBUG] memberlist: Initiating push/pull sync with: narwaldeMacBook-Pro.local--18d26675-d04f-4114-bea2-163e7a68a219 192.168.0.103:7947
```

### Performance (messages/second)

```shell
mqtt-stresser -broker tcp://localhost:1883 -num-clients=500 -num-messages=100
```
Start two nodes on one laptop and test at the same time.

A laptop：MacBook Pro (13-inch, M1, 16G).

#### 1. First Node

*Publishing Throughput*

Fastest: 215575 msg/sec

Slowest: 7336 msg/sec

Median: 72832 msg/sec

*Receiving Througput*

Fastest: 72040 msg/sec

Slowest: 463 msg/sec

Median: 704 msg/sec

####2. Second Node

*Publishing Throughput*

Fastest: 208207 msg/sec

Slowest: 1009 msg/sec

Median: 16794 msg/sec

*Receiving Througput*

Fastest: 305033 msg/sec

Slowest: 512 msg/sec

Median: 960 msg/sec
