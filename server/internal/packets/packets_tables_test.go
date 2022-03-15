package packets

type packetTestData struct {
	group       string      // group specifies a group that should run the test, blank for all
	rawBytes    []byte      // the bytes that make the packet
	actualBytes []byte      // the actual byte array that is created in the event of a byte mutation (eg. MQTT-2.3.1-1 qos/packet id)
	packet      *Packet     // the packet that is expected
	desc        string      // a description of the test
	failFirst   error       // expected fail result to be run immediately after the method is called
	expect      interface{} // generic expected fail result to be checked
	isolate     bool        // isolate can be used to isolate a test
	primary     bool        // primary is a test that should be run using readPackets
	meta        interface{} // meta conains a metadata value used in testing on a case-by-case basis.
	code        byte        // code is an expected validation return code
}

func encodeTestOK(wanted packetTestData) bool {
	if wanted.rawBytes == nil {
		return false
	}
	if wanted.group != "" && wanted.group != "encode" {
		return false
	}
	return true
}

func decodeTestOK(wanted packetTestData) bool {
	if wanted.group != "" && wanted.group != "decode" {
		return false
	}
	return true
}

var expectedPackets = map[byte][]packetTestData{
	Connect: {
		{
			desc:    "MQTT 3.1",
			primary: true,
			rawBytes: []byte{
				byte(Connect << 4), 17, // Fixed header
				0, 6, // Protocol Name - MSB+LSB
				'M', 'Q', 'I', 's', 'd', 'p', // Protocol Name
				3,     // Protocol Version
				0,     // Packet Flags
				0, 30, // Keepalive
				0, 3, // Client ID - MSB+LSB
				'z', 'e', 'n', // Client ID "zen"
			},
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Connect,
					Remaining: 17,
				},
				ProtocolName:     []byte("MQIsdp"),
				ProtocolVersion:  3,
				CleanSession:     false,
				Keepalive:        30,
				ClientIdentifier: "zen",
			},
		},

		{
			desc:    "MQTT 3.1.1",
			primary: true,
			rawBytes: []byte{
				byte(Connect << 4), 16, // Fixed header
				0, 4, // Protocol Name - MSB+LSB
				'M', 'Q', 'T', 'T', // Protocol Name
				4,     // Protocol Version
				0,     // Packet Flags
				0, 60, // Keepalive
				0, 4, // Client ID - MSB+LSB
				'z', 'e', 'n', '3', // Client ID "zen"
			},
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Connect,
					Remaining: 16,
				},
				ProtocolName:     []byte("MQTT"),
				ProtocolVersion:  4,
				CleanSession:     false,
				Keepalive:        60,
				ClientIdentifier: "zen3",
			},
		},
		{
			desc: "MQTT 3.1.1, Clean Session",
			rawBytes: []byte{
				byte(Connect << 4), 15, // Fixed header
				0, 4, // Protocol Name - MSB+LSB
				'M', 'Q', 'T', 'T', // Protocol Name
				4,     // Protocol Version
				2,     // Packet Flags
				0, 45, // Keepalive
				0, 3, // Client ID - MSB+LSB
				'z', 'e', 'n', // Client ID "zen"
			},
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Connect,
					Remaining: 15,
				},
				ProtocolName:     []byte("MQTT"),
				ProtocolVersion:  4,
				CleanSession:     true,
				Keepalive:        45,
				ClientIdentifier: "zen",
			},
		},
		{
			desc: "MQTT 3.1.1, Clean Session, LWT",
			rawBytes: []byte{
				byte(Connect << 4), 31, // Fixed header
				0, 4, // Protocol Name - MSB+LSB
				'M', 'Q', 'T', 'T', // Protocol Name
				4,     // Protocol Version
				14,    // Packet Flags
				0, 27, // Keepalive
				0, 3, // Client ID - MSB+LSB
				'z', 'e', 'n', // Client ID "zen"
				0, 3, // Will Topic - MSB+LSB
				'l', 'w', 't',
				0, 9, // Will Message MSB+LSB
				'n', 'o', 't', ' ', 'a', 'g', 'a', 'i', 'n',
			},
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Connect,
					Remaining: 31,
				},
				ProtocolName:     []byte("MQTT"),
				ProtocolVersion:  4,
				CleanSession:     true,
				Keepalive:        27,
				ClientIdentifier: "zen",
				WillFlag:         true,
				WillTopic:        "lwt",
				WillMessage:      []byte("not again"),
				WillQos:          1,
			},
		},
		{
			desc: "MQTT 3.1.1, Username, Password",
			rawBytes: []byte{
				byte(Connect << 4), 28, // Fixed header
				0, 4, // Protocol Name - MSB+LSB
				'M', 'Q', 'T', 'T', // Protocol Name
				4,     // Protocol Version
				194,   // Packet Flags
				0, 20, // Keepalive
				0, 3, // Client ID - MSB+LSB
				'z', 'e', 'n', // Client ID "zen"
				0, 5, // Username MSB+LSB
				'm', 'o', 'c', 'h', 'i',
				0, 4, // Password MSB+LSB
				',', '.', '/', ';',
			},
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Connect,
					Remaining: 28,
				},
				ProtocolName:     []byte("MQTT"),
				ProtocolVersion:  4,
				CleanSession:     true,
				Keepalive:        20,
				ClientIdentifier: "zen",
				UsernameFlag:     true,
				PasswordFlag:     true,
				Username:         []byte("mochi"),
				Password:         []byte(",./;"),
			},
		},
		{
			desc:    "MQTT 3.1.1, Username, Password, LWT",
			primary: true,
			rawBytes: []byte{
				byte(Connect << 4), 44, // Fixed header
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
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Connect,
					Remaining: 44,
				},
				ProtocolName:     []byte("MQTT"),
				ProtocolVersion:  4,
				CleanSession:     true,
				Keepalive:        120,
				ClientIdentifier: "zen",
				UsernameFlag:     true,
				PasswordFlag:     true,
				Username:         []byte("mochi"),
				Password:         []byte(",./;"),
				WillFlag:         true,
				WillTopic:        "lwt",
				WillMessage:      []byte("not again"),
				WillQos:          1,
			},
		},

		// Fail States
		{
			desc:      "Malformed Connect - protocol name",
			group:     "decode",
			failFirst: ErrMalformedProtocolName,
			rawBytes: []byte{
				byte(Connect << 4), 0, // Fixed header
				0, 7, // Protocol Name - MSB+LSB
				'M', 'Q', 'I', 's', 'd', // Protocol Name
			},
		},

		{
			desc:      "Malformed Connect - protocol version",
			group:     "decode",
			failFirst: ErrMalformedProtocolVersion,
			rawBytes: []byte{
				byte(Connect << 4), 0, // Fixed header
				0, 4, // Protocol Name - MSB+LSB
				'M', 'Q', 'T', 'T', // Protocol Name
			},
		},

		{
			desc:      "Malformed Connect - flags",
			group:     "decode",
			failFirst: ErrMalformedFlags,
			rawBytes: []byte{
				byte(Connect << 4), 0, // Fixed header
				0, 4, // Protocol Name - MSB+LSB
				'M', 'Q', 'T', 'T', // Protocol Name
				4, // Protocol Version

			},
		},
		{
			desc:      "Malformed Connect - keepalive",
			group:     "decode",
			failFirst: ErrMalformedKeepalive,
			rawBytes: []byte{
				byte(Connect << 4), 0, // Fixed header
				0, 4, // Protocol Name - MSB+LSB
				'M', 'Q', 'T', 'T', // Protocol Name
				4, // Protocol Version
				0, // Flags
			},
		},
		{
			desc:      "Malformed Connect - client id",
			group:     "decode",
			failFirst: ErrMalformedClientID,
			rawBytes: []byte{
				byte(Connect << 4), 0, // Fixed header
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
			desc:      "Malformed Connect - will topic",
			group:     "decode",
			failFirst: ErrMalformedWillTopic,
			rawBytes: []byte{
				byte(Connect << 4), 0, // Fixed header
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
			desc:      "Malformed Connect - will flag",
			group:     "decode",
			failFirst: ErrMalformedWillMessage,
			rawBytes: []byte{
				byte(Connect << 4), 0, // Fixed header
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
			desc:      "Malformed Connect - username",
			group:     "decode",
			failFirst: ErrMalformedUsername,
			rawBytes: []byte{
				byte(Connect << 4), 0, // Fixed header
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
			desc:      "Malformed Connect - password",
			group:     "decode",
			failFirst: ErrMalformedPassword,
			rawBytes: []byte{
				byte(Connect << 4), 0, // Fixed header
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

		// Validation Tests
		{
			desc:  "Invalid Protocol Name",
			group: "validate",
			code:  CodeConnectProtocolViolation,
			packet: &Packet{
				ProtocolName: []byte("stuff"),
			},
		},
		{
			desc:  "Invalid Protocol Version",
			group: "validate",
			code:  CodeConnectBadProtocolVersion,
			packet: &Packet{
				ProtocolName:    []byte("MQTT"),
				ProtocolVersion: 2,
			},
		},
		{
			desc:  "Invalid Protocol Version",
			group: "validate",
			code:  CodeConnectBadProtocolVersion,
			packet: &Packet{
				ProtocolName:    []byte("MQIsdp"),
				ProtocolVersion: 2,
			},
		},
		{
			desc:  "Reserved bit not 0",
			group: "validate",
			code:  CodeConnectProtocolViolation,
			packet: &Packet{
				ProtocolName:    []byte("MQTT"),
				ProtocolVersion: 4,
				ReservedBit:     1,
			},
		},
		{
			desc:  "Client ID too long",
			group: "validate",
			code:  CodeConnectProtocolViolation,
			packet: &Packet{
				ProtocolName:    []byte("MQTT"),
				ProtocolVersion: 4,
				ClientIdentifier: func() string {
					return string(make([]byte, 65536))
				}(),
			},
		},
		{
			desc:  "Has Password Flag but no Username flag",
			group: "validate",
			code:  CodeConnectProtocolViolation,
			packet: &Packet{
				ProtocolName:    []byte("MQTT"),
				ProtocolVersion: 4,
				PasswordFlag:    true,
			},
		},
		{
			desc:  "Username too long",
			group: "validate",
			code:  CodeConnectProtocolViolation,
			packet: &Packet{
				ProtocolName:    []byte("MQTT"),
				ProtocolVersion: 4,
				UsernameFlag:    true,
				Username: func() []byte {
					return make([]byte, 65536)
				}(),
			},
		},
		{
			desc:  "Password too long",
			group: "validate",
			code:  CodeConnectProtocolViolation,
			packet: &Packet{
				ProtocolName:    []byte("MQTT"),
				ProtocolVersion: 4,
				UsernameFlag:    true,
				Username:        []byte{},
				PasswordFlag:    true,
				Password: func() []byte {
					return make([]byte, 65536)
				}(),
			},
		},
		{
			desc:  "Clean session false and client id not set",
			group: "validate",
			code:  CodeConnectBadClientID,
			packet: &Packet{
				ProtocolName:    []byte("MQTT"),
				ProtocolVersion: 4,
				CleanSession:    false,
			},
		},

		// Spec Tests
		{
			// @SPEC [MQTT-1.4.0-1]
			// The character data in a UTF-8 encoded string MUST be well-formed UTF-8
			// as defined by the Unicode specification [Unicode] and restated in RFC 3629 [RFC 3629].
			// In particular this data MUST NOT include encodings of code points between U+D800 and U+DFFF.
			desc:      "Invalid UTF8 string (a) - Code point U+D800.",
			group:     "decode",
			failFirst: ErrMalformedClientID,
			rawBytes: []byte{
				byte(Connect << 4), 0, // Fixed header
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
			desc:      "Invalid UTF8 string (b) - Code point U+DFFF.",
			group:     "decode",
			failFirst: ErrMalformedClientID,
			rawBytes: []byte{
				byte(Connect << 4), 0, // Fixed header
				0, 4, // Protocol Name - MSB+LSB
				'M', 'Q', 'T', 'T', // Protocol Name
				4,     // Protocol Version
				0,     // Flags
				0, 20, // Keepalive
				0, 4, // Client ID - MSB+LSB
				'e', 0xed, 0xa3, 0xbf, // Client id bearing U+D8FF
			},
		},

		// @SPEC [MQTT-1.4.0-2]
		// A UTF-8 encoded string MUST NOT include an encoding of the null character U+0000.
		{
			desc:      "Invalid UTF8 string (c) - Code point U+0000.",
			group:     "decode",
			failFirst: ErrMalformedClientID,
			rawBytes: []byte{
				byte(Connect << 4), 0, // Fixed header
				0, 4, // Protocol Name - MSB+LSB
				'M', 'Q', 'T', 'T', // Protocol Name
				4,     // Protocol Version
				0,     // Flags
				0, 20, // Keepalive
				0, 3, // Client ID - MSB+LSB
				'e', 0xc0, 0x80, // Client id bearing U+0000
			},
		},

		// @ SPEC [MQTT-1.4.0-3]
		// A UTF-8 encoded sequence 0xEF 0xBB 0xBF is always to be interpreted to mean U+FEFF ("ZERO WIDTH NO-BREAK SPACE")
		// wherever it appears in a string and MUST NOT be skipped over or stripped off by a packet receiver.
		{
			desc: "UTF8 string must not skip or strip code point U+FEFF.",
			//group: "decode",
			//failFirst: ErrMalformedClientID,
			rawBytes: []byte{
				byte(Connect << 4), 18, // Fixed header
				0, 4, // Protocol Name - MSB+LSB
				'M', 'Q', 'T', 'T', // Protocol Name
				4,     // Protocol Version
				0,     // Flags
				0, 20, // Keepalive
				0, 6, // Client ID - MSB+LSB
				'e', 'b', 0xEF, 0xBB, 0xBF, 'd', // Client id bearing U+FEFF
			},
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Connect,
					Remaining: 16,
				},
				ProtocolName:     []byte("MQTT"),
				ProtocolVersion:  4,
				Keepalive:        20,
				ClientIdentifier: string([]byte{'e', 'b', 0xEF, 0xBB, 0xBF, 'd'}),
			},
		},
	},
	Connack: {
		{
			desc:    "Accepted, No Session",
			primary: true,
			rawBytes: []byte{
				byte(Connack << 4), 2, // fixed header
				0, // No existing session
				Accepted,
			},
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Connack,
					Remaining: 2,
				},
				SessionPresent: false,
				ReturnCode:     Accepted,
			},
		},
		{
			desc:    "Accepted, Session Exists",
			primary: true,
			rawBytes: []byte{
				byte(Connack << 4), 2, // fixed header
				1, // Session present
				Accepted,
			},
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Connack,
					Remaining: 2,
				},
				SessionPresent: true,
				ReturnCode:     Accepted,
			},
		},
		{
			desc: "Bad Protocol Version",
			rawBytes: []byte{
				byte(Connack << 4), 2, // fixed header
				1, // Session present
				CodeConnectBadProtocolVersion,
			},
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Connack,
					Remaining: 2,
				},
				SessionPresent: true,
				ReturnCode:     CodeConnectBadProtocolVersion,
			},
		},
		{
			desc: "Bad Client ID",
			rawBytes: []byte{
				byte(Connack << 4), 2, // fixed header
				1, // Session present
				CodeConnectBadClientID,
			},
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Connack,
					Remaining: 2,
				},
				SessionPresent: true,
				ReturnCode:     CodeConnectBadClientID,
			},
		},
		{
			desc: "Server Unavailable",
			rawBytes: []byte{
				byte(Connack << 4), 2, // fixed header
				1, // Session present
				CodeConnectServerUnavailable,
			},
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Connack,
					Remaining: 2,
				},
				SessionPresent: true,
				ReturnCode:     CodeConnectServerUnavailable,
			},
		},
		{
			desc: "Bad Username or Password",
			rawBytes: []byte{
				byte(Connack << 4), 2, // fixed header
				1, // Session present
				CodeConnectBadAuthValues,
			},
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Connack,
					Remaining: 2,
				},
				SessionPresent: true,
				ReturnCode:     CodeConnectBadAuthValues,
			},
		},
		{
			desc: "Not Authorised",
			rawBytes: []byte{
				byte(Connack << 4), 2, // fixed header
				1, // Session present
				CodeConnectNotAuthorised,
			},
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Connack,
					Remaining: 2,
				},
				SessionPresent: true,
				ReturnCode:     CodeConnectNotAuthorised,
			},
		},

		// Fail States
		{
			desc:      "Malformed Connack - session present",
			group:     "decode",
			failFirst: ErrMalformedSessionPresent,
			rawBytes: []byte{
				byte(Connect << 4), 2, // Fixed header
			},
		},
		{
			desc:  "Malformed Connack - bad return code",
			group: "decode",
			//primary:   true,
			failFirst: ErrMalformedReturnCode,
			rawBytes: []byte{
				byte(Connect << 4), 2, // Fixed header
				0,
			},
		},
	},

	Publish: {
		{
			desc:    "Publish - No payload",
			primary: true,
			rawBytes: []byte{
				byte(Publish << 4), 7, // Fixed header
				0, 5, // Topic Name - LSB+MSB
				'a', '/', 'b', '/', 'c', // Topic Name
			},
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Publish,
					Remaining: 7,
				},
				TopicName: "a/b/c",
				Payload:   []byte{},
			},
		},
		{
			desc:    "Publish - basic",
			primary: true,
			rawBytes: []byte{
				byte(Publish << 4), 18, // Fixed header
				0, 5, // Topic Name - LSB+MSB
				'a', '/', 'b', '/', 'c', // Topic Name
				'h', 'e', 'l', 'l', 'o', ' ', 'm', 'o', 'c', 'h', 'i', // Payload
			},
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Publish,
					Remaining: 18,
				},
				TopicName: "a/b/c",
				Payload:   []byte("hello mochi"),
			},
		},
		{
			desc:    "Publish - QoS:1, Packet ID",
			primary: true,
			rawBytes: []byte{
				byte(Publish<<4) | 2, 14, // Fixed header
				0, 5, // Topic Name - LSB+MSB
				'a', '/', 'b', '/', 'c', // Topic Name
				0, 7, // Packet ID - LSB+MSB
				'h', 'e', 'l', 'l', 'o', // Payload
			},
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Publish,
					Qos:       1,
					Remaining: 14,
				},
				TopicName: "a/b/c",
				Payload:   []byte("hello"),
				PacketID:  7,
			},
			meta: byte(2),
		},
		{
			desc:    "Publish - QoS:1, Packet ID, No payload",
			primary: true,
			rawBytes: []byte{
				byte(Publish<<4) | 2, 9, // Fixed header
				0, 5, // Topic Name - LSB+MSB
				'y', '/', 'u', '/', 'i', // Topic Name
				0, 8, // Packet ID - LSB+MSB
			},
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Publish,
					Qos:       1,
					Remaining: 9,
				},
				TopicName: "y/u/i",
				PacketID:  8,
				Payload:   []byte{},
			},
			meta: byte(2),
		},
		{
			desc: "Publish - Retain",
			rawBytes: []byte{
				byte(Publish<<4) | 1, 10, // Fixed header
				0, 3, // Topic Name - LSB+MSB
				'a', '/', 'b', // Topic Name
				'h', 'e', 'l', 'l', 'o', // Payload
			},
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type:   Publish,
					Retain: true,
				},
				TopicName: "a/b",
				Payload:   []byte("hello"),
			},
			meta: byte(1),
		},
		{
			desc: "Publish - Dup",
			rawBytes: []byte{
				byte(Publish<<4) | 8, 10, // Fixed header
				0, 3, // Topic Name - LSB+MSB
				'a', '/', 'b', // Topic Name
				'h', 'e', 'l', 'l', 'o', // Payload
			},
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type: Publish,
					Dup:  true,
				},
				TopicName: "a/b",
				Payload:   []byte("hello"),
			},
			meta: byte(8),
		},

		// Fail States
		{
			desc:      "Malformed Publish - topic name",
			group:     "decode",
			failFirst: ErrMalformedTopic,
			rawBytes: []byte{
				byte(Publish << 4), 7, // Fixed header
				0, 5, // Topic Name - LSB+MSB
				'a', '/',
				0, 11, // Packet ID - LSB+MSB
			},
		},

		{
			desc:      "Malformed Publish - Packet ID",
			group:     "decode",
			failFirst: ErrMalformedPacketID,
			rawBytes: []byte{
				byte(Publish<<4) | 2, 7, // Fixed header
				0, 5, // Topic Name - LSB+MSB
				'x', '/', 'y', '/', 'z', // Topic Name
				0, // Packet ID - LSB+MSB
			},
		},

		// Copy tests
		{
			desc:  "Publish - basic copyable",
			group: "copy",
			rawBytes: []byte{
				byte(Publish << 4), 18, // Fixed header
				0, 5, // Topic Name - LSB+MSB
				'z', '/', 'e', '/', 'n', // Topic Name
				'm', 'o', 'c', 'h', 'i', ' ', 'm', 'o', 'c', 'h', 'i', // Payload
			},
			packet: &Packet{
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
			// @SPEC [MQTT-2.3.1-5]
			// A PUBLISH Packet MUST NOT contain a Packet Identifier if its QoS value is set to 0.
			desc:  "[MQTT-2.3.1-5] Packet ID must be 0 if QoS is 0 (a)",
			group: "encode",
			// this version tests for correct byte array mutuation.
			// this does not check if -incoming- packets are parsed as correct,
			// it is impossible for the parser to determine if the payload start is incorrect.
			rawBytes: []byte{
				byte(Publish << 4), 12, // Fixed header
				0, 5, // Topic Name - LSB+MSB
				'a', '/', 'b', '/', 'c', // Topic Name
				0, 3, // Packet ID - LSB+MSB
				'h', 'e', 'l', 'l', 'o', // Payload
			},
			actualBytes: []byte{
				byte(Publish << 4), 12, // Fixed header
				0, 5, // Topic Name - LSB+MSB
				'a', '/', 'b', '/', 'c', // Topic Name
				// Packet ID is removed.
				'h', 'e', 'l', 'l', 'o', // Payload
			},
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Publish,
					Remaining: 12,
				},
				TopicName: "a/b/c",
				Payload:   []byte("hello"),
			},
		},
		{
			// @SPEC [MQTT-2.3.1-1].
			// SUBSCRIBE, UNSUBSCRIBE, and PUBLISH (in cases where QoS > 0) Control Packets MUST contain a non-zero 16-bit Packet Identifier.
			desc:   "[MQTT-2.3.1-1] No Packet ID with QOS > 0",
			group:  "encode",
			expect: ErrMissingPacketID,
			code:   Failed,
			rawBytes: []byte{
				byte(Publish<<4) | 2, 14, // Fixed header
				0, 5, // Topic Name - LSB+MSB
				'a', '/', 'b', '/', 'c', // Topic Name
				0, 0, // Packet ID - LSB+MSB
				'h', 'e', 'l', 'l', 'o', // Payload
			},
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type: Publish,
					Qos:  1,
				},
				TopicName: "a/b/c",
				Payload:   []byte("hello"),
				PacketID:  0,
			},
			meta: byte(2),
		},
		/*
			{
				// @SPEC [MQTT-2.3.1-1].
				// SUBSCRIBE, UNSUBSCRIBE, and PUBLISH (in cases where QoS > 0) Control Packets MUST contain a non-zero 16-bit Packet Identifier.
				desc:  "[MQTT-2.3.1-1] No Packet ID with QOS > 0",
				group: "validate",
				//primary: true,
				expect: ErrMissingPacketID,
				code:   Failed,
				rawBytes: []byte{
					byte(Publish<<4) | 2, 14, // Fixed header
					0, 5, // Topic Name - LSB+MSB
					'a', '/', 'b', '/', 'c', // Topic Name
					0, 0, // Packet ID - LSB+MSB
					'h', 'e', 'l', 'l', 'o', // Payload
				},
				packet: &Packet{
					FixedHeader: FixedHeader{
						Type: Publish,
						Qos:  1,
					},
					TopicName: "a/b/c",
					Payload:   []byte("hello"),
					PacketID:  0,
				},
				meta: byte(2),
			},

		*/

		// Validation Tests
		{
			// @SPEC [MQTT-2.3.1-5]
			desc:   "[MQTT-2.3.1-5] Packet ID must be 0 if QoS is 0 (b)",
			group:  "validate",
			expect: ErrSurplusPacketID,
			code:   Failed,
			packet: &Packet{
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
			// @SPEC [MQTT-2.3.1-1].
			// SUBSCRIBE, UNSUBSCRIBE, and PUBLISH (in cases where QoS > 0) Control Packets MUST contain a non-zero 16-bit Packet Identifier.
			desc:   "[MQTT-2.3.1-1] No Packet ID with QOS > 0",
			group:  "validate",
			expect: ErrMissingPacketID,
			code:   Failed,
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type: Publish,
					Qos:  1,
				},
				PacketID: 0,
			},
		},
	},

	Puback: {
		{
			desc:    "Puback",
			primary: true,
			rawBytes: []byte{
				byte(Puback << 4), 2, // Fixed header
				0, 11, // Packet ID - LSB+MSB
			},
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Puback,
					Remaining: 2,
				},
				PacketID: 11,
			},
		},

		// Fail states
		{
			desc:      "Malformed Puback - Packet ID",
			group:     "decode",
			failFirst: ErrMalformedPacketID,
			rawBytes: []byte{
				byte(Puback << 4), 2, // Fixed header
				0, // Packet ID - LSB+MSB
			},
		},
	},
	Pubrec: {
		{
			desc:    "Pubrec",
			primary: true,
			rawBytes: []byte{
				byte(Pubrec << 4), 2, // Fixed header
				0, 12, // Packet ID - LSB+MSB
			},
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Pubrec,
					Remaining: 2,
				},
				PacketID: 12,
			},
		},

		// Fail states
		{
			desc:      "Malformed Pubrec - Packet ID",
			group:     "decode",
			failFirst: ErrMalformedPacketID,
			rawBytes: []byte{
				byte(Pubrec << 4), 2, // Fixed header
				0, // Packet ID - LSB+MSB
			},
		},
	},
	Pubrel: {
		{
			desc:    "Pubrel",
			primary: true,
			rawBytes: []byte{
				byte(Pubrel<<4) | 2, 2, // Fixed header
				0, 12, // Packet ID - LSB+MSB
			},
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Pubrel,
					Remaining: 2,
					Qos:       1,
				},
				PacketID: 12,
			},
			meta: byte(2),
		},

		// Fail states
		{
			desc:      "Malformed Pubrel - Packet ID",
			group:     "decode",
			failFirst: ErrMalformedPacketID,
			rawBytes: []byte{
				byte(Pubrel << 4), 2, // Fixed header
				0, // Packet ID - LSB+MSB
			},
		},
	},
	Pubcomp: {
		{
			desc:    "Pubcomp",
			primary: true,
			rawBytes: []byte{
				byte(Pubcomp << 4), 2, // Fixed header
				0, 14, // Packet ID - LSB+MSB
			},
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Pubcomp,
					Remaining: 2,
				},
				PacketID: 14,
			},
		},

		// Fail states
		{
			desc:      "Malformed Pubcomp - Packet ID",
			group:     "decode",
			failFirst: ErrMalformedPacketID,
			rawBytes: []byte{
				byte(Pubcomp << 4), 2, // Fixed header
				0, // Packet ID - LSB+MSB
			},
		},
	},
	Subscribe: {
		{
			desc:    "Subscribe",
			primary: true,
			rawBytes: []byte{
				byte(Subscribe << 4), 30, // Fixed header
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
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Subscribe,
					Remaining: 30,
				},
				PacketID: 15,
				Topics: []string{
					"a/b",
					"d/e/f/g/h/i",
					"x/y/z",
				},
				Qoss: []byte{0, 1, 2},
			},
		},

		// Fail states
		{
			desc:      "Malformed Subscribe - Packet ID",
			group:     "decode",
			failFirst: ErrMalformedPacketID,
			rawBytes: []byte{
				byte(Subscribe << 4), 2, // Fixed header
				0, // Packet ID - LSB+MSB
			},
		},
		{
			desc:      "Malformed Subscribe - topic",
			group:     "decode",
			failFirst: ErrMalformedTopic,
			rawBytes: []byte{
				byte(Subscribe << 4), 2, // Fixed header
				0, 21, // Packet ID - LSB+MSB

				0, 3, // Topic Name - LSB+MSB
				'a', '/',
			},
		},
		{
			desc:      "Malformed Subscribe - qos",
			group:     "decode",
			failFirst: ErrMalformedQoS,
			rawBytes: []byte{
				byte(Subscribe << 4), 2, // Fixed header
				0, 22, // Packet ID - LSB+MSB

				0, 3, // Topic Name - LSB+MSB
				'j', '/', 'b', // Topic Name

			},
		},
		{
			desc:      "Malformed Subscribe - qos out of range",
			group:     "decode",
			failFirst: ErrMalformedQoS,
			rawBytes: []byte{
				byte(Subscribe << 4), 2, // Fixed header
				0, 22, // Packet ID - LSB+MSB

				0, 3, // Topic Name - LSB+MSB
				'c', '/', 'd', // Topic Name
				5, // QoS

			},
		},

		// Validation
		{
			// @SPEC [MQTT-2.3.1-1].
			// SUBSCRIBE, UNSUBSCRIBE, and PUBLISH (in cases where QoS > 0) Control Packets MUST contain a non-zero 16-bit Packet Identifier.
			desc:   "[MQTT-2.3.1-1] Subscribe No Packet ID with QOS > 0",
			group:  "validate",
			expect: ErrMissingPacketID,
			code:   Failed,
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type: Subscribe,
					Qos:  1,
				},
				PacketID: 0,
			},
		},

		// Spec tests
		{
			// @SPEC [MQTT-2.3.1-1].
			// SUBSCRIBE, UNSUBSCRIBE, and PUBLISH (in cases where QoS > 0) Control Packets MUST contain a non-zero 16-bit Packet Identifier.
			desc:   "[MQTT-2.3.1-1] Subscribe No Packet ID with QOS > 0",
			group:  "encode",
			code:   Failed,
			expect: ErrMissingPacketID,
			rawBytes: []byte{
				byte(Subscribe<<4) | 1<<1, 10, // Fixed header
				0, 0, // Packet ID - LSB+MSB
				0, 5, // Topic Name - LSB+MSB
				'a', '/', 'b', '/', 'c', // Topic Name
				1, // QoS
			},
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Subscribe,
					Qos:       1,
					Remaining: 10,
				},
				Topics: []string{
					"a/b/c",
				},
				Qoss:     []byte{1},
				PacketID: 0,
			},
			meta: byte(2),
		},
	},
	Suback: {
		{
			desc:    "Suback",
			primary: true,
			rawBytes: []byte{
				byte(Suback << 4), 6, // Fixed header
				0, 17, // Packet ID - LSB+MSB
				0,    // Return Code QoS 0
				1,    // Return Code QoS 1
				2,    // Return Code QoS 2
				0x80, // Return Code fail
			},
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Suback,
					Remaining: 6,
				},
				PacketID:    17,
				ReturnCodes: []byte{0, 1, 2, 0x80},
			},
		},

		// Fail states
		{
			desc:      "Malformed Suback - Packet ID",
			group:     "decode",
			failFirst: ErrMalformedPacketID,
			rawBytes: []byte{
				byte(Subscribe << 4), 2, // Fixed header
				0, // Packet ID - LSB+MSB
			},
		},
	},

	Unsubscribe: {
		{
			desc:    "Unsubscribe",
			primary: true,
			rawBytes: []byte{
				byte(Unsubscribe << 4), 27, // Fixed header
				0, 35, // Packet ID - LSB+MSB

				0, 3, // Topic Name - LSB+MSB
				'a', '/', 'b', // Topic Name

				0, 11, // Topic Name - LSB+MSB
				'd', '/', 'e', '/', 'f', '/', 'g', '/', 'h', '/', 'i', // Topic Name

				0, 5, // Topic Name - LSB+MSB
				'x', '/', 'y', '/', 'z', // Topic Name
			},
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Unsubscribe,
					Remaining: 27,
				},
				PacketID: 35,
				Topics: []string{
					"a/b",
					"d/e/f/g/h/i",
					"x/y/z",
				},
			},
		},
		// Fail states
		{
			desc:      "Malformed Unsubscribe - Packet ID",
			group:     "decode",
			failFirst: ErrMalformedPacketID,
			rawBytes: []byte{
				byte(Unsubscribe << 4), 2, // Fixed header
				0, // Packet ID - LSB+MSB
			},
		},
		{
			desc:      "Malformed Unsubscribe - topic",
			group:     "decode",
			failFirst: ErrMalformedTopic,
			rawBytes: []byte{
				byte(Unsubscribe << 4), 2, // Fixed header
				0, 21, // Packet ID - LSB+MSB
				0, 3, // Topic Name - LSB+MSB
				'a', '/',
			},
		},

		// Validation
		{
			// @SPEC [MQTT-2.3.1-1].
			// SUBSCRIBE, UNSUBSCRIBE, and PUBLISH (in cases where QoS > 0) Control Packets MUST contain a non-zero 16-bit Packet Identifier.
			desc:   "[MQTT-2.3.1-1] Subscribe No Packet ID with QOS > 0",
			group:  "validate",
			expect: ErrMissingPacketID,
			code:   Failed,
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type: Unsubscribe,
					Qos:  1,
				},
				PacketID: 0,
			},
		},

		// Spec tests
		{
			// @SPEC [MQTT-2.3.1-1].
			// SUBSCRIBE, UNSUBSCRIBE, and PUBLISH (in cases where QoS > 0) Control Packets MUST contain a non-zero 16-bit Packet Identifier.
			desc:   "[MQTT-2.3.1-1] Unsubscribe No Packet ID with QOS > 0",
			group:  "encode",
			code:   Failed,
			expect: ErrMissingPacketID,
			rawBytes: []byte{
				byte(Unsubscribe<<4) | 1<<1, 9, // Fixed header
				0, 0, // Packet ID - LSB+MSB
				0, 5, // Topic Name - LSB+MSB
				'a', '/', 'b', '/', 'c', // Topic Name
			},
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Unsubscribe,
					Qos:       1,
					Remaining: 9,
				},
				Topics: []string{
					"a/b/c",
				},
				PacketID: 0,
			},
			meta: byte(2),
		},
	},
	Unsuback: {
		{
			desc:    "Unsuback",
			primary: true,
			rawBytes: []byte{
				byte(Unsuback << 4), 2, // Fixed header
				0, 37, // Packet ID - LSB+MSB

			},
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Unsuback,
					Remaining: 2,
				},
				PacketID: 37,
			},
		},

		// Fail states
		{
			desc:      "Malformed Unsuback - Packet ID",
			group:     "decode",
			failFirst: ErrMalformedPacketID,
			rawBytes: []byte{
				byte(Unsuback << 4), 2, // Fixed header
				0, // Packet ID - LSB+MSB
			},
		},
	},

	Pingreq: {
		{
			desc:    "Ping request",
			primary: true,
			rawBytes: []byte{
				byte(Pingreq << 4), 0, // fixed header
			},
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Pingreq,
					Remaining: 0,
				},
			},
		},
	},
	Pingresp: {
		{
			desc:    "Ping response",
			primary: true,
			rawBytes: []byte{
				byte(Pingresp << 4), 0, // fixed header
			},
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Pingresp,
					Remaining: 0,
				},
			},
		},
	},

	Disconnect: {
		{
			desc:    "Disconnect",
			primary: true,
			rawBytes: []byte{
				byte(Disconnect << 4), 0, // fixed header
			},
			packet: &Packet{
				FixedHeader: FixedHeader{
					Type:      Disconnect,
					Remaining: 0,
				},
			},
		},
	},
}
