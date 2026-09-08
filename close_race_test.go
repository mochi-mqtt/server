package mqtt

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/mochi-mqtt/server/v2/listeners"
)

// TestCloseWhileConnecting covers the shutdown accounting.
//
// attachClient registers each client with Listeners.ClientsWg, and
// CloseAll waits on it. sync.WaitGroup forbids a positive Add concurrent
// with Wait, so a connection accepted while the server is closing used to
// race the very wait that was meant to cover it — and Close could return
// while that client was still being attached.
//
// Run it under -race: without the fix the detector reports a write in
// Close against a read in EstablishConnection.
func TestCloseWhileConnecting(t *testing.T) {
	for i := 0; i < 40; i++ {
		s := New(nil)
		tcp := listeners.NewTCP(listeners.Config{ID: "t", Address: "127.0.0.1:0"})
		if err := s.AddListener(tcp); err != nil {
			t.Fatalf("add listener: %v", err)
		}
		go func() { _ = s.Serve() }()
		addr := tcp.Address()

		// Keep connecting until the server is closed, so that a connection
		// is always in flight when Close runs.
		stop := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				if c, err := net.Dial("tcp", addr); err == nil {
					_ = c.Close()
				}
			}
		}()

		time.Sleep(time.Millisecond)
		_ = s.Close()
		close(stop)
		wg.Wait()
	}
}
