package mqtt

import (
	"log"
)

// Server is an MQTT broker server.
type Server struct {
}

// New returns a pointer to a new instance of the MQTT broker.
func New() *Server {
	return &Server{}
}
