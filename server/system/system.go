package system

// Info contains atomic counters and values for various server statistics
// commonly found in $SYS topics.
type Info struct {
	Version string `json:"version"` // The current version of the server.

	BytesRecv           int64 `json:"bytes_recv"`           // The total number of bytes received in all packets.
	BytesSent           int64 `json:"bytes_sent"`           // The total number of bytes sent to clients.
	ClientsConnected    int64 `json:"clients_connected"`    // The number of currently connected clients.
	ClientsDisconnected int64 `json:"clients_disconnected"` // The number of disconnected non-cleansession clients.
	ClientsMax          int64 `json:"clients_max"`          // The maximum number of clients that have been concurrently connected.
	ClientsTotal        int64 `json:"clients_total"`        // The sum of all clients, connected and disconnected.
	MessagesRecv        int64 `json:"messages_recv"`        // The total number of packets received.
	MessagesSent        int64 `json:"messages_sent"`        // The total number of packets sent.
	PublishDropped      int64 `json:"publish_dropped"`      // The number of in-flight publish messages which were dropped.
	PublishRecv         int64 `json:"publish_recv"`         // The total number of received publish packets.
	PublishSent         int64 `json:"publish_sent"`         // The total number of sent publish packets.
	Retained            int64 `json:"retained"`             // The number of messages currently retained.
	InFlight            int64 `json:"inflight"`             // The number of messages currently in-flight.
	Subscriptions       int64 `json:"subscriptions"`        // The total number of filter subscriptions.
	Started             int64 `json:"started"`              // The time the server started in unix seconds.
}
