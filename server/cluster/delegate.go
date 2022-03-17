package cluster

import (
	"encoding/json"
	"github.com/hashicorp/memberlist"
	mqtt "github.com/mochi-co/mqtt/server"
	"github.com/mochi-co/mqtt/server/system"
	"sync"
)

type Delegate struct {
	sync.RWMutex
	State      map[string]*system.Info
	MsgChan    chan []byte
	Broadcasts *memberlist.TransmitLimitedQueue
	Cluster    *Cluster
	Server     *mqtt.Server
}

func NewDelegate() *Delegate {
	return &Delegate{
		MsgChan: make(chan []byte, 1024),
		State: make(map[string]*system.Info, 2),
	}
}

func (d *Delegate) NotifyMsg(msg []byte) {
	d.MsgChan <- msg
}

func (d *Delegate) NodeMeta(limit int) []byte {
	return []byte{}
}

func (d *Delegate) LocalState(join bool) []byte {
	if d.Server != nil {
		d.State[d.Cluster.LocalNode().Name] = d.Server.System
	}
	bs, _ := json.Marshal(d.State)
	return bs
}

func (d *Delegate) GetBroadcasts(overhead, limit int) [][]byte {
	return d.Broadcasts.GetBroadcasts(overhead, limit)
}

func (d *Delegate) MergeRemoteState(buf []byte, join bool) {
	//fmt.Println("MergeRemoteState ", string(buf))
	var m map[string]*system.Info
	if err := json.Unmarshal(buf, &m); err != nil {
		return
	}
	d.Lock()
	for k, v := range m {
		d.State[k] = v
	}
	d.Unlock()
}

func (d *Delegate) SetCluster(cluster *Cluster) {
	d.Cluster = cluster
	d.Broadcasts = &memberlist.TransmitLimitedQueue{
		NumNodes: func() int {
			return d.Cluster.List.NumMembers()
		},
		RetransmitMult: 3,
	}
}

func (d *Delegate) SetMqttServer(server *mqtt.Server) {
	d.Server = server
}

//Broadcast broadcast to everyone including yourself
func (d *Delegate) Broadcast(data []byte) {
	d.Broadcasts.QueueBroadcast(&Broadcast{
		msg:    data,
		notify: nil,
	})
}

//BroadcastExceptSelf send message to all nodes except yourself
func (d *Delegate) BroadcastExceptSelf(data []byte) {
	d.Cluster.Broadcast(data)
}

type Broadcast struct {
	msg    []byte
	notify chan<- struct{}
}

func (b *Broadcast) Invalidates(other memberlist.Broadcast) bool {
	return false
}

func (b *Broadcast) Message() []byte {
	return b.msg
}

func (b *Broadcast) Finished() {
	if b.notify != nil {
		close(b.notify)
	}
}
