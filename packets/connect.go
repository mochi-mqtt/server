package packets

import (
	"bytes"
	"errors"
)

// ConnectPacket contains the values of an MQTT CONNECT packet.
type ConnectPacket struct {
	FixedHeader

	ProtocolName     string
	ProtocolVersion  byte
	CleanSession     bool
	WillFlag         bool
	WillQos          byte
	WillRetain       bool
	UsernameFlag     bool
	PasswordFlag     bool
	ReservedBit      byte
	Keepalive        uint16
	ClientIdentifier string
	WillTopic        string
	WillMessage      []byte // WillMessage is a payload, so store as byte array.
	Username         string
	Password         string
}

// Encode encodes and writes the packet data values to the buffer.
func (pk *ConnectPacket) Encode(buf *bytes.Buffer) error {

	protoName := encodeString(pk.ProtocolName)
	protoVersion := pk.ProtocolVersion
	flag := encodeBool(pk.CleanSession)<<1 | encodeBool(pk.WillFlag)<<2 | pk.WillQos<<3 | encodeBool(pk.WillRetain)<<5 | encodeBool(pk.PasswordFlag)<<6 | encodeBool(pk.UsernameFlag)<<7
	keepalive := encodeUint16(pk.Keepalive)
	clientID := encodeString(pk.ClientIdentifier)

	var willTopic, willFlag, usernameFlag, passwordFlag []byte

	// If will flag is set, add topic and message.
	if pk.WillFlag {
		willTopic = encodeString(pk.WillTopic)
		willFlag = encodeBytes(pk.WillMessage)
	}

	// If username flag is set, add username.
	if pk.UsernameFlag {
		usernameFlag = encodeString(pk.Username)
	}

	// If password flag is set, add password.
	if pk.PasswordFlag {
		passwordFlag = encodeString(pk.Password)
	}

	// Get a length for the connect header. This is not super pretty, but it works.
	pk.FixedHeader.Remaining =
		len(protoName) + 1 + 1 + len(keepalive) + len(clientID) +
			len(willTopic) + len(willFlag) +
			len(usernameFlag) + len(passwordFlag)

	pk.FixedHeader.encode(buf)

	// Eschew magic for readability.
	buf.Write(protoName)
	buf.WriteByte(protoVersion)
	buf.WriteByte(flag)
	buf.Write(keepalive)
	buf.Write(clientID)
	buf.Write(willTopic)
	buf.Write(willFlag)
	buf.Write(usernameFlag)
	buf.Write(passwordFlag)

	/*
		var body bytes.Buffer

		// Write flags to packet body.
		body.Write(encodeString(pk.ProtocolName))
		body.WriteByte(pk.ProtocolVersion)
		body.WriteByte(encodeBool(pk.CleanSession)<<1 | encodeBool(pk.WillFlag)<<2 | pk.WillQos<<3 | encodeBool(pk.WillRetain)<<5 | encodeBool(pk.PasswordFlag)<<6 | encodeBool(pk.UsernameFlag)<<7)
		body.Write(encodeUint16(pk.Keepalive))
		body.Write(encodeString(pk.ClientIdentifier))

		// If will flag is set, add topic and message.
		if pk.WillFlag {
			body.Write(encodeString(pk.WillTopic))
			body.Write(encodeBytes(pk.WillMessage))
		}

		// If username flag is set, add username.
		if pk.UsernameFlag {
			body.Write(encodeString(pk.Username))
		}

		// If password flag is set, add password.
		if pk.PasswordFlag {
			body.Write(encodeString(pk.Password))
		}

		pk.FixedHeader.Remaining = body.Len()
		pk.FixedHeader.encode(buf)
		buf.Write(body.Bytes())
	*/
	return nil
}

// Decode extracts the data values from the packet.
func (pk *ConnectPacket) Decode(buf []byte) error {

	var offset int
	var err error

	// Unpack protocol name and version.
	pk.ProtocolName, offset, err = decodeString(buf, 0)
	if err != nil {
		return errors.New(ErrMalformedProtocolName)
	}

	pk.ProtocolVersion, offset, err = decodeByte(buf, offset)
	if err != nil {
		return errors.New(ErrMalformedProtocolVersion)
	}
	// Unpack flags byte.
	flags, offset, err := decodeByte(buf, offset)
	if err != nil {
		return errors.New(ErrMalformedFlags)
	}
	pk.ReservedBit = 1 & flags
	pk.CleanSession = 1&(flags>>1) > 0
	pk.WillFlag = 1&(flags>>2) > 0
	pk.WillQos = 3 & (flags >> 3) // this one is not a bool
	pk.WillRetain = 1&(flags>>5) > 0
	pk.PasswordFlag = 1&(flags>>6) > 0
	pk.UsernameFlag = 1&(flags>>7) > 0

	// Get keepalive interval.
	pk.Keepalive, offset, err = decodeUint16(buf, offset)
	if err != nil {
		return errors.New(ErrMalformedKeepalive)
	}

	// Get client ID.
	pk.ClientIdentifier, offset, err = decodeString(buf, offset)
	if err != nil {
		return errors.New(ErrMalformedClientID)
	}

	// Get Last Will and Testament topic and message if applicable.
	if pk.WillFlag {

		pk.WillTopic, offset, err = decodeString(buf, offset)
		if err != nil {
			return errors.New(ErrMalformedWillTopic)
		}

		pk.WillMessage, offset, err = decodeBytes(buf, offset)
		if err != nil {
			return errors.New(ErrMalformedWillMessage)
		}
	}

	// Get username and password if applicable.
	if pk.UsernameFlag {
		pk.Username, offset, err = decodeString(buf, offset)
		if err != nil {
			return errors.New(ErrMalformedUsername)
		}
	}

	if pk.PasswordFlag {
		pk.Password, offset, err = decodeString(buf, offset)
		if err != nil {
			return errors.New(ErrMalformedPassword)
		}
	}

	return nil

}

// Validate ensures the packet is compliant.
func (pk *ConnectPacket) Validate() (b byte, err error) {

	// End if protocol name is bad.
	if pk.ProtocolName != "MQIsdp" && pk.ProtocolName != "MQTT" {
		return ErrConnectProtocolViolation, errors.New(ErrProtocolViolation)
	}

	// End if protocol version is bad.
	if (pk.ProtocolName == "MQIsdp" && pk.ProtocolVersion != 3) ||
		(pk.ProtocolName == "MQTT" && pk.ProtocolVersion != 4) {
		return ErrConnectBadProtocolVersion, errors.New(ErrProtocolViolation)
	}

	// End if reserved bit is not 0.
	if pk.ReservedBit != 0 {
		return ErrConnectProtocolViolation, errors.New(ErrProtocolViolation)
	}

	// End if ClientID is too long.
	if len(pk.ClientIdentifier) > 65535 {
		return ErrConnectProtocolViolation, errors.New(ErrProtocolViolation)
	}

	// End if password flag is set without a username.
	if pk.PasswordFlag && !pk.UsernameFlag {
		return ErrConnectProtocolViolation, errors.New(ErrProtocolViolation)
	}

	// End if Username or Password is too long.
	if len(pk.Username) > 65535 || len(pk.Password) > 65535 {
		return ErrConnectProtocolViolation, errors.New(ErrProtocolViolation)
	}

	// End if client id isn't set and clean session is false.
	if !pk.CleanSession && len(pk.ClientIdentifier) == 0 {
		return ErrConnectBadClientID, errors.New(ErrProtocolViolation)
	}

	return Accepted, nil
}
