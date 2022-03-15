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
	ErrNotFound     = errors.New("not found")
	ErrEmptyStruct  = errors.New("the structure cannot be empty")
)

const (
	KSubscription = "mqtt:" + persistence.KSubscription
	KServerInfo   = "mqtt:" + persistence.KServerInfo
	KRetained     = "mqtt:" + persistence.KRetained
	KInflight     = "mqtt:" + persistence.KInflight
	KClient       = "mqtt:" + persistence.KClient
)

type Store struct {
	opts *redis.Options
	db   *redis.Client
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
	s.db = redis.NewClient(s.opts)
	_, err := s.db.Ping(context.TODO()).Result()
	return err
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
	return s.db.Set(context.Background(), KServerInfo, val, 0).Err()
}

// WriteSubscription writes a single subscription to the redis instance.
func (s *Store) WriteSubscription(v persistence.Subscription) error {
	if v.ID == "" || v.Client == "" || v.Filter == "" {
		return ErrEmptyStruct
	}
	return s.HSet(KSubscription, v.ID, v)
}

// WriteInflight writes a single inflight message to the redis instance.
func (s *Store) WriteInflight(v persistence.Message) error {
	if v.ID == "" || v.TopicName == "" {
		return ErrEmptyStruct
	}
	return s.HSet(KInflight, v.ID, v)
}

// WriteRetained writes a single retained message to the redis instance.
func (s *Store) WriteRetained(v persistence.Message) error {
	if v.ID == "" || v.TopicName == "" {
		return ErrEmptyStruct
	}
	return s.HSet(KRetained, v.ID, v)
}

// WriteClient writes a single client to the redis instance.
func (s *Store) WriteClient(v persistence.Client) error {
	if v.ClientID == "" {
		return ErrEmptyStruct
	}
	return s.HSet(KClient, v.ID, v)
}

func (s *Store) Del(key, id string) error {
	if s.db == nil {
		return ErrNotConnected
	}

	return s.db.HDel(context.Background(), key, id).Err()
}

// DeleteSubscription deletes a subscription from the redis instance.
func (s *Store) DeleteSubscription(id string) error {
	return s.Del(KSubscription, id)
}

// DeleteClient deletes a client from the redis instance.
func (s *Store) DeleteClient(id string) error {
	return s.Del(KClient, id)
}

// DeleteInflight deletes an inflight message from the redis instance.
func (s *Store) DeleteInflight(id string) error {
	return s.Del(KInflight, id)
}

// DeleteRetained deletes a retained message from the redis instance.
func (s *Store) DeleteRetained(id string) error {
	return s.Del(KRetained, id)
}

// ReadSubscriptions loads all the subscriptions from the redis instance.
func (s *Store) ReadSubscriptions() (v []persistence.Subscription, err error) {
	if s.db == nil {
		return v, ErrNotConnected
	}

	res, err := s.db.HGetAll(context.Background(), KSubscription).Result()
	if err != nil && err != redis.Nil {
		return
	}

	for _, val := range res {
		sub := persistence.Subscription{}
		err = json.Unmarshal([]byte(val), &sub)
		if err != nil {
			return v, err
		}

		v = append(v, sub)
	}

	return v, nil
}

// ReadClients loads all the clients from the redis instance.
func (s *Store) ReadClients() (v []persistence.Client, err error) {
	if s.db == nil {
		return v, ErrNotConnected
	}
	res, err := s.db.HGetAll(context.Background(), KClient).Result()
	if err != nil && err != redis.Nil {
		return
	}

	for _, val := range res {
		cli := persistence.Client{}
		err = json.Unmarshal([]byte(val), &cli)
		if err != nil {
			return v, err
		}

		v = append(v, cli)
	}

	return v, nil
}

// ReadInflight loads all the inflight messages from the redis instance.
func (s *Store) ReadInflight() (v []persistence.Message, err error) {
	if s.db == nil {
		return v, ErrNotConnected
	}

	res, err := s.db.HGetAll(context.Background(), KInflight).Result()
	if err != nil && err != redis.Nil {
		return
	}

	for _, val := range res {
		msg := persistence.Message{}
		err = json.Unmarshal([]byte(val), &msg)
		if err != nil {
			return v, err
		}

		v = append(v, msg)
	}

	return v, nil
}

// ReadRetained loads all the retained messages from the redis instance.
func (s *Store) ReadRetained() (v []persistence.Message, err error) {
	if s.db == nil {
		return v, ErrNotConnected
	}

	res, err := s.db.HGetAll(context.Background(), KRetained).Result()
	if err != nil && err != redis.Nil {
		return
	}

	for _, val := range res {
		msg := persistence.Message{}
		err = json.Unmarshal([]byte(val), &msg)
		if err != nil {
			return v, err
		}

		v = append(v, msg)
	}

	return v, nil
}

//ReadServerInfo loads the server info from the redis instance.
func (s *Store) ReadServerInfo() (v persistence.ServerInfo, err error) {
	if s.db == nil {
		return v, ErrNotConnected
	}

	res, err := s.db.Get(context.Background(), KServerInfo).Result()
	if err != nil && err != redis.Nil {
		return
	}

	if res != "" {
		err = json.Unmarshal([]byte(res), &v)
		if err != nil {
			return
		}
	}

	return v, nil
}
