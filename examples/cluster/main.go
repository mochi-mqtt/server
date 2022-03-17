package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/logrusorgru/aurora"
	mqtt "github.com/mochi-co/mqtt/server"
	"github.com/mochi-co/mqtt/server/cluster"
	red "github.com/mochi-co/mqtt/server/cluster/persistence/redis"
	"github.com/mochi-co/mqtt/server/events"
	"github.com/mochi-co/mqtt/server/listeners"
	"github.com/mochi-co/mqtt/server/packets"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
)

var server *mqtt.Server
var delegate *cluster.Delegate

func main() {
	tcpAddr   := flag.String("tcp", ":1883", "network address for TCP listener")
	wsAddr    := flag.String("ws", ":1882", "network address for Websocket listener")
	infoAddr  := flag.String("info", ":8080", "network address for web info dashboard listener")
	mode      := flag.Bool("mode", false, "optional value true or false for cluster mode")
	redisAddr := flag.String("redis", "localhost:6379", "redis address for cluster mode")
	bindPort  := flag.Int("port", 0, "listening port for cluster node,if this parameter is not set,then port is dynamically bound")
	members   := flag.String("members", "", "seeds member list of cluster,such as 192.168.0.103:7946,192.168.0.104:7946")

	flag.Parse()

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		done <- true
	}()

	fmt.Println(aurora.Magenta("Mochi MQTT Broker initializing..."))
	fmt.Println(aurora.Cyan("TCP"), *tcpAddr)
	fmt.Println(aurora.Cyan("Websocket"), *wsAddr)
	fmt.Println(aurora.Cyan("$SYS Dashboard"), *infoAddr)

	server = mqtt.New()
	if *mode {
		server.Cluster = true
		server.Events.OnMessage = OnMessage
		server.Events.OnConnect = OnConnect

		store := red.New(&redis.Options{
			Addr:     *redisAddr,
			Password: "", // no password set
			DB:       0,  // use default DB
		})
		err := server.AddStore(store)
		if err != nil {
			log.Fatal(err)
		}
	}

	tcp := listeners.NewTCP("t1", *tcpAddr)
	err := server.AddListener(tcp, nil)
	if err != nil {
		log.Fatal(err)
	}

	ws := listeners.NewWebsocket("ws1", *wsAddr)
	err = server.AddListener(ws, nil)
	if err != nil {
		log.Fatal(err)
	}

	stats := listeners.NewHTTPStats("stats", *infoAddr)
	err = server.AddListener(stats, nil)
	if err != nil {
		log.Fatal(err)
	}

	go server.Serve()
	fmt.Println(aurora.BgMagenta("Mqtt Server Started!  "))

	if *mode {
		delegate, _ = cluster.LaunchNode(cluster.GenNodeName(), *bindPort, *members)
		delegate.SetMqttServer(server)
		go ProcessInboundMsg(delegate.MsgChan)
		fmt.Println(aurora.BgMagenta("Cluster Node Created! "))
	}

	<-done
	fmt.Println(aurora.BgRed("  Caught Signal  "))

	//server.Close()
	fmt.Println(aurora.BgGreen("  Finished  "))
}

//ProcessInboundMsg process messages from other nodes in the cluster
func ProcessInboundMsg(ch chan []byte) {
	for {
		select {
		case bs := <-ch:
			var msg cluster.Message
			msg.Load(bs)
			//fmt.Println("Inbound: ", msg)
			switch msg.Type {
			case packets.Publish:
				pk := &packets.Packet{FixedHeader: packets.FixedHeader{Type: packets.Publish}}
				pk.FixedHeader.Decode(msg.Data[0])  // Unpack fixedheader.
				err := pk.PublishDecode(msg.Data[2:]) // Unpack skips fixedheader.
				if err != nil {
					break
				}
				server.Publish(pk.TopicName, pk.Payload, false)
			case packets.Connect:
				//If a client is connected to another node, the client's data cached on the node needs to be cleared
				cid := string(msg.Data)
				if existing, ok := server.Clients.Get(cid); ok {
					existing.Lock()
					existing.Stop()
					for k := range existing.Subscriptions {
						q := server.Topics.Unsubscribe(k, existing.ID)
						if q {
							atomic.AddInt64(&server.System.Subscriptions, -1)
						}
					}
					server.Clients.Delete(cid)
					existing.Unlock()
				}

			}
		}
	}
}

func OnMessage(cl events.Client, pk packets.Packet) (pkx packets.Packet, err error) {
	pkx = pk
	switch pk.FixedHeader.Type {
	case packets.Publish:
		var buf bytes.Buffer
		pk.PublishEncode(&buf)
		msg := cluster.Message{
			Type: packets.Publish,
			Data: buf.Bytes(),
		}
		delegate.BroadcastExceptSelf(msg.Bytes())
	}

	return pkx, nil
}

func OnConnect(cl events.Client, pk packets.Packet)  {
	var buf bytes.Buffer
	pk.PublishEncode(&buf)
	msg := cluster.Message{
		Type: packets.Connect,
		Data: []byte(cl.ID),
	}

	delegate.BroadcastExceptSelf(msg.Bytes())
}
