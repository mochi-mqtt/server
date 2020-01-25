package bolt

import (
	//"encoding/gob"
	"errors"
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

// WriteServerInfo writes the server info to the boltdb instance.
func (s *Store) WriteServerInfo(v persistence.ServerInfo) error {
	if s.db == nil {
		return errors.New("boltdb not opened")
	}

	err := s.db.Save(&v)
	if err != nil {
		return err
	}
	return nil
}

// WriteSubscription writes a single subscription to the boltdb instance.
func (s *Store) WriteSubscription(v persistence.Subscription) error {
	if s.db == nil {
		return errors.New("boltdb not opened")
	}

	err := s.db.Save(&v)
	if err != nil {
		return err
	}
	return nil
}

// WriteInflight writes a single inflight message to the boltdb instance.
func (s *Store) WriteInflight(v persistence.Message) error {
	if s.db == nil {
		return errors.New("boltdb not opened")
	}

	err := s.db.Save(&v)
	if err != nil {
		return err
	}
	return nil
}

// WriteRetained writes a single retained message to the boltdb instance.
func (s *Store) WriteRetained(v persistence.Message) error {
	if s.db == nil {
		return errors.New("boltdb not opened")
	}

	err := s.db.Save(&v)
	if err != nil {
		return err
	}
	return nil
}

// WriteClient writes a single client to the boltdb instance.
func (s *Store) WriteClient(v persistence.Client) error {
	if s.db == nil {
		return errors.New("boltdb not opened")
	}

	err := s.db.Save(&v)
	if err != nil {
		return err
	}
	return nil
}

// ReadSubscriptions loads all the subscriptions from the boltdb instance.
func (s *Store) ReadSubscriptions() (v []persistence.Subscription, err error) {
	if s.db == nil {
		err = errors.New("boltdb not opened")
		return
	}

	err = s.db.Find("T", persistence.KSubscription, &v)
	if err != nil {
		return
	}
	return
}

// ReadClients loads all the clients from the boltdb instance.
func (s *Store) ReadClients() (v []persistence.Client, err error) {
	if s.db == nil {
		err = errors.New("boltdb not opened")
		return
	}

	err = s.db.Find("T", persistence.KClient, &v)
	if err != nil {
		return
	}
	return
}

// ReadInflight loads all the inflight messages from the boltdb instance.
func (s *Store) ReadInflight() (v []persistence.Message, err error) {
	if s.db == nil {
		err = errors.New("boltdb not opened")
		return
	}

	err = s.db.Find("T", persistence.KInflight, &v)
	if err != nil {
		return
	}
	return
}

// ReadRetained loads all the retained messages from the boltdb instance.
func (s *Store) ReadRetained() (v []persistence.Message, err error) {
	if s.db == nil {
		err = errors.New("boltdb not opened")
		return
	}

	err = s.db.Find("T", persistence.KRetained, &v)
	if err != nil {
		return
	}
	return
}

//ReadServerInfo loads the server info from the boltdb instance.
func (s *Store) ReadServerInfo() (v persistence.ServerInfo, err error) {
	if s.db == nil {
		err = errors.New("boltdb not opened")
		return
	}

	err = s.db.One("ID", persistence.KServerInfo, &v)
	if err != nil {
		return
	}
	return
}
