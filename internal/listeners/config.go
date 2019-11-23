package listeners

import (
	"github.com/mochi-co/mqtt/internal/auth"
)

// Config contains configuration values for a listener.
type Config struct {
	Auth auth.Controller //  an authentication controller containing auth and ACL logic.
	TLS  *TLS            // the TLS certficates and settings for the connection.
}

// TLS contains the TLS certificates and settings for the listener connection.
type TLS struct {
}
