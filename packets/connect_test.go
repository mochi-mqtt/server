package packets

import (
	"bytes"
	"testing"

	"github.com/jinzhu/copier"
	"github.com/stretchr/testify/require"
)

func TestConnectEncode(t *testing.T) {
	require.Contains(t, expectedPackets, Connect)
	for i, wanted := range expectedPackets[Connect] {
		if !encodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(1), Connect, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)
		pk := new(ConnectPacket)
		copier.Copy(pk, wanted.packet.(*ConnectPacket))

		require.Equal(t, Connect, pk.Type, "Mismatched Packet Type [i:%d] %s", i, wanted.desc)
		require.Equal(t, Connect, pk.FixedHeader.Type, "Mismatched FixedHeader Type [i:%d] %s", i, wanted.desc)

		buf := new(bytes.Buffer)
		pk.Encode(buf)
		encoded := buf.Bytes()

		require.Equal(t, len(wanted.rawBytes), len(encoded), "Mismatched packet length [i:%d] %s", i, wanted.desc)
		require.Equal(t, byte(Connect<<4), encoded[0], "Mismatched fixed header packets [i:%d] %s", i, wanted.desc)
		require.EqualValues(t, wanted.rawBytes, encoded, "Mismatched byte values [i:%d] %s", i, wanted.desc)

		ok, _ := pk.Validate()
		require.Equal(t, byte(Accepted), ok, "Connect packet didn't validate - %v", ok)

		require.Equal(t, wanted.packet.(*ConnectPacket).FixedHeader.Type, pk.FixedHeader.Type, "Mismatched packet fixed header type [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*ConnectPacket).FixedHeader.Dup, pk.FixedHeader.Dup, "Mismatched packet fixed header dup [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*ConnectPacket).FixedHeader.Qos, pk.FixedHeader.Qos, "Mismatched packet fixed header qos [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*ConnectPacket).FixedHeader.Retain, pk.FixedHeader.Retain, "Mismatched packet fixed header retain [i:%d] %s", i, wanted.desc)

		require.Equal(t, wanted.packet.(*ConnectPacket).ProtocolVersion, pk.ProtocolVersion, "Mismatched packet protocol version [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*ConnectPacket).ProtocolName, pk.ProtocolName, "Mismatched packet protocol name [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*ConnectPacket).CleanSession, pk.CleanSession, "Mismatched packet cleansession [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*ConnectPacket).ClientIdentifier, pk.ClientIdentifier, "Mismatched packet client id [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*ConnectPacket).Keepalive, pk.Keepalive, "Mismatched keepalive value [i:%d] %s", i, wanted.desc)

		require.Equal(t, wanted.packet.(*ConnectPacket).UsernameFlag, pk.UsernameFlag, "Mismatched packet username flag [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*ConnectPacket).Username, pk.Username, "Mismatched packet username [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*ConnectPacket).PasswordFlag, pk.PasswordFlag, "Mismatched packet password flag [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*ConnectPacket).Password, pk.Password, "Mismatched packet password [i:%d] %s", i, wanted.desc)

		require.Equal(t, wanted.packet.(*ConnectPacket).WillFlag, pk.WillFlag, "Mismatched packet will flag [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*ConnectPacket).WillTopic, pk.WillTopic, "Mismatched packet will topic [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*ConnectPacket).WillMessage, pk.WillMessage, "Mismatched packet will message [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*ConnectPacket).WillQos, pk.WillQos, "Mismatched packet will qos [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*ConnectPacket).WillRetain, pk.WillRetain, "Mismatched packet will retain [i:%d] %s", i, wanted.desc)
	}
}

func BenchmarkConnectEncode(b *testing.B) {
	pk := new(ConnectPacket)
	copier.Copy(pk, expectedPackets[Connect][0].packet.(*ConnectPacket))

	buf := new(bytes.Buffer)
	for n := 0; n < b.N; n++ {
		pk.Encode(buf)
	}
}

func TestConnectDecode(t *testing.T) {
	require.Contains(t, expectedPackets, Connect)
	for i, wanted := range expectedPackets[Connect] {

		if !decodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(1), Connect, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)
		require.Equal(t, true, (len(wanted.rawBytes) > 2), "Insufficent bytes in packet [i:%d] %s", i, wanted.desc)

		pk := &ConnectPacket{FixedHeader: FixedHeader{Type: Connect}}
		err := pk.Decode(wanted.rawBytes[2:]) // Unpack skips fixedheader.
		if wanted.failFirst != nil {
			require.Error(t, err, "Expected error unpacking buffer [i:%d] %s", i, wanted.desc)
			require.Equal(t, wanted.failFirst, err, "Expected fail state; %v [i:%d] %s", err.Error(), i, wanted.desc)
			continue
		}

		require.NoError(t, err, "Error unpacking buffer [i:%d] %s", i, wanted.desc)

		require.Equal(t, wanted.packet.(*ConnectPacket).FixedHeader.Type, pk.FixedHeader.Type, "Mismatched packet fixed header type [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*ConnectPacket).FixedHeader.Dup, pk.FixedHeader.Dup, "Mismatched packet fixed header dup [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*ConnectPacket).FixedHeader.Qos, pk.FixedHeader.Qos, "Mismatched packet fixed header qos [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*ConnectPacket).FixedHeader.Retain, pk.FixedHeader.Retain, "Mismatched packet fixed header retain [i:%d] %s", i, wanted.desc)

		require.Equal(t, wanted.packet.(*ConnectPacket).ProtocolVersion, pk.ProtocolVersion, "Mismatched packet protocol version [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*ConnectPacket).ProtocolName, pk.ProtocolName, "Mismatched packet protocol name [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*ConnectPacket).CleanSession, pk.CleanSession, "Mismatched packet cleansession [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*ConnectPacket).ClientIdentifier, pk.ClientIdentifier, "Mismatched packet client id [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*ConnectPacket).Keepalive, pk.Keepalive, "Mismatched keepalive value [i:%d] %s", i, wanted.desc)

		require.Equal(t, wanted.packet.(*ConnectPacket).UsernameFlag, pk.UsernameFlag, "Mismatched packet username flag [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*ConnectPacket).Username, pk.Username, "Mismatched packet username [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*ConnectPacket).PasswordFlag, pk.PasswordFlag, "Mismatched packet password flag [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*ConnectPacket).Password, pk.Password, "Mismatched packet password [i:%d] %s", i, wanted.desc)

		require.Equal(t, wanted.packet.(*ConnectPacket).WillFlag, pk.WillFlag, "Mismatched packet will flag [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*ConnectPacket).WillTopic, pk.WillTopic, "Mismatched packet will topic [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*ConnectPacket).WillMessage, pk.WillMessage, "Mismatched packet will message [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*ConnectPacket).WillQos, pk.WillQos, "Mismatched packet will qos [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*ConnectPacket).WillRetain, pk.WillRetain, "Mismatched packet will retain [i:%d] %s", i, wanted.desc)
	}
}

func BenchmarkConnectDecode(b *testing.B) {
	pk := &ConnectPacket{FixedHeader: FixedHeader{Type: Connect}}
	pk.FixedHeader.Decode(expectedPackets[Connect][0].rawBytes[0])

	for n := 0; n < b.N; n++ {
		pk.Decode(expectedPackets[Connect][0].rawBytes[2:])
	}
}

func TestConnectValidate(t *testing.T) {
	require.Contains(t, expectedPackets, Connect)
	for i, wanted := range expectedPackets[Connect] {
		if wanted.group == "validate" {
			pk := wanted.packet.(*ConnectPacket)
			ok, _ := pk.Validate()
			require.Equal(t, wanted.code, ok, "Connect packet didn't validate [i:%d] %s", i, wanted.desc)
		}
	}
}

func BenchmarkConnectValidate(b *testing.B) {
	pk := &ConnectPacket{FixedHeader: FixedHeader{Type: Connect}}
	pk.FixedHeader.Decode(expectedPackets[Connect][0].rawBytes[0])

	for n := 0; n < b.N; n++ {
		pk.Validate()
	}
}
