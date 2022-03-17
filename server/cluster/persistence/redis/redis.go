package redis

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/go-redis/redis/v8"
	"github.com/mochi-co/mqtt/server/internal/utils"
	"github.com/mochi-co/mqtt/server/persistence"
	"github.com/mochi-co/mqtt/server/system"
	"strconv"
)

var (
	LocalIP   = "127.0.0.1"
	ErrNotConnected = errors.New("redis not connected")
	ErrEmptyStruct = errors.New("struct cannot be empty")
)

const (
	KPrefix       = "wind:mqtt:"
	KSuffix       = "all"
	KSubscription = KPrefix + persistence.KSubscription
	KServerInfo   = KPrefix + persistence.KServerInfo
	KRetained     = KPrefix + persistence.KRetained
	KInflight     = KPrefix + persistence.KInflight
	KClient       = KPrefix + persistence.KClient
)

func init() {
	LocalIP, _ = utils.GetOutBoundIP()
}

type Store struct {
	opts *redis.Options
	db *redis.Client
	persistence.Store
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

func (s *Store) Set(key string, v interface{}) error {
	if s.db == nil {
		return ErrNotConnected
	}
	val, _ := json.Marshal(v)
	return s.db.Set(context.Background(), key, val, 0).Err()
}

func (s *Store) Del(key string) error {
	if s.db == nil {
		return ErrNotConnected
	}
	return s.db.Del(context.Background(), key).Err()
}

func (s *Store) HSet(key string, id string, v interface{}) error {
	if s.db == nil {
		return ErrNotConnected
	}
	val, _ := json.Marshal(v)
	return s.db.HSet(context.Background(), key, id, val).Err()
}

func (s *Store) HDel(key, id string) error {
	if s.db == nil {
		return ErrNotConnected
	}
	return s.db.HDel(context.Background(), key, id).Err()
}

func (s *Store ) GenSubscriptionId(cid, filter string) string {
	return filter
}

func (s *Store ) GenInflightId(cid string, pid uint16) string {
	return strconv.Itoa(int(pid))
}

func (s *Store ) GenRetainedId(topic string) string {
	return topic
}

// WriteServerInfo writes the server info to the redis instance.
func (s *Store) WriteServerInfo(v persistence.ServerInfo) error {
	if v.ID == "" || v.Info == (system.Info{}) {
		return ErrEmptyStruct
	}
	return s.HSet(utils.JoinStrings(KServerInfo, KSuffix), LocalIP, v)
}

// WriteSubscription writes a single subscription to the redis instance.
func (s *Store) WriteSubscription(v persistence.Subscription) error {
	if v.ID == "" || v.Client == "" || v.Filter == "" {
		return ErrEmptyStruct
	}
	return s.HSet(utils.JoinStrings(KSubscription, v.Client), v.ID, v)
}

// WriteInflight writes a single inflight message to the redis instance.
func (s *Store) WriteInflight(v persistence.Message) error {
	if v.ID == "" || v.TopicName == "" {
		return ErrEmptyStruct
	}
	return s.HSet(utils.JoinStrings(KInflight, v.Client), v.ID, v)
}

// WriteRetained writes a single retained message to the redis instance.
func (s *Store) WriteRetained(v persistence.Message) error {
	if v.ID == "" || v.TopicName == "" {
		return ErrEmptyStruct
	}
	return s.HSet(utils.JoinStrings(KRetained, KSuffix), v.ID, v)
}

// WriteClient writes a single client to the redis instance.
func (s *Store) WriteClient(v persistence.Client) error {
	if v.ClientID == "" {
		return ErrEmptyStruct
	}
	return s.HSet(utils.JoinStrings(KClient, KSuffix), v.ClientID, v)
}

// DeleteSubscription deletes a subscription from the redis instance.
func (s *Store) DeleteSubscription(cid, filter string) error {
	return s.HDel(utils.JoinStrings(KSubscription, cid), s.GenSubscriptionId(cid, filter))
}

// DeleteClient deletes a client from the redis instance.
func (s *Store) DeleteClient(cid string) error {
	return s.HDel(utils.JoinStrings(KClient, KSuffix), cid)
}

// DeleteInflight deletes an inflight message from the redis instance by the client id.
func (s *Store) DeleteInflight(cid string, pid uint16) error {
	return s.HDel(utils.JoinStrings(KInflight, cid), s.GenInflightId(cid, pid))
}

// DeleteRetained deletes a retained message from the redis instance by the topic.
func (s *Store) DeleteRetained(topic string) error {
	return s.HDel(utils.JoinStrings(KRetained, KSuffix), s.GenRetainedId(topic))
}

// ReadSubscriptionsByCid loads all the subscriptions from the redis instance by the client id.
func (s *Store) ReadSubscriptionsByCid(cid string) (v []persistence.Subscription, err error) {
	if s.db == nil {
		return v, ErrNotConnected
	}

	res, err := s.db.HGetAll(context.Background(), utils.JoinStrings(KSubscription, cid)).Result()
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

// ReadClientByCid read a client from the redis instance by the client id.
func (s *Store) ReadClientByCid(cid string) (v persistence.Client, err error) {
	if s.db == nil {
		return v, ErrNotConnected
	}

	res, err := s.db.HGet(context.Background(), utils.JoinStrings(KClient, KSuffix), cid).Result()
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

// ReadInflightByCid loads all the inflight messages from the redis instance by the client id.
func (s *Store) ReadInflightByCid(cid string) (v []persistence.Message, err error) {
	if s.db == nil {
		return v, ErrNotConnected
	}

	res, err := s.db.HGetAll(context.Background(), utils.JoinStrings(KInflight, cid)).Result()
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

// ReadRetainedByTopic loads the retained message from the redis instance by the topic.
func (s *Store) ReadRetainedByTopic(topic string) (v persistence.Message, err error) {
	if s.db == nil {
		return v, ErrNotConnected
	}

	res, err := s.db.HGet(context.Background(), utils.JoinStrings(KRetained, KSuffix), topic).Result()
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

//ReadServerInfo loads the server info from the redis instance.
func (s *Store) ReadServerInfo() (v persistence.ServerInfo, err error) {
	if s.db == nil {
		return v, ErrNotConnected
	}

	res, err := s.db.HGet(context.Background(), utils.JoinStrings(KServerInfo, KSuffix), LocalIP).Result()
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

