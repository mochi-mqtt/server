package cluster

import (
	"github.com/hashicorp/memberlist"
	"sync"
)

type RoundRobinBalancer struct {
	sync.Mutex

	cluster *Cluster
	current int
	pool    []*memberlist.Node
}

func NewRoundRobinBalancer(cs *Cluster) *RoundRobinBalancer {
	return &RoundRobinBalancer{
		cluster: cs,
		current: 0,
		pool:    cs.List.Members(),
	}
}

func (r *RoundRobinBalancer) Get() *memberlist.Node {
	r.Lock()
	defer r.Unlock()

	r.pool = r.cluster.List.Members()
	if r.current >= len(r.pool) {
		r.current = r.current % len(r.pool)
	}

	result := r.pool[r.current]
	r.current++
	return result
}
