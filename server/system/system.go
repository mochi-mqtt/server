package system

// Info contains atomic counters and values for various server statistics
// commonly found in $SYS topics.
type Info struct {
	BytesRecv           int64  // The total number of bytes received in all packets.
	BytesSent           int64  // The total number of bytes sent to clients.
	ClientsConnected    int64  // The number of currently connected clients.
	ClientsDisconnected int64  // The number of disconnected non-cleansession clients.
	ClientsMax          int64  // The maximum number of clients that have been concurrently connected.
	ClientsTotal        int64  // The sum of all clients, connected and disconnected.
	MessagesRecv        int64  // The total number of packets received.
	MessagesSent        int64  // The total number of packets sent.
	PublishDropped      int64  // The number of in-flight publish messages which were dropped.
	PublishRecv         int64  // The total number of received publish packets.
	PublishSent         int64  // The total number of sent publish packets.
	Retained            int64  // The number of messages currently retained.
	InFlight            int64  // The number of messages currently in-flight.
	Subscriptions       int64  // The total number of filter subscriptions.
	Started             int64  // The time the server started in unix seconds.
	Version             string // The current version of the server.
}
