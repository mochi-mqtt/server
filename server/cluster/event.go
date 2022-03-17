package cluster

import (
	"fmt"
	"github.com/hashicorp/memberlist"
)

type NodeEvents struct{}

func NewEvents() *NodeEvents {
	return &NodeEvents{}
}

func (n *NodeEvents) NotifyJoin(node *memberlist.Node) {
	fmt.Println("A node has joined: " + node.String())
}

func (n *NodeEvents) NotifyLeave(node *memberlist.Node) {
	fmt.Println("A node has left: " + node.String())
}

func (n *NodeEvents) NotifyUpdate(node *memberlist.Node) {
	fmt.Println("A node was updated: " + node.String())
}
