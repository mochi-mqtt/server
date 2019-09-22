package packets

const (
	Accepted byte = 0x00
	Failed   byte = 0xFF

	ErrConnectBadProtocolVersion byte = 0x01
	ErrConnectBadClientID        byte = 0x02
	ErrConnectServerUnavailable  byte = 0x03
	ErrConnectBadAuthValues      byte = 0x04
	ErrConnectNotAuthorised      byte = 0x05
	ErrConnectNetworkError       byte = 0xFE
	ErrConnectProtocolViolation  byte = 0xFF

	ErrSubAckNetworkError byte = 0x80
)

const (
	// CONNECT
	ErrMalformedProtocolName    = "malformed packet: protocol name"
	ErrMalformedProtocolVersion = "malformed packet: protocol version"
	ErrMalformedFlags           = "malformed packet: flags"
	ErrMalformedKeepalive       = "malformed packet: keepalive"
	ErrMalformedClientID        = "malformed packet: client id"
	ErrMalformedWillTopic       = "malformed packet: will topic"
	ErrMalformedWillMessage     = "malformed packet: will message"
	ErrMalformedUsername        = "malformed packet: username"
	ErrMalformedPassword        = "malformed packet: password"

	// CONNACK
	ErrMalformedSessionPresent = "malformed packet: session present"
	ErrMalformedReturnCode     = "malformed packet: return code"

	// PUBLISH
	ErrMalformedTopic    = "malformed packet: topic name"
	ErrMalformedPacketID = "malformed packet: packet id"

	// SUBSCRIBE
	ErrMalformedQoS = "malformed packet: qos"

	// PACKETS
	ErrProtocolViolation        = "protocol violation"
	ErrOffsetStrOutOfRange      = "offset string out of range"
	ErrOffsetBytesOutOfRange    = "offset bytes out of range"
	ErrOffsetByteOutOfRange     = "offset byte out of range"
	ErrOffsetBoolOutOfRange     = "offset bool out of range"
	ErrOffsetUintOutOfRange     = "offset uint out of range"
	ErrOffsetStrInvalidUTF8     = "offset string invalid utf8"
	ErrInvalidFlags             = "invalid flags set for packet"
	ErrOversizedLengthIndicator = "protocol violation: oversized length indicator"
	ErrMissingPacketID          = "missing packet id"
	ErrSurplusPacketID          = "surplus packet id"
)

// validateQoS ensures the QoS byte is within the correct range.
func validateQoS(qos byte) bool {

	if qos >= 0 && qos <= 2 {
		return true
	}

	return false
}
