package particle
/*
import (
	"fmt"
	"log"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	index := New(32)
	require.NotNil(t, index)
	require.NotNil(t, index.buckets)
	require.NotNil(t, index.buckets[0])
	require.NotNil(t, index.buckets[31])
}

func BenchmarkNew(b *testing.B) {
	for n := 0; n < b.N; n++ {
		New(32)
	}
}

func TestNewParticleBuckets(t *testing.T) {
	index := newParticleBuckets(32)
	require.NotNil(t, index)
	require.NotNil(t, index[0])
	require.NotNil(t, index[31])
}

func BenchmarkNewParticleBuckets(b *testing.B) {
	for n := 0; n < b.N; n++ {
		newParticleBuckets(32)
	}
}

func TestSubscribe(t *testing.T) {
	index := New(2)
	index.Subscribe("path/to/mqtt/basic", "client-1", 0)

	require.NotNil(t, index.buckets[0])
	require.NotNil(t, index.buckets[0]["path"])
	require.NotNil(t, index.buckets[0]["path"].subscribers["client-1"])

	for k, v := range index.buckets {
		for kk, vv := range v {
			for h, y := range vv.subscribers {
				log.Println(k, kk, h, y)
			}
		}
	}
}

func BenchmarkSubscribe(b *testing.B) {
	index := New(2)
	for n := 0; n < b.N; n++ {
		index.Subscribe("path/to/mqtt/basic", "client-1", 0)
	}
}

func TestUnsubscribe(t *testing.T) {
	index := New(2)
	index.Subscribe("path/to/mqtt/basic", "client-1", 0)
	index.Subscribe("path/to/mqtt/basic", "client-2", 1)
	index.Subscribe("path/to/mqtt/secondary", "client-1", 0)

	require.NotNil(t, index.buckets[3])
	require.NotNil(t, index.buckets[3]["basic"])
	require.NotNil(t, index.buckets[3]["basic"].subscribers["client-1"])
	require.NotNil(t, index.buckets[3]["basic"].subscribers["client-1"]["path/to/mqtt/basic"])
	require.NotNil(t, index.buckets[3]["basic"].subscribers["client-2"]["path/to/mqtt/basic"])
	require.NotNil(t, index.buckets[3]["secondary"].subscribers["client-1"]["path/to/mqtt/secondary"])

	require.Equal(t, 2, len(index.buckets[3]["basic"].subscribers))
	require.Equal(t, 1, len(index.buckets[3]["secondary"].subscribers))

	index.Unsubscribe("path/to/mqtt/basic", "client-1")
	require.Equal(t, 1, len(index.buckets[3]["basic"].subscribers))
	require.Equal(t, 1, len(index.buckets[3]["secondary"].subscribers))

	index.Unsubscribe("path/to/mqtt/secondary", "client-1")
	require.Equal(t, 1, len(index.buckets[3]["basic"].subscribers))
	require.Equal(t, 0, len(index.buckets[3]["secondary"].subscribers))

	for k, v := range index.buckets {
		fmt.Println(k)
		for kk, vv := range v {
			fmt.Println("  ", kk)
			for h, y := range vv.subscribers {
				fmt.Println("\t", h, y)
			}
		}
	}
}

func BenchmarkUnsubscribe(b *testing.B) {
	index := New(2)
	index.Subscribe("path/to/mqtt/basic", "client-1", 0)

	for n := 0; n < b.N; n++ {
		index.buckets[0]["path"].subscribers["client-1"]["path/to/mqtt/basic"] = subscription{}
		index.buckets[1]["to"].subscribers["client-1"]["path/to/mqtt/basic"] = subscription{}
		index.buckets[2]["mqtt"].subscribers["client-1"]["path/to/mqtt/basic"] = subscription{}
		index.buckets[3]["basic"].subscribers["client-1"]["path/to/mqtt/basic"] = subscription{}

		index.Unsubscribe("path/to/mqtt/basic", "client-2")
	}
}

/*


func BenchmarkParticleIndexSubscribe(b *testing.B) {
	index := NewParticleIndex(2)
	for n := 0; n < b.N; n++ {
		index.Subscribe("path/to/mqtt/basic", "client-1", 0)
	}
}

func TestParticleIndexUnsubscribe(t *testing.T) {
	index := NewParticleIndex(2)
	index.Subscribe("path/to/mqtt/basic", "client-1", 0)
	index.Subscribe("path/to/mqtt/basic", "client-2", 1)
	index.Subscribe("path/to/mqtt/secondary", "client-1", 0)

	require.NotNil(t, index.buckets[3])
	require.NotNil(t, index.buckets[3]["basic"])
	require.NotNil(t, index.buckets[3]["basic"].partial["client-1"])
	require.NotNil(t, index.buckets[3]["basic"].partial["client-1"].filters["path/to/mqtt/basic"])
	require.NotNil(t, index.buckets[3]["basic"].partial["client-2"].filters["path/to/mqtt/basic"])
	require.NotNil(t, index.buckets[3]["secondary"].partial["client-1"].filters["path/to/mqtt/secondary"])

	require.Equal(t, 2, len(index.buckets[3]["basic"].partial))
	require.Equal(t, 1, len(index.buckets[3]["secondary"].partial))

	index.Unsubscribe("path/to/mqtt/basic", "client-1")
	require.Equal(t, 1, len(index.buckets[3]["basic"].partial))
	require.Equal(t, 1, len(index.buckets[3]["secondary"].partial))

	index.Unsubscribe("path/to/mqtt/secondary", "client-1")
	require.Equal(t, 1, len(index.buckets[3]["basic"].partial))
	require.Equal(t, 0, len(index.buckets[3]["secondary"].partial))

	for k, v := range index.buckets {
		fmt.Println(k)
		for kk, vv := range v {
			fmt.Println("  ", kk)
			for h, y := range vv.partial {
				fmt.Println("\t", h, *y)
			}
		}
	}
}

func BenchmarkParticleIndexUnsubscribe(b *testing.B) {
	index := NewParticleIndex(2)
	index.Subscribe("path/to/mqtt/basic", "client-1", 0)

	for n := 0; n < b.N; n++ {
		//	index.buckets[0]["path"].partial["client-1"].filters["path/to/mqtt/basic"] = 0
		//	index.buckets[1]["to"].partial["client-1"].filters["path/to/mqtt/basic"] = 0
		//	index.buckets[2]["mqtt"].partial["client-1"].filters["path/to/mqtt/basic"] = 0
		//	index.buckets[3]["basic"].partial["client-1"].filters["path/to/mqtt/basic"] = 0

		index.Unsubscribe("path/to/mqtt/basic", "client-2")
	}
}

func TestParticleIndexSubscribers(t *testing.T) {
	index := NewParticleIndex(16)
	index.Subscribe("path/to/mqtt/basic", "client-1", 0)
	index.Subscribe("+/to/mqtt/basic", "client-1", 2)
	index.Subscribe("path/to/#", "client-2", 0)
	index.Subscribe("path/to/mqtt/secondary", "client-3", 0)
	index.Subscribe("path/to/stuff/basic", "client-3", 0)
	index.Subscribe("+/+/+/+", "client-4", 0)
	index.Subscribe("#", "client-5", 0)

	index.Subscribers("stuff/to/basic")

}
*/
*/