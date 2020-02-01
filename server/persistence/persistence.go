package persistence

import (
	"errors"

	"github.com/mochi-co/mqtt/server/system"
)

const (
	KSubscription = "sub"
	KServerInfo   = "srv"
	KRetained     = "ret"
	KInflight     = "ifm"
	KClient       = "cl"
)

// Store is an interface which details a persistent storage connector.
type Store interface {
	Open() error
	Close()
	WriteSubscription(v Subscription) error
	WriteClient(v Client) error
	WriteInflight(v Message) error
	WriteServerInfo(v ServerInfo) error
	WriteRetained(v Message) error
	ReadSubscriptions() (v []Subscription, err error)
	ReadInflight() (v []Message, err error)
	ReadRetained() (v []Message, err error)
	ReadClients() (v []Client, err error)
	ReadServerInfo() (v ServerInfo, err error)
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
	ID          string      // the storage key.
	T           string      // the type of the stored data.
	Client      string      // the id of the client who sent the message (if inflight).
	FixedHeader FixedHeader // the header properties of the message.
	PacketID    uint16      // the unique id of the packet (if inflight).
	TopicName   string      // the topic the message was sent to (if retained).
	Payload     []byte      // the message payload (if retained).
	Sent        int64       // the last time the message was sent (for retries) in unixtime (if inflight).
	Resends     int         // the number of times the message was attempted to be sent (if inflight).
}

// FixedHeader contains the fixed header properties of a message.
type FixedHeader struct {
	Type      byte // the type of the packet (PUBLISH, SUBSCRIBE, etc) from bits 7 - 4 (byte 1).
	Dup       bool // indicates if the packet was already sent at an earlier time.
	Qos       byte // indicates the quality of service expected.
	Retain    bool // whether the message should be retained.
	Remaining int  // the number of remaining bytes in the payload.
}

// Client contains client data that can be persistently stored.
type Client struct {
	ID            string          // the id of the client
	T             string          // the type of the stored data.
	Listener      string          // the last known listener id for the client
	Username      []byte          // the username the client authenticated with.
	CleanSession  bool            // indicates if the client connected expecting a clean-session.
	Subscriptions map[string]byte // a list of the subscriptions the user has (qos keyed on filter).
	LWT           LWT             // the last-will-and-testament message for the client.
}

// LWT contains details about a clients LWT payload.
type LWT struct {
	Topic   string // the topic the will message shall be sent to.
	Message []byte // the message that shall be sent when the client disconnects.
	Qos     byte   // the quality of service desired.
	Retain  bool   // indicates whether the will message should be retained
}

// MockStore is a mock storage backend for testing.
type MockStore struct {
	FailOpen bool
	Closed   bool
	Opened   bool
	Fail     bool
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
	if s.Fail {
		return errors.New("test")
	}
	return nil
}

// WriteClient writes a single client to the storage instance.
func (s *MockStore) WriteClient(v Client) error {
	if s.Fail {
		return errors.New("test")
	}
	return nil
}

// WriteInFlight writes a single InFlight message to the storage instance.
func (s *MockStore) WriteInflight(v Message) error {
	if s.Fail {
		return errors.New("test")
	}
	return nil
}

// WriteRetained writes a single retained message to the storage instance.
func (s *MockStore) WriteRetained(v Message) error {
	if s.Fail {
		return errors.New("test")
	}
	return nil
}

// WriteServerInfo writes server info to the storage instance.
func (s *MockStore) WriteServerInfo(v ServerInfo) error {
	if s.Fail {
		return errors.New("test")
	}
	return nil
}

// ReadSubscriptions loads the subscriptions from the storage instance.
func (s *MockStore) ReadSubscriptions() (v []Subscription, err error) {
	if s.Fail {
		return v, errors.New("test")
	}
	return
}

// ReadClients loads the clients from the storage instance.
func (s *MockStore) ReadClients() (v []Client, err error) {
	if s.Fail {
		return v, errors.New("test")
	}
	return
}

// ReadInflight loads the inflight messages from the storage instance.
func (s *MockStore) ReadInflight() (v []Message, err error) {
	if s.Fail {
		return v, errors.New("test")
	}
	return
}

// ReadRetained loads the retained messages from the storage instance.
func (s *MockStore) ReadRetained() (v []Message, err error) {
	if s.Fail {
		return v, errors.New("test")
	}
	return
}

//ReadServerInfo loads the server info from the storage instance.
func (s *MockStore) ReadServerInfo() (v ServerInfo, err error) {
	if s.Fail {
		return v, errors.New("test")
	}
	return ServerInfo{
		system.Info{
			Version: "test",
		},
		KServerInfo,
	}, nil
}
