// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package packets

// TPacketCase contains data for cross-checking the encoding and decoding
// of packets and expected scenarios.
type TPacketCase struct {
	RawBytes     []byte  // the bytes that make the packet
	ActualBytes  []byte  // the actual byte array that is created in the event of a byte mutation
	Group        string  // a group that should run the test, blank for all
	Desc         string  // a description of the test
	FailFirst    error   // expected fail result to be run immediately after the method is called
	Packet       *Packet // the packet that is Expected
	ActualPacket *Packet // the actual packet after mutations
	Expect       error   // generic Expected fail result to be checked
	Isolate      bool    // isolate can be used to isolate a test
	Primary      bool    // primary is a test that should be run using readPackets
	Case         byte    // the identifying byte of the case
}

// TPacketCases is a slice of TPacketCase.
type TPacketCases []TPacketCase

// Get returns a case matching a given T byte.
func (f TPacketCases) Get(b byte) TPacketCase {
	for _, v := range f {
		if v.Case == b {
			return v
		}
	}

	return TPacketCase{}
}

const (
	TConnectMqtt31 byte = iota
	TConnectMqtt311
	TConnectMqtt5
	TConnectMqtt5LWT
	TConnectClean
	TConnectUserPass
	TConnectUserPassLWT
	TConnectMalProtocolName
	TConnectMalProtocolVersion
	TConnectMalFlags
	TConnectMalKeepalive
	TConnectMalClientID
	TConnectMalWillTopic
	TConnectMalWillFlag
	TConnectMalUsername
	TConnectMalPassword
	TConnectMalFixedHeader
	TConnectMalReservedBit
	TConnectMalProperties
	TConnectMalWillProperties
	TConnectInvalidProtocolName
	TConnectInvalidProtocolVersion
	TConnectInvalidProtocolVersion2
	TConnectInvalidReservedBit
	TConnectInvalidClientIDTooLong
	TConnectInvalidFlagNoUsername
	TConnectInvalidFlagNoPassword
	TConnectInvalidUsernameNoFlag
	TConnectInvalidPasswordNoFlag
	TConnectInvalidUsernameTooLong
	TConnectInvalidPasswordTooLong
	TConnectInvalidWillFlagNoPayload
	TConnectInvalidWillFlagQosOutOfRange
	TConnectInvalidWillSurplusRetain
	TConnectZeroByteUsername
	TConnectSpecInvalidUTF8D800
	TConnectSpecInvalidUTF8DFFF
	TConnectSpecInvalidUTF80000
	TConnectSpecInvalidUTF8NoSkip
	TConnackAcceptedNoSession
	TConnackAcceptedSessionExists
	TConnackAcceptedMqtt5
	TConnackAcceptedAdjustedExpiryInterval
	TConnackMinMqtt5
	TConnackMinCleanMqtt5
	TConnackServerKeepalive
	TConnackInvalidMinMqtt5
	TConnackBadProtocolVersion
	TConnackProtocolViolationNoSession
	TConnackBadClientID
	TConnackServerUnavailable
	TConnackBadUsernamePassword
	TConnackBadUsernamePasswordNoSession
	TConnackMqtt5BadUsernamePasswordNoSession
	TConnackNotAuthorised
	TConnackMalSessionPresent
	TConnackMalReturnCode
	TConnackMalProperties
	TConnackDropProperties
	TConnackDropPropertiesPartial
	TPublishNoPayload
	TPublishBasic
	TPublishBasicTopicAliasOnly
	TPublishBasicMqtt5
	TPublishMqtt5
	TPublishQos1
	TPublishQos1Mqtt5
	TPublishQos1NoPayload
	TPublishQos1Dup
	TPublishQos2
	TPublishQos2Mqtt5
	TPublishQos2Upgraded
	TPublishSubscriberIdentifier
	TPublishRetain
	TPublishRetainMqtt5
	TPublishDup
	TPublishMalTopicName
	TPublishMalPacketID
	TPublishMalProperties
	TPublishCopyBasic
	TPublishSpecQos0NoPacketID
	TPublishSpecQosMustPacketID
	TPublishDropOversize
	TPublishInvalidQos0NoPacketID
	TPublishInvalidQosMustPacketID
	TPublishInvalidSurplusSubID
	TPublishInvalidSurplusWildcard
	TPublishInvalidSurplusWildcard2
	TPublishInvalidNoTopic
	TPublishInvalidTopicAlias
	TPublishInvalidExcessTopicAlias
	TPublishSpecDenySysTopic
	TPuback
	TPubackMqtt5
	TPubackMqtt5NotAuthorized
	TPubackMalPacketID
	TPubackMalProperties
	TPubackUnexpectedError
	TPubrec
	TPubrecMqtt5
	TPubrecMqtt5IDInUse
	TPubrecMqtt5NotAuthorized
	TPubrecMalPacketID
	TPubrecMalProperties
	TPubrecMalReasonCode
	TPubrecInvalidReason
	TPubrel
	TPubrelMqtt5
	TPubrelMqtt5AckNoPacket
	TPubrelMalPacketID
	TPubrelMalProperties
	TPubrelInvalidReason
	TPubcomp
	TPubcompMqtt5
	TPubcompMqtt5AckNoPacket
	TPubcompMalPacketID
	TPubcompMalProperties
	TPubcompInvalidReason
	TSubscribe
	TSubscribeMany
	TSubscribeMqtt5
	TSubscribeRetainHandling1
	TSubscribeRetainHandling2
	TSubscribeRetainAsPublished
	TSubscribeMalPacketID
	TSubscribeMalTopic
	TSubscribeMalQos
	TSubscribeMalQosRange
	TSubscribeMalProperties
	TSubscribeInvalidQosMustPacketID
	TSubscribeSpecQosMustPacketID
	TSubscribeInvalidNoFilters
	TSubscribeInvalidSharedNoLocal
	TSubscribeInvalidFilter
	TSubscribeInvalidIdentifierOversize
	TSuback
	TSubackMany
	TSubackDeny
	TSubackUnspecifiedError
	TSubackUnspecifiedErrorMqtt5
	TSubackMqtt5
	TSubackPacketIDInUse
	TSubackInvalidFilter
	TSubackInvalidSharedNoLocal
	TSubackMalPacketID
	TSubackMalProperties
	TUnsubscribe
	TUnsubscribeMany
	TUnsubscribeMqtt5
	TUnsubscribeMalPacketID
	TUnsubscribeMalTopicName
	TUnsubscribeMalProperties
	TUnsubscribeInvalidQosMustPacketID
	TUnsubscribeSpecQosMustPacketID
	TUnsubscribeInvalidNoFilters
	TUnsuback
	TUnsubackMany
	TUnsubackMqtt5
	TUnsubackPacketIDInUse
	TUnsubackMalPacketID
	TUnsubackMalProperties
	TPingreq
	TPingresp
	TDisconnect
	TDisconnectTakeover
	TDisconnectMqtt5
	TDisconnectSecondConnect
	TDisconnectReceiveMaximum
	TDisconnectDropProperties
	TDisconnectShuttingDown
	TDisconnectMalProperties
	TDisconnectMalReasonCode
	TDisconnectZeroNonZeroExpiry
	TAuth
	TAuthMalReasonCode
	TAuthMalProperties
	TAuthInvalidReason
	TAuthInvalidReason2
)

// TPacketData contains individual encoding and decoding scenarios for each packet type.
var TPacketData = map[byte]TPacketCases{
	Connect: {
		{
			Case:    TConnectMqtt31,
			Desc:    "mqtt v3.1",
			Primary: true,
			RawBytes: []byte{
				Connect << 4, 17, // Fixed header
				0, 6, // Protocol Name - MSB+LSB
				'M', 'Q', 'I', 's', 'd', 'p', // Protocol Name
				3,     // Protocol Version
				0,     // Packet Flags
				0, 30, // Keepalive
				0, 3, // Client ID - MSB+LSB
				'z', 'e', 'n', // Client ID "zen"
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Connect,
					Remaining: 17,
				},
				ProtocolVersion: 3,
				Connect: ConnectParams{
					ProtocolName:     []byte("MQIsdp"),
					Clean:            false,
					Keepalive:        30,
					ClientIdentifier: "zen",
				},
			},
		},
		{
			Case:    TConnectMqtt311,
			Desc:    "mqtt v3.1.1",
			Primary: true,
			RawBytes: []byte{
				Connect << 4, 15, // Fixed header
				0, 4, // Protocol Name - MSB+LSB
				'M', 'Q', 'T', 'T', // Protocol Name
				4,     // Protocol Version
				0,     // Packet Flags
				0, 60, // Keepalive
				0, 3, // Client ID - MSB+LSB
				'z', 'e', 'n', // Client ID "zen"
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Connect,
					Remaining: 15,
				},
				ProtocolVersion: 4,
				Connect: ConnectParams{
					ProtocolName:     []byte("MQTT"),
					Clean:            false,
					Keepalive:        60,
					ClientIdentifier: "zen",
				},
			},
		},
		{
			Case:    TConnectMqtt5,
			Desc:    "mqtt v5",
			Primary: true,
			RawBytes: []byte{
				Connect << 4, 87, // Fixed header
				0, 4, // Protocol Name - MSB+LSB
				'M', 'Q', 'T', 'T', // Protocol Name
				5,     // Protocol Version
				0,     // Packet Flags
				0, 30, // Keepalive

				// Properties
				71,               // length
				17, 0, 0, 0, 120, // Session Expiry Interval (17)
				21, 0, 5, 'S', 'H', 'A', '-', '1', // Authentication Method (21)
				22, 0, 9, 'a', 'u', 't', 'h', '-', 'd', 'a', 't', 'a', // Authentication Data (22)
				23, 1, // Request Problem Info (23)
				25, 1, // Request Response Info (25)
				33, 1, 244, // Receive Maximum (33)
				34, 3, 231, // Topic Alias Maximum (34)
				38, // User Properties (38)
				0, 5, 'h', 'e', 'l', 'l', 'o',
				0, 6, 228, 184, 150, 231, 149, 140,
				38, // User Properties (38)
				0, 4, 'k', 'e', 'y', '2',
				0, 6, 'v', 'a', 'l', 'u', 'e', '2',
				39, 0, 0, 125, 0, // Maximum Packet Size (39)

				0, 3, // Client ID - MSB+LSB
				'z', 'e', 'n', // Client ID "zen"
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Connect,
					Remaining: 87,
				},
				ProtocolVersion: 5,
				Connect: ConnectParams{
					ProtocolName:     []byte("MQTT"),
					Clean:            false,
					Keepalive:        30,
					ClientIdentifier: "zen",
				},
				Properties: Properties{
					SessionExpiryInterval:     uint32(120),
					SessionExpiryIntervalFlag: true,
					AuthenticationMethod:      "SHA-1",
					AuthenticationData:        []byte("auth-data"),
					RequestProblemInfo:        byte(1),
					RequestProblemInfoFlag:    true,
					RequestResponseInfo:       byte(1),
					ReceiveMaximum:            uint16(500),
					TopicAliasMaximum:         uint16(999),
					User: []UserProperty{
						{
							Key: "hello",
							Val: "世界",
						},
						{
							Key: "key2",
							Val: "value2",
						},
					},
					MaximumPacketSize: uint32(32000),
				},
			},
		},
		{
			Case: TConnectClean,
			Desc: "mqtt 3.1.1, clean session",
			RawBytes: []byte{
				Connect << 4, 15, // Fixed header
				0, 4, // Protocol Name - MSB+LSB
				'M', 'Q', 'T', 'T', // Protocol Name
				4,     // Protocol Version
				2,     // Packet Flags
				0, 45, // Keepalive
				0, 3, // Client ID - MSB+LSB
				'z', 'e', 'n', // Client ID "zen"
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Connect,
					Remaining: 15,
				},
				ProtocolVersion: 4,
				Connect: ConnectParams{
					ProtocolName:     []byte("MQTT"),
					Clean:            true,
					Keepalive:        45,
					ClientIdentifier: "zen",
				},
			},
		},
		{
			Case: TConnectMqtt5LWT,
			Desc: "mqtt 5 clean session, lwt",
			RawBytes: []byte{
				Connect << 4, 47, // Fixed header
				0, 4, // Protocol Name - MSB+LSB
				'M', 'Q', 'T', 'T', // Protocol Name
				5,     // Protocol Version
				14,    // Packet Flags
				0, 30, // Keepalive

				// Properties
				10,               // length
				17, 0, 0, 0, 120, // Session Expiry Interval (17)
				39, 0, 0, 125, 0, // Maximum Packet Size (39)
				0, 3, // Client ID - MSB+LSB
				'z', 'e', 'n', // Client ID "zen"
				5,               // will properties length
				24, 0, 0, 2, 88, // will delay interval (24)

				0, 3, // Will Topic - MSB+LSB
				'l', 'w', 't',
				0, 8, // Will Message MSB+LSB
				'n', 'o', 't', 'a', 'g', 'a', 'i', 'n',
			},
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Connect,
					Remaining: 42,
				},
				Connect: ConnectParams{
					ProtocolName:     []byte("MQTT"),
					Clean:            true,
					Keepalive:        30,
					ClientIdentifier: "zen",
					WillFlag:         true,
					WillTopic:        "lwt",
					WillPayload:      []byte("notagain"),
					WillQos:          1,
					WillProperties: Properties{
						WillDelayInterval: uint32(600),
					},
				},
				Properties: Properties{
					SessionExpiryInterval:     uint32(120),
					SessionExpiryIntervalFlag: true,
					MaximumPacketSize:         uint32(32000),
				},
			},
		},
		{
			Case: TConnectUserPass,
			Desc: "mqtt 3.1.1, username, password",
			RawBytes: []byte{
				Connect << 4, 28, // Fixed header
				0, 4, // Protocol Name - MSB+LSB
				'M', 'Q', 'T', 'T', // Protocol Name
				4,               // Protocol Version
				0 | 1<<6 | 1<<7, // Packet Flags
				0, 20,           // Keepalive
				0, 3, // Client ID - MSB+LSB
				'z', 'e', 'n', // Client ID "zen"
				0, 5, // Username MSB+LSB
				'm', 'o', 'c', 'h', 'i',
				0, 4, // Password MSB+LSB
				',', '.', '/', ';',
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Connect,
					Remaining: 28,
				},
				ProtocolVersion: 4,
				Connect: ConnectParams{
					ProtocolName:     []byte("MQTT"),
					Clean:            false,
					Keepalive:        20,
					ClientIdentifier: "zen",
					UsernameFlag:     true,
					PasswordFlag:     true,
					Username:         []byte("mochi"),
					Password:         []byte(",./;"),
				},
			},
		},
		{
			Case:    TConnectUserPassLWT,
			Desc:    "mqtt 3.1.1, username, password, lwt",
			Primary: true,
			RawBytes: []byte{
				Connect << 4, 44, // Fixed header
				0, 4, // Protocol Name - MSB+LSB
				'M', 'Q', 'T', 'T', // Protocol Name
				4,      // Protocol Version
				206,    // Packet Flags
				0, 120, // Keepalive
				0, 3, // Client ID - MSB+LSB
				'z', 'e', 'n', // Client ID "zen"
				0, 3, // Will Topic - MSB+LSB
				'l', 'w', 't',
				0, 9, // Will Message MSB+LSB
				'n', 'o', 't', ' ', 'a', 'g', 'a', 'i', 'n',
				0, 5, // Username MSB+LSB
				'm', 'o', 'c', 'h', 'i',
				0, 4, // Password MSB+LSB
				',', '.', '/', ';',
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Connect,
					Remaining: 44,
				},
				ProtocolVersion: 4,
				Connect: ConnectParams{
					ProtocolName:     []byte("MQTT"),
					Clean:            true,
					Keepalive:        120,
					ClientIdentifier: "zen",
					UsernameFlag:     true,
					PasswordFlag:     true,
					Username:         []byte("mochi"),
					Password:         []byte(",./;"),
					WillFlag:         true,
					WillTopic:        "lwt",
					WillPayload:      []byte("not again"),
					WillQos:          1,
				},
			},
		},
		{
			Case:  TConnectZeroByteUsername,
			Desc:  "username flag but 0 byte username",
			Group: "decode",
			RawBytes: []byte{
				Connect << 4, 23, // Fixed header
				0, 4, // Protocol Name - MSB+LSB
				'M', 'Q', 'T', 'T', // Protocol Name
				5,     // Protocol Version
				130,   // Packet Flags
				0, 30, // Keepalive
				5,                // length
				17, 0, 0, 0, 120, // Session Expiry Interval (17)
				0, 3, // Client ID - MSB+LSB
				'z', 'e', 'n', // Client ID "zen"
				0, 0, // Username MSB+LSB
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Connect,
					Remaining: 23,
				},
				ProtocolVersion: 5,
				Connect: ConnectParams{
					ProtocolName:     []byte("MQTT"),
					Clean:            true,
					Keepalive:        30,
					ClientIdentifier: "zen",
					Username:         []byte{},
					UsernameFlag:     true,
				},
				Properties: Properties{
					SessionExpiryInterval:     uint32(120),
					SessionExpiryIntervalFlag: true,
				},
			},
		},

		// Fail States
		{
			Case:      TConnectMalProtocolName,
			Desc:      "malformed protocol name",
			Group:     "decode",
			FailFirst: ErrMalformedProtocolName,
			RawBytes: []byte{
				Connect << 4, 0, // Fixed header
				0, 7, // Protocol Name - MSB+LSB
				'M', 'Q', 'I', 's', 'd', // Protocol Name
			},
		},
		{
			Case:      TConnectMalProtocolVersion,
			Desc:      "malformed protocol version",
			Group:     "decode",
			FailFirst: ErrMalformedProtocolVersion,
			RawBytes: []byte{
				Connect << 4, 0, // Fixed header
				0, 4, // Protocol Name - MSB+LSB
				'M', 'Q', 'T', 'T', // Protocol Name
			},
		},
		{
			Case:      TConnectMalFlags,
			Desc:      "malformed flags",
			Group:     "decode",
			FailFirst: ErrMalformedFlags,
			RawBytes: []byte{
				Connect << 4, 0, // Fixed header
				0, 4, // Protocol Name - MSB+LSB
				'M', 'Q', 'T', 'T', // Protocol Name
				4, // Protocol Version

			},
		},
		{
			Case:      TConnectMalKeepalive,
			Desc:      "malformed keepalive",
			Group:     "decode",
			FailFirst: ErrMalformedKeepalive,
			RawBytes: []byte{
				Connect << 4, 0, // Fixed header
				0, 4, // Protocol Name - MSB+LSB
				'M', 'Q', 'T', 'T', // Protocol Name
				4, // Protocol Version
				0, // Flags
			},
		},
		{
			Case:      TConnectMalClientID,
			Desc:      "malformed client id",
			Group:     "decode",
			FailFirst: ErrClientIdentifierNotValid,
			RawBytes: []byte{
				Connect << 4, 0, // Fixed header
				0, 4, // Protocol Name - MSB+LSB
				'M', 'Q', 'T', 'T', // Protocol Name
				4,     // Protocol Version
				0,     // Flags
				0, 20, // Keepalive
				0, 3, // Client ID - MSB+LSB
				'z', 'e', // Client ID "zen"
			},
		},
		{
			Case:      TConnectMalWillTopic,
			Desc:      "malformed will topic",
			Group:     "decode",
			FailFirst: ErrMalformedWillTopic,
			RawBytes: []byte{
				Connect << 4, 0, // Fixed header
				0, 4, // Protocol Name - MSB+LSB
				'M', 'Q', 'T', 'T', // Protocol Name
				4,     // Protocol Version
				14,    // Flags
				0, 20, // Keepalive
				0, 3, // Client ID - MSB+LSB
				'z', 'e', 'n', // Client ID "zen"
				0, 6, // Will Topic - MSB+LSB
				'l',
			},
		},
		{
			Case:      TConnectMalWillFlag,
			Desc:      "malformed will flag",
			Group:     "decode",
			FailFirst: ErrMalformedWillPayload,
			RawBytes: []byte{
				Connect << 4, 0, // Fixed header
				0, 4, // Protocol Name - MSB+LSB
				'M', 'Q', 'T', 'T', // Protocol Name
				4,     // Protocol Version
				14,    // Flags
				0, 20, // Keepalive
				0, 3, // Client ID - MSB+LSB
				'z', 'e', 'n', // Client ID "zen"
				0, 3, // Will Topic - MSB+LSB
				'l', 'w', 't',
				0, 9, // Will Message MSB+LSB
				'n', 'o', 't', ' ', 'a',
			},
		},
		{
			Case:      TConnectMalUsername,
			Desc:      "malformed username",
			Group:     "decode",
			FailFirst: ErrMalformedUsername,
			RawBytes: []byte{
				Connect << 4, 0, // Fixed header
				0, 4, // Protocol Name - MSB+LSB
				'M', 'Q', 'T', 'T', // Protocol Name
				4,     // Protocol Version
				206,   // Flags
				0, 20, // Keepalive
				0, 3, // Client ID - MSB+LSB
				'z', 'e', 'n', // Client ID "zen"
				0, 3, // Will Topic - MSB+LSB
				'l', 'w', 't',
				0, 9, // Will Message MSB+LSB
				'n', 'o', 't', ' ', 'a', 'g', 'a', 'i', 'n',
				0, 5, // Username MSB+LSB
				'm', 'o', 'c',
			},
		},

		{
			Case:      TConnectInvalidFlagNoUsername,
			Desc:      "username flag with no username bytes",
			Group:     "decode",
			FailFirst: ErrProtocolViolationFlagNoUsername,
			RawBytes: []byte{
				Connect << 4, 17, // Fixed header
				0, 4, // Protocol Name - MSB+LSB
				'M', 'Q', 'T', 'T', // Protocol Name
				5,     // Protocol Version
				130,   // Flags
				0, 20, // Keepalive
				0,
				0, 3, // Client ID - MSB+LSB
				'z', 'e', 'n', // Client ID "zen"
			},
		},
		{
			Case:      TConnectMalPassword,
			Desc:      "malformed password",
			Group:     "decode",
			FailFirst: ErrMalformedPassword,
			RawBytes: []byte{
				Connect << 4, 0, // Fixed header
				0, 4, // Protocol Name - MSB+LSB
				'M', 'Q', 'T', 'T', // Protocol Name
				4,     // Protocol Version
				206,   // Flags
				0, 20, // Keepalive
				0, 3, // Client ID - MSB+LSB
				'z', 'e', 'n', // Client ID "zen"
				0, 3, // Will Topic - MSB+LSB
				'l', 'w', 't',
				0, 9, // Will Message MSB+LSB
				'n', 'o', 't', ' ', 'a', 'g', 'a', 'i', 'n',
				0, 5, // Username MSB+LSB
				'm', 'o', 'c', 'h', 'i',
				0, 4, // Password MSB+LSB
				',', '.',
			},
		},
		{
			Case:      TConnectMalFixedHeader,
			Desc:      "malformed fixedheader oversize",
			Group:     "decode",
			FailFirst: ErrMalformedProtocolName, // packet test doesn't test fixedheader oversize
			RawBytes: []byte{
				Connect << 4, 255, 255, 255, 255, 255, // Fixed header
			},
		},
		{
			Case:      TConnectMalReservedBit,
			Desc:      "reserved bit not 0",
			Group:     "nodecode",
			FailFirst: ErrProtocolViolation,
			RawBytes: []byte{
				Connect << 4, 15, // Fixed header
				0, 4, // Protocol Name - MSB+LSB
				'M', 'Q', 'T', 'T', // Protocol Name
				4,     // Protocol Version
				1,     // Packet Flags
				0, 45, // Keepalive
				0, 3, // Client ID - MSB+LSB
				'z', 'e', 'n',
			},
		},
		{
			Case:      TConnectMalProperties,
			Desc:      "malformed properties",
			Group:     "decode",
			FailFirst: ErrMalformedProperties,
			RawBytes: []byte{
				Connect << 4, 47, // Fixed header
				0, 4, // Protocol Name - MSB+LSB
				'M', 'Q', 'T', 'T', // Protocol Name
				5,     // Protocol Version
				14,    // Packet Flags
				0, 30, // Keepalive
				10,
			},
			Packet: &Packet{
				ProtocolVersion: 5,
			},
		},
		{
			Case:      TConnectMalWillProperties,
			Desc:      "malformed will properties",
			Group:     "decode",
			FailFirst: ErrMalformedWillProperties,
			RawBytes: []byte{
				Connect << 4, 47, // Fixed header
				0, 4, // Protocol Name - MSB+LSB
				'M', 'Q', 'T', 'T', // Protocol Name
				5,     // Protocol Version
				14,    // Packet Flags
				0, 30, // Keepalive
				10,               // length
				17, 0, 0, 0, 120, // Session Expiry Interval (17)
				39, 0, 0, 125, 0, // Maximum Packet Size (39)
				0, 3, // Client ID - MSB+LSB
				'z', 'e', 'n', // Client ID "zen"
				5, // will properties length
			},
			Packet: &Packet{
				ProtocolVersion: 5,
			},
		},

		// Validation Tests
		{
			Case:   TConnectInvalidProtocolName,
			Desc:   "invalid protocol name",
			Group:  "validate",
			Expect: ErrProtocolViolationProtocolName,
			Packet: &Packet{
				FixedHeader: FixedHeader{Type: Connect},
				Connect: ConnectParams{
					ProtocolName: []byte("stuff"),
				},
			},
		},
		{
			Case:   TConnectInvalidProtocolVersion,
			Desc:   "invalid protocol version",
			Group:  "validate",
			Expect: ErrProtocolViolationProtocolVersion,
			Packet: &Packet{
				FixedHeader:     FixedHeader{Type: Connect},
				ProtocolVersion: 2,
				Connect: ConnectParams{
					ProtocolName: []byte("MQTT"),
				},
			},
		},
		{
			Case:   TConnectInvalidProtocolVersion2,
			Desc:   "invalid protocol version",
			Group:  "validate",
			Expect: ErrProtocolViolationProtocolVersion,
			Packet: &Packet{
				FixedHeader:     FixedHeader{Type: Connect},
				ProtocolVersion: 2,
				Connect: ConnectParams{
					ProtocolName: []byte("MQIsdp"),
				},
			},
		},
		{
			Case:   TConnectInvalidReservedBit,
			Desc:   "reserved bit not 0",
			Group:  "validate",
			Expect: ErrProtocolViolationReservedBit,
			Packet: &Packet{
				FixedHeader:     FixedHeader{Type: Connect},
				ProtocolVersion: 4,
				Connect: ConnectParams{
					ProtocolName: []byte("MQTT"),
				},
				ReservedBit: 1,
			},
		},
		{
			Case:   TConnectInvalidClientIDTooLong,
			Desc:   "client id too long",
			Group:  "validate",
			Expect: ErrClientIdentifierNotValid,
			Packet: &Packet{
				FixedHeader:     FixedHeader{Type: Connect},
				ProtocolVersion: 4,
				Connect: ConnectParams{
					ProtocolName: []byte("MQTT"),
					ClientIdentifier: func() string {
						return string(make([]byte, 65536))
					}(),
				},
			},
		},
		{
			Case:   TConnectInvalidUsernameNoFlag,
			Desc:   "has username but no flag",
			Group:  "validate",
			Expect: ErrProtocolViolationUsernameNoFlag,
			Packet: &Packet{
				FixedHeader:     FixedHeader{Type: Connect},
				ProtocolVersion: 4,
				Connect: ConnectParams{
					ProtocolName: []byte("MQTT"),
					Username:     []byte("username"),
				},
			},
		},
		{
			Case:   TConnectInvalidFlagNoPassword,
			Desc:   "has password flag but no password",
			Group:  "validate",
			Expect: ErrProtocolViolationFlagNoPassword,
			Packet: &Packet{
				FixedHeader:     FixedHeader{Type: Connect},
				ProtocolVersion: 4,
				Connect: ConnectParams{
					ProtocolName: []byte("MQTT"),
					PasswordFlag: true,
				},
			},
		},
		{
			Case:   TConnectInvalidPasswordNoFlag,
			Desc:   "has password flag but no password",
			Group:  "validate",
			Expect: ErrProtocolViolationPasswordNoFlag,
			Packet: &Packet{
				FixedHeader:     FixedHeader{Type: Connect},
				ProtocolVersion: 4,
				Connect: ConnectParams{
					ProtocolName: []byte("MQTT"),
					Password:     []byte("password"),
				},
			},
		},
		{
			Case:   TConnectInvalidUsernameTooLong,
			Desc:   "username too long",
			Group:  "validate",
			Expect: ErrProtocolViolationUsernameTooLong,
			Packet: &Packet{
				FixedHeader:     FixedHeader{Type: Connect},
				ProtocolVersion: 4,
				Connect: ConnectParams{
					ProtocolName: []byte("MQTT"),
					UsernameFlag: true,
					Username: func() []byte {
						return make([]byte, 65536)
					}(),
				},
			},
		},
		{
			Case:   TConnectInvalidPasswordTooLong,
			Desc:   "password too long",
			Group:  "validate",
			Expect: ErrProtocolViolationPasswordTooLong,
			Packet: &Packet{
				FixedHeader:     FixedHeader{Type: Connect},
				ProtocolVersion: 4,
				Connect: ConnectParams{
					ProtocolName: []byte("MQTT"),
					UsernameFlag: true,
					Username:     []byte{},
					PasswordFlag: true,
					Password: func() []byte {
						return make([]byte, 65536)
					}(),
				},
			},
		},
		{
			Case:   TConnectInvalidWillFlagNoPayload,
			Desc:   "will flag no payload",
			Group:  "validate",
			Expect: ErrProtocolViolationWillFlagNoPayload,
			Packet: &Packet{
				FixedHeader:     FixedHeader{Type: Connect},
				ProtocolVersion: 4,
				Connect: ConnectParams{
					ProtocolName: []byte("MQTT"),
					WillFlag:     true,
				},
			},
		},
		{
			Case:   TConnectInvalidWillFlagQosOutOfRange,
			Desc:   "will flag no payload",
			Group:  "validate",
			Expect: ErrProtocolViolationQosOutOfRange,
			Packet: &Packet{
				FixedHeader:     FixedHeader{Type: Connect},
				ProtocolVersion: 4,
				Connect: ConnectParams{
					ProtocolName: []byte("MQTT"),
					WillFlag:     true,
					WillTopic:    "a/b/c",
					WillPayload:  []byte{'b'},
					WillQos:      4,
				},
			},
		},
		{
			Case:   TConnectInvalidWillSurplusRetain,
			Desc:   "no will flag surplus retain",
			Group:  "validate",
			Expect: ErrProtocolViolationWillFlagSurplusRetain,
			Packet: &Packet{
				FixedHeader:     FixedHeader{Type: Connect},
				ProtocolVersion: 4,
				Connect: ConnectParams{
					ProtocolName: []byte("MQTT"),
					WillRetain:   true,
				},
			},
		},

		// Spec Tests
		{
			Case:      TConnectSpecInvalidUTF8D800,
			Desc:      "invalid utf8 string (a) - code point U+D800",
			Group:     "decode",
			FailFirst: ErrClientIdentifierNotValid,
			RawBytes: []byte{
				Connect << 4, 0, // Fixed header
				0, 4, // Protocol Name - MSB+LSB
				'M', 'Q', 'T', 'T', // Protocol Name
				4,     // Protocol Version
				0,     // Flags
				0, 20, // Keepalive
				0, 4, // Client ID - MSB+LSB
				'e', 0xed, 0xa0, 0x80, // Client id bearing U+D800
			},
		},
		{
			Case:      TConnectSpecInvalidUTF8DFFF,
			Desc:      "invalid utf8 string (b) - code point U+DFFF",
			Group:     "decode",
			FailFirst: ErrClientIdentifierNotValid,
			RawBytes: []byte{
				Connect << 4, 0, // Fixed header
				0, 4, // Protocol Name - MSB+LSB
				'M', 'Q', 'T', 'T', // Protocol Name
				4,     // Protocol Version
				0,     // Flags
				0, 20, // Keepalive
				0, 4, // Client ID - MSB+LSB
				'e', 0xed, 0xa3, 0xbf, // Client id bearing U+D8FF
			},
		},

		{
			Case:      TConnectSpecInvalidUTF80000,
			Desc:      "invalid utf8 string (c) - code point U+0000",
			Group:     "decode",
			FailFirst: ErrClientIdentifierNotValid,
			RawBytes: []byte{
				Connect << 4, 0, // Fixed header
				0, 4, // Protocol Name - MSB+LSB
				'M', 'Q', 'T', 'T', // Protocol Name
				4,     // Protocol Version
				0,     // Flags
				0, 20, // Keepalive
				0, 3, // Client ID - MSB+LSB
				'e', 0xc0, 0x80, // Client id bearing U+0000
			},
		},

		{
			Case: TConnectSpecInvalidUTF8NoSkip,
			Desc: "utf8 string must not skip or strip code point U+FEFF",
			//Group: "decode",
			//FailFirst: ErrMalformedClientID,
			RawBytes: []byte{
				Connect << 4, 18, // Fixed header
				0, 4, // Protocol Name - MSB+LSB
				'M', 'Q', 'T', 'T', // Protocol Name
				4,     // Protocol Version
				0,     // Flags
				0, 20, // Keepalive
				0, 6, // Client ID - MSB+LSB
				'e', 'b', 0xEF, 0xBB, 0xBF, 'd', // Client id bearing U+FEFF
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Connect,
					Remaining: 16,
				},
				ProtocolVersion: 4,
				Connect: ConnectParams{
					ProtocolName:     []byte("MQTT"),
					Keepalive:        20,
					ClientIdentifier: string([]byte{'e', 'b', 0xEF, 0xBB, 0xBF, 'd'}),
				},
			},
		},
	},
	Connack: {
		{
			Case:    TConnackAcceptedNoSession,
			Desc:    "accepted, no session",
			Primary: true,
			RawBytes: []byte{
				Connack << 4, 2, // fixed header
				0, // No existing session
				CodeSuccess.Code,
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Connack,
					Remaining: 2,
				},
				SessionPresent: false,
				ReasonCode:     CodeSuccess.Code,
			},
		},
		{
			Case:    TConnackAcceptedSessionExists,
			Desc:    "accepted, session exists",
			Primary: true,
			RawBytes: []byte{
				Connack << 4, 2, // fixed header
				1, // Session present
				CodeSuccess.Code,
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Connack,
					Remaining: 2,
				},
				SessionPresent: true,
				ReasonCode:     CodeSuccess.Code,
			},
		},
		{
			Case:    TConnackAcceptedAdjustedExpiryInterval,
			Desc:    "accepted, no session, adjusted expiry interval mqtt5",
			Primary: true,
			RawBytes: []byte{
				Connack << 4, 8, // fixed header
				0, // Session present
				CodeSuccess.Code,
				5,                // length
				17, 0, 0, 0, 120, // Session Expiry Interval (17)
			},
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Connack,
					Remaining: 8,
				},
				ReasonCode: CodeSuccess.Code,
				Properties: Properties{
					SessionExpiryInterval:     uint32(120),
					SessionExpiryIntervalFlag: true,
				},
			},
		},
		{
			Case:    TConnackAcceptedMqtt5,
			Desc:    "accepted no session mqtt5",
			Primary: true,
			RawBytes: []byte{
				Connack << 4, 124, // fixed header
				0, // No existing session
				CodeSuccess.Code,
				// Properties
				121,              // length
				17, 0, 0, 0, 120, // Session Expiry Interval (17)
				18, 0, 8, 'm', 'o', 'c', 'h', 'i', '-', 'v', '5', // Assigned Client ID (18)
				19, 0, 20, // Server Keep Alive (19)
				21, 0, 5, 'S', 'H', 'A', '-', '1', // Authentication Method (21)
				22, 0, 9, 'a', 'u', 't', 'h', '-', 'd', 'a', 't', 'a', // Authentication Data (22)
				26, 0, 8, 'r', 'e', 's', 'p', 'o', 'n', 's', 'e', // Response Info (26)
				28, 0, 7, 'm', 'o', 'c', 'h', 'i', '-', '2', // Server Reference (28)
				31, 0, 6, 'r', 'e', 'a', 's', 'o', 'n', // Reason String (31)
				33, 1, 244, // Receive Maximum (33)
				34, 3, 231, // Topic Alias Maximum (34)
				36, 1, // Maximum Qos (36)
				37, 1, // Retain Available (37)
				38, // User Properties (38)
				0, 5, 'h', 'e', 'l', 'l', 'o',
				0, 6, 228, 184, 150, 231, 149, 140,
				38, // User Properties (38)
				0, 4, 'k', 'e', 'y', '2',
				0, 6, 'v', 'a', 'l', 'u', 'e', '2',
				39, 0, 0, 125, 0, // Maximum Packet Size (39)
				40, 1, // Wildcard Subscriptions Available (40)
				41, 1, // Subscription ID Available (41)
				42, 1, // Shared Subscriptions Available (42)
			},
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Connack,
					Remaining: 124,
				},
				SessionPresent: false,
				ReasonCode:     CodeSuccess.Code,
				Properties: Properties{
					SessionExpiryInterval:     uint32(120),
					SessionExpiryIntervalFlag: true,
					AssignedClientID:          "mochi-v5",
					ServerKeepAlive:           uint16(20),
					ServerKeepAliveFlag:       true,
					AuthenticationMethod:      "SHA-1",
					AuthenticationData:        []byte("auth-data"),
					ResponseInfo:              "response",
					ServerReference:           "mochi-2",
					ReasonString:              "reason",
					ReceiveMaximum:            uint16(500),
					TopicAliasMaximum:         uint16(999),
					MaximumQos:                byte(1),
					MaximumQosFlag:            true,
					RetainAvailable:           byte(1),
					RetainAvailableFlag:       true,
					User: []UserProperty{
						{
							Key: "hello",
							Val: "世界",
						},
						{
							Key: "key2",
							Val: "value2",
						},
					},
					MaximumPacketSize:        uint32(32000),
					WildcardSubAvailable:     byte(1),
					WildcardSubAvailableFlag: true,
					SubIDAvailable:           byte(1),
					SubIDAvailableFlag:       true,
					SharedSubAvailable:       byte(1),
					SharedSubAvailableFlag:   true,
				},
			},
		},
		{
			Case:    TConnackMinMqtt5,
			Desc:    "accepted min properties mqtt5",
			Primary: true,
			RawBytes: []byte{
				Connack << 4, 13, // fixed header
				1, // existing session
				CodeSuccess.Code,
				10,                                // Properties length
				18, 0, 5, 'm', 'o', 'c', 'h', 'i', // Assigned Client ID (18)
				36, 1, // Maximum Qos (36)
			},
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Connack,
					Remaining: 13,
				},
				SessionPresent: true,
				ReasonCode:     CodeSuccess.Code,
				Properties: Properties{
					AssignedClientID: "mochi",
					MaximumQos:       byte(1),
					MaximumQosFlag:   true,
				},
			},
		},
		{
			Case:    TConnackMinCleanMqtt5,
			Desc:    "accepted min properties mqtt5b",
			Primary: true,
			RawBytes: []byte{
				Connack << 4, 3, // fixed header
				0, // existing session
				CodeSuccess.Code,
				0, // Properties length
			},
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Connack,
					Remaining: 16,
				},
				SessionPresent: false,
				ReasonCode:     CodeSuccess.Code,
			},
		},
		{
			Case:    TConnackServerKeepalive,
			Desc:    "server set keepalive",
			Primary: true,
			RawBytes: []byte{
				Connack << 4, 6, // fixed header
				1, // existing session
				CodeSuccess.Code,
				3,         // Properties length
				19, 0, 10, // server keepalive
			},
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Connack,
					Remaining: 6,
				},
				SessionPresent: true,
				ReasonCode:     CodeSuccess.Code,
				Properties: Properties{
					ServerKeepAlive:     uint16(10),
					ServerKeepAliveFlag: true,
				},
			},
		},
		{
			Case:    TConnackInvalidMinMqtt5,
			Desc:    "failure min properties mqtt5",
			Primary: true,
			RawBytes: append([]byte{
				Connack << 4, 23, // fixed header
				0, // No existing session
				ErrUnspecifiedError.Code,
				// Properties
				20,        // length
				31, 0, 17, // Reason String (31)
			}, []byte(ErrUnspecifiedError.Reason)...),
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Connack,
					Remaining: 23,
				},
				SessionPresent: false,
				ReasonCode:     ErrUnspecifiedError.Code,
				Properties: Properties{
					ReasonString: ErrUnspecifiedError.Reason,
				},
			},
		},

		{
			Case: TConnackProtocolViolationNoSession,
			Desc: "miscellaneous protocol violation",
			RawBytes: []byte{
				Connack << 4, 2, // fixed header
				0, // Session present
				ErrProtocolViolation.Code,
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Connack,
					Remaining: 2,
				},
				ReasonCode: ErrProtocolViolation.Code,
			},
		},
		{
			Case: TConnackBadProtocolVersion,
			Desc: "bad protocol version",
			RawBytes: []byte{
				Connack << 4, 2, // fixed header
				1, // Session present
				ErrProtocolViolationProtocolVersion.Code,
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Connack,
					Remaining: 2,
				},
				SessionPresent: true,
				ReasonCode:     ErrProtocolViolationProtocolVersion.Code,
			},
		},
		{
			Case: TConnackBadClientID,
			Desc: "bad client id",
			RawBytes: []byte{
				Connack << 4, 2, // fixed header
				1, // Session present
				ErrClientIdentifierNotValid.Code,
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Connack,
					Remaining: 2,
				},
				SessionPresent: true,
				ReasonCode:     ErrClientIdentifierNotValid.Code,
			},
		},
		{
			Case: TConnackServerUnavailable,
			Desc: "server unavailable",
			RawBytes: []byte{
				Connack << 4, 2, // fixed header
				1, // Session present
				ErrServerUnavailable.Code,
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Connack,
					Remaining: 2,
				},
				SessionPresent: true,
				ReasonCode:     ErrServerUnavailable.Code,
			},
		},
		{
			Case: TConnackBadUsernamePassword,
			Desc: "bad username or password",
			RawBytes: []byte{
				Connack << 4, 2, // fixed header
				1, // Session present
				ErrBadUsernameOrPassword.Code,
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Connack,
					Remaining: 2,
				},
				SessionPresent: true,
				ReasonCode:     ErrBadUsernameOrPassword.Code,
			},
		},
		{
			Case: TConnackBadUsernamePasswordNoSession,
			Desc: "bad username or password no session",
			RawBytes: []byte{
				Connack << 4, 2, // fixed header
				0,                      // No session present
				Err3NotAuthorized.Code, // use v3 remapping
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Connack,
					Remaining: 2,
				},
				ReasonCode: Err3NotAuthorized.Code,
			},
		},
		{
			Case: TConnackMqtt5BadUsernamePasswordNoSession,
			Desc: "mqtt5 bad username or password no session",
			RawBytes: []byte{
				Connack << 4, 3, // fixed header
				0, // No session present
				ErrBadUsernameOrPassword.Code,
				0,
			},
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Connack,
					Remaining: 2,
				},
				ReasonCode: ErrBadUsernameOrPassword.Code,
			},
		},

		{
			Case: TConnackNotAuthorised,
			Desc: "not authorised",
			RawBytes: []byte{
				Connack << 4, 2, // fixed header
				1, // Session present
				ErrNotAuthorized.Code,
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Connack,
					Remaining: 2,
				},
				SessionPresent: true,
				ReasonCode:     ErrNotAuthorized.Code,
			},
		},
		{
			Case:  TConnackDropProperties,
			Desc:  "drop oversize properties",
			Group: "encode",
			RawBytes: []byte{
				Connack << 4, 40, // fixed header
				0, // No existing session
				CodeSuccess.Code,
				19,                                          // length
				28, 0, 7, 'm', 'o', 'c', 'h', 'i', '-', '2', // Server Reference (28)
				31, 0, 6, 'r', 'e', 'a', 's', 'o', 'n', // Reason String (31)
				38, // User Properties (38)
				0, 5, 'h', 'e', 'l', 'l', 'o',
				0, 6, 228, 184, 150, 231, 149, 140,
			},
			ActualBytes: []byte{
				Connack << 4, 13, // fixed header
				0, // No existing session
				CodeSuccess.Code,
				10,                                          // length
				28, 0, 7, 'm', 'o', 'c', 'h', 'i', '-', '2', // Server Reference (28)
			},
			Packet: &Packet{
				Mods: Mods{
					MaxSize: 5,
				},
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Connack,
					Remaining: 40,
				},
				ReasonCode: CodeSuccess.Code,
				Properties: Properties{
					ReasonString:    "reason",
					ServerReference: "mochi-2",
					User: []UserProperty{
						{
							Key: "hello",
							Val: "世界",
						},
					},
				},
			},
		},
		{
			Case:  TConnackDropPropertiesPartial,
			Desc:  "drop oversize properties partial",
			Group: "encode",
			RawBytes: []byte{
				Connack << 4, 40, // fixed header
				0, // No existing session
				CodeSuccess.Code,
				19,                                          // length
				28, 0, 7, 'm', 'o', 'c', 'h', 'i', '-', '2', // Server Reference (28)
				31, 0, 6, 'r', 'e', 'a', 's', 'o', 'n', // Reason String (31)
				38, // User Properties (38)
				0, 5, 'h', 'e', 'l', 'l', 'o',
				0, 6, 228, 184, 150, 231, 149, 140,
			},
			ActualBytes: []byte{
				Connack << 4, 22, // fixed header
				0, // No existing session
				CodeSuccess.Code,
				19,                                          // length
				28, 0, 7, 'm', 'o', 'c', 'h', 'i', '-', '2', // Server Reference (28)
				31, 0, 6, 'r', 'e', 'a', 's', 'o', 'n', // Reason String (31)
			},
			Packet: &Packet{
				Mods: Mods{
					MaxSize: 18,
				},
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Connack,
					Remaining: 40,
				},
				ReasonCode: CodeSuccess.Code,
				Properties: Properties{
					ReasonString:    "reason",
					ServerReference: "mochi-2",
					User: []UserProperty{
						{
							Key: "hello",
							Val: "世界",
						},
					},
				},
			},
		},
		// Fail States
		{
			Case:      TConnackMalSessionPresent,
			Desc:      "malformed session present",
			Group:     "decode",
			FailFirst: ErrMalformedSessionPresent,
			RawBytes: []byte{
				Connect << 4, 2, // Fixed header
			},
		},
		{
			Case:  TConnackMalReturnCode,
			Desc:  "malformed bad return Code",
			Group: "decode",
			//Primary:   true,
			FailFirst: ErrMalformedReasonCode,
			RawBytes: []byte{
				Connect << 4, 2, // Fixed header
				0,
			},
		},
		{
			Case:      TConnackMalProperties,
			Desc:      "malformed properties",
			Group:     "decode",
			FailFirst: ErrMalformedProperties,
			RawBytes: []byte{
				Connack << 4, 40, // fixed header
				0, // No existing session
				CodeSuccess.Code,
				19, // length
			},
			Packet: &Packet{
				ProtocolVersion: 5,
			},
		},
	},

	Publish: {
		{
			Case:    TPublishNoPayload,
			Desc:    "no payload",
			Primary: true,
			RawBytes: []byte{
				Publish << 4, 7, // Fixed header
				0, 5, // Topic Name - LSB+MSB
				'a', '/', 'b', '/', 'c', // Topic Name
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Publish,
					Remaining: 7,
				},
				TopicName: "a/b/c",
				Payload:   []byte{},
			},
		},
		{
			Case:    TPublishBasic,
			Desc:    "basic",
			Primary: true,
			RawBytes: []byte{
				Publish << 4, 18, // Fixed header
				0, 5, // Topic Name - LSB+MSB
				'a', '/', 'b', '/', 'c', // Topic Name
				'h', 'e', 'l', 'l', 'o', ' ', 'm', 'o', 'c', 'h', 'i', // Payload
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Publish,
					Remaining: 18,
				},
				TopicName: "a/b/c",
				Payload:   []byte("hello mochi"),
			},
		},

		{
			Case:    TPublishMqtt5,
			Desc:    "mqtt v5",
			Primary: true,
			RawBytes: []byte{
				Publish << 4, 77, // Fixed header
				0, 5, // Topic Name - LSB+MSB
				'a', '/', 'b', '/', 'c', // Topic Name
				58,   // length
				1, 1, // Payload Format (1)
				2, 0, 0, 0, 2, // Message Expiry (2)
				3, 0, 10, 't', 'e', 'x', 't', '/', 'p', 'l', 'a', 'i', 'n', // Content Type (3)
				8, 0, 5, 'a', '/', 'b', '/', 'c', // Response Topic (8)
				9, 0, 4, 'd', 'a', 't', 'a', // Correlations Data (9)
				11, 202, 212, 19, // Subscription Identifier (11)
				35, 0, 3, // Topic Alias (35)
				38, // User Properties (38)
				0, 5, 'h', 'e', 'l', 'l', 'o',
				0, 6, 228, 184, 150, 231, 149, 140,
				'h', 'e', 'l', 'l', 'o', ' ', 'm', 'o', 'c', 'h', 'i', // Payload
			},
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Publish,
					Remaining: 77,
				},
				TopicName: "a/b/c",
				Properties: Properties{
					PayloadFormat:          byte(1), // UTF-8 Format
					PayloadFormatFlag:      true,
					MessageExpiryInterval:  uint32(2),
					ContentType:            "text/plain",
					ResponseTopic:          "a/b/c",
					CorrelationData:        []byte("data"),
					SubscriptionIdentifier: []int{322122},
					TopicAlias:             uint16(3),
					TopicAliasFlag:         true,
					User: []UserProperty{
						{
							Key: "hello",
							Val: "世界",
						},
					},
				},
				Payload: []byte("hello mochi"),
			},
		},
		{
			Case:    TPublishBasicTopicAliasOnly,
			Desc:    "mqtt v5 topic alias only",
			Primary: true,
			RawBytes: []byte{
				Publish << 4, 17, // Fixed header
				0, 0, // Topic Name - LSB+MSB
				3,        // length
				35, 0, 1, // Topic Alias (35)
				'h', 'e', 'l', 'l', 'o', ' ', 'm', 'o', 'c', 'h', 'i', // Payload
			},
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Publish,
					Remaining: 17,
				},
				Properties: Properties{
					TopicAlias:     1,
					TopicAliasFlag: true,
				},
				Payload: []byte("hello mochi"),
			},
		},
		{
			Case:    TPublishBasicMqtt5,
			Desc:    "mqtt basic v5",
			Primary: true,
			RawBytes: []byte{
				Publish << 4, 22, // Fixed header
				0, 5, // Topic Name - LSB+MSB
				'a', '/', 'b', '/', 'c', // Topic Name
				3,        // length
				35, 0, 1, // Topic Alias (35)
				'h', 'e', 'l', 'l', 'o', ' ', 'm', 'o', 'c', 'h', 'i', // Payload
			},
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Publish,
					Remaining: 22,
				},
				TopicName: "a/b/c",
				Properties: Properties{
					TopicAlias:     uint16(1),
					TopicAliasFlag: true,
				},
				Payload: []byte("hello mochi"),
			},
		},

		{
			Case:    TPublishQos1,
			Desc:    "qos:1, packet id",
			Primary: true,
			RawBytes: []byte{
				Publish<<4 | 1<<1, 20, // Fixed header
				0, 5, // Topic Name - LSB+MSB
				'a', '/', 'b', '/', 'c', // Topic Name
				0, 7, // Packet ID - LSB+MSB
				'h', 'e', 'l', 'l', 'o', ' ', 'm', 'o', 'c', 'h', 'i', // Payload
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Publish,
					Qos:       1,
					Remaining: 20,
				},
				TopicName: "a/b/c",
				Payload:   []byte("hello mochi"),
				PacketID:  7,
			},
		},
		{
			Case:    TPublishQos1Mqtt5,
			Desc:    "mqtt v5",
			Primary: true,
			RawBytes: []byte{
				Publish<<4 | 1<<1, 37, // Fixed header
				0, 5, // Topic Name - LSB+MSB
				'a', '/', 'b', '/', 'c', // Topic Name
				0, 7, // Packet ID - LSB+MSB
				// Properties
				16, // length
				38, // User Properties (38)
				0, 5, 'h', 'e', 'l', 'l', 'o',
				0, 6, 228, 184, 150, 231, 149, 140,
				'h', 'e', 'l', 'l', 'o', ' ', 'm', 'o', 'c', 'h', 'i', // Payload
			},
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Publish,
					Remaining: 37,
					Qos:       1,
				},
				PacketID:  7,
				TopicName: "a/b/c",
				Properties: Properties{
					User: []UserProperty{
						{
							Key: "hello",
							Val: "世界",
						},
					},
				},
				Payload: []byte("hello mochi"),
			},
		},

		{
			Case:    TPublishQos1Dup,
			Desc:    "qos:1, dup:true, packet id",
			Primary: true,
			RawBytes: []byte{
				Publish<<4 | 2 | 8, 20, // Fixed header
				0, 5, // Topic Name - LSB+MSB
				'a', '/', 'b', '/', 'c', // Topic Name
				0, 7, // Packet ID - LSB+MSB
				'h', 'e', 'l', 'l', 'o', ' ', 'm', 'o', 'c', 'h', 'i', // Payload
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Publish,
					Qos:       1,
					Remaining: 20,
					Dup:       true,
				},
				TopicName: "a/b/c",
				Payload:   []byte("hello mochi"),
				PacketID:  7,
			},
		},
		{
			Case:    TPublishQos1NoPayload,
			Desc:    "qos:1, packet id, no payload",
			Primary: true,
			RawBytes: []byte{
				Publish<<4 | 2, 9, // Fixed header
				0, 5, // Topic Name - LSB+MSB
				'y', '/', 'u', '/', 'i', // Topic Name
				0, 7, // Packet ID - LSB+MSB
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Publish,
					Qos:       1,
					Remaining: 9,
				},
				TopicName: "y/u/i",
				PacketID:  7,
				Payload:   []byte{},
			},
		},
		{
			Case:    TPublishQos2,
			Desc:    "qos:2, packet id",
			Primary: true,
			RawBytes: []byte{
				Publish<<4 | 2<<1, 14, // Fixed header
				0, 5, // Topic Name - LSB+MSB
				'a', '/', 'b', '/', 'c', // Topic Name
				0, 7, // Packet ID - LSB+MSB
				'h', 'e', 'l', 'l', 'o', // Payload
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Publish,
					Qos:       2,
					Remaining: 14,
				},
				TopicName: "a/b/c",
				Payload:   []byte("hello"),
				PacketID:  7,
			},
		},
		{
			Case:    TPublishQos2Mqtt5,
			Desc:    "mqtt v5",
			Primary: true,
			RawBytes: []byte{
				Publish<<4 | 2<<1, 37, // Fixed header
				0, 5, // Topic Name - LSB+MSB
				'a', '/', 'b', '/', 'c', // Topic Name
				0, 7, // Packet ID - LSB+MSB
				// Properties
				16, // length
				38, // User Properties (38)
				0, 5, 'h', 'e', 'l', 'l', 'o',
				0, 6, 228, 184, 150, 231, 149, 140,
				'h', 'e', 'l', 'l', 'o', ' ', 'm', 'o', 'c', 'h', 'i', // Payload
			},
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Publish,
					Remaining: 37,
					Qos:       2,
				},
				PacketID:  7,
				TopicName: "a/b/c",
				Properties: Properties{
					User: []UserProperty{
						{
							Key: "hello",
							Val: "世界",
						},
					},
				},
				Payload: []byte("hello mochi"),
			},
		},
		{
			Case:    TPublishSubscriberIdentifier,
			Desc:    "subscription identifiers",
			Primary: true,
			RawBytes: []byte{
				Publish << 4, 23, // Fixed header
				0, 5, // Topic Name - LSB+MSB
				'a', '/', 'b', '/', 'c', // Topic Name
				4,     // properties length
				11, 2, // Subscription Identifier (11)
				11, 3, // Subscription Identifier (11)
				'h', 'e', 'l', 'l', 'o', ' ', 'm', 'o', 'c', 'h', 'i', // Payload
			},
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Publish,
					Remaining: 23,
				},
				TopicName: "a/b/c",
				Properties: Properties{
					SubscriptionIdentifier: []int{2, 3},
				},
				Payload: []byte("hello mochi"),
			},
		},

		{
			Case:    TPublishQos2Upgraded,
			Desc:    "qos:2, upgraded from publish to qos2 sub",
			Primary: true,
			RawBytes: []byte{
				Publish<<4 | 2<<1, 20, // Fixed header
				0, 5, // Topic Name - LSB+MSB
				'a', '/', 'b', '/', 'c', // Topic Name
				0, 1, // Packet ID - LSB+MSB
				'h', 'e', 'l', 'l', 'o', ' ', 'm', 'o', 'c', 'h', 'i', // Payload
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Publish,
					Qos:       2,
					Remaining: 18,
				},
				TopicName: "a/b/c",
				Payload:   []byte("hello mochi"),
				PacketID:  1,
			},
		},
		{
			Case: TPublishRetain,
			Desc: "retain",
			RawBytes: []byte{
				Publish<<4 | 1<<0, 18, // Fixed header
				0, 5, // Topic Name - LSB+MSB
				'a', '/', 'b', '/', 'c', // Topic Name
				'h', 'e', 'l', 'l', 'o', ' ', 'm', 'o', 'c', 'h', 'i', // Payload
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:   Publish,
					Retain: true,
				},
				TopicName: "a/b/c",
				Payload:   []byte("hello mochi"),
			},
		},
		{
			Case: TPublishRetainMqtt5,
			Desc: "retain mqtt5",
			RawBytes: []byte{
				Publish<<4 | 1<<0, 19, // Fixed header
				0, 5, // Topic Name - LSB+MSB
				'a', '/', 'b', '/', 'c', // Topic Name
				0,                                                     // properties length
				'h', 'e', 'l', 'l', 'o', ' ', 'm', 'o', 'c', 'h', 'i', // Payload
			},
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Publish,
					Retain:    true,
					Remaining: 19,
				},
				TopicName:  "a/b/c",
				Properties: Properties{},
				Payload:    []byte("hello mochi"),
			},
		},
		{
			Case: TPublishDup,
			Desc: "dup",
			RawBytes: []byte{
				Publish<<4 | 8, 10, // Fixed header
				0, 3, // Topic Name - LSB+MSB
				'a', '/', 'b', // Topic Name
				'h', 'e', 'l', 'l', 'o', // Payload
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type: Publish,
					Dup:  true,
				},
				TopicName: "a/b",
				Payload:   []byte("hello"),
			},
		},

		// Fail States
		{
			Case:      TPublishMalTopicName,
			Desc:      "malformed topic name",
			Group:     "decode",
			FailFirst: ErrMalformedTopic,
			RawBytes: []byte{
				Publish << 4, 7, // Fixed header
				0, 5, // Topic Name - LSB+MSB
				'a', '/',
				0, 11, // Packet ID - LSB+MSB
			},
		},
		{
			Case:      TPublishMalPacketID,
			Desc:      "malformed packet id",
			Group:     "decode",
			FailFirst: ErrMalformedPacketID,
			RawBytes: []byte{
				Publish<<4 | 2, 7, // Fixed header
				0, 5, // Topic Name - LSB+MSB
				'x', '/', 'y', '/', 'z', // Topic Name
				0, // Packet ID - LSB+MSB
			},
		},
		{
			Case:      TPublishMalProperties,
			Desc:      "malformed properties",
			Group:     "decode",
			FailFirst: ErrMalformedProperties,
			RawBytes: []byte{
				Publish << 4, 35, // Fixed header
				0, 5, // Topic Name - LSB+MSB
				'a', '/', 'b', '/', 'c', // Topic Name
				16,
			},
			Packet: &Packet{
				ProtocolVersion: 5,
			},
		},

		// Copy tests
		{
			Case:  TPublishCopyBasic,
			Desc:  "basic copyable",
			Group: "copy",
			RawBytes: []byte{
				Publish << 4, 18, // Fixed header
				0, 5, // Topic Name - LSB+MSB
				'z', '/', 'e', '/', 'n', // Topic Name
				'm', 'o', 'c', 'h', 'i', ' ', 'm', 'o', 'c', 'h', 'i', // Payload
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:   Publish,
					Dup:    true,
					Retain: true,
					Qos:    1,
				},
				TopicName: "z/e/n",
				Payload:   []byte("mochi mochi"),
			},
		},

		// Spec tests
		{
			Case:  TPublishSpecQos0NoPacketID,
			Desc:  "packet id must be 0 if qos is 0 (a)",
			Group: "encode",
			// this version tests for correct byte array mutuation.
			// this does not check if -incoming- Packets are parsed as correct,
			// it is impossible for the parser to determine if the payload start is incorrect.
			RawBytes: []byte{
				Publish << 4, 12, // Fixed header
				0, 5, // Topic Name - LSB+MSB
				'a', '/', 'b', '/', 'c', // Topic Name
				0, 3, // Packet ID - LSB+MSB
				'h', 'e', 'l', 'l', 'o', // Payload
			},
			ActualBytes: []byte{
				Publish << 4, 12, // Fixed header
				0, 5, // Topic Name - LSB+MSB
				'a', '/', 'b', '/', 'c', // Topic Name
				// Packet ID is removed.
				'h', 'e', 'l', 'l', 'o', // Payload
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Publish,
					Remaining: 12,
				},
				TopicName: "a/b/c",
				Payload:   []byte("hello"),
			},
		},
		{
			Case:   TPublishSpecQosMustPacketID,
			Desc:   "no packet id with qos > 0",
			Group:  "encode",
			Expect: ErrProtocolViolationNoPacketID,
			RawBytes: []byte{
				Publish<<4 | 2, 14, // Fixed header
				0, 5, // Topic Name - LSB+MSB
				'a', '/', 'b', '/', 'c', // Topic Name
				0, 0, // Packet ID - LSB+MSB
				'h', 'e', 'l', 'l', 'o', // Payload
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type: Publish,
					Qos:  1,
				},
				TopicName: "a/b/c",
				Payload:   []byte("hello"),
				PacketID:  0,
			},
		},
		{
			Case:      TPublishDropOversize,
			Desc:      "drop oversized publish packet",
			Group:     "encode",
			FailFirst: ErrPacketTooLarge,
			RawBytes: []byte{
				Publish << 4, 10, // Fixed header
				0, 3, // Topic Name - LSB+MSB
				'a', '/', 'b', // Topic Name
				'h', 'e', 'l', 'l', 'o', // Payload
			},
			Packet: &Packet{
				Mods: Mods{
					MaxSize: 2,
				},
				FixedHeader: FixedHeader{
					Type: Publish,
				},
				TopicName: "a/b",
				Payload:   []byte("hello"),
			},
		},

		// Validation Tests
		{
			Case:   TPublishInvalidQos0NoPacketID,
			Desc:   "packet id must be 0 if qos is 0 (b)",
			Group:  "validate",
			Expect: ErrProtocolViolationSurplusPacketID,
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Publish,
					Remaining: 12,
					Qos:       0,
				},
				TopicName: "a/b/c",
				Payload:   []byte("hello"),
				PacketID:  3,
			},
		},
		{
			Case:   TPublishInvalidQosMustPacketID,
			Desc:   "no packet id with qos > 0",
			Group:  "validate",
			Expect: ErrProtocolViolationNoPacketID,
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type: Publish,
					Qos:  1,
				},
				PacketID: 0,
			},
		},
		{
			Case:   TPublishInvalidSurplusSubID,
			Desc:   "surplus subscription identifier",
			Group:  "validate",
			Expect: ErrProtocolViolationSurplusSubID,
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type: Publish,
				},
				Properties: Properties{
					SubscriptionIdentifier: []int{1},
				},
				TopicName: "a/b",
			},
		},
		{
			Case:   TPublishInvalidSurplusWildcard,
			Desc:   "topic contains wildcards",
			Group:  "validate",
			Expect: ErrProtocolViolationSurplusWildcard,
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type: Publish,
				},
				TopicName: "a/+",
			},
		},
		{
			Case:   TPublishInvalidSurplusWildcard2,
			Desc:   "topic contains wildcards 2",
			Group:  "validate",
			Expect: ErrProtocolViolationSurplusWildcard,
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type: Publish,
				},
				TopicName: "a/#",
			},
		},
		{
			Case:   TPublishInvalidNoTopic,
			Desc:   "no topic or alias specified",
			Group:  "validate",
			Expect: ErrProtocolViolationNoTopic,
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type: Publish,
				},
			},
		},
		{
			Case:   TPublishInvalidExcessTopicAlias,
			Desc:   "topic alias over maximum",
			Group:  "validate",
			Expect: ErrTopicAliasInvalid,
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type: Publish,
				},
				Properties: Properties{
					TopicAlias: 1025,
				},
				TopicName: "a/b",
			},
		},
		{
			Case:   TPublishInvalidTopicAlias,
			Desc:   "topic alias flag and no alias",
			Group:  "validate",
			Expect: ErrTopicAliasInvalid,
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type: Publish,
				},
				Properties: Properties{
					TopicAliasFlag: true,
					TopicAlias:     0,
				},
				TopicName: "a/b/",
			},
		},
		{
			Case:   TPublishSpecDenySysTopic,
			Desc:   "deny publishing to $SYS topics",
			Group:  "validate",
			Expect: CodeSuccess,
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type: Publish,
				},
				TopicName: "$SYS/any",
				Payload:   []byte("y"),
			},
			RawBytes: []byte{
				Publish << 4, 11, // Fixed header
				0, 5, // Topic Name - LSB+MSB
				'$', 'S', 'Y', 'S', '/', 'a', 'n', 'y', // Topic Name
				'y', // Payload
			},
		},
	},

	Puback: {
		{
			Case:    TPuback,
			Desc:    "puback",
			Primary: true,
			RawBytes: []byte{
				Puback << 4, 2, // Fixed header
				0, 7, // Packet ID - LSB+MSB
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Puback,
					Remaining: 2,
				},
				PacketID: 7,
			},
		},
		{
			Case:    TPubackMqtt5,
			Desc:    "mqtt5",
			Primary: true,
			RawBytes: []byte{
				Puback << 4, 20, // Fixed header
				0, 7, // Packet ID - LSB+MSB
				CodeGrantedQos0.Code, // Reason Code
				16,                   // Properties Length
				// 31, 0, 6, 'r', 'e', 'a', 's', 'o', 'n', // Reason String (31)
				38, // User Properties (38)
				0, 5, 'h', 'e', 'l', 'l', 'o',
				0, 6, 228, 184, 150, 231, 149, 140,
			},
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Puback,
					Remaining: 20,
				},
				PacketID:   7,
				ReasonCode: CodeGrantedQos0.Code,
				Properties: Properties{
					User: []UserProperty{
						{
							Key: "hello",
							Val: "世界",
						},
					},
				},
			},
		},
		{
			Case:    TPubackMqtt5NotAuthorized,
			Desc:    "QOS 1 publish not authorized mqtt5",
			Primary: true,
			RawBytes: []byte{
				Puback << 4, 37, // Fixed header
				0, 7, // Packet ID - LSB+MSB
				ErrNotAuthorized.Code, // Reason Code
				33,                    // Properties Length
				31, 0, 14, 'n', 'o', 't', ' ', 'a', 'u',
				't', 'h', 'o', 'r', 'i', 'z', 'e', 'd', // Reason String (31)
				38, // User Properties (38)
				0, 5, 'h', 'e', 'l', 'l', 'o',
				0, 6, 228, 184, 150, 231, 149, 140,
			},
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Puback,
					Remaining: 31,
				},
				PacketID:   7,
				ReasonCode: ErrNotAuthorized.Code,
				Properties: Properties{
					ReasonString: ErrNotAuthorized.Reason,
					User: []UserProperty{
						{
							Key: "hello",
							Val: "世界",
						},
					},
				},
			},
		},
		{
			Case:  TPubackUnexpectedError,
			Desc:  "unexpected error",
			Group: "decode",
			RawBytes: []byte{
				Puback << 4, 29, // Fixed header
				0, 7, // Packet ID - LSB+MSB
				ErrPayloadFormatInvalid.Code, // Reason Code
				25,                           // Properties Length
				31, 0, 22, 'p', 'a', 'y', 'l', 'o', 'a', 'd',
				' ', 'f', 'o', 'r', 'm', 'a', 't',
				' ', 'i', 'n', 'v', 'a', 'l', 'i', 'd', // Reason String (31)
			},
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Puback,
					Remaining: 28,
				},
				PacketID:   7,
				ReasonCode: ErrPayloadFormatInvalid.Code,
				Properties: Properties{
					ReasonString: ErrPayloadFormatInvalid.Reason,
				},
			},
		},

		// Fail states
		{
			Case:      TPubackMalPacketID,
			Desc:      "malformed packet id",
			Group:     "decode",
			FailFirst: ErrMalformedPacketID,
			RawBytes: []byte{
				Puback << 4, 2, // Fixed header
				0, // Packet ID - LSB+MSB
			},
		},
		{
			Case:      TPubackMalProperties,
			Desc:      "malformed properties",
			Group:     "decode",
			FailFirst: ErrMalformedProperties,
			RawBytes: []byte{
				Puback << 4, 20, // Fixed header
				0, 7, // Packet ID - LSB+MSB
				CodeGrantedQos0.Code, // Reason Code
				16,
			},
			Packet: &Packet{
				ProtocolVersion: 5,
			},
		},
	},
	Pubrec: {
		{
			Case:    TPubrec,
			Desc:    "pubrec",
			Primary: true,
			RawBytes: []byte{
				Pubrec << 4, 2, // Fixed header
				0, 7, // Packet ID - LSB+MSB
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Pubrec,
					Remaining: 2,
				},
				PacketID: 7,
			},
		},
		{
			Case:    TPubrecMqtt5,
			Desc:    "pubrec mqtt5",
			Primary: true,
			RawBytes: []byte{
				Pubrec << 4, 20, // Fixed header
				0, 7, // Packet ID - LSB+MSB
				CodeSuccess.Code, // Reason Code
				16,               // Properties Length
				38,               // User Properties (38)
				0, 5, 'h', 'e', 'l', 'l', 'o',
				0, 6, 228, 184, 150, 231, 149, 140,
			},
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Pubrec,
					Remaining: 20,
				},
				PacketID:   7,
				ReasonCode: CodeSuccess.Code,
				Properties: Properties{
					User: []UserProperty{
						{
							Key: "hello",
							Val: "世界",
						},
					},
				},
			},
		},
		{
			Case:    TPubrecMqtt5IDInUse,
			Desc:    "packet id in use mqtt5",
			Primary: true,
			RawBytes: []byte{
				Pubrec << 4, 47, // Fixed header
				0, 7, // Packet ID - LSB+MSB
				ErrPacketIdentifierInUse.Code, // Reason Code
				43,                            // Properties Length
				31, 0, 24, 'p', 'a', 'c', 'k', 'e', 't',
				' ', 'i', 'd', 'e', 'n', 't', 'i', 'f', 'i', 'e', 'r',
				' ', 'i', 'n',
				' ', 'u', 's', 'e', // Reason String (31)
				38, // User Properties (38)
				0, 5, 'h', 'e', 'l', 'l', 'o',
				0, 6, 228, 184, 150, 231, 149, 140,
			},
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Pubrec,
					Remaining: 31,
				},
				PacketID:   7,
				ReasonCode: ErrPacketIdentifierInUse.Code,
				Properties: Properties{
					ReasonString: ErrPacketIdentifierInUse.Reason,
					User: []UserProperty{
						{
							Key: "hello",
							Val: "世界",
						},
					},
				},
			},
		},
		{
			Case:    TPubrecMqtt5NotAuthorized,
			Desc:    "QOS 2 publish not authorized mqtt5",
			Primary: true,
			RawBytes: []byte{
				Pubrec << 4, 37, // Fixed header
				0, 7, // Packet ID - LSB+MSB
				ErrNotAuthorized.Code, // Reason Code
				33,                    // Properties Length
				31, 0, 14, 'n', 'o', 't', ' ', 'a', 'u',
				't', 'h', 'o', 'r', 'i', 'z', 'e', 'd', // Reason String (31)
				38, // User Properties (38)
				0, 5, 'h', 'e', 'l', 'l', 'o',
				0, 6, 228, 184, 150, 231, 149, 140,
			},
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Pubrec,
					Remaining: 31,
				},
				PacketID:   7,
				ReasonCode: ErrNotAuthorized.Code,
				Properties: Properties{
					ReasonString: ErrNotAuthorized.Reason,
					User: []UserProperty{
						{
							Key: "hello",
							Val: "世界",
						},
					},
				},
			},
		},
		{
			Case:      TPubrecMalReasonCode,
			Desc:      "malformed reason code",
			Group:     "decode",
			FailFirst: ErrMalformedReasonCode,
			RawBytes: []byte{
				Pubrec << 4, 31, // Fixed header
				0, 7, // Packet ID - LSB+MSB
			},
			Packet: &Packet{
				ProtocolVersion: 5,
			},
		},
		// Validation
		{
			Case:      TPubrecInvalidReason,
			Desc:      "invalid reason code",
			Group:     "validate",
			FailFirst: ErrProtocolViolationInvalidReason,
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type: Pubrec,
				},
				PacketID:   7,
				ReasonCode: ErrConnectionRateExceeded.Code,
			},
		},
		// Fail states
		{
			Case:      TPubrecMalPacketID,
			Desc:      "malformed packet id",
			Group:     "decode",
			FailFirst: ErrMalformedPacketID,
			RawBytes: []byte{
				Pubrec << 4, 2, // Fixed header
				0, // Packet ID - LSB+MSB
			},
		},
		{
			Case:      TPubrecMalProperties,
			Desc:      "malformed properties",
			Group:     "decode",
			FailFirst: ErrMalformedProperties,
			RawBytes: []byte{
				Pubrec << 4, 31, // Fixed header
				0, 7, // Packet ID - LSB+MSB
				ErrPacketIdentifierInUse.Code, // Reason Code
				27,
			},
			Packet: &Packet{
				ProtocolVersion: 5,
			},
		},
	},
	Pubrel: {
		{
			Case:    TPubrel,
			Desc:    "pubrel",
			Primary: true,
			RawBytes: []byte{
				Pubrel<<4 | 1<<1, 2, // Fixed header
				0, 7, // Packet ID - LSB+MSB
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Pubrel,
					Remaining: 2,
					Qos:       1,
				},
				PacketID: 7,
			},
		},
		{
			Case:    TPubrelMqtt5,
			Desc:    "pubrel mqtt5",
			Primary: true,
			RawBytes: []byte{
				Pubrel<<4 | 1<<1, 20, // Fixed header
				0, 7, // Packet ID - LSB+MSB
				CodeSuccess.Code, // Reason Code
				16,               // Properties Length
				38,               // User Properties (38)
				0, 5, 'h', 'e', 'l', 'l', 'o',
				0, 6, 228, 184, 150, 231, 149, 140,
			},
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Pubrel,
					Remaining: 20,
					Qos:       1,
				},
				PacketID:   7,
				ReasonCode: CodeSuccess.Code,
				Properties: Properties{
					User: []UserProperty{
						{
							Key: "hello",
							Val: "世界",
						},
					},
				},
			},
		},
		{
			Case:    TPubrelMqtt5AckNoPacket,
			Desc:    "mqtt5 no packet id ack",
			Primary: true,
			RawBytes: append([]byte{
				Pubrel<<4 | 1<<1, 34, // Fixed header
				0, 7, // Packet ID - LSB+MSB
				ErrPacketIdentifierNotFound.Code, // Reason Code
				30,                               // Properties Length
				31, 0, byte(len(ErrPacketIdentifierNotFound.Reason)),
			}, []byte(ErrPacketIdentifierNotFound.Reason)...),
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Pubrel,
					Remaining: 34,
					Qos:       1,
				},
				PacketID:   7,
				ReasonCode: ErrPacketIdentifierNotFound.Code,
				Properties: Properties{
					ReasonString: ErrPacketIdentifierNotFound.Reason,
				},
			},
		},
		// Validation
		{
			Case:      TPubrelInvalidReason,
			Desc:      "invalid reason code",
			Group:     "validate",
			FailFirst: ErrProtocolViolationInvalidReason,
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type: Pubrel,
				},
				PacketID:   7,
				ReasonCode: ErrConnectionRateExceeded.Code,
			},
		},
		// Fail states
		{
			Case:      TPubrelMalPacketID,
			Desc:      "malformed packet id",
			Group:     "decode",
			FailFirst: ErrMalformedPacketID,
			RawBytes: []byte{
				Pubrel << 4, 2, // Fixed header
				0, // Packet ID - LSB+MSB
			},
		},
		{
			Case:      TPubrelMalProperties,
			Desc:      "malformed properties",
			Group:     "decode",
			FailFirst: ErrMalformedProperties,
			RawBytes: []byte{
				Pubrel<<4 | 1<<1, 20, // Fixed header
				0, 7, // Packet ID - LSB+MSB
				CodeSuccess.Code, // Reason Code
				16,
			},
			Packet: &Packet{
				ProtocolVersion: 5,
			},
		},
	},
	Pubcomp: {
		{
			Case:    TPubcomp,
			Desc:    "pubcomp",
			Primary: true,
			RawBytes: []byte{
				Pubcomp << 4, 2, // Fixed header
				0, 7, // Packet ID - LSB+MSB
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Pubcomp,
					Remaining: 2,
				},
				PacketID: 7,
			},
		},
		{
			Case:    TPubcompMqtt5,
			Desc:    "mqtt5",
			Primary: true,
			RawBytes: []byte{
				Pubcomp << 4, 20, // Fixed header
				0, 7, // Packet ID - LSB+MSB
				CodeSuccess.Code, // Reason Code
				16,               // Properties Length
				38,               // User Properties (38)
				0, 5, 'h', 'e', 'l', 'l', 'o',
				0, 6, 228, 184, 150, 231, 149, 140,
			},
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Pubcomp,
					Remaining: 20,
				},
				PacketID:   7,
				ReasonCode: CodeSuccess.Code,
				Properties: Properties{
					User: []UserProperty{
						{
							Key: "hello",
							Val: "世界",
						},
					},
				},
			},
		},
		{
			Case:    TPubcompMqtt5AckNoPacket,
			Desc:    "mqtt5 no packet id ack",
			Primary: true,
			RawBytes: append([]byte{
				Pubcomp << 4, 34, // Fixed header
				0, 7, // Packet ID - LSB+MSB
				ErrPacketIdentifierNotFound.Code, // Reason Code
				30,                               // Properties Length
				31, 0, byte(len(ErrPacketIdentifierNotFound.Reason)),
			}, []byte(ErrPacketIdentifierNotFound.Reason)...),
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Pubcomp,
					Remaining: 34,
				},
				PacketID:   7,
				ReasonCode: ErrPacketIdentifierNotFound.Code,
				Properties: Properties{
					ReasonString: ErrPacketIdentifierNotFound.Reason,
				},
			},
		},
		// Validation
		{
			Case:      TPubcompInvalidReason,
			Desc:      "invalid reason code",
			Group:     "validate",
			FailFirst: ErrProtocolViolationInvalidReason,
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type: Pubcomp,
				},
				ReasonCode: ErrConnectionRateExceeded.Code,
			},
		},
		// Fail states
		{
			Case:      TPubcompMalPacketID,
			Desc:      "malformed packet id",
			Group:     "decode",
			FailFirst: ErrMalformedPacketID,
			RawBytes: []byte{
				Pubcomp << 4, 2, // Fixed header
				0, // Packet ID - LSB+MSB
			},
		},
		{
			Case:      TPubcompMalProperties,
			Desc:      "malformed properties",
			Group:     "decode",
			FailFirst: ErrMalformedProperties,
			RawBytes: []byte{
				Pubcomp << 4, 34, // Fixed header
				0, 7, // Packet ID - LSB+MSB
				ErrPacketIdentifierNotFound.Code, // Reason Code
				22,
			},
			Packet: &Packet{
				ProtocolVersion: 5,
			},
		},
	},
	Subscribe: {
		{
			Case:    TSubscribe,
			Desc:    "subscribe",
			Primary: true,
			RawBytes: []byte{
				Subscribe<<4 | 1<<1, 10, // Fixed header
				0, 15, // Packet ID - LSB+MSB
				0, 5, // Topic Name - LSB+MSB
				'a', '/', 'b', '/', 'c', // Topic Name
				0, // QoS
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Subscribe,
					Remaining: 10,
					Qos:       1,
				},
				PacketID: 15,
				Filters: Subscriptions{
					{Filter: "a/b/c"},
				},
			},
		},
		{
			Case:    TSubscribeMany,
			Desc:    "many",
			Primary: true,
			RawBytes: []byte{
				Subscribe<<4 | 1<<1, 30, // Fixed header
				0, 15, // Packet ID - LSB+MSB

				0, 3, // Topic Name - LSB+MSB
				'a', '/', 'b', // Topic Name
				0, // QoS

				0, 11, // Topic Name - LSB+MSB
				'd', '/', 'e', '/', 'f', '/', 'g', '/', 'h', '/', 'i', // Topic Name
				1, // QoS

				0, 5, // Topic Name - LSB+MSB
				'x', '/', 'y', '/', 'z', // Topic Name
				2, // QoS
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Subscribe,
					Remaining: 30,
					Qos:       1,
				},
				PacketID: 15,
				Filters: Subscriptions{
					{Filter: "a/b", Qos: 0},
					{Filter: "d/e/f/g/h/i", Qos: 1},
					{Filter: "x/y/z", Qos: 2},
				},
			},
		},
		{
			Case:    TSubscribeMqtt5,
			Desc:    "mqtt5",
			Primary: true,
			RawBytes: []byte{
				Subscribe<<4 | 1<<1, 31, // Fixed header
				0, 15, // Packet ID - LSB+MSB
				20,
				11, 202, 212, 19, // Subscription Identifier (11)
				38, // User Properties (38)
				0, 5, 'h', 'e', 'l', 'l', 'o',
				0, 6, 228, 184, 150, 231, 149, 140,

				0, 5, 'a', '/', 'b', '/', 'c', // Topic Name
				46, // subscription options
			},
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Subscribe,
					Remaining: 31,
					Qos:       1,
				},
				PacketID: 15,
				Filters: Subscriptions{
					{
						Filter:            "a/b/c",
						Qos:               2,
						NoLocal:           true,
						RetainAsPublished: true,
						RetainHandling:    2,
						Identifier:        322122,
					},
				},
				Properties: Properties{
					SubscriptionIdentifier: []int{322122},
					User: []UserProperty{
						{
							Key: "hello",
							Val: "世界",
						},
					},
				},
			},
		},
		{
			Case:    TSubscribeRetainHandling1,
			Desc:    "retain handling 1",
			Primary: true,
			RawBytes: []byte{
				Subscribe<<4 | 1<<1, 11, // Fixed header
				0, 15, // Packet ID - LSB+MSB
				0,    // no properties
				0, 5, // Topic Name - LSB+MSB
				'a', '/', 'b', '/', 'c', // Topic Name
				0 | 1<<4, // subscription options
			},
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Subscribe,
					Remaining: 11,
					Qos:       1,
				},
				PacketID: 15,
				Filters: Subscriptions{
					{
						Filter:         "a/b/c",
						RetainHandling: 1,
					},
				},
			},
		},
		{
			Case:    TSubscribeRetainHandling2,
			Desc:    "retain handling 2",
			Primary: true,
			RawBytes: []byte{
				Subscribe<<4 | 1<<1, 11, // Fixed header
				0, 15, // Packet ID - LSB+MSB
				0,    // no properties
				0, 5, // Topic Name - LSB+MSB
				'a', '/', 'b', '/', 'c', // Topic Name
				0 | 2<<4, // subscription options
			},
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Subscribe,
					Remaining: 11,
					Qos:       1,
				},
				PacketID: 15,
				Filters: Subscriptions{
					{
						Filter:         "a/b/c",
						RetainHandling: 2,
					},
				},
			},
		},
		{
			Case:    TSubscribeRetainAsPublished,
			Desc:    "retain as published",
			Primary: true,
			RawBytes: []byte{
				Subscribe<<4 | 1<<1, 11, // Fixed header
				0, 15, // Packet ID - LSB+MSB
				0,    // no properties
				0, 5, // Topic Name - LSB+MSB
				'a', '/', 'b', '/', 'c', // Topic Name
				0 | 1<<3, // subscription options
			},
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Subscribe,
					Remaining: 11,
					Qos:       1,
				},
				PacketID: 15,
				Filters: Subscriptions{
					{
						Filter:            "a/b/c",
						RetainAsPublished: true,
					},
				},
			},
		},
		{
			Case:  TSubscribeInvalidFilter,
			Desc:  "invalid filter",
			Group: "reference",
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type: Subscribe,
					Qos:  1,
				},
				PacketID: 15,
				Filters: Subscriptions{
					{Filter: "$SHARE/#", Identifier: 5},
				},
			},
		},
		{
			Case:  TSubscribeInvalidSharedNoLocal,
			Desc:  "shared and no local",
			Group: "reference",
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type: Subscribe,
					Qos:  1,
				},
				PacketID: 15,
				Filters: Subscriptions{
					{Filter: "$SHARE/tmp/a/b/c", Identifier: 5, NoLocal: true},
				},
			},
		},

		// Fail states
		{
			Case:      TSubscribeMalPacketID,
			Desc:      "malformed packet id",
			Group:     "decode",
			FailFirst: ErrMalformedPacketID,
			RawBytes: []byte{
				Subscribe<<4 | 1<<1, 2, // Fixed header
				0, // Packet ID - LSB+MSB
			},
		},
		{
			Case:      TSubscribeMalTopic,
			Desc:      "malformed topic",
			Group:     "decode",
			FailFirst: ErrMalformedTopic,
			RawBytes: []byte{
				Subscribe<<4 | 1<<1, 2, // Fixed header
				0, 21, // Packet ID - LSB+MSB

				0, 3, // Topic Name - LSB+MSB
				'a', '/',
			},
		},
		{
			Case:      TSubscribeMalQos,
			Desc:      "malformed subscribe - qos",
			Group:     "decode",
			FailFirst: ErrMalformedQos,
			RawBytes: []byte{
				Subscribe<<4 | 1<<1, 2, // Fixed header
				0, 22, // Packet ID - LSB+MSB

				0, 3, // Topic Name - LSB+MSB
				'j', '/', 'b', // Topic Name

			},
		},
		{
			Case:      TSubscribeMalQosRange,
			Desc:      "malformed qos out of range",
			Group:     "decode",
			FailFirst: ErrProtocolViolationQosOutOfRange,
			RawBytes: []byte{
				Subscribe<<4 | 1<<1, 2, // Fixed header
				0, 22, // Packet ID - LSB+MSB

				0, 3, // Topic Name - LSB+MSB
				'c', '/', 'd', // Topic Name
				5, // QoS

			},
		},
		{
			Case:      TSubscribeMalProperties,
			Desc:      "malformed properties",
			Group:     "decode",
			FailFirst: ErrMalformedProperties,
			RawBytes: []byte{
				Subscribe << 4, 11, // Fixed header
				0, 15, // Packet ID - LSB+MSB
				4,
			},
			Packet: &Packet{
				ProtocolVersion: 5,
			},
		},

		// Validation
		{
			Case:   TSubscribeInvalidQosMustPacketID,
			Desc:   "no packet id with qos > 0",
			Group:  "validate",
			Expect: ErrProtocolViolationNoPacketID,
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type: Subscribe,
					Qos:  1,
				},
				PacketID: 0,
				Filters: Subscriptions{
					{Filter: "a/b"},
				},
			},
		},
		{
			Case:   TSubscribeInvalidNoFilters,
			Desc:   "no filters",
			Group:  "validate",
			Expect: ErrProtocolViolationNoFilters,
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type: Subscribe,
					Qos:  1,
				},
				PacketID: 2,
			},
		},

		{
			Case:   TSubscribeInvalidIdentifierOversize,
			Desc:   "oversize identifier",
			Group:  "validate",
			Expect: ErrProtocolViolationOversizeSubID,
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type: Subscribe,
					Qos:  1,
				},
				PacketID: 2,
				Filters: Subscriptions{
					{Filter: "a/b", Identifier: 5},
					{Filter: "d/f", Identifier: 268435456},
				},
			},
		},

		// Spec tests
		{
			Case:   TSubscribeSpecQosMustPacketID,
			Desc:   "no packet id with qos > 0",
			Group:  "encode",
			Expect: ErrProtocolViolationNoPacketID,
			RawBytes: []byte{
				Subscribe<<4 | 1<<1, 10, // Fixed header
				0, 0, // Packet ID - LSB+MSB
				0, 5, // Topic Name - LSB+MSB
				'a', '/', 'b', '/', 'c', // Topic Name
				1, // QoS
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Subscribe,
					Qos:       1,
					Remaining: 10,
				},
				Filters: Subscriptions{
					{Filter: "a/b/c", Qos: 1},
				},
				PacketID: 0,
			},
		},
	},
	Suback: {
		{
			Case:    TSuback,
			Desc:    "suback",
			Primary: true,
			RawBytes: []byte{
				Suback << 4, 3, // Fixed header
				0, 15, // Packet ID - LSB+MSB
				0, // Return Code QoS 0
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Suback,
					Remaining: 3,
				},
				PacketID:    15,
				ReasonCodes: []byte{0},
			},
		},
		{
			Case:    TSubackMany,
			Desc:    "many",
			Primary: true,
			RawBytes: []byte{
				Suback << 4, 6, // Fixed header
				0, 15, // Packet ID - LSB+MSB
				0,    // Return Code QoS 0
				1,    // Return Code QoS 1
				2,    // Return Code QoS 2
				0x80, // Return Code fail
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Suback,
					Remaining: 6,
				},
				PacketID:    15,
				ReasonCodes: []byte{0, 1, 2, 0x80},
			},
		},
		{
			Case:    TSubackDeny,
			Desc:    "deny mqtt5",
			Primary: true,
			RawBytes: []byte{
				Suback << 4, 4, // Fixed header
				0, 15, // Packet ID - LSB+MSB
				0,                     // no properties
				ErrNotAuthorized.Code, // Return Code QoS 0
			},
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Suback,
					Remaining: 4,
				},
				PacketID:    15,
				ReasonCodes: []byte{ErrNotAuthorized.Code},
			},
		},
		{
			Case:    TSubackUnspecifiedError,
			Desc:    "unspecified error",
			Primary: true,
			RawBytes: []byte{
				Suback << 4, 3, // Fixed header
				0, 15, // Packet ID - LSB+MSB
				ErrUnspecifiedError.Code, // Return Code QoS 0
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Suback,
					Remaining: 3,
				},
				PacketID:    15,
				ReasonCodes: []byte{ErrUnspecifiedError.Code},
			},
		},
		{
			Case:    TSubackUnspecifiedErrorMqtt5,
			Desc:    "unspecified error mqtt5",
			Primary: true,
			RawBytes: []byte{
				Suback << 4, 4, // Fixed header
				0, 15, // Packet ID - LSB+MSB
				0,                        // no properties
				ErrUnspecifiedError.Code, // Return Code QoS 0
			},
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Suback,
					Remaining: 4,
				},
				PacketID:    15,
				ReasonCodes: []byte{ErrUnspecifiedError.Code},
			},
		},
		{
			Case:    TSubackMqtt5,
			Desc:    "mqtt5",
			Primary: true,
			RawBytes: []byte{
				Suback << 4, 20, // Fixed header
				0, 15, // Packet ID
				16, // Properties Length
				38, // User Properties (38)
				0, 5, 'h', 'e', 'l', 'l', 'o',
				0, 6, 228, 184, 150, 231, 149, 140,
				CodeGrantedQos2.Code, // Return Code QoS 0
			},
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Suback,
					Remaining: 20,
				},
				PacketID:    15,
				ReasonCodes: []byte{CodeGrantedQos2.Code},
				Properties: Properties{
					User: []UserProperty{
						{
							Key: "hello",
							Val: "世界",
						},
					},
				},
			},
		},
		{
			Case:    TSubackPacketIDInUse,
			Desc:    "packet id in use",
			Primary: true,
			RawBytes: []byte{
				Suback << 4, 47, // Fixed header
				0, 15, // Packet ID
				43, // Properties Length
				31, 0, 24, 'p', 'a', 'c', 'k', 'e', 't',
				' ', 'i', 'd', 'e', 'n', 't', 'i', 'f', 'i', 'e', 'r',
				' ', 'i', 'n',
				' ', 'u', 's', 'e', // Reason String (31)
				38, // User Properties (38)
				0, 5, 'h', 'e', 'l', 'l', 'o',
				0, 6, 228, 184, 150, 231, 149, 140,
				ErrPacketIdentifierInUse.Code,
			},
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Suback,
					Remaining: 47,
				},
				PacketID:    15,
				ReasonCodes: []byte{ErrPacketIdentifierInUse.Code},
				Properties: Properties{
					ReasonString: ErrPacketIdentifierInUse.Reason,
					User: []UserProperty{
						{
							Key: "hello",
							Val: "世界",
						},
					},
				},
			},
		},

		// Fail states
		{
			Case:      TSubackMalPacketID,
			Desc:      "malformed packet id",
			Group:     "decode",
			FailFirst: ErrMalformedPacketID,
			RawBytes: []byte{
				Suback << 4, 2, // Fixed header
				0, // Packet ID - LSB+MSB
			},
		},
		{
			Case:      TSubackMalProperties,
			Desc:      "malformed properties",
			Group:     "decode",
			FailFirst: ErrMalformedProperties,
			RawBytes: []byte{
				Suback << 4, 47,
				0, 15,
				43,
			},
			Packet: &Packet{
				ProtocolVersion: 5,
			},
		},

		{
			Case:  TSubackInvalidFilter,
			Desc:  "malformed packet id",
			Group: "reference",
			RawBytes: []byte{
				Suback << 4, 4,
				0, 15,
				0, // no properties
				ErrTopicFilterInvalid.Code,
			},
			Packet: &Packet{
				ProtocolVersion: 5,
			},
		},
		{
			Case:  TSubackInvalidSharedNoLocal,
			Desc:  "invalid shared no local",
			Group: "reference",
			RawBytes: []byte{
				Suback << 4, 4,
				0, 15,
				0, // no properties
				ErrProtocolViolationInvalidSharedNoLocal.Code,
			},
			Packet: &Packet{
				ProtocolVersion: 5,
			},
		},
	},

	Unsubscribe: {
		{
			Case:    TUnsubscribe,
			Desc:    "unsubscribe",
			Primary: true,
			RawBytes: []byte{
				Unsubscribe<<4 | 1<<1, 9, // Fixed header
				0, 15, // Packet ID - LSB+MSB
				0, 5, // Topic Name - LSB+MSB
				'a', '/', 'b', '/', 'c', // Topic Name

			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Unsubscribe,
					Remaining: 9,
					Qos:       1,
				},
				PacketID: 15,
				Filters: Subscriptions{
					{Filter: "a/b/c"},
				},
			},
		},
		{
			Case:    TUnsubscribeMany,
			Desc:    "unsubscribe many",
			Primary: true,
			RawBytes: []byte{
				Unsubscribe<<4 | 1<<1, 27, // Fixed header
				0, 35, // Packet ID - LSB+MSB

				0, 3, // Topic Name - LSB+MSB
				'a', '/', 'b', // Topic Name

				0, 11, // Topic Name - LSB+MSB
				'd', '/', 'e', '/', 'f', '/', 'g', '/', 'h', '/', 'i', // Topic Name

				0, 5, // Topic Name - LSB+MSB
				'x', '/', 'y', '/', 'z', // Topic Name
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Unsubscribe,
					Remaining: 27,
					Qos:       1,
				},
				PacketID: 35,
				Filters: Subscriptions{
					{Filter: "a/b"},
					{Filter: "d/e/f/g/h/i"},
					{Filter: "x/y/z"},
				},
			},
		},
		{
			Case:    TUnsubscribeMqtt5,
			Desc:    "mqtt5",
			Primary: true,
			RawBytes: []byte{
				Unsubscribe<<4 | 1<<1, 31, // Fixed header
				0, 15, // Packet ID - LSB+MSB

				16,
				38, // User Properties (38)
				0, 5, 'h', 'e', 'l', 'l', 'o',
				0, 6, 228, 184, 150, 231, 149, 140,

				0, 3, // Topic Name - LSB+MSB
				'a', '/', 'b',

				0, 5, // Topic Name - LSB+MSB
				'x', '/', 'y', '/', 'w',
			},
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Unsubscribe,
					Remaining: 31,
					Qos:       1,
				},
				PacketID: 15,
				Properties: Properties{
					User: []UserProperty{
						{
							Key: "hello",
							Val: "世界",
						},
					},
				},
				Filters: Subscriptions{
					{Filter: "a/b"},
					{Filter: "x/y/w"},
				},
			},
		},

		// Fail states
		{
			Case:      TUnsubscribeMalPacketID,
			Desc:      "malformed packet id",
			Group:     "decode",
			FailFirst: ErrMalformedPacketID,
			RawBytes: []byte{
				Unsubscribe << 4, 2, // Fixed header
				0, // Packet ID - LSB+MSB
			},
		},
		{
			Case:      TUnsubscribeMalTopicName,
			Desc:      "malformed topic",
			Group:     "decode",
			FailFirst: ErrMalformedTopic,
			RawBytes: []byte{
				Unsubscribe << 4, 2, // Fixed header
				0, 21, // Packet ID - LSB+MSB
				0, 3, // Topic Name - LSB+MSB
				'a', '/',
			},
		},
		{
			Case:      TUnsubscribeMalProperties,
			Desc:      "malformed properties",
			Group:     "decode",
			FailFirst: ErrMalformedProperties,
			RawBytes: []byte{
				Unsubscribe<<4 | 1<<1, 31, // Fixed header
				0, 15, // Packet ID - LSB+MSB
				16,
			},
			Packet: &Packet{
				ProtocolVersion: 5,
			},
		},

		{
			Case:   TUnsubscribeInvalidQosMustPacketID,
			Desc:   "no packet id with qos > 0",
			Group:  "validate",
			Expect: ErrProtocolViolationNoPacketID,
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type: Unsubscribe,
					Qos:  1,
				},
				PacketID: 0,
				Filters: Subscriptions{
					Subscription{Filter: "a/b"},
				},
			},
		},
		{
			Case:   TUnsubscribeInvalidNoFilters,
			Desc:   "no filters",
			Group:  "validate",
			Expect: ErrProtocolViolationNoFilters,
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type: Unsubscribe,
					Qos:  1,
				},
				PacketID: 2,
			},
		},

		{
			Case:   TUnsubscribeSpecQosMustPacketID,
			Desc:   "no packet id with qos > 0",
			Group:  "encode",
			Expect: ErrProtocolViolationNoPacketID,
			RawBytes: []byte{
				Unsubscribe<<4 | 1<<1, 9, // Fixed header
				0, 0, // Packet ID - LSB+MSB
				0, 5, // Topic Name - LSB+MSB
				'a', '/', 'b', '/', 'c', // Topic Name
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Unsubscribe,
					Qos:       1,
					Remaining: 9,
				},
				PacketID: 0,
				Filters: Subscriptions{
					{Filter: "a/b/c"},
				},
			},
		},
	},
	Unsuback: {
		{
			Case:    TUnsuback,
			Desc:    "unsuback",
			Primary: true,
			RawBytes: []byte{
				Unsuback << 4, 2, // Fixed header
				0, 15, // Packet ID - LSB+MSB
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Unsuback,
					Remaining: 2,
				},
				PacketID: 15,
			},
		},
		{
			Case:    TUnsubackMany,
			Desc:    "unsuback many",
			Primary: true,
			RawBytes: []byte{
				Unsuback << 4, 5, // Fixed header
				0, 15, // Packet ID - LSB+MSB
				0,
				CodeSuccess.Code, CodeSuccess.Code,
			},
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Unsuback,
					Remaining: 5,
				},
				PacketID:    15,
				ReasonCodes: []byte{CodeSuccess.Code, CodeSuccess.Code},
			},
		},
		{
			Case:    TUnsubackMqtt5,
			Desc:    "mqtt5",
			Primary: true,
			RawBytes: []byte{
				Unsuback << 4, 21, // Fixed header
				0, 15, // Packet ID - LSB+MSB
				16, // Properties Length
				38, // User Properties (38)
				0, 5, 'h', 'e', 'l', 'l', 'o',
				0, 6, 228, 184, 150, 231, 149, 140,
				CodeSuccess.Code, CodeNoSubscriptionExisted.Code,
			},
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Unsuback,
					Remaining: 21,
				},
				PacketID:    15,
				ReasonCodes: []byte{CodeSuccess.Code, CodeNoSubscriptionExisted.Code},
				Properties: Properties{
					User: []UserProperty{
						{
							Key: "hello",
							Val: "世界",
						},
					},
				},
			},
		},
		{
			Case:    TUnsubackPacketIDInUse,
			Desc:    "packet id in use",
			Primary: true,
			RawBytes: []byte{
				Unsuback << 4, 48, // Fixed header
				0, 15, // Packet ID - LSB+MSB
				43, // Properties Length
				31, 0, 24, 'p', 'a', 'c', 'k', 'e', 't',
				' ', 'i', 'd', 'e', 'n', 't', 'i', 'f', 'i', 'e', 'r',
				' ', 'i', 'n',
				' ', 'u', 's', 'e', // Reason String (31)
				38, // User Properties (38)
				0, 5, 'h', 'e', 'l', 'l', 'o',
				0, 6, 228, 184, 150, 231, 149, 140,
				ErrPacketIdentifierInUse.Code, ErrPacketIdentifierInUse.Code,
			},
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Unsuback,
					Remaining: 48,
				},
				PacketID:    15,
				ReasonCodes: []byte{ErrPacketIdentifierInUse.Code, ErrPacketIdentifierInUse.Code},
				Properties: Properties{
					ReasonString: ErrPacketIdentifierInUse.Reason,
					User: []UserProperty{
						{
							Key: "hello",
							Val: "世界",
						},
					},
				},
			},
		},

		// Fail states
		{
			Case:      TUnsubackMalPacketID,
			Desc:      "malformed packet id",
			Group:     "decode",
			FailFirst: ErrMalformedPacketID,
			RawBytes: []byte{
				Unsuback << 4, 2, // Fixed header
				0, // Packet ID - LSB+MSB
			},
		},
		{
			Case:      TUnsubackMalProperties,
			Desc:      "malformed properties",
			Group:     "decode",
			FailFirst: ErrMalformedProperties,
			RawBytes: []byte{
				Unsuback << 4, 48, // Fixed header
				0, 15, // Packet ID - LSB+MSB
				43,
			},
			Packet: &Packet{
				ProtocolVersion: 5,
			},
		},
	},

	Pingreq: {
		{
			Case:    TPingreq,
			Desc:    "ping request",
			Primary: true,
			RawBytes: []byte{
				Pingreq << 4, 0, // fixed header
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Pingreq,
					Remaining: 0,
				},
			},
		},
	},
	Pingresp: {
		{
			Case:    TPingresp,
			Desc:    "ping response",
			Primary: true,
			RawBytes: []byte{
				Pingresp << 4, 0, // fixed header
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Pingresp,
					Remaining: 0,
				},
			},
		},
	},

	Disconnect: {
		{
			Case:    TDisconnect,
			Desc:    "disconnect",
			Primary: true,
			RawBytes: []byte{
				Disconnect << 4, 0, // fixed header
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Disconnect,
					Remaining: 0,
				},
			},
		},
		{
			Case:    TDisconnectTakeover,
			Desc:    "takeover",
			Primary: true,
			RawBytes: append([]byte{
				Disconnect << 4, 21, // fixed header
				ErrSessionTakenOver.Code, // Reason Code
				19,                       // Properties Length
				31, 0, 16,                // Reason String (31)
			}, []byte(ErrSessionTakenOver.Reason)...),
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Disconnect,
					Remaining: 0,
				},
				ReasonCode: ErrSessionTakenOver.Code,
				Properties: Properties{
					ReasonString: ErrSessionTakenOver.Reason,
				},
			},
		},
		{
			Case:    TDisconnectShuttingDown,
			Desc:    "shutting down",
			Primary: true,
			RawBytes: append([]byte{
				Disconnect << 4, 25, // fixed header
				ErrServerShuttingDown.Code, // Reason Code
				23,                         // Properties Length
				31, 0, 20,                  // Reason String (31)
			}, []byte(ErrServerShuttingDown.Reason)...),
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Disconnect,
					Remaining: 0,
				},
				ReasonCode: ErrServerShuttingDown.Code,
				Properties: Properties{
					ReasonString: ErrServerShuttingDown.Reason,
				},
			},
		},
		{
			Case:    TDisconnectMqtt5,
			Desc:    "mqtt5",
			Primary: true,
			RawBytes: append([]byte{
				Disconnect << 4, 22, // fixed header
				CodeDisconnect.Code, // Reason Code
				20,                  // Properties Length
				17, 0, 0, 0, 120,    // Session Expiry Interval (17)
				31, 0, 12, // Reason String (31)
			}, []byte(CodeDisconnect.Reason)...),
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Disconnect,
					Remaining: 22,
				},
				ReasonCode: CodeDisconnect.Code,
				Properties: Properties{
					ReasonString:              CodeDisconnect.Reason,
					SessionExpiryInterval:     120,
					SessionExpiryIntervalFlag: true,
				},
			},
		},
		{
			Case: TDisconnectSecondConnect,
			Desc: "second connect packet mqtt5",
			RawBytes: append([]byte{
				Disconnect << 4, 46, // fixed header
				ErrProtocolViolationSecondConnect.Code,
				44,
				31, 0, 41, // Reason String (31)
			}, []byte(ErrProtocolViolationSecondConnect.Reason)...),
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Disconnect,
					Remaining: 45,
				},
				ReasonCode: ErrProtocolViolationSecondConnect.Code,
				Properties: Properties{
					ReasonString: ErrProtocolViolationSecondConnect.Reason,
				},
			},
		},
		{
			Case: TDisconnectZeroNonZeroExpiry,
			Desc: "zero non zero expiry",
			RawBytes: []byte{
				Disconnect << 4, 2, // fixed header
				ErrProtocolViolationZeroNonZeroExpiry.Code,
				0,
			},
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Disconnect,
					Remaining: 2,
				},
				ReasonCode: ErrProtocolViolationZeroNonZeroExpiry.Code,
			},
		},
		{
			Case: TDisconnectReceiveMaximum,
			Desc: "receive maximum mqtt5",
			RawBytes: append([]byte{
				Disconnect << 4, 29, // fixed header
				ErrReceiveMaximum.Code,
				27,        // Properties Length
				31, 0, 24, // Reason String (31)
			}, []byte(ErrReceiveMaximum.Reason)...),
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Disconnect,
					Remaining: 29,
				},
				ReasonCode: ErrReceiveMaximum.Code,
				Properties: Properties{
					ReasonString: ErrReceiveMaximum.Reason,
				},
			},
		},
		{
			Case:  TDisconnectDropProperties,
			Desc:  "drop oversize properties partial",
			Group: "encode",
			RawBytes: []byte{
				Disconnect << 4, 39, // fixed header
				CodeDisconnect.Code,
				19,                                          // length
				28, 0, 7, 'm', 'o', 'c', 'h', 'i', '-', '2', // Server Reference (28)
				31, 0, 6, 'r', 'e', 'a', 's', 'o', 'n', // Reason String (31)
				38, // User Properties (38)
				0, 5, 'h', 'e', 'l', 'l', 'o',
				0, 6, 228, 184, 150, 231, 149, 140,
			},
			ActualBytes: []byte{
				Disconnect << 4, 12, // fixed header
				CodeDisconnect.Code,
				10,                                          // length
				28, 0, 7, 'm', 'o', 'c', 'h', 'i', '-', '2', // Server Reference (28)
			},
			Packet: &Packet{
				Mods: Mods{
					MaxSize: 3,
				},
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Disconnect,
					Remaining: 40,
				},
				ReasonCode: CodeSuccess.Code,
				Properties: Properties{
					ReasonString:    "reason",
					ServerReference: "mochi-2",
					User: []UserProperty{
						{
							Key: "hello",
							Val: "世界",
						},
					},
				},
			},
		},
		// fail states
		{
			Case:      TDisconnectMalProperties,
			Desc:      "malformed properties",
			Group:     "decode",
			FailFirst: ErrMalformedProperties,
			RawBytes: []byte{
				Disconnect << 4, 48, // fixed header
				CodeDisconnect.Code, // Reason Code
				46,
			},
			Packet: &Packet{
				ProtocolVersion: 5,
			},
		},
		{
			Case:      TDisconnectMalReasonCode,
			Desc:      "malformed reason code",
			Group:     "decode",
			FailFirst: ErrMalformedReasonCode,
			RawBytes: []byte{
				Disconnect << 4, 48, // fixed header
			},
			Packet: &Packet{
				ProtocolVersion: 5,
			},
		},
	},
	Auth: {
		{
			Case:    TAuth,
			Desc:    "auth",
			Primary: true,
			RawBytes: []byte{
				Auth << 4, 47,
				CodeSuccess.Code, // reason code
				45,
				21, 0, 5, 'S', 'H', 'A', '-', '1', // Authentication Method (21)
				22, 0, 9, 'a', 'u', 't', 'h', '-', 'd', 'a', 't', 'a', // Authentication Data (22)
				31, 0, 6, 'r', 'e', 'a', 's', 'o', 'n', // Reason String (31)
				38, // User Properties (38)
				0, 5, 'h', 'e', 'l', 'l', 'o',
				0, 6, 228, 184, 150, 231, 149, 140,
			},
			Packet: &Packet{
				ProtocolVersion: 5,
				FixedHeader: FixedHeader{
					Type:      Auth,
					Remaining: 47,
				},
				ReasonCode: CodeSuccess.Code,
				Properties: Properties{
					AuthenticationMethod: "SHA-1",
					AuthenticationData:   []byte("auth-data"),
					ReasonString:         "reason",
					User: []UserProperty{
						{
							Key: "hello",
							Val: "世界",
						},
					},
				},
			},
		},
		{
			Case:      TAuthMalReasonCode,
			Desc:      "malformed reason code",
			Group:     "decode",
			FailFirst: ErrMalformedReasonCode,
			RawBytes: []byte{
				Auth << 4, 47,
			},
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type: Auth,
				},
				ReasonCode: CodeNoMatchingSubscribers.Code,
			},
		},
		// fail states
		{
			Case:      TAuthMalProperties,
			Desc:      "malformed properties",
			Group:     "decode",
			FailFirst: ErrMalformedProperties,
			RawBytes: []byte{
				Auth << 4, 3,
				CodeSuccess.Code,
				12,
			},
			Packet: &Packet{
				ProtocolVersion: 5,
			},
		},
		// Validation
		{
			Case:   TAuthInvalidReason,
			Desc:   "invalid reason code",
			Group:  "validate",
			Expect: ErrProtocolViolationInvalidReason,
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type: Auth,
				},
				ReasonCode: CodeNoMatchingSubscribers.Code,
			},
		},
		{
			Case:   TAuthInvalidReason2,
			Desc:   "invalid reason code",
			Group:  "validate",
			Expect: ErrProtocolViolationInvalidReason,
			Packet: &Packet{
				FixedHeader: FixedHeader{
					Type: Auth,
				},
				ReasonCode: CodeNoMatchingSubscribers.Code,
			},
		},
	},
}
