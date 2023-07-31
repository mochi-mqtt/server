// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package packets

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	propertiesStruct = Properties{
		PayloadFormat:             byte(1), // UTF-8 Format
		PayloadFormatFlag:         true,
		MessageExpiryInterval:     uint32(2),
		ContentType:               "text/plain",
		ResponseTopic:             "a/b/c",
		CorrelationData:           []byte("data"),
		SubscriptionIdentifier:    []int{322122},
		SessionExpiryInterval:     uint32(120),
		SessionExpiryIntervalFlag: true,
		AssignedClientID:          "mochi-v5",
		ServerKeepAlive:           uint16(20),
		ServerKeepAliveFlag:       true,
		AuthenticationMethod:      "SHA-1",
		AuthenticationData:        []byte("auth-data"),
		RequestProblemInfo:        byte(1),
		RequestProblemInfoFlag:    true,
		WillDelayInterval:         uint32(600),
		RequestResponseInfo:       byte(1),
		ResponseInfo:              "response",
		ServerReference:           "mochi-2",
		ReasonString:              "reason",
		ReceiveMaximum:            uint16(500),
		TopicAliasMaximum:         uint16(999),
		TopicAlias:                uint16(3),
		TopicAliasFlag:            true,
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
	}

	propertiesBytes = []byte{
		172, 1, // VBI

		// Payload Format (1) (vbi:2)
		1, 1,

		// Message Expiry (2) (vbi:7)
		2, 0, 0, 0, 2,

		// Content Type (3) (vbi:20)
		3,
		0, 10, 't', 'e', 'x', 't', '/', 'p', 'l', 'a', 'i', 'n',

		// Response Topic (8) (vbi:28)
		8,
		0, 5, 'a', '/', 'b', '/', 'c',

		// Correlations Data (9) (vbi:35)
		9,
		0, 4, 'd', 'a', 't', 'a',

		// Subscription Identifier (11) (vbi:39)
		11,
		202, 212, 19,

		// Session Expiry Interval (17) (vbi:43)
		17,
		0, 0, 0, 120,

		// Assigned Client ID (18) (vbi:55)
		18,
		0, 8, 'm', 'o', 'c', 'h', 'i', '-', 'v', '5',

		// Server Keep Alive (19) (vbi:58)
		19,
		0, 20,

		// Authentication Method (21) (vbi:66)
		21,
		0, 5, 'S', 'H', 'A', '-', '1',

		// Authentication Data (22) (vbi:78)
		22,
		0, 9, 'a', 'u', 't', 'h', '-', 'd', 'a', 't', 'a',

		// Request Problem Info (23) (vbi:80)
		23, 1,

		// Will Delay Interval (24) (vbi:85)
		24,
		0, 0, 2, 88,

		// Request Response Info (25) (vbi:87)
		25, 1,

		// Response Info (26) (vbi:98)
		26,
		0, 8, 'r', 'e', 's', 'p', 'o', 'n', 's', 'e',

		// Server Reference (28) (vbi:108)
		28,
		0, 7, 'm', 'o', 'c', 'h', 'i', '-', '2',

		// Reason String (31) (vbi:117)
		31,
		0, 6, 'r', 'e', 'a', 's', 'o', 'n',

		// Receive Maximum (33) (vbi:120)
		33,
		1, 244,

		// Topic Alias Maximum (34) (vbi:123)
		34,
		3, 231,

		// Topic Alias (35) (vbi:126)
		35,
		0, 3,

		// Maximum Qos (36) (vbi:128)
		36, 1,

		// Retain Available (37) (vbi: 130)
		37, 1,

		// User Properties (38) (vbi:161)
		38,
		0, 5, 'h', 'e', 'l', 'l', 'o',
		0, 6, 228, 184, 150, 231, 149, 140,
		38,
		0, 4, 'k', 'e', 'y', '2',
		0, 6, 'v', 'a', 'l', 'u', 'e', '2',

		// Maximum Packet Size (39) (vbi:166)
		39,
		0, 0, 125, 0,

		// Wildcard Subscriptions Available (40)  (vbi:168)
		40, 1,

		// Subscription ID Available (41) (vbi:170)
		41, 1,

		// Shared Subscriptions Available (42) (vbi:172)
		42, 1,
	}
)

func init() {
	validPacketProperties[PropPayloadFormat][Reserved] = 1
	validPacketProperties[PropMessageExpiryInterval][Reserved] = 1
	validPacketProperties[PropContentType][Reserved] = 1
	validPacketProperties[PropResponseTopic][Reserved] = 1
	validPacketProperties[PropCorrelationData][Reserved] = 1
	validPacketProperties[PropSubscriptionIdentifier][Reserved] = 1
	validPacketProperties[PropSessionExpiryInterval][Reserved] = 1
	validPacketProperties[PropAssignedClientID][Reserved] = 1
	validPacketProperties[PropServerKeepAlive][Reserved] = 1
	validPacketProperties[PropAuthenticationMethod][Reserved] = 1
	validPacketProperties[PropAuthenticationData][Reserved] = 1
	validPacketProperties[PropRequestProblemInfo][Reserved] = 1
	validPacketProperties[PropWillDelayInterval][Reserved] = 1
	validPacketProperties[PropRequestResponseInfo][Reserved] = 1
	validPacketProperties[PropResponseInfo][Reserved] = 1
	validPacketProperties[PropServerReference][Reserved] = 1
	validPacketProperties[PropReasonString][Reserved] = 1
	validPacketProperties[PropReceiveMaximum][Reserved] = 1
	validPacketProperties[PropTopicAliasMaximum][Reserved] = 1
	validPacketProperties[PropTopicAlias][Reserved] = 1
	validPacketProperties[PropMaximumQos][Reserved] = 1
	validPacketProperties[PropRetainAvailable][Reserved] = 1
	validPacketProperties[PropUser][Reserved] = 1
	validPacketProperties[PropMaximumPacketSize][Reserved] = 1
	validPacketProperties[PropWildcardSubAvailable][Reserved] = 1
	validPacketProperties[PropSubIDAvailable][Reserved] = 1
	validPacketProperties[PropSharedSubAvailable][Reserved] = 1
}

func TestEncodeProperties(t *testing.T) {
	props := propertiesStruct
	b := bytes.NewBuffer([]byte{})
	props.Encode(Reserved, Mods{AllowResponseInfo: true}, b, 0)
	require.Equal(t, propertiesBytes, b.Bytes())
}

func TestEncodePropertiesDisallowProblemInfo(t *testing.T) {
	props := propertiesStruct
	b := bytes.NewBuffer([]byte{})
	props.Encode(Reserved, Mods{DisallowProblemInfo: true}, b, 0)
	require.NotEqual(t, propertiesBytes, b.Bytes())
	require.False(t, bytes.Contains(b.Bytes(), []byte{31, 0, 6}))
	require.False(t, bytes.Contains(b.Bytes(), []byte{38, 0, 5}))
	require.False(t, bytes.Contains(b.Bytes(), []byte{26, 0, 8}))
}

func TestEncodePropertiesDisallowResponseInfo(t *testing.T) {
	props := propertiesStruct
	b := bytes.NewBuffer([]byte{})
	props.Encode(Reserved, Mods{AllowResponseInfo: false}, b, 0)
	require.NotEqual(t, propertiesBytes, b.Bytes())
	require.NotContains(t, b.Bytes(), []byte{8, 0, 5})
	require.NotContains(t, b.Bytes(), []byte{9, 0, 4})
}

func TestEncodePropertiesNil(t *testing.T) {
	type tmp struct {
		p *Properties
	}

	pr := tmp{}
	b := bytes.NewBuffer([]byte{})
	pr.p.Encode(Reserved, Mods{}, b, 0)
	require.Equal(t, []byte{}, b.Bytes())
}

func TestEncodeZeroProperties(t *testing.T) {
	// [MQTT-2.2.2-1] If there are no properties, this MUST be indicated by including a Property Length of zero.
	props := new(Properties)
	b := bytes.NewBuffer([]byte{})
	props.Encode(Reserved, Mods{AllowResponseInfo: true}, b, 0)
	require.Equal(t, []byte{0x00}, b.Bytes())
}

func TestDecodeProperties(t *testing.T) {
	b := bytes.NewBuffer(propertiesBytes)

	props := new(Properties)
	n, err := props.Decode(Reserved, b)
	require.NoError(t, err)
	require.Equal(t, 172+2, n)
	require.EqualValues(t, propertiesStruct, *props)
}

func TestDecodePropertiesNil(t *testing.T) {
	b := bytes.NewBuffer(propertiesBytes)

	type tmp struct {
		p *Properties
	}

	pr := tmp{}
	n, err := pr.p.Decode(Reserved, b)
	require.NoError(t, err)
	require.Equal(t, 0, n)
}

func TestDecodePropertiesBadInitialVBI(t *testing.T) {
	b := bytes.NewBuffer([]byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255})
	props := new(Properties)
	_, err := props.Decode(Reserved, b)
	require.Error(t, err)
	require.ErrorIs(t, ErrMalformedVariableByteInteger, err)
}

func TestDecodePropertiesZeroLengthVBI(t *testing.T) {
	b := bytes.NewBuffer([]byte{0})
	props := new(Properties)
	_, err := props.Decode(Reserved, b)
	require.NoError(t, err)
	require.Equal(t, props, new(Properties))
}

func TestDecodePropertiesBadKeyByte(t *testing.T) {
	b := bytes.NewBuffer([]byte{64, 1})
	props := new(Properties)
	_, err := props.Decode(Reserved, b)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrMalformedOffsetByteOutOfRange)
}

func TestDecodePropertiesInvalidForPacket(t *testing.T) {
	b := bytes.NewBuffer([]byte{1, 99})
	props := new(Properties)
	_, err := props.Decode(Reserved, b)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrProtocolViolationUnsupportedProperty)
}

func TestDecodePropertiesGeneralFailure(t *testing.T) {
	b := bytes.NewBuffer([]byte{10, 11, 202, 212, 19})
	props := new(Properties)
	_, err := props.Decode(Reserved, b)
	require.Error(t, err)
}

func TestDecodePropertiesBadSubscriptionID(t *testing.T) {
	b := bytes.NewBuffer([]byte{10, 11, 255, 255, 255, 255, 255, 255, 255, 255})
	props := new(Properties)
	_, err := props.Decode(Reserved, b)
	require.Error(t, err)
}

func TestDecodePropertiesBadUserProps(t *testing.T) {
	b := bytes.NewBuffer([]byte{10, 38, 255, 255, 255, 255, 255, 255, 255, 255})
	props := new(Properties)
	_, err := props.Decode(Reserved, b)
	require.Error(t, err)
}

func TestCopyProperties(t *testing.T) {
	require.EqualValues(t, propertiesStruct, propertiesStruct.Copy(true))
}

func TestCopyPropertiesNoTransfer(t *testing.T) {
	pkA := propertiesStruct
	pkB := pkA.Copy(false)

	// Properties which should never be transferred from one connection to another
	require.Equal(t, uint16(0), pkB.TopicAlias)
}
