package packets

import (
	"errors"
)

const (
	Accepted byte = 0x00
	Failed   byte = 0xFF

	CodeConnectBadProtocolVersion byte = 0x01
	CodeConnectBadClientID        byte = 0x02
	CodeConnectServerUnavailable  byte = 0x03
	CodeConnectBadAuthValues      byte = 0x04
	CodeConnectNotAuthorised      byte = 0x05
	CodeConnectNetworkError       byte = 0xFE
	CodeConnectProtocolViolation  byte = 0xFF

	//ErrSubAckNetworkError byte = 0x80
)

var (
	// CONNECT
	ErrMalformedProtocolName    = errors.New("malformed packet: protocol name")
	ErrMalformedProtocolVersion = errors.New("malformed packet: protocol version")
	ErrMalformedFlags           = errors.New("malformed packet: flags")
	ErrMalformedKeepalive       = errors.New("malformed packet: keepalive")
	ErrMalformedClientID        = errors.New("malformed packet: client id")
	ErrMalformedWillTopic       = errors.New("malformed packet: will topic")
	ErrMalformedWillMessage     = errors.New("malformed packet: will message")
	ErrMalformedUsername        = errors.New("malformed packet: username")
	ErrMalformedPassword        = errors.New("malformed packet: password")

	// CONNACK
	ErrMalformedSessionPresent = errors.New("malformed packet: session present")
	ErrMalformedReturnCode     = errors.New("malformed packet: return code")

	// PUBLISH
	ErrMalformedTopic    = errors.New("malformed packet: topic name")
	ErrMalformedPacketID = errors.New("malformed packet: packet id")

	// SUBSCRIBE
	ErrMalformedQoS = errors.New("malformed packet: qos")

	// PACKETS
	ErrProtocolViolation        = errors.New("protocol violation")
	ErrOffsetStrOutOfRange      = errors.New("offset string out of range")
	ErrOffsetBytesOutOfRange    = errors.New("offset bytes out of range")
	ErrOffsetByteOutOfRange     = errors.New("offset byte out of range")
	ErrOffsetBoolOutOfRange     = errors.New("offset bool out of range")
	ErrOffsetUintOutOfRange     = errors.New("offset uint out of range")
	ErrOffsetStrInvalidUTF8     = errors.New("offset string invalid utf8")
	ErrInvalidFlags             = errors.New("invalid flags set for packet")
	ErrOversizedLengthIndicator = errors.New("protocol violation: oversized length indicator")
	ErrMissingPacketID          = errors.New("missing packet id")
	ErrSurplusPacketID          = errors.New("surplus packet id")
)

// validateQoS ensures the QoS byte is within the correct range.
func validateQoS(qos byte) bool {

	if qos >= 0 && qos <= 2 {
		return true
	}

	return false
}
