package persistence

import (
	"errors"

	"github.com/mochi-co/mqtt/server/system"
)

const (

	// KSubscription is the key for subscription data.
	KSubscription = "sub"

	// KServerInfo is the key for server info data.
	KServerInfo = "srv"

	// KRetained is the key for retained messages data.
	KRetained = "ret"

	// KInflight is the key for inflight messages data.
	KInflight = "ifm"

	// KClient is the key for client data.
	KClient = "cl"
)

// Store is an interface which details a persistent storage connector.
type Store interface {
	Open() error
	Close()

	ReadSubscriptions() (v []Subscription, err error)
	WriteSubscription(v Subscription) error
	DeleteSubscription(id string) error

	ReadClients() (v []Client, err error)
	WriteClient(v Client) error
	DeleteClient(id string) error

	ReadInflight() (v []Message, err error)
	WriteInflight(v Message) error
	DeleteInflight(id string) error

	SetInflightTTL(seconds int64)
	ClearExpiredInflight(expiry int64) error

	ReadServerInfo() (v ServerInfo, err error)
	WriteServerInfo(v ServerInfo) error

	ReadRetained() (v []Message, err error)
	WriteRetained(v Message) error
	DeleteRetained(id string) error
}

// ServerInfo contains information and statistics about the server.
type ServerInfo struct {
	system.Info        // embed the system info struct.
	ID          string // the storage key.
}

// Subscription contains the details of a topic filter subscription.
type Subscription struct {
	ID     string // the storage key.
	T      string // the type of the stored data.
	Client string // the id of the client who the subscription belongs to.
	Filter string // the topic filter being subscribed to.
	QoS    byte   // the desired QoS byte.
}

// Message contains the details of a retained or inflight message.
type Message struct {
	Payload     []byte      // the message payload (if retained).
	FixedHeader FixedHeader // the header properties of the message.
	T           string      // the type of the stored data.
	ID          string      // the storage key.
	Client      string      // the id of the client who sent the message (if inflight).
	TopicName   string      // the topic the message was sent to (if retained).
	Created     int64       // the time the message was created in unixtime (if inflight).
	Sent        int64       // the last time the message was sent (for retries) in unixtime (if inflight).
	Resends     int         // the number of times the message was attempted to be sent (if inflight).
	PacketID    uint16      // the unique id of the packet (if inflight).
}

// FixedHeader contains the fixed header properties of a message.
type FixedHeader struct {
	Remaining int  // the number of remaining bytes in the payload.
	Type      byte // the type of the packet (PUBLISH, SUBSCRIBE, etc) from bits 7 - 4 (byte 1).
	Qos       byte // indicates the quality of service expected.
	Dup       bool // indicates if the packet was already sent at an earlier time.
	Retain    bool // whether the message should be retained.
}

// Client contains client data that can be persistently stored.
type Client struct {
	LWT      LWT    // the last-will-and-testament message for the client.
	Username []byte // the username the client authenticated with.
	ID       string // the storage key.
	ClientID string // the id of the client.
	T        string // the type of the stored data.
	Listener string // the last known listener id for the client
}

// LWT contains details about a clients LWT payload.
type LWT struct {
	Message []byte // the message that shall be sent when the client disconnects.
	Topic   string // the topic the will message shall be sent to.
	Qos     byte   // the quality of service desired.
	Retain  bool   // indicates whether the will message should be retained
}

// MockStore is a mock storage backend for testing.
type MockStore struct {
	Fail        map[string]bool // issue errors for different methods.
	FailOpen    bool            // error on open.
	Closed      bool            // indicate mock store is closed.
	Opened      bool            // indicate mock store is open.
	inflightTTL int64           // inflight expiry duration.
}

// Close closes the storage instance.
func (s *MockStore) SetInflightTTL(seconds int64) {
	s.inflightTTL = seconds
}

// Open opens the storage instance.
func (s *MockStore) Open() error {
	if s.FailOpen {
		return errors.New("test")
	}

	s.Opened = true
	return nil
}

// Close closes the storage instance.
func (s *MockStore) Close() {
	s.Closed = true
}

// WriteSubscription writes a single subscription to the storage instance.
func (s *MockStore) WriteSubscription(v Subscription) error {
	if _, ok := s.Fail["write_subs"]; ok {
		return errors.New("test")
	}
	return nil
}

// WriteClient writes a single client to the storage instance.
func (s *MockStore) WriteClient(v Client) error {
	if _, ok := s.Fail["write_clients"]; ok {
		return errors.New("test")
	}
	return nil
}

// WriteInFlight writes a single InFlight message to the storage instance.
func (s *MockStore) WriteInflight(v Message) error {
	if _, ok := s.Fail["write_inflight"]; ok {
		return errors.New("test")
	}
	return nil
}

// WriteRetained writes a single retained message to the storage instance.
func (s *MockStore) WriteRetained(v Message) error {
	if _, ok := s.Fail["write_retained"]; ok {
		return errors.New("test")
	}
	return nil
}

// WriteServerInfo writes server info to the storage instance.
func (s *MockStore) WriteServerInfo(v ServerInfo) error {
	if _, ok := s.Fail["write_info"]; ok {
		return errors.New("test")
	}
	return nil
}

// DeleteSubscription deletes a subscription from the persistent store.
func (s *MockStore) DeleteSubscription(id string) error {
	if _, ok := s.Fail["delete_subs"]; ok {
		return errors.New("test")
	}

	return nil
}

// DeleteClient deletes a client from the persistent store.
func (s *MockStore) DeleteClient(id string) error {
	if _, ok := s.Fail["delete_clients"]; ok {
		return errors.New("test")
	}

	return nil
}

// DeleteInflight deletes an inflight message from the persistent store.
func (s *MockStore) DeleteInflight(id string) error {
	if _, ok := s.Fail["delete_inflight"]; ok {
		return errors.New("test")
	}

	return nil
}

// DeleteRetained deletes a retained message from the persistent store.
func (s *MockStore) DeleteRetained(id string) error {
	if _, ok := s.Fail["delete_retained"]; ok {
		return errors.New("test")
	}

	return nil
}

// ReadSubscriptions loads the subscriptions from the storage instance.
func (s *MockStore) ReadSubscriptions() (v []Subscription, err error) {
	if _, ok := s.Fail["read_subs"]; ok {
		return v, errors.New("test_subs")
	}

	return []Subscription{
		{
			ID:     "test:a/b/c",
			Client: "test",
			Filter: "a/b/c",
			QoS:    1,
			T:      KSubscription,
		},
	}, nil
}

// ReadClients loads the clients from the storage instance.
func (s *MockStore) ReadClients() (v []Client, err error) {
	if _, ok := s.Fail["read_clients"]; ok {
		return v, errors.New("test_clients")
	}

	return []Client{
		{
			ID:       "cl_client1",
			ClientID: "client1",
			T:        KClient,
			Listener: "tcp1",
		},
	}, nil
}

// ReadInflight loads the inflight messages from the storage instance.
func (s *MockStore) ReadInflight() (v []Message, err error) {
	if _, ok := s.Fail["read_inflight"]; ok {
		return v, errors.New("test_inflight")
	}

	return []Message{
		{
			ID:        "client1_if_100",
			T:         KInflight,
			Client:    "client1",
			PacketID:  100,
			TopicName: "d/e/f",
			Payload:   []byte{'y', 'e', 's'},
			Sent:      200,
			Resends:   1,
		},
	}, nil
}

// ReadRetained loads the retained messages from the storage instance.
func (s *MockStore) ReadRetained() (v []Message, err error) {
	if _, ok := s.Fail["read_retained"]; ok {
		return v, errors.New("test_retained")
	}

	return []Message{
		{
			ID: "client1_ret_200",
			T:  KRetained,
			FixedHeader: FixedHeader{
				Retain: true,
			},
			PacketID:  200,
			TopicName: "a/b/c",
			Payload:   []byte{'h', 'e', 'l', 'l', 'o'},
			Sent:      100,
			Resends:   0,
		},
	}, nil
}

// ReadServerInfo loads the server info from the storage instance.
func (s *MockStore) ReadServerInfo() (v ServerInfo, err error) {
	if _, ok := s.Fail["read_info"]; ok {
		return v, errors.New("test_info")
	}

	return ServerInfo{
		system.Info{
			Version: "test",
			Started: 100,
		},
		KServerInfo,
	}, nil
}

// ReadServerInfo loads the server info from the storage instance.
func (s *MockStore) ClearExpiredInflight(d int64) error {
	return nil
}
