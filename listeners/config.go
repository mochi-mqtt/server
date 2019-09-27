package listeners

import (
	"github.com/mochi-co/mqtt/auth"
)

// Config contains configuration values for a listener.
type Config struct {

	// Auth is an authentication controller for the listener, containing methods
	// for Connect auth and topic ACL.
	Auth auth.Controller

	// TLS contains the TLS certficates and settings for the connection.
	TLS *TLS
}

// TLS contains the TLS certificates and settings for the listener connection.
type TLS struct {
}
