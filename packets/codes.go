// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package packets

// Code contains a reason code and reason string for a response.
type Code struct {
	Reason string
	Code   byte
}

// String returns the readable reason for a code.
func (c Code) String() string {
	return c.Reason
}

// Error returns the readable reason for a code.
func (c Code) Error() string {
	return c.Reason
}

var (
	// QosCodes indicates the reason codes for each Qos byte.
	QosCodes = map[byte]Code{
		0: CodeGrantedQos0,
		1: CodeGrantedQos1,
		2: CodeGrantedQos2,
	}

	CodeSuccessIgnore                         = Code{Code: 0x00, Reason: "ignore packet"}
	CodeSuccess                               = Code{Code: 0x00, Reason: "success"}
	CodeDisconnect                            = Code{Code: 0x00, Reason: "disconnected"}
	CodeGrantedQos0                           = Code{Code: 0x00, Reason: "granted qos 0"}
	CodeGrantedQos1                           = Code{Code: 0x01, Reason: "granted qos 1"}
	CodeGrantedQos2                           = Code{Code: 0x02, Reason: "granted qos 2"}
	CodeDisconnectWillMessage                 = Code{Code: 0x04, Reason: "disconnect with will message"}
	CodeNoMatchingSubscribers                 = Code{Code: 0x10, Reason: "no matching subscribers"}
	CodeNoSubscriptionExisted                 = Code{Code: 0x11, Reason: "no subscription existed"}
	CodeContinueAuthentication                = Code{Code: 0x18, Reason: "continue authentication"}
	CodeReAuthenticate                        = Code{Code: 0x19, Reason: "re-authenticate"}
	ErrUnspecifiedError                       = Code{Code: 0x80, Reason: "unspecified error"}
	ErrMalformedPacket                        = Code{Code: 0x81, Reason: "malformed packet"}
	ErrMalformedProtocolName                  = Code{Code: 0x81, Reason: "malformed packet: protocol name"}
	ErrMalformedProtocolVersion               = Code{Code: 0x81, Reason: "malformed packet: protocol version"}
	ErrMalformedFlags                         = Code{Code: 0x81, Reason: "malformed packet: flags"}
	ErrMalformedKeepalive                     = Code{Code: 0x81, Reason: "malformed packet: keepalive"}
	ErrMalformedPacketID                      = Code{Code: 0x81, Reason: "malformed packet: packet identifier"}
	ErrMalformedTopic                         = Code{Code: 0x81, Reason: "malformed packet: topic"}
	ErrMalformedWillTopic                     = Code{Code: 0x81, Reason: "malformed packet: will topic"}
	ErrMalformedWillPayload                   = Code{Code: 0x81, Reason: "malformed packet: will message"}
	ErrMalformedUsername                      = Code{Code: 0x81, Reason: "malformed packet: username"}
	ErrMalformedPassword                      = Code{Code: 0x81, Reason: "malformed packet: password"}
	ErrMalformedQos                           = Code{Code: 0x81, Reason: "malformed packet: qos"}
	ErrMalformedOffsetUintOutOfRange          = Code{Code: 0x81, Reason: "malformed packet: offset uint out of range"}
	ErrMalformedOffsetBytesOutOfRange         = Code{Code: 0x81, Reason: "malformed packet: offset bytes out of range"}
	ErrMalformedOffsetByteOutOfRange          = Code{Code: 0x81, Reason: "malformed packet: offset byte out of range"}
	ErrMalformedOffsetBoolOutOfRange          = Code{Code: 0x81, Reason: "malformed packet: offset boolean out of range"}
	ErrMalformedInvalidUTF8                   = Code{Code: 0x81, Reason: "malformed packet: invalid utf-8 string"}
	ErrMalformedVariableByteInteger           = Code{Code: 0x81, Reason: "malformed packet: variable byte integer out of range"}
	ErrMalformedBadProperty                   = Code{Code: 0x81, Reason: "malformed packet: unknown property"}
	ErrMalformedProperties                    = Code{Code: 0x81, Reason: "malformed packet: properties"}
	ErrMalformedWillProperties                = Code{Code: 0x81, Reason: "malformed packet: will properties"}
	ErrMalformedSessionPresent                = Code{Code: 0x81, Reason: "malformed packet: session present"}
	ErrMalformedReasonCode                    = Code{Code: 0x81, Reason: "malformed packet: reason code"}
	ErrProtocolViolation                      = Code{Code: 0x82, Reason: "protocol violation"}
	ErrProtocolViolationProtocolName          = Code{Code: 0x82, Reason: "protocol violation: protocol name"}
	ErrProtocolViolationProtocolVersion       = Code{Code: 0x82, Reason: "protocol violation: protocol version"}
	ErrProtocolViolationReservedBit           = Code{Code: 0x82, Reason: "protocol violation: reserved bit not 0"}
	ErrProtocolViolationFlagNoUsername        = Code{Code: 0x82, Reason: "protocol violation: username flag set but no value"}
	ErrProtocolViolationFlagNoPassword        = Code{Code: 0x82, Reason: "protocol violation: password flag set but no value"}
	ErrProtocolViolationUsernameNoFlag        = Code{Code: 0x82, Reason: "protocol violation: username set but no flag"}
	ErrProtocolViolationPasswordNoFlag        = Code{Code: 0x82, Reason: "protocol violation: username set but no flag"}
	ErrProtocolViolationPasswordTooLong       = Code{Code: 0x82, Reason: "protocol violation: password too long"}
	ErrProtocolViolationUsernameTooLong       = Code{Code: 0x82, Reason: "protocol violation: username too long"}
	ErrProtocolViolationNoPacketID            = Code{Code: 0x82, Reason: "protocol violation: missing packet id"}
	ErrProtocolViolationSurplusPacketID       = Code{Code: 0x82, Reason: "protocol violation: surplus packet id"}
	ErrProtocolViolationQosOutOfRange         = Code{Code: 0x82, Reason: "protocol violation: qos out of range"}
	ErrProtocolViolationSecondConnect         = Code{Code: 0x82, Reason: "protocol violation: second connect packet"}
	ErrProtocolViolationZeroNonZeroExpiry     = Code{Code: 0x82, Reason: "protocol violation: non-zero expiry"}
	ErrProtocolViolationRequireFirstConnect   = Code{Code: 0x82, Reason: "protocol violation: first packet must be connect"}
	ErrProtocolViolationWillFlagNoPayload     = Code{Code: 0x82, Reason: "protocol violation: will flag no payload"}
	ErrProtocolViolationWillFlagSurplusRetain = Code{Code: 0x82, Reason: "protocol violation: will flag surplus retain"}
	ErrProtocolViolationSurplusWildcard       = Code{Code: 0x82, Reason: "protocol violation: topic contains wildcards"}
	ErrProtocolViolationSurplusSubID          = Code{Code: 0x82, Reason: "protocol violation: contained subscription identifier"}
	ErrProtocolViolationInvalidTopic          = Code{Code: 0x82, Reason: "protocol violation: invalid topic"}
	ErrProtocolViolationInvalidSharedNoLocal  = Code{Code: 0x82, Reason: "protocol violation: invalid shared no local"}
	ErrProtocolViolationNoFilters             = Code{Code: 0x82, Reason: "protocol violation: must contain at least one filter"}
	ErrProtocolViolationInvalidReason         = Code{Code: 0x82, Reason: "protocol violation: invalid reason"}
	ErrProtocolViolationOversizeSubID         = Code{Code: 0x82, Reason: "protocol violation: oversize subscription id"}
	ErrProtocolViolationDupNoQos              = Code{Code: 0x82, Reason: "protocol violation: dup true with no qos"}
	ErrProtocolViolationUnsupportedProperty   = Code{Code: 0x82, Reason: "protocol violation: unsupported property"}
	ErrProtocolViolationNoTopic               = Code{Code: 0x82, Reason: "protocol violation: no topic or alias"}
	ErrImplementationSpecificError            = Code{Code: 0x83, Reason: "implementation specific error"}
	ErrRejectPacket                           = Code{Code: 0x83, Reason: "packet rejected"}
	ErrUnsupportedProtocolVersion             = Code{Code: 0x84, Reason: "unsupported protocol version"}
	ErrClientIdentifierNotValid               = Code{Code: 0x85, Reason: "client identifier not valid"}
	ErrClientIdentifierTooLong                = Code{Code: 0x85, Reason: "client identifier too long"}
	ErrBadUsernameOrPassword                  = Code{Code: 0x86, Reason: "bad username or password"}
	ErrNotAuthorized                          = Code{Code: 0x87, Reason: "not authorized"}
	ErrServerUnavailable                      = Code{Code: 0x88, Reason: "server unavailable"}
	ErrServerBusy                             = Code{Code: 0x89, Reason: "server busy"}
	ErrBanned                                 = Code{Code: 0x8A, Reason: "banned"}
	ErrServerShuttingDown                     = Code{Code: 0x8B, Reason: "server shutting down"}
	ErrBadAuthenticationMethod                = Code{Code: 0x8C, Reason: "bad authentication method"}
	ErrKeepAliveTimeout                       = Code{Code: 0x8D, Reason: "keep alive timeout"}
	ErrSessionTakenOver                       = Code{Code: 0x8E, Reason: "session takeover"}
	ErrTopicFilterInvalid                     = Code{Code: 0x8F, Reason: "topic filter invalid"}
	ErrTopicNameInvalid                       = Code{Code: 0x90, Reason: "topic name invalid"}
	ErrPacketIdentifierInUse                  = Code{Code: 0x91, Reason: "packet identifier in use"}
	ErrPacketIdentifierNotFound               = Code{Code: 0x92, Reason: "packet identifier not found"}
	ErrReceiveMaximum                         = Code{Code: 0x93, Reason: "receive maximum exceeded"}
	ErrTopicAliasInvalid                      = Code{Code: 0x94, Reason: "topic alias invalid"}
	ErrPacketTooLarge                         = Code{Code: 0x95, Reason: "packet too large"}
	ErrMessageRateTooHigh                     = Code{Code: 0x96, Reason: "message rate too high"}
	ErrQuotaExceeded                          = Code{Code: 0x97, Reason: "quota exceeded"}
	ErrPendingClientWritesExceeded            = Code{Code: 0x97, Reason: "too many pending writes"}
	ErrAdministrativeAction                   = Code{Code: 0x98, Reason: "administrative action"}
	ErrPayloadFormatInvalid                   = Code{Code: 0x99, Reason: "payload format invalid"}
	ErrRetainNotSupported                     = Code{Code: 0x9A, Reason: "retain not supported"}
	ErrQosNotSupported                        = Code{Code: 0x9B, Reason: "qos not supported"}
	ErrUseAnotherServer                       = Code{Code: 0x9C, Reason: "use another server"}
	ErrServerMoved                            = Code{Code: 0x9D, Reason: "server moved"}
	ErrSharedSubscriptionsNotSupported        = Code{Code: 0x9E, Reason: "shared subscriptions not supported"}
	ErrConnectionRateExceeded                 = Code{Code: 0x9F, Reason: "connection rate exceeded"}
	ErrMaxConnectTime                         = Code{Code: 0xA0, Reason: "maximum connect time"}
	ErrSubscriptionIdentifiersNotSupported    = Code{Code: 0xA1, Reason: "subscription identifiers not supported"}
	ErrWildcardSubscriptionsNotSupported      = Code{Code: 0xA2, Reason: "wildcard subscriptions not supported"}
	ErrInlineSubscriptionHandlerInvalid       = Code{Code: 0xA3, Reason: "inline subscription handler not valid."}

	// MQTTv3 specific bytes.
	Err3UnsupportedProtocolVersion = Code{Code: 0x01}
	Err3ClientIdentifierNotValid   = Code{Code: 0x02}
	Err3ServerUnavailable          = Code{Code: 0x03}
	ErrMalformedUsernameOrPassword = Code{Code: 0x04}
	Err3NotAuthorized              = Code{Code: 0x05}

	// V5CodesToV3 maps MQTTv5 Connack reason codes to MQTTv3 return codes.
	// This is required because MQTTv3 has different return byte specification.
	// See http://docs.oasis-open.org/mqtt/mqtt/v3.1.1/os/mqtt-v3.1.1-os.html#_Toc385349257
	V5CodesToV3 = map[Code]Code{
		ErrUnsupportedProtocolVersion: Err3UnsupportedProtocolVersion,
		ErrClientIdentifierNotValid:   Err3ClientIdentifierNotValid,
		ErrServerUnavailable:          Err3ServerUnavailable,
		ErrMalformedUsername:          ErrMalformedUsernameOrPassword,
		ErrMalformedPassword:          ErrMalformedUsernameOrPassword,
		ErrBadUsernameOrPassword:      Err3NotAuthorized,
	}
)
