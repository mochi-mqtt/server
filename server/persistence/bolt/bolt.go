package bolt

import (
	"errors"
	"strconv"
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

var (
	errNotFound = "not found"
	errNotOpened = errors.New("boltdb not opened")
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

func (s *Store ) GenSubscriptionId(cid, filter string) string {
	return persistence.KSubscription + "_" + cid + ":" + filter
}

func (s *Store ) GenInflightId(cid string, pid uint16) string {
	return persistence.KInflight + "_" + cid + "_" + strconv.Itoa(int(pid))
}

func (s *Store ) GenRetainedId(topic string) string {
	return persistence.KRetained + "_" + topic
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

// DeleteSubscription deletes a subscription from the boltdb instance.
func (s *Store) DeleteSubscription(cid, filter string) error {
	if s.db == nil {
		return errors.New("boltdb not opened")
	}
	err := s.db.DeleteStruct(&persistence.Subscription{
		ID: s.GenSubscriptionId(cid, filter),
	})
	if err != nil {
		return err
	}

	return nil
}

// DeleteClient deletes a client from the boltdb instance.
func (s *Store) DeleteClient(id string) error {
	if s.db == nil {
		return errors.New("boltdb not opened")
	}

	err := s.db.DeleteStruct(&persistence.Client{
		ID: id,
	})
	if err != nil {
		return err
	}

	return nil
}

// DeleteInflight deletes an inflight message from the boltdb instance.
func (s *Store) DeleteInflight(cid string, pid uint16) error {
	if s.db == nil {
		return errors.New("boltdb not opened")
	}

	err := s.db.DeleteStruct(&persistence.Message{
		ID: s.GenInflightId(cid, pid),
	})
	if err != nil {
		return err
	}

	return nil
}

// DeleteRetained deletes a retained message from the boltdb instance.
func (s *Store) DeleteRetained(topic string) error {
	if s.db == nil {
		return errors.New("boltdb not opened")
	}

	err := s.db.DeleteStruct(&persistence.Message{
		ID: s.GenRetainedId(topic),
	})
	if err != nil {
		return err
	}

	return nil
}

// ReadSubscriptions loads all the subscriptions from the boltdb instance.
func (s *Store) ReadSubscriptions() (v []persistence.Subscription, err error) {
	if s.db == nil {
		return v, errNotOpened
	}

	err = s.db.Find("T", persistence.KSubscription, &v)
	if err != nil && err.Error() != errNotFound {
		return
	}

	return v, nil
}

// ReadClients loads all the clients from the boltdb instance.
func (s *Store) ReadClients() (v []persistence.Client, err error) {
	if s.db == nil {
		return v, errNotOpened
	}

	err = s.db.Find("T", persistence.KClient, &v)
	if err != nil && err.Error() != errNotFound {
		return
	}

	return v, nil
}

// ReadInflight loads all the inflight messages from the boltdb instance.
func (s *Store) ReadInflight() (v []persistence.Message, err error) {
	if s.db == nil {
		return v, errNotOpened
	}

	err = s.db.Find("T", persistence.KInflight, &v)
	if err != nil && err.Error() != errNotFound {
		return
	}

	return v, nil
}

// ReadRetained loads all the retained messages from the boltdb instance.
func (s *Store) ReadRetained() (v []persistence.Message, err error) {
	if s.db == nil {
		return v, errNotOpened
	}

	err = s.db.Find("T", persistence.KRetained, &v)
	if err != nil && err.Error() != errNotFound {
		return
	}

	return v, nil
}

//ReadServerInfo loads the server info from the boltdb instance.
func (s *Store) ReadServerInfo() (v persistence.ServerInfo, err error) {
	if s.db == nil {
		return v, errNotOpened
	}

	err = s.db.One("ID", persistence.KServerInfo, &v)
	if err != nil && err.Error() != errNotFound {
		return
	}

	return v, nil
}

//ReadSubscriptionsByCid loads all the subscriptions from the boltdb instance by the client id.
func (s *Store) ReadSubscriptionsByCid(cid string) (v []persistence.Subscription, err error) {
	if s.db == nil {
		return v, errNotOpened
	}

	err = s.db.Find("Client", cid, &v)
	if err != nil && err.Error() != errNotFound {
		return
	}

	return v, nil
}

//ReadInflightByCid loads all the inflight messages from the boltdb instance by the client id.
func (s *Store) ReadInflightByCid(cid string) (v []persistence.Message, err error) {
	if s.db == nil {
		return v, errNotOpened
	}

	err = s.db.Find("Client", cid, &v)
	if err != nil && err.Error() != errNotFound {
		return
	}

	return v, nil
}

//ReadRetainedByTopic loads the retained message from the boltdb instance by the topic.
func (s *Store) ReadRetainedByTopic(topic string) (v persistence.Message, err error) {
	if s.db == nil {
		return v, errNotOpened
	}

	err = s.db.One("ID", s.GenRetainedId(topic), &v)
	if err != nil && err.Error() != errNotFound {
		return
	}

	return v, nil
}

//ReadClientByCid loads the client info from the boltdb instance.
func (s *Store) ReadClientByCid(cid string) (v persistence.Client, err error) {
	if s.db == nil {
		return v, errNotOpened
	}

	err = s.db.One("ClientID", cid, &v)
	if err != nil && err.Error() != errNotFound {
		return
	}

	return v, nil
}