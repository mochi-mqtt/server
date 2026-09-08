package mqtt

import (
	"sync"
	"testing"
	"time"
)

// TestClientsGetByListenerDoesNotRecurseRLock pins the MaestroHub deadlock
// patch in GetByListener.
//
// Before the patch its capacity hint called cl.Len(), taking RLock a second
// time while already holding it. sync.RWMutex forbids recursive read locking
// precisely because a Lock() queued in between blocks the nested RLock, while
// the writer waits on the read lock the caller still holds — a permanent
// deadlock, not a slow path.
//
// The interleaving is reachable in production: a client connecting
// (attachClient -> Clients.Delete -> Lock) as a listener closes
// (closeListenerClients -> GetByListener). It hung two MaestroHub CI packages
// for the full 30-minute test timeout.
//
// The test drives readers and writers concurrently and requires completion
// inside a deadline. Without the patch it does not fail an assertion — it
// hangs, which is why the deadline is the assertion.
func TestClientsGetByListenerDoesNotRecurseRLock(t *testing.T) {
	cl := NewClients()
	for i := 0; i < 64; i++ {
		id := string(rune('a'+i%26)) + string(rune('0'+i/26))
		cl.Add(&Client{ID: id, Net: ClientConnection{Listener: "tcp1"}})
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() { // readers: the nested-RLock path
				defer wg.Done()
				for n := 0; n < 2000; n++ {
					_ = cl.GetByListener("tcp1")
				}
			}()
		}
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func(i int) { // writers: queue Lock() between the two RLocks
				defer wg.Done()
				id := string(rune('a' + i))
				for n := 0; n < 2000; n++ {
					cl.Add(&Client{ID: id, Net: ClientConnection{Listener: "tcp1"}})
					cl.Delete(id)
				}
			}(i)
		}
		wg.Wait()
	}()

	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("GetByListener deadlocked against a concurrent Delete — the capacity hint is " +
			"taking RLock recursively while a writer is queued (MaestroHub patch reverted?)")
	}
}
