// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package packets

import (
	"bytes"
	"fmt"
	"strings"
)

const (
	PropPayloadFormat          byte = 1
	PropMessageExpiryInterval  byte = 2
	PropContentType            byte = 3
	PropResponseTopic          byte = 8
	PropCorrelationData        byte = 9
	PropSubscriptionIdentifier byte = 11
	PropSessionExpiryInterval  byte = 17
	PropAssignedClientID       byte = 18
	PropServerKeepAlive        byte = 19
	PropAuthenticationMethod   byte = 21
	PropAuthenticationData     byte = 22
	PropRequestProblemInfo     byte = 23
	PropWillDelayInterval      byte = 24
	PropRequestResponseInfo    byte = 25
	PropResponseInfo           byte = 26
	PropServerReference        byte = 28
	PropReasonString           byte = 31
	PropReceiveMaximum         byte = 33
	PropTopicAliasMaximum      byte = 34
	PropTopicAlias             byte = 35
	PropMaximumQos             byte = 36
	PropRetainAvailable        byte = 37
	PropUser                   byte = 38
	PropMaximumPacketSize      byte = 39
	PropWildcardSubAvailable   byte = 40
	PropSubIDAvailable         byte = 41
	PropSharedSubAvailable     byte = 42
)

// validPacketProperties indicates which properties are valid for which packet types.
var validPacketProperties = map[byte]map[byte]byte{
	PropPayloadFormat:          {Publish: 1, WillProperties: 1},
	PropMessageExpiryInterval:  {Publish: 1, WillProperties: 1},
	PropContentType:            {Publish: 1, WillProperties: 1},
	PropResponseTopic:          {Publish: 1, WillProperties: 1},
	PropCorrelationData:        {Publish: 1, WillProperties: 1},
	PropSubscriptionIdentifier: {Publish: 1, Subscribe: 1},
	PropSessionExpiryInterval:  {Connect: 1, Connack: 1, Disconnect: 1},
	PropAssignedClientID:       {Connack: 1},
	PropServerKeepAlive:        {Connack: 1},
	PropAuthenticationMethod:   {Connect: 1, Connack: 1, Auth: 1},
	PropAuthenticationData:     {Connect: 1, Connack: 1, Auth: 1},
	PropRequestProblemInfo:     {Connect: 1},
	PropWillDelayInterval:      {WillProperties: 1},
	PropRequestResponseInfo:    {Connect: 1},
	PropResponseInfo:           {Connack: 1},
	PropServerReference:        {Connack: 1, Disconnect: 1},
	PropReasonString:           {Connack: 1, Puback: 1, Pubrec: 1, Pubrel: 1, Pubcomp: 1, Suback: 1, Unsuback: 1, Disconnect: 1, Auth: 1},
	PropReceiveMaximum:         {Connect: 1, Connack: 1},
	PropTopicAliasMaximum:      {Connect: 1, Connack: 1},
	PropTopicAlias:             {Publish: 1},
	PropMaximumQos:             {Connack: 1},
	PropRetainAvailable:        {Connack: 1},
	PropUser:                   {Connect: 1, Connack: 1, Publish: 1, Puback: 1, Pubrec: 1, Pubrel: 1, Pubcomp: 1, Subscribe: 1, Suback: 1, Unsubscribe: 1, Unsuback: 1, Disconnect: 1, Auth: 1, WillProperties: 1},
	PropMaximumPacketSize:      {Connect: 1, Connack: 1},
	PropWildcardSubAvailable:   {Connack: 1},
	PropSubIDAvailable:         {Connack: 1},
	PropSharedSubAvailable:     {Connack: 1},
}

// UserProperty is an arbitrary key-value pair for a packet user properties array.
type UserProperty struct { // [MQTT-1.5.7-1]
	Key string `json:"k"`
	Val string `json:"v"`
}

// Properties contains all mqtt v5 properties available for a packet.
// Some properties have valid values of 0 or not-present. In this case, we opt for
// property flags to indicate the usage of property.
// Refer to mqtt v5 2.2.2.2 Property spec for more information.
type Properties struct {
	CorrelationData           []byte         `json:"cd"`
	SubscriptionIdentifier    []int          `json:"si"`
	AuthenticationData        []byte         `json:"ad"`
	User                      []UserProperty `json:"user"`
	ContentType               string         `json:"ct"`
	ResponseTopic             string         `json:"rt"`
	AssignedClientID          string         `json:"aci"`
	AuthenticationMethod      string         `json:"am"`
	ResponseInfo              string         `json:"ri"`
	ServerReference           string         `json:"sr"`
	ReasonString              string         `json:"rs"`
	MessageExpiryInterval     uint32         `json:"me"`
	SessionExpiryInterval     uint32         `json:"sei"`
	WillDelayInterval         uint32         `json:"wdi"`
	MaximumPacketSize         uint32         `json:"mps"`
	ServerKeepAlive           uint16         `json:"ska"`
	ReceiveMaximum            uint16         `json:"rm"`
	TopicAliasMaximum         uint16         `json:"tam"`
	TopicAlias                uint16         `json:"ta"`
	PayloadFormat             byte           `json:"pf"`
	PayloadFormatFlag         bool           `json:"fpf"`
	SessionExpiryIntervalFlag bool           `json:"fsei"`
	ServerKeepAliveFlag       bool           `json:"fska"`
	RequestProblemInfo        byte           `json:"rpi"`
	RequestProblemInfoFlag    bool           `json:"frpi"`
	RequestResponseInfo       byte           `json:"rri"`
	TopicAliasFlag            bool           `json:"fta"`
	MaximumQos                byte           `json:"mqos"`
	MaximumQosFlag            bool           `json:"fmqos"`
	RetainAvailable           byte           `json:"ra"`
	RetainAvailableFlag       bool           `json:"fra"`
	WildcardSubAvailable      byte           `json:"wsa"`
	WildcardSubAvailableFlag  bool           `json:"fwsa"`
	SubIDAvailable            byte           `json:"sida"`
	SubIDAvailableFlag        bool           `json:"fsida"`
	SharedSubAvailable        byte           `json:"ssa"`
	SharedSubAvailableFlag    bool           `json:"fssa"`
}

// Copy creates a new Properties struct with copies of the values.
func (p *Properties) Copy(allowTransfer bool) Properties {
	pr := Properties{
		PayloadFormat:             p.PayloadFormat, // [MQTT-3.3.2-4]
		PayloadFormatFlag:         p.PayloadFormatFlag,
		MessageExpiryInterval:     p.MessageExpiryInterval,
		ContentType:               p.ContentType,   // [MQTT-3.3.2-20]
		ResponseTopic:             p.ResponseTopic, // [MQTT-3.3.2-15]
		SessionExpiryInterval:     p.SessionExpiryInterval,
		SessionExpiryIntervalFlag: p.SessionExpiryIntervalFlag,
		AssignedClientID:          p.AssignedClientID,
		ServerKeepAlive:           p.ServerKeepAlive,
		ServerKeepAliveFlag:       p.ServerKeepAliveFlag,
		AuthenticationMethod:      p.AuthenticationMethod,
		RequestProblemInfo:        p.RequestProblemInfo,
		RequestProblemInfoFlag:    p.RequestProblemInfoFlag,
		WillDelayInterval:         p.WillDelayInterval,
		RequestResponseInfo:       p.RequestResponseInfo,
		ResponseInfo:              p.ResponseInfo,
		ServerReference:           p.ServerReference,
		ReasonString:              p.ReasonString,
		ReceiveMaximum:            p.ReceiveMaximum,
		TopicAliasMaximum:         p.TopicAliasMaximum,
		TopicAlias:                0, // NB; do not copy topic alias [MQTT-3.3.2-7] + we do not send to clients (currently) [MQTT-3.1.2-26] [MQTT-3.1.2-27]
		MaximumQos:                p.MaximumQos,
		MaximumQosFlag:            p.MaximumQosFlag,
		RetainAvailable:           p.RetainAvailable,
		RetainAvailableFlag:       p.RetainAvailableFlag,
		MaximumPacketSize:         p.MaximumPacketSize,
		WildcardSubAvailable:      p.WildcardSubAvailable,
		WildcardSubAvailableFlag:  p.WildcardSubAvailableFlag,
		SubIDAvailable:            p.SubIDAvailable,
		SubIDAvailableFlag:        p.SubIDAvailableFlag,
		SharedSubAvailable:        p.SharedSubAvailable,
		SharedSubAvailableFlag:    p.SharedSubAvailableFlag,
	}

	if allowTransfer {
		pr.TopicAlias = p.TopicAlias
		pr.TopicAliasFlag = p.TopicAliasFlag
	}

	if len(p.CorrelationData) > 0 {
		pr.CorrelationData = append([]byte{}, p.CorrelationData...) // [MQTT-3.3.2-16]
	}

	if len(p.SubscriptionIdentifier) > 0 {
		pr.SubscriptionIdentifier = append([]int{}, p.SubscriptionIdentifier...)
	}

	if len(p.AuthenticationData) > 0 {
		pr.AuthenticationData = append([]byte{}, p.AuthenticationData...)
	}

	if len(p.User) > 0 {
		pr.User = []UserProperty{}
		for _, v := range p.User {
			pr.User = append(pr.User, UserProperty{ // [MQTT-3.3.2-17]
				Key: v.Key,
				Val: v.Val,
			})
		}
	}

	return pr
}

// canEncode returns true if the property type is valid for the packet type.
func (p *Properties) canEncode(pkt byte, k byte) bool {
	return validPacketProperties[k][pkt] == 1
}

// Encode encodes properties into a bytes buffer.
func (p *Properties) Encode(pkt byte, mods Mods, b *bytes.Buffer, n int) {
	if p == nil {
		return
	}

	var buf bytes.Buffer
	if p.canEncode(pkt, PropPayloadFormat) && p.PayloadFormatFlag {
		buf.WriteByte(PropPayloadFormat)
		buf.WriteByte(p.PayloadFormat)
	}

	if p.canEncode(pkt, PropMessageExpiryInterval) && p.MessageExpiryInterval > 0 {
		buf.WriteByte(PropMessageExpiryInterval)
		buf.Write(encodeUint32(p.MessageExpiryInterval))
	}

	if p.canEncode(pkt, PropContentType) && p.ContentType != "" {
		buf.WriteByte(PropContentType)
		buf.Write(encodeString(p.ContentType)) // [MQTT-3.3.2-19]
	}

	if mods.AllowResponseInfo && p.canEncode(pkt, PropResponseTopic) && //  [MQTT-3.3.2-14]
		p.ResponseTopic != "" && !strings.ContainsAny(p.ResponseTopic, "+#") { // [MQTT-3.1.2-28]
		buf.WriteByte(PropResponseTopic)
		buf.Write(encodeString(p.ResponseTopic)) // [MQTT-3.3.2-13]
	}

	if mods.AllowResponseInfo && p.canEncode(pkt, PropCorrelationData) && len(p.CorrelationData) > 0 { // [MQTT-3.1.2-28]
		buf.WriteByte(PropCorrelationData)
		buf.Write(encodeBytes(p.CorrelationData))
	}

	if p.canEncode(pkt, PropSubscriptionIdentifier) && len(p.SubscriptionIdentifier) > 0 {
		for _, v := range p.SubscriptionIdentifier {
			if v > 0 {
				buf.WriteByte(PropSubscriptionIdentifier)
				encodeLength(&buf, int64(v))
			}
		}
	}

	if p.canEncode(pkt, PropSessionExpiryInterval) && p.SessionExpiryIntervalFlag { // [MQTT-3.14.2-2]
		buf.WriteByte(PropSessionExpiryInterval)
		buf.Write(encodeUint32(p.SessionExpiryInterval))
	}

	if p.canEncode(pkt, PropAssignedClientID) && p.AssignedClientID != "" {
		buf.WriteByte(PropAssignedClientID)
		buf.Write(encodeString(p.AssignedClientID))
	}

	if p.canEncode(pkt, PropServerKeepAlive) && p.ServerKeepAliveFlag {
		buf.WriteByte(PropServerKeepAlive)
		buf.Write(encodeUint16(p.ServerKeepAlive))
	}

	if p.canEncode(pkt, PropAuthenticationMethod) && p.AuthenticationMethod != "" {
		buf.WriteByte(PropAuthenticationMethod)
		buf.Write(encodeString(p.AuthenticationMethod))
	}

	if p.canEncode(pkt, PropAuthenticationData) && len(p.AuthenticationData) > 0 {
		buf.WriteByte(PropAuthenticationData)
		buf.Write(encodeBytes(p.AuthenticationData))
	}

	if p.canEncode(pkt, PropRequestProblemInfo) && p.RequestProblemInfoFlag {
		buf.WriteByte(PropRequestProblemInfo)
		buf.WriteByte(p.RequestProblemInfo)
	}

	if p.canEncode(pkt, PropWillDelayInterval) && p.WillDelayInterval > 0 {
		buf.WriteByte(PropWillDelayInterval)
		buf.Write(encodeUint32(p.WillDelayInterval))
	}

	if p.canEncode(pkt, PropRequestResponseInfo) && p.RequestResponseInfo > 0 {
		buf.WriteByte(PropRequestResponseInfo)
		buf.WriteByte(p.RequestResponseInfo)
	}

	if mods.AllowResponseInfo && p.canEncode(pkt, PropResponseInfo) && len(p.ResponseInfo) > 0 { // [MQTT-3.1.2-28]
		buf.WriteByte(PropResponseInfo)
		buf.Write(encodeString(p.ResponseInfo))
	}

	if p.canEncode(pkt, PropServerReference) && len(p.ServerReference) > 0 {
		buf.WriteByte(PropServerReference)
		buf.Write(encodeString(p.ServerReference))
	}

	// [MQTT-3.2.2-19] [MQTT-3.14.2-3] [MQTT-3.4.2-2] [MQTT-3.5.2-2]
	// [MQTT-3.6.2-2] [MQTT-3.9.2-1] [MQTT-3.11.2-1] [MQTT-3.15.2-2]
	if !mods.DisallowProblemInfo && p.canEncode(pkt, PropReasonString) && p.ReasonString != "" {
		b := encodeString(p.ReasonString)
		if mods.MaxSize == 0 || uint32(n+len(b)+1) < mods.MaxSize {
			buf.WriteByte(PropReasonString)
			buf.Write(b)
		}
	}

	if p.canEncode(pkt, PropReceiveMaximum) && p.ReceiveMaximum > 0 {
		buf.WriteByte(PropReceiveMaximum)
		buf.Write(encodeUint16(p.ReceiveMaximum))
	}

	if p.canEncode(pkt, PropTopicAliasMaximum) && p.TopicAliasMaximum > 0 {
		buf.WriteByte(PropTopicAliasMaximum)
		buf.Write(encodeUint16(p.TopicAliasMaximum))
	}

	if p.canEncode(pkt, PropTopicAlias) && p.TopicAliasFlag && p.TopicAlias > 0 { // [MQTT-3.3.2-8]
		buf.WriteByte(PropTopicAlias)
		buf.Write(encodeUint16(p.TopicAlias))
	}

	if p.canEncode(pkt, PropMaximumQos) && p.MaximumQosFlag && p.MaximumQos < 2 {
		buf.WriteByte(PropMaximumQos)
		buf.WriteByte(p.MaximumQos)
	}

	if p.canEncode(pkt, PropRetainAvailable) && p.RetainAvailableFlag {
		buf.WriteByte(PropRetainAvailable)
		buf.WriteByte(p.RetainAvailable)
	}

	if !mods.DisallowProblemInfo && p.canEncode(pkt, PropUser) {
		pb := bytes.NewBuffer([]byte{})
		for _, v := range p.User {
			pb.WriteByte(PropUser)
			pb.Write(encodeString(v.Key))
			pb.Write(encodeString(v.Val))
		}
		// [MQTT-3.2.2-20] [MQTT-3.14.2-4] [MQTT-3.4.2-3] [MQTT-3.5.2-3]
		// [MQTT-3.6.2-3] [MQTT-3.9.2-2] [MQTT-3.11.2-2] [MQTT-3.15.2-3]
		if mods.MaxSize == 0 || uint32(n+pb.Len()+1) < mods.MaxSize {
			buf.Write(pb.Bytes())
		}
	}

	if p.canEncode(pkt, PropMaximumPacketSize) && p.MaximumPacketSize > 0 {
		buf.WriteByte(PropMaximumPacketSize)
		buf.Write(encodeUint32(p.MaximumPacketSize))
	}

	if p.canEncode(pkt, PropWildcardSubAvailable) && p.WildcardSubAvailableFlag {
		buf.WriteByte(PropWildcardSubAvailable)
		buf.WriteByte(p.WildcardSubAvailable)
	}

	if p.canEncode(pkt, PropSubIDAvailable) && p.SubIDAvailableFlag {
		buf.WriteByte(PropSubIDAvailable)
		buf.WriteByte(p.SubIDAvailable)
	}

	if p.canEncode(pkt, PropSharedSubAvailable) && p.SharedSubAvailableFlag {
		buf.WriteByte(PropSharedSubAvailable)
		buf.WriteByte(p.SharedSubAvailable)
	}

	encodeLength(b, int64(buf.Len()))
	_, _ = buf.WriteTo(b) // [MQTT-3.1.3-10]
}

// Decode decodes property bytes into a properties struct.
func (p *Properties) Decode(pkt byte, b *bytes.Buffer) (n int, err error) {
	if p == nil {
		return 0, nil
	}

	var bu int
	n, bu, err = DecodeLength(b)
	if err != nil {
		return n + bu, err
	}

	if n == 0 {
		return n + bu, nil
	}

	bt := b.Bytes()
	var k byte
	for offset := 0; offset < n; {
		k, offset, err = decodeByte(bt, offset)
		if err != nil {
			return n + bu, err
		}

		if _, ok := validPacketProperties[k][pkt]; !ok {
			return n + bu, fmt.Errorf("property type %v not valid for packet type %v: %w", k, pkt, ErrProtocolViolationUnsupportedProperty)
		}

		switch k {
		case PropPayloadFormat:
			p.PayloadFormat, offset, err = decodeByte(bt, offset)
			p.PayloadFormatFlag = true
		case PropMessageExpiryInterval:
			p.MessageExpiryInterval, offset, err = decodeUint32(bt, offset)
		case PropContentType:
			p.ContentType, offset, err = decodeString(bt, offset)
		case PropResponseTopic:
			p.ResponseTopic, offset, err = decodeString(bt, offset)
		case PropCorrelationData:
			p.CorrelationData, offset, err = decodeBytes(bt, offset)
		case PropSubscriptionIdentifier:
			if p.SubscriptionIdentifier == nil {
				p.SubscriptionIdentifier = []int{}
			}

			n, bu, err := DecodeLength(bytes.NewBuffer(bt[offset:]))
			if err != nil {
				return n + bu, err
			}
			p.SubscriptionIdentifier = append(p.SubscriptionIdentifier, n)
			offset += bu
		case PropSessionExpiryInterval:
			p.SessionExpiryInterval, offset, err = decodeUint32(bt, offset)
			p.SessionExpiryIntervalFlag = true
		case PropAssignedClientID:
			p.AssignedClientID, offset, err = decodeString(bt, offset)
		case PropServerKeepAlive:
			p.ServerKeepAlive, offset, err = decodeUint16(bt, offset)
			p.ServerKeepAliveFlag = true
		case PropAuthenticationMethod:
			p.AuthenticationMethod, offset, err = decodeString(bt, offset)
		case PropAuthenticationData:
			p.AuthenticationData, offset, err = decodeBytes(bt, offset)
		case PropRequestProblemInfo:
			p.RequestProblemInfo, offset, err = decodeByte(bt, offset)
			p.RequestProblemInfoFlag = true
		case PropWillDelayInterval:
			p.WillDelayInterval, offset, err = decodeUint32(bt, offset)
		case PropRequestResponseInfo:
			p.RequestResponseInfo, offset, err = decodeByte(bt, offset)
		case PropResponseInfo:
			p.ResponseInfo, offset, err = decodeString(bt, offset)
		case PropServerReference:
			p.ServerReference, offset, err = decodeString(bt, offset)
		case PropReasonString:
			p.ReasonString, offset, err = decodeString(bt, offset)
		case PropReceiveMaximum:
			p.ReceiveMaximum, offset, err = decodeUint16(bt, offset)
		case PropTopicAliasMaximum:
			p.TopicAliasMaximum, offset, err = decodeUint16(bt, offset)
		case PropTopicAlias:
			p.TopicAlias, offset, err = decodeUint16(bt, offset)
			p.TopicAliasFlag = true
		case PropMaximumQos:
			p.MaximumQos, offset, err = decodeByte(bt, offset)
			p.MaximumQosFlag = true
		case PropRetainAvailable:
			p.RetainAvailable, offset, err = decodeByte(bt, offset)
			p.RetainAvailableFlag = true
		case PropUser:
			var k, v string
			k, offset, err = decodeString(bt, offset)
			if err != nil {
				return n + bu, err
			}
			v, offset, err = decodeString(bt, offset)
			p.User = append(p.User, UserProperty{Key: k, Val: v})
		case PropMaximumPacketSize:
			p.MaximumPacketSize, offset, err = decodeUint32(bt, offset)
		case PropWildcardSubAvailable:
			p.WildcardSubAvailable, offset, err = decodeByte(bt, offset)
			p.WildcardSubAvailableFlag = true
		case PropSubIDAvailable:
			p.SubIDAvailable, offset, err = decodeByte(bt, offset)
			p.SubIDAvailableFlag = true
		case PropSharedSubAvailable:
			p.SharedSubAvailable, offset, err = decodeByte(bt, offset)
			p.SharedSubAvailableFlag = true
		}

		if err != nil {
			return n + bu, err
		}
	}

	return n + bu, nil
}
