package redis

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/go-redis/redis/v8"
	"github.com/mochi-co/mqtt/server/persistence"
	"github.com/mochi-co/mqtt/server/system"
)

var (
	ErrNotConnected = errors.New("redis not connected")
	ErrNotFound = errors.New("not found")
	ErrEmptyStruct = errors.New("the structure cannot be empty")
)

type Store struct {
	opts *redis.Options
	db *redis.Client
}

func New(opts *redis.Options) *Store {
	if opts == nil {
		opts = &redis.Options{
			Password: "", // no password set
			DB:       0,  // use default DB
		}
	}
	return &Store{
		opts: opts,
	}
}

func (s *Store) Open() error {
	//if s.opts.Addr == "" {
	//	return errors.New("addr cannot be empty")
	//}
	db := redis.NewClient(s.opts)
	_, err := db.Ping(context.TODO()).Result()
	if err != nil {
		return err
	}
	s.db = db
	return nil
}

// Close closes the redis instance.
func (s *Store) Close() {
	s.db.Close()
}

func (s *Store) HSet(key string, id string, v interface{}) error {
	if s.db == nil {
		return ErrNotConnected
	}
	val, _ := json.Marshal(v)
	return s.db.HSet(context.Background(), key, id, val).Err()
}

// WriteServerInfo writes the server info to the redis instance.
func (s *Store) WriteServerInfo(v persistence.ServerInfo) error {
	if v.ID == "" || v.Info == (system.Info{}) {
		return ErrEmptyStruct
	}
	val, _ := json.Marshal(v)
	return s.db.Set(context.Background(), persistence.KServerInfo, val, 0).Err()
}

// WriteSubscription writes a single subscription to the redis instance.
func (s *Store) WriteSubscription(v persistence.Subscription) error {
	if v.ID == "" || v.Client == "" || v.Filter == "" {
		return ErrEmptyStruct
	}
	return s.HSet(persistence.KSubscription, v.ID, v)
}

// WriteInflight writes a single inflight message to the redis instance.
func (s *Store) WriteInflight(v persistence.Message) error {
	if v.ID == "" || v.TopicName == "" {
		return ErrEmptyStruct
	}
	return s.HSet(persistence.KInflight, v.ID, v)
}

// WriteRetained writes a single retained message to the redis instance.
func (s *Store) WriteRetained(v persistence.Message) error {
	if v.ID == "" || v.TopicName == "" {
		return ErrEmptyStruct
	}
	return s.HSet(persistence.KRetained, v.ID, v)
}

// WriteClient writes a single client to the redis instance.
func (s *Store) WriteClient(v persistence.Client) error {
	if v.ClientID == "" {
		return ErrEmptyStruct
	}
	return s.HSet(persistence.KClient, v.ID, v)
}

func (s *Store ) Del(key, id string) error {
	if s.db == nil {
		return ErrNotConnected
	}

	return s.db.HDel(context.Background(), key, id).Err()
}

// DeleteSubscription deletes a subscription from the redis instance.
func (s *Store) DeleteSubscription(id string) error {
	return s.Del(persistence.KSubscription, id)
}

// DeleteClient deletes a client from the redis instance.
func (s *Store) DeleteClient(id string) error {
	return s.Del(persistence.KClient, id)
}

// DeleteInflight deletes an inflight message from the redis instance.
func (s *Store) DeleteInflight(id string) error {
	return s.Del(persistence.KInflight, id)
}

// DeleteRetained deletes a retained message from the redis instance.
func (s *Store) DeleteRetained(id string) error {
	return s.Del(persistence.KRetained, id)
}

// ReadSubscriptions loads all the subscriptions from the redis instance.
func (s *Store) ReadSubscriptions() (v []persistence.Subscription, err error) {
	if s.db == nil {
		return v, ErrNotConnected
	}

	res, err := s.db.HGetAll(context.Background(), persistence.KSubscription).Result()
	for _, val := range res {
		sub := persistence.Subscription{}
		json.Unmarshal([]byte(val), &sub)
		v = append(v, sub)
	}

	return
}

// ReadClients loads all the clients from the redis instance.
func (s *Store) ReadClients() (v []persistence.Client, err error) {
	if s.db == nil {
		return v, ErrNotConnected
	}

	res, err := s.db.HGetAll(context.Background(), persistence.KClient).Result()
	for _, val := range res {
		cli := persistence.Client{}
		json.Unmarshal([]byte(val), &cli)
		v = append(v, cli)
	}

	return
}

// ReadInflight loads all the inflight messages from the redis instance.
func (s *Store) ReadInflight() (v []persistence.Message, err error) {
	if s.db == nil {
		return v, ErrNotConnected
	}

	res, err := s.db.HGetAll(context.Background(), persistence.KInflight).Result()
	for _, val := range res {
		msg := persistence.Message{}
		json.Unmarshal([]byte(val), &msg)
		v = append(v, msg)
		//log.Printf("result %s, %v", k, msg)
	}

	return
}

// ReadRetained loads all the retained messages from the redis instance.
func (s *Store) ReadRetained() (v []persistence.Message, err error) {
	if s.db == nil {
		return v, ErrNotConnected
	}

	res, err := s.db.HGetAll(context.Background(), persistence.KRetained).Result()
	for _, val := range res {
		msg := persistence.Message{}
		json.Unmarshal([]byte(val), &msg)
		v = append(v, msg)
	}

	return
}

//ReadServerInfo loads the server info from the redis instance.
func (s *Store) ReadServerInfo() (v persistence.ServerInfo, err error) {
	if s.db == nil {
		return v, ErrNotConnected
	}

	res, err := s.db.Get(context.Background(), persistence.KServerInfo).Result()
	if res == "" {
		return v, ErrNotFound
	}

	json.Unmarshal([]byte(res), &v)
	return
}

