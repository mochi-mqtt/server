package cluster

import (
	"fmt"
	"strings"

	"github.com/hashicorp/memberlist"
)

type Cluster struct {
	Config   *memberlist.Config
	List     *memberlist.Memberlist
}

func DefaultLocalConfig() *memberlist.Config {
	return memberlist.DefaultLocalConfig()
	//return memberlist.DefaultLANConfig()
}

func LaunchNode(nodeName string, bindPort int, members string) (*Delegate, error) {
	msgDelegate := NewDelegate()
	eventDelegate := NewEvents()
	conf := DefaultLocalConfig()
	conf.Delegate = msgDelegate
	conf.Events = eventDelegate
	conf.BindPort = bindPort      //0 dynamically bind a port
	conf.Name = nodeName          //The name of this node. This must be unique in the cluster.
	cls, err := Create(conf)
	if err != nil {
		return nil, err
	}
	if len(members) > 0 {
		parts := strings.Split(members, ",")
		_, err := cls.Join(parts)
		if err != nil {
			return nil, err
		}
	}
	msgDelegate.SetCluster(cls)
	node := cls.LocalNode()
	fmt.Printf("Local member %s:%d\n", node.Addr, node.Port)
	return msgDelegate, nil
}

func Create(conf *memberlist.Config) (c *Cluster, err error) {
	if conf == nil {
		conf = memberlist.DefaultLocalConfig()
	}
	list, err := memberlist.Create(conf)
	if err != nil {
		return nil, err
	}

	c = &Cluster{conf, list}
	return c, nil
}

func (c *Cluster) Join(members []string) (int, error) {
	if len(members) > 0 {
		count, err := c.List.Join(members)
		return count, err
	}

	return 0, nil
}

func (c *Cluster) LocalNode() *memberlist.Node {
	return c.List.LocalNode()
}

func (c *Cluster) Members() []*memberlist.Node {
	return c.List.Members()
}

func (c *Cluster) NumMembers() int {
	return c.List.NumMembers()
}

func (c *Cluster) Send(to *memberlist.Node, msg []byte) error{
	//return c.List.SendReliable(to, msg)      //tcp reliable
	return c.List.SendBestEffort(to, msg)      //udp unreliable
}

//Broadcast send message to all nodes except yourself
func (c *Cluster) Broadcast(msg []byte) {
	for _, node := range c.Members() {
		if node.Name == c.Config.Name {
			continue // skip self
		}
		c.Send(node, msg)
	}
}
