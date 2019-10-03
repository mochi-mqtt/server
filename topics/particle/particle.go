package particle

/*
import (
	//"fmt"
	//"log"
	"strings"
	"sync"

	"github.com/mochi-co/mqtt/packets"
)

type potential struct {
	qos      byte
	wildhash bool
	matched  int
}

// Index is a subscriptions and messages index which stores topics
// as bucketed particles.
type Index struct {
	sync.RWMutex

	// buckets is a slice of buckets which will contain the particles.
	buckets []particles
}

// particles is a map of particle, keyed on topic/filter fragment.
type particles map[string]*particle

// particle contains information about the subscribers who have the fragment in
// one or more of their filter paths, and any retained message.
type particle struct {

	// retained a message that has been retained for the topic.
	retained *packets.PublishPacket

	// subscribers is a map of the subscribers which have subscribed to the particle.
	subscribers map[string]subscriber
}

// subscriber is a map of a client's subscribed filters, keyed on client path.
type subscriber map[string]subscription

// subscription contains information about a filter subscription for a user.
type subscription struct {

	// qos is the quality of service byte the client subscribed with.
	qos byte

	// end indicates that the particle subscription was the last in the path.
	end bool
}

// New returns a pointer to a new instance of Index, and
// takes a size of buckets to pre-initialize.
func New(size int) *Index {
	return &Index{
		buckets: newParticleBuckets(size),
	}
}

// newParticleBuckets returns a slice of populated particles.
func newParticleBuckets(size int) []particles {
	buckets := make([]particles, size)
	for i := 0; i < len(buckets); i++ {
		buckets[i] = make(particles)
	}
	return buckets
}

// Subscribe adds a new filter subscription for a client.
func (x *Index) Subscribe(filter, client string, qos byte) {
	parts := strings.Split(filter, "/")
	ix := len(parts)

	x.Lock()
	defer x.Unlock()
	for i := 0; i < ix; i++ {

		// Ensure there are enough buckets and particle keys.
		if len(x.buckets) < ix {
			x.buckets = append(x.buckets, make(particles))
		}

		if _, ok := x.buckets[i][parts[i]]; !ok {
			x.buckets[i][parts[i]] = &particle{
				subscribers: make(map[string]subscriber),
			}
		}

		// Ensure there is a key for the subscribed client.
		if _, ok := x.buckets[i][parts[i]].subscribers[client]; !ok {
			x.buckets[i][parts[i]].subscribers[client] = make(subscriber)
		}

		// Overwrite any existing filter with new qos.
		x.buckets[i][parts[i]].subscribers[client][filter] = subscription{
			qos: qos,
			end: i == ix-1,
		}
	}
}

// Unsubscribe removes new filter subscription for a client.
func (x *Index) Unsubscribe(filter, client string) {
	parts := strings.Split(filter, "/")
	ix := len(parts)

	x.Lock()
	defer x.Unlock()
	if len(x.buckets) < ix {
		return
	}

	for i := 0; i < ix; i++ {
		if _, ok := x.buckets[i][parts[i]]; !ok {
			break
		}

		if subscription, ok := x.buckets[i][parts[i]].subscribers[client]; ok {
			delete(subscription, filter)

			if len(x.buckets[i][parts[i]].subscribers[client]) == 0 {
				delete(x.buckets[i][parts[i]].subscribers, client)
			}
		}
	}
}

// Subscribers
func (x *Index) Subscribers(topic string) {
	parts := strings.Split(topic, "/")
	ix := len(parts)

	x.RLock()
	defer x.Unlock()
	if len(x.buckets) < ix {
		return
	}

}

/*



// Subscribers returns the clients with filters matching a topic.
func (x *ParticleIndex) Subscribers(topic string) {
	parts := strings.Split(topic, "/")

	x.RLock()
	if len(x.buckets) < len(parts) {
		return
	}

	viable := make(map[string]map[string]*potential)

	for i := 0; i < len(parts); i++ {
		var any bool
		for _, k := range []string{parts[i], "+", "#"} {
			fmt.Println(i, k)
			if _, ok := x.buckets[i][k]; !ok {
				continue
			}
			any = true

			for client, subscriptions := range x.buckets[i][k].partial {

				// Only add clients which exist at the starting filter particle 0.
				if _, ok := viable[client]; !ok && i == 0 {
					viable[client] = make(map[string]*potential)
				}

				if _, ok := viable[client]; !ok {
					continue
				}

				for filter, sm := range subscriptions.filters {
					fmt.Println("\t-- ", i, k, client, filter, sm, viable[client][filter])

					if _, ok := viable[client][filter]; !ok {
						fmt.Println("SETTING", client, filter)
						viable[client][filter] = new(potential)
					}

					// Increment the matched count - know how many particles in the filter.
					// matched at the end.
					viable[client][filter].matched += 1

					// Add the QoS for the filter.
					viable[client][filter].qos = sm.qos

					// If the particle is a wildhash, indicate that for final assessment.
					if k == "#" {
						viable[client][filter].wildhash = true
					}

					fmt.Println("\t\t", viable[client][filter])
				}
			}

		}

		if !any {
			break
		}
	}
	x.RUnlock()

	for k, v := range viable {
		for kk, vv := range v {
			if vv.matched == len(parts) || vv.wildhash {
				fmt.Println("=", k, kk, vv, len(parts), vv.matched == len(parts))
			}
		}
	}

}

// Messages returns any message payloads retained for topics matching a filter.
func (x *ParticleIndex) Messages() {
	log.Println()

}

// Retain retains a message payload for a topic.
func (x *ParticleIndex) Retain() {

}

/*
// subscriptions contains the details of a client's (matching) subscriptions.
type subscriptions struct {

	// filters is a map of topic filters that were used to subscribe to the path.
	filters map[string]*submeta
}

// submeta
type submeta struct {
	qos byte
	end bool
}
*/
