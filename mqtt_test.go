package mqtt

import (
	"fmt"
	"io/ioutil"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/mochi-co/mqtt/internal/auth"
	//	"github.com/mochi-co/mqtt/internal/circ"
	//	"github.com/mochi-co/mqtt/internal/clients"
	"github.com/mochi-co/mqtt/internal/listeners"
	"github.com/mochi-co/mqtt/internal/packets"
)

/*
 * Server
 */

func TestNew(t *testing.T) {
	s := New()
	require.NotNil(t, s)
	require.NotNil(t, s.Listeners)
	require.NotNil(t, s.Clients)
	require.NotNil(t, s.Topics)
}

func BenchmarkNew(b *testing.B) {
	for n := 0; n < b.N; n++ {
		New()
	}
}

func TestServerAddListener(t *testing.T) {
	s := New()
	require.NotNil(t, s)

	err := s.AddListener(listeners.NewMockListener("t1", ":1882"), nil)
	require.NoError(t, err)

	// Add listener with config.
	err = s.AddListener(listeners.NewMockListener("t2", ":1882"), &listeners.Config{
		Auth: new(auth.Disallow),
	})
	require.NoError(t, err)
	l, ok := s.Listeners.Get("t2")
	require.Equal(t, true, ok)
	require.Equal(t, new(auth.Disallow), l.(*listeners.MockListener).Config.Auth)

	// Add listener on existing id
	err = s.AddListener(listeners.NewMockListener("t1", ":1883"), nil)
	require.Error(t, err)
	require.Equal(t, ErrListenerIDExists, err)
}

func BenchmarkServerAddListener(b *testing.B) {
	s := New()
	l := listeners.NewMockListener("t1", ":1882")
	for n := 0; n < b.N; n++ {
		err := s.AddListener(l, nil)
		if err != nil {
			panic(err)
		}
		s.Listeners.Delete("t1")
	}
}

func TestServerServe(t *testing.T) {
	s := New()
	require.NotNil(t, s)

	err := s.AddListener(listeners.NewMockListener("t1", ":1882"), nil)
	require.NoError(t, err)

	s.Serve()
	time.Sleep(time.Millisecond)
	require.Equal(t, 1, s.Listeners.Len())
	listener, ok := s.Listeners.Get("t1")
	require.Equal(t, true, ok)
	require.Equal(t, true, listener.(*listeners.MockListener).IsServing)
}

func BenchmarkServerServe(b *testing.B) {
	s := New()
	l := listeners.NewMockListener("t1", ":1882")
	err := s.AddListener(l, nil)
	if err != nil {
		panic(err)
	}
	for n := 0; n < b.N; n++ {
		s.Serve()
	}
}

/*

 * Server Establish Connection

 */
func TestServerEstablishConnectionOKCleanSession(t *testing.T) {
	s := New()

	/*
		c, _ := net.Pipe()
		cl := clients.NewClient(c, circ.NewReader(256, 8), circ.NewWriter(256, 8))
		cl.ID = "mochi2"
		cl.Subscriptions = map[string]byte{
			"a/b/c": 1,
		}
		s.Clients.Add(cl)
	*/

	r, w := net.Pipe()
	o := make(chan error)
	go func() {
		o <- s.EstablishConnection("tcp", r, new(auth.Allow))
	}()

	go func() {
		w.Write([]byte{
			byte(packets.Connect << 4), 17, // Fixed header
			0, 4, // Protocol Name - MSB+LSB
			'M', 'Q', 'T', 'T', // Protocol Name
			4,     // Protocol Version
			2,     // Packet Flags - clean session
			0, 45, // Keepalive
			0, 5, // Client ID - MSB+LSB
			'm', 'o', 'c', 'h', 'i', // Client ID
		})
		w.Write([]byte{byte(packets.Disconnect << 4), 0})
	}()

	// Receive the Connack
	recv := make(chan []byte)
	go func() {
		buf, err := ioutil.ReadAll(w)
		if err != nil {
			panic(err)
		}
		fmt.Println("<< ", buf)
		recv <- buf
	}()

	//	clw, ok := s.Clients.Get("mochi")
	//	require.Equal(t, true, ok)
	//	clw.Stop()

	errx := <-o
	require.NoError(t, errx)
	require.Equal(t, []byte{
		byte(packets.Connack << 4), 2,
		0, packets.Accepted,
	}, <-recv)

	//require.Empty(t, s.Clients.internal["mochi2"].Subscriptions)

	w.Close()

}
