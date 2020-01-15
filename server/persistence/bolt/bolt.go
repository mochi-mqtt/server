package bolt

import (
	//"encoding/gob"
	"time"

	sgob "github.com/asdine/storm/codec/gob"
	"github.com/asdine/storm/v3"
	"go.etcd.io/bbolt"

	"github.com/mochi-co/mqtt/server/persistence"
)

const (
	defaultPath    = "mochi.db"
	defaultTimeout = 250 * time.Millisecond
)

// Store is a backend for writing and reading to bolt persistent storage.
type Store struct {
	path string         // the path on which to store the db file.
	opts *bbolt.Options // options for configuring the boltdb instance.
	db   *storm.DB      // the boltdb instance.
}

// init registers storage structs in gob.
func init() {
	//gob.Register(map[string]interface{}{})
}

// New returns a configured instance of the boltdb store.
func New(path string, opts *bbolt.Options) *Store {
	if path == "" || path == "." {
		path = defaultPath
	}

	if opts == nil {
		opts = &bbolt.Options{
			Timeout: defaultTimeout,
		}
	}

	return &Store{
		path: path,
		opts: opts,
	}
}

// Open opens the boltdb instance.
func (s *Store) Open() error {
	var err error
	s.db, err = storm.Open(s.path, storm.BoltOptions(0600, s.opts), storm.Codec(sgob.Codec))
	if err != nil {
		return err
	}

	return nil
}

// Close closes the boltdb instance.
func (s *Store) Close() {
	s.db.Close()
}

// StoreSubscriptions writes all subscriptions to the boltdb instance as
// a bulk operation.
func (s *Store) StoreSubscriptions() {

}

// StoreClients writes all clients to the boltdb instance as a bulk operation.
func (s *Store) StoreClients() {

}

// StoreInflight writes all inflight messages to the boltdb instance as a bulk operation.
func (s *Store) StoreInflight() {

}

// StoreInflight writes all inflight messages to the boltdb instance as a bulk operation.
func (s *Store) StoreRetained() {

}

// StoreServerInfo writes the server info to the boltdb instance.
func (s *Store) StoreServerInfo(v persistence.ServerInfo) error {
	err := s.db.Save(&v)
	if err != nil {
		return err
	}
	return nil
}

// WriteSubscription writes a single subscription to the boltdb instance.
func (s *Store) WriteSubscription(v persistence.Subscription) error {
	err := s.db.Save(&v)
	if err != nil {
		return err
	}
	return nil
}

// WriteInflight writes a single inflight message to the boltdb instance.
func (s *Store) WriteInflight() {

}

// WriteClient writes a single client to the boltdb instance.
func (s *Store) WriteClient() {

}

// LoadSubscriptions loads all the subscriptions from the boltdb instance.
func (s *Store) LoadSubscriptions() (v []persistence.Subscription, err error) {
	err = s.db.Find("T", "subscription", &v)
	if err != nil {
		return
	}
	return
}

// LoadClients loads all the clients from the boltdb instance.
func (s *Store) LoadClients() {

}

// LoadInflight loads all the inflight messages from the boltdb instance.
func (s *Store) LoadInflight() {

}

// LoadServerInfo loads the server info from the boltdb instance.
func (s *Store) LoadServerInfo() (v persistence.ServerInfo, err error) {
	err = s.db.One("ID", "server_info", &v)
	if err != nil {
		return
	}
	return
}
