package system

// Info contains atomic counters and values for various server statistics
// commonly found in $SYS topics.
type Info struct {
	Version             string `json:"version"`              // the current version of the server.
	Started             int64  `json:"started"`              // the time the server started in unix seconds.
	Uptime              int64  `json:"uptime"`               // the number of seconds the server has been online.
	BytesRecv           int64  `json:"bytes_recv"`           // the total number of bytes received in all packets.
	BytesSent           int64  `json:"bytes_sent"`           // the total number of bytes sent to clients.
	ClientsConnected    int64  `json:"clients_connected"`    // the number of currently connected clients.
	ClientsDisconnected int64  `json:"clients_disconnected"` // the number of disconnected non-cleansession clients.
	ClientsMax          int64  `json:"clients_max"`          // the maximum number of clients that have been concurrently connected.
	ClientsTotal        int64  `json:"clients_total"`        // the sum of all clients, connected and disconnected.
	ConnectionsTotal    int64  `json:"connections_total"`    // the sum number of clients which have ever connected.
	MessagesRecv        int64  `json:"messages_recv"`        // the total number of packets received.
	MessagesSent        int64  `json:"messages_sent"`        // the total number of packets sent.
	PublishDropped      int64  `json:"publish_dropped"`      // the number of in-flight publish messages which were dropped.
	PublishRecv         int64  `json:"publish_recv"`         // the total number of received publish packets.
	PublishSent         int64  `json:"publish_sent"`         // the total number of sent publish packets.
	Retained            int64  `json:"retained"`             // the number of messages currently retained.
	Inflight            int64  `json:"inflight"`             // the number of messages currently in-flight.
	Subscriptions       int64  `json:"subscriptions"`        // the total number of filter subscriptions.
}
