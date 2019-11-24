package auth

// Controller is an interface for authentication controllers.
type Controller interface {

	// Authenticate authenticates a user on CONNECT and returns true if a user is
	// allowed to join the server.
	Authenticate(user, password []byte) bool

	// ACL returns true if a user has read or write access to a given topic.
	ACL(user []byte, topic string, write bool) bool
}
