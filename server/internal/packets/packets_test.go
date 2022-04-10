package packets

import (
	"bytes"
	"errors"
	"fmt"
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
		pk := new(Packet)
		copier.Copy(pk, wanted.packet)

		require.Equal(t, Connect, pk.FixedHeader.Type, "Mismatched FixedHeader Type [i:%d] %s", i, wanted.desc)

		buf := new(bytes.Buffer)
		pk.ConnectEncode(buf)
		encoded := buf.Bytes()

		require.Equal(t, len(wanted.rawBytes), len(encoded), "Mismatched packet length [i:%d] %s", i, wanted.desc)
		require.Equal(t, byte(Connect<<4), encoded[0], "Mismatched fixed header packets [i:%d] %s", i, wanted.desc)
		require.EqualValues(t, wanted.rawBytes, encoded, "Mismatched byte values [i:%d] %s", i, wanted.desc)

		ok, _ := pk.ConnectValidate()
		require.Equal(t, byte(Accepted), ok, "Connect packet didn't validate - %v", ok)

		require.Equal(t, wanted.packet.FixedHeader.Type, pk.FixedHeader.Type, "Mismatched packet fixed header type [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.FixedHeader.Dup, pk.FixedHeader.Dup, "Mismatched packet fixed header dup [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.FixedHeader.Qos, pk.FixedHeader.Qos, "Mismatched packet fixed header qos [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.FixedHeader.Retain, pk.FixedHeader.Retain, "Mismatched packet fixed header retain [i:%d] %s", i, wanted.desc)

		require.Equal(t, wanted.packet.ProtocolVersion, pk.ProtocolVersion, "Mismatched packet protocol version [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.ProtocolName, pk.ProtocolName, "Mismatched packet protocol name [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.CleanSession, pk.CleanSession, "Mismatched packet cleansession [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.ClientIdentifier, pk.ClientIdentifier, "Mismatched packet client id [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.Keepalive, pk.Keepalive, "Mismatched keepalive value [i:%d] %s", i, wanted.desc)

		require.Equal(t, wanted.packet.UsernameFlag, pk.UsernameFlag, "Mismatched packet username flag [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.Username, pk.Username, "Mismatched packet username [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.PasswordFlag, pk.PasswordFlag, "Mismatched packet password flag [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.Password, pk.Password, "Mismatched packet password [i:%d] %s", i, wanted.desc)

		require.Equal(t, wanted.packet.WillFlag, pk.WillFlag, "Mismatched packet will flag [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.WillTopic, pk.WillTopic, "Mismatched packet will topic [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.WillMessage, pk.WillMessage, "Mismatched packet will message [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.WillQos, pk.WillQos, "Mismatched packet will qos [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.WillRetain, pk.WillRetain, "Mismatched packet will retain [i:%d] %s", i, wanted.desc)
	}
}

func BenchmarkConnectEncode(b *testing.B) {
	pk := new(Packet)
	copier.Copy(pk, expectedPackets[Connect][0].packet)

	buf := new(bytes.Buffer)
	for n := 0; n < b.N; n++ {
		pk.ConnectEncode(buf)
	}
}

func TestConnectDecode(t *testing.T) {
	require.Contains(t, expectedPackets, Connect)
	for i, wanted := range expectedPackets[Connect] {
		if !decodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(1), Connect, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)
		require.Equal(t, true, (len(wanted.rawBytes) > 2), "Insufficient bytes in packet [i:%d] %s", i, wanted.desc)

		pk := &Packet{FixedHeader: FixedHeader{Type: Connect}}
		err := pk.ConnectDecode(wanted.rawBytes[2:]) // Unpack skips fixedheader.
		if wanted.failFirst != nil {
			require.Error(t, err, "Expected error unpacking buffer [i:%d] %s", i, wanted.desc)
			require.True(t, errors.Is(err, wanted.failFirst), "Expected fail state; %v [i:%d] %s", err.Error(), i, wanted.desc)
			continue
		}

		require.NoError(t, err, "Error unpacking buffer [i:%d] %s", i, wanted.desc)

		require.Equal(t, wanted.packet.FixedHeader.Type, pk.FixedHeader.Type, "Mismatched packet fixed header type [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.FixedHeader.Dup, pk.FixedHeader.Dup, "Mismatched packet fixed header dup [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.FixedHeader.Qos, pk.FixedHeader.Qos, "Mismatched packet fixed header qos [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.FixedHeader.Retain, pk.FixedHeader.Retain, "Mismatched packet fixed header retain [i:%d] %s", i, wanted.desc)

		require.Equal(t, wanted.packet.ProtocolVersion, pk.ProtocolVersion, "Mismatched packet protocol version [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.ProtocolName, pk.ProtocolName, "Mismatched packet protocol name [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.CleanSession, pk.CleanSession, "Mismatched packet cleansession [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.ClientIdentifier, pk.ClientIdentifier, "Mismatched packet client id [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.Keepalive, pk.Keepalive, "Mismatched keepalive value [i:%d] %s", i, wanted.desc)

		require.Equal(t, wanted.packet.UsernameFlag, pk.UsernameFlag, "Mismatched packet username flag [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.Username, pk.Username, "Mismatched packet username [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.PasswordFlag, pk.PasswordFlag, "Mismatched packet password flag [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.Password, pk.Password, "Mismatched packet password [i:%d] %s", i, wanted.desc)

		require.Equal(t, wanted.packet.WillFlag, pk.WillFlag, "Mismatched packet will flag [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.WillTopic, pk.WillTopic, "Mismatched packet will topic [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.WillMessage, pk.WillMessage, "Mismatched packet will message [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.WillQos, pk.WillQos, "Mismatched packet will qos [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.WillRetain, pk.WillRetain, "Mismatched packet will retain [i:%d] %s", i, wanted.desc)
	}
}

func BenchmarkConnectDecode(b *testing.B) {
	pk := &Packet{FixedHeader: FixedHeader{Type: Connect}}
	pk.FixedHeader.Decode(expectedPackets[Connect][0].rawBytes[0])

	for n := 0; n < b.N; n++ {
		pk.ConnectDecode(expectedPackets[Connect][0].rawBytes[2:])
	}
}

func TestConnectValidate(t *testing.T) {
	require.Contains(t, expectedPackets, Connect)
	for i, wanted := range expectedPackets[Connect] {
		if wanted.group == "validate" {
			pk := wanted.packet
			ok, _ := pk.ConnectValidate()
			require.Equal(t, wanted.code, ok, "Connect packet didn't validate [i:%d] %s", i, wanted.desc)
		}
	}
}

func TestConnackEncode(t *testing.T) {
	require.Contains(t, expectedPackets, Connack)
	for i, wanted := range expectedPackets[Connack] {
		if !encodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(2), Connack, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)
		pk := new(Packet)
		copier.Copy(pk, wanted.packet)

		require.Equal(t, Connack, pk.FixedHeader.Type, "Mismatched FixedHeader Type [i:%d] %s", i, wanted.desc)

		buf := new(bytes.Buffer)
		err := pk.ConnackEncode(buf)
		require.NoError(t, err, "Expected no error writing buffer [i:%d] %s", i, wanted.desc)
		encoded := buf.Bytes()

		require.Equal(t, len(wanted.rawBytes), len(encoded), "Mismatched packet length [i:%d] %s", i, wanted.desc)
		require.Equal(t, byte(Connack<<4), encoded[0], "Mismatched fixed header packets [i:%d] %s", i, wanted.desc)
		require.EqualValues(t, wanted.rawBytes, encoded, "Mismatched byte values [i:%d] %s", i, wanted.desc)

		require.Equal(t, wanted.packet.ReturnCode, pk.ReturnCode, "Mismatched return code [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.SessionPresent, pk.SessionPresent, "Mismatched session present bool [i:%d] %s", i, wanted.desc)
	}
}

func BenchmarkConnackEncode(b *testing.B) {
	pk := new(Packet)
	copier.Copy(pk, expectedPackets[Connack][0].packet)

	buf := new(bytes.Buffer)
	for n := 0; n < b.N; n++ {
		pk.ConnackEncode(buf)
	}
}

func TestConnackDecode(t *testing.T) {
	require.Contains(t, expectedPackets, Connack)
	for i, wanted := range expectedPackets[Connack] {
		if !decodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(2), Connack, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)

		pk := &Packet{FixedHeader: FixedHeader{Type: Connack}}
		err := pk.ConnackDecode(wanted.rawBytes[2:]) // Unpack skips fixedheader.
		if wanted.failFirst != nil {
			require.Error(t, err, "Expected error unpacking buffer [i:%d] %s", i, wanted.desc)
			require.True(t, errors.Is(err, wanted.failFirst), "Expected fail state; %v [i:%d] %s", err.Error(), i, wanted.desc)
			continue
		}

		require.NoError(t, err, "Error unpacking buffer [i:%d] %s", i, wanted.desc)

		require.Equal(t, wanted.packet.ReturnCode, pk.ReturnCode, "Mismatched return code [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.SessionPresent, pk.SessionPresent, "Mismatched session present bool [i:%d] %s", i, wanted.desc)
	}
}

func BenchmarkConnackDecode(b *testing.B) {
	pk := &Packet{FixedHeader: FixedHeader{Type: Connack}}
	pk.FixedHeader.Decode(expectedPackets[Connack][0].rawBytes[0])

	for n := 0; n < b.N; n++ {
		pk.ConnackDecode(expectedPackets[Connack][0].rawBytes[2:])
	}
}

func TestDisconnectEncode(t *testing.T) {
	require.Contains(t, expectedPackets, Disconnect)
	for i, wanted := range expectedPackets[Disconnect] {
		require.Equal(t, uint8(14), Disconnect, "Incorrect Packet Type [i:%d]", i)

		pk := new(Packet)
		copier.Copy(pk, wanted.packet)

		require.Equal(t, Disconnect, pk.FixedHeader.Type, "Mismatched FixedHeader Type [i:%d]", i)

		buf := new(bytes.Buffer)
		err := pk.DisconnectEncode(buf)
		require.NoError(t, err, "Expected no error writing buffer [i:%d] %s", i, wanted.desc)
		encoded := buf.Bytes()

		require.Equal(t, len(wanted.rawBytes), len(encoded), "Mismatched packet length [i:%d]", i)
		require.EqualValues(t, wanted.rawBytes, encoded, "Mismatched byte values [i:%d]", i)
	}
}

func BenchmarkDisconnectEncode(b *testing.B) {
	pk := new(Packet)
	copier.Copy(pk, expectedPackets[Disconnect][0].packet)

	buf := new(bytes.Buffer)
	for n := 0; n < b.N; n++ {
		pk.DisconnectEncode(buf)
	}
}

func TestPingreqEncode(t *testing.T) {
	require.Contains(t, expectedPackets, Pingreq)
	for i, wanted := range expectedPackets[Pingreq] {
		require.Equal(t, uint8(12), Pingreq, "Incorrect Packet Type [i:%d]", i)
		pk := new(Packet)
		copier.Copy(pk, wanted.packet)

		require.Equal(t, Pingreq, pk.FixedHeader.Type, "Mismatched FixedHeader Type [i:%d]", i)

		buf := new(bytes.Buffer)
		err := pk.PingreqEncode(buf)
		require.NoError(t, err, "Expected no error writing buffer [i:%d] %s", i, wanted.desc)
		encoded := buf.Bytes()

		require.Equal(t, len(wanted.rawBytes), len(encoded), "Mismatched packet length [i:%d]", i)
		require.EqualValues(t, wanted.rawBytes, encoded, "Mismatched byte values [i:%d]", i)
	}
}

func BenchmarkPingreqEncode(b *testing.B) {
	pk := new(Packet)
	copier.Copy(pk, expectedPackets[Pingreq][0].packet)

	buf := new(bytes.Buffer)
	for n := 0; n < b.N; n++ {
		pk.PingreqEncode(buf)
	}
}

func TestPingrespEncode(t *testing.T) {
	require.Contains(t, expectedPackets, Pingresp)
	for i, wanted := range expectedPackets[Pingresp] {
		require.Equal(t, uint8(13), Pingresp, "Incorrect Packet Type [i:%d]", i)

		pk := new(Packet)
		copier.Copy(pk, wanted.packet)

		require.Equal(t, Pingresp, pk.FixedHeader.Type, "Mismatched FixedHeader Type [i:%d]", i)

		buf := new(bytes.Buffer)
		err := pk.PingrespEncode(buf)
		require.NoError(t, err, "Expected no error writing buffer [i:%d] %s", i, wanted.desc)
		encoded := buf.Bytes()

		require.Equal(t, len(wanted.rawBytes), len(encoded), "Mismatched packet length [i:%d]", i)
		require.EqualValues(t, wanted.rawBytes, encoded, "Mismatched byte values [i:%d]", i)
	}
}

func BenchmarkPingrespEncode(b *testing.B) {
	pk := new(Packet)
	copier.Copy(pk, expectedPackets[Pingresp][0].packet)

	buf := new(bytes.Buffer)
	for n := 0; n < b.N; n++ {
		pk.PingrespEncode(buf)
	}
}

func TestPubackEncode(t *testing.T) {
	require.Contains(t, expectedPackets, Puback)
	for i, wanted := range expectedPackets[Puback] {
		if !encodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(4), Puback, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)
		pk := new(Packet)
		copier.Copy(pk, wanted.packet)

		require.Equal(t, Puback, pk.FixedHeader.Type, "Mismatched FixedHeader Type [i:%d] %s", i, wanted.desc)

		buf := new(bytes.Buffer)
		err := pk.PubackEncode(buf)
		require.NoError(t, err, "Expected no error writing buffer [i:%d] %s", i, wanted.desc)
		encoded := buf.Bytes()

		require.Equal(t, len(wanted.rawBytes), len(encoded), "Mismatched packet length [i:%d] %s", i, wanted.desc)
		require.Equal(t, byte(Puback<<4), encoded[0], "Mismatched fixed header packets [i:%d] %s", i, wanted.desc)
		require.EqualValues(t, wanted.rawBytes, encoded, "Mismatched byte values [i:%d] %s", i, wanted.desc)

		require.Equal(t, wanted.packet.PacketID, pk.PacketID, "Mismatched Packet ID [i:%d] %s", i, wanted.desc)
	}
}

func BenchmarkPubackEncode(b *testing.B) {
	pk := new(Packet)
	copier.Copy(pk, expectedPackets[Puback][0].packet)

	buf := new(bytes.Buffer)
	for n := 0; n < b.N; n++ {
		pk.PubackEncode(buf)
	}
}

func TestPubackDecode(t *testing.T) {
	require.Contains(t, expectedPackets, Puback)
	for i, wanted := range expectedPackets[Puback] {
		if !decodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(4), Puback, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)

		pk := &Packet{FixedHeader: FixedHeader{Type: Puback}}
		err := pk.PubackDecode(wanted.rawBytes[2:]) // Unpack skips fixedheader.

		if wanted.failFirst != nil {
			require.Error(t, err, "Expected error unpacking buffer [i:%d] %s", i, wanted.desc)
			require.True(t, errors.Is(err, wanted.failFirst), "Expected fail state; %v [i:%d] %s", err.Error(), i, wanted.desc)
			continue
		}

		require.NoError(t, err, "Error unpacking buffer [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.PacketID, pk.PacketID, "Mismatched Packet ID [i:%d] %s", i, wanted.desc)
	}
}

func BenchmarkPubackDecode(b *testing.B) {
	pk := &Packet{FixedHeader: FixedHeader{Type: Puback}}
	pk.FixedHeader.Decode(expectedPackets[Puback][0].rawBytes[0])

	for n := 0; n < b.N; n++ {
		pk.PubackDecode(expectedPackets[Puback][0].rawBytes[2:])
	}
}

func TestPubcompEncode(t *testing.T) {
	require.Contains(t, expectedPackets, Pubcomp)
	for i, wanted := range expectedPackets[Pubcomp] {
		if !encodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(7), Pubcomp, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)
		pk := new(Packet)
		copier.Copy(pk, wanted.packet)

		require.Equal(t, Pubcomp, pk.FixedHeader.Type, "Mismatched FixedHeader Type [i:%d] %s", i, wanted.desc)

		buf := new(bytes.Buffer)
		err := pk.PubcompEncode(buf)
		require.NoError(t, err, "Expected no error writing buffer [i:%d] %s", i, wanted.desc)
		encoded := buf.Bytes()

		require.Equal(t, len(wanted.rawBytes), len(encoded), "Mismatched packet length [i:%d] %s", i, wanted.desc)
		require.Equal(t, byte(Pubcomp<<4), encoded[0], "Mismatched fixed header packets [i:%d] %s", i, wanted.desc)
		require.EqualValues(t, wanted.rawBytes, encoded, "Mismatched byte values [i:%d] %s", i, wanted.desc)

		require.Equal(t, wanted.packet.PacketID, pk.PacketID, "Mismatched Packet ID [i:%d] %s", i, wanted.desc)
	}
}

func BenchmarkPubcompEncode(b *testing.B) {
	pk := &Packet{FixedHeader: FixedHeader{Type: Pubcomp}}
	copier.Copy(pk, expectedPackets[Pubcomp][0].packet)

	buf := new(bytes.Buffer)
	for n := 0; n < b.N; n++ {
		pk.PubcompEncode(buf)
	}
}

func TestPubcompDecode(t *testing.T) {
	require.Contains(t, expectedPackets, Pubcomp)
	for i, wanted := range expectedPackets[Pubcomp] {
		if !decodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(7), Pubcomp, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)

		pk := &Packet{FixedHeader: FixedHeader{Type: Pubcomp}}
		err := pk.PubcompDecode(wanted.rawBytes[2:]) // Unpack skips fixedheader.

		if wanted.failFirst != nil {
			require.Error(t, err, "Expected error unpacking buffer [i:%d] %s", i, wanted.desc)
			require.True(t, errors.Is(err, wanted.failFirst), "Expected fail state; %v [i:%d] %s", err.Error(), i, wanted.desc)
			continue
		}

		require.NoError(t, err, "Error unpacking buffer [i:%d] %s", i, wanted.desc)

		require.Equal(t, wanted.packet.PacketID, pk.PacketID, "Mismatched Packet ID [i:%d] %s", i, wanted.desc)
	}
}

func BenchmarkPubcompDecode(b *testing.B) {
	pk := &Packet{FixedHeader: FixedHeader{Type: Pubcomp}}
	pk.FixedHeader.Decode(expectedPackets[Pubcomp][0].rawBytes[0])

	for n := 0; n < b.N; n++ {
		pk.PubcompDecode(expectedPackets[Pubcomp][0].rawBytes[2:])
	}
}

func TestPublishEncode(t *testing.T) {
	require.Contains(t, expectedPackets, Publish)
	for i, wanted := range expectedPackets[Publish] {
		if !encodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(3), Publish, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)
		pk := new(Packet)
		copier.Copy(pk, wanted.packet)

		require.Equal(t, Publish, pk.FixedHeader.Type, "Mismatched FixedHeader Type [i:%d] %s", i, wanted.desc)

		buf := new(bytes.Buffer)
		err := pk.PublishEncode(buf)
		encoded := buf.Bytes()

		if wanted.expect != nil {
			require.Error(t, err, "Expected error writing buffer [i:%d] %s", i, wanted.desc)
		} else {

			// If actualBytes is set, compare mutated version of byte string instead (to avoid length mismatches, etc).
			if len(wanted.actualBytes) > 0 {
				wanted.rawBytes = wanted.actualBytes
			}

			require.Equal(t, len(wanted.rawBytes), len(encoded), "Mismatched packet length [i:%d] %s", i, wanted.desc)
			if wanted.meta != nil {
				require.Equal(t, byte(Publish<<4)|wanted.meta.(byte), encoded[0], "Mismatched fixed header bytes [i:%d] %s", i, wanted.desc)
			} else {
				require.Equal(t, byte(Publish<<4), encoded[0], "Mismatched fixed header bytes [i:%d] %s", i, wanted.desc)
			}

			require.EqualValues(t, wanted.rawBytes, encoded, "Mismatched byte values [i:%d] %s", i, wanted.desc)
			require.Equal(t, wanted.packet.FixedHeader.Qos, pk.FixedHeader.Qos, "Mismatched QOS [i:%d] %s", i, wanted.desc)
			require.Equal(t, wanted.packet.FixedHeader.Dup, pk.FixedHeader.Dup, "Mismatched Dup [i:%d] %s", i, wanted.desc)
			require.Equal(t, wanted.packet.FixedHeader.Retain, pk.FixedHeader.Retain, "Mismatched Retain [i:%d] %s", i, wanted.desc)
			require.Equal(t, wanted.packet.PacketID, pk.PacketID, "Mismatched Packet ID [i:%d] %s", i, wanted.desc)
			require.NoError(t, err, "Expected no error writing buffer [i:%d] %s", i, wanted.desc)
		}
	}
}

func BenchmarkPublishEncode(b *testing.B) {
	pk := new(Packet)
	copier.Copy(pk, expectedPackets[Publish][0].packet)

	buf := new(bytes.Buffer)
	for n := 0; n < b.N; n++ {
		pk.PublishEncode(buf)
	}
}

func TestPublishDecode(t *testing.T) {
	require.Contains(t, expectedPackets, Publish)
	for i, wanted := range expectedPackets[Publish] {
		if !decodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(3), Publish, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)

		pk := &Packet{FixedHeader: FixedHeader{Type: Publish}}
		pk.FixedHeader.Decode(wanted.rawBytes[0])

		err := pk.PublishDecode(wanted.rawBytes[2:]) // Unpack skips fixedheader.
		if wanted.failFirst != nil {
			require.Error(t, err, "Expected fh error unpacking buffer [i:%d] %s", i, wanted.desc)
			require.True(t, errors.Is(err, wanted.failFirst), "Expected fail state; %v [i:%d] %s", err.Error(), i, wanted.desc)
			continue
		}

		require.NoError(t, err, "Error unpacking buffer [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.FixedHeader.Qos, pk.FixedHeader.Qos, "Mismatched QOS [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.FixedHeader.Dup, pk.FixedHeader.Dup, "Mismatched Dup [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.FixedHeader.Retain, pk.FixedHeader.Retain, "Mismatched Retain [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.PacketID, pk.PacketID, "Mismatched Packet ID [i:%d] %s", i, wanted.desc)

	}
}

func BenchmarkPublishDecode(b *testing.B) {
	pk := &Packet{FixedHeader: FixedHeader{Type: Publish}}
	pk.FixedHeader.Decode(expectedPackets[Publish][1].rawBytes[0])

	for n := 0; n < b.N; n++ {
		pk.PublishDecode(expectedPackets[Publish][1].rawBytes[2:])
	}
}

func TestPublishCopy(t *testing.T) {
	require.Contains(t, expectedPackets, Publish)
	for i, wanted := range expectedPackets[Publish] {
		if wanted.group == "copy" {

			pk := &Packet{FixedHeader: FixedHeader{Type: Publish}}
			err := pk.PublishDecode(wanted.rawBytes[2:]) // Unpack skips fixedheader.
			require.NoError(t, err, "Error unpacking buffer [i:%d] %s", i, wanted.desc)

			copied := pk.PublishCopy()

			require.Equal(t, byte(0), copied.FixedHeader.Qos, "Mismatched QOS [i:%d] %s", i, wanted.desc)
			require.Equal(t, false, copied.FixedHeader.Dup, "Mismatched Dup [i:%d] %s", i, wanted.desc)
			require.Equal(t, false, copied.FixedHeader.Retain, "Mismatched Retain [i:%d] %s", i, wanted.desc)

			require.Equal(t, pk.Payload, copied.Payload, "Mismatched Payload [i:%d] %s", i, wanted.desc)
			require.Equal(t, pk.TopicName, copied.TopicName, "Mismatched Topic Name [i:%d] %s", i, wanted.desc)

		}
	}
}

func BenchmarkPublishCopy(b *testing.B) {
	pk := &Packet{FixedHeader: FixedHeader{Type: Publish}}
	pk.FixedHeader.Decode(expectedPackets[Publish][1].rawBytes[0])

	for n := 0; n < b.N; n++ {
		pk.PublishCopy()
	}
}

func TestPublishValidate(t *testing.T) {
	require.Contains(t, expectedPackets, Publish)
	for i, wanted := range expectedPackets[Publish] {
		if wanted.group == "validate" || i == 0 {
			pk := wanted.packet
			ok, err := pk.PublishValidate()

			if i == 0 {
				require.NoError(t, err, "Publish should have validated - error incorrect [i:%d] %s", i, wanted.desc)
				require.Equal(t, Accepted, ok, "Publish should have validated - code incorrect [i:%d] %s", i, wanted.desc)
			} else {
				require.Equal(t, Failed, ok, "Publish packet didn't validate - code incorrect [i:%d] %s", i, wanted.desc)
				if err != nil {
					require.Equal(t, wanted.expect, err, "Publish packet didn't validate - error incorrect [i:%d] %s", i, wanted.desc)
				}
			}
		}
	}
}

func BenchmarkPublishValidate(b *testing.B) {
	pk := &Packet{FixedHeader: FixedHeader{Type: Publish}}
	pk.FixedHeader.Decode(expectedPackets[Publish][1].rawBytes[0])

	for n := 0; n < b.N; n++ {
		_, err := pk.PublishValidate()
		if err != nil {
			panic(err)
		}
	}
}

func TestPubrecEncode(t *testing.T) {
	require.Contains(t, expectedPackets, Pubrec)
	for i, wanted := range expectedPackets[Pubrec] {
		if !encodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(5), Pubrec, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)
		pk := new(Packet)
		copier.Copy(pk, wanted.packet)

		require.Equal(t, Pubrec, pk.FixedHeader.Type, "Mismatched FixedHeader Type [i:%d] %s", i, wanted.desc)

		buf := new(bytes.Buffer)
		err := pk.PubrecEncode(buf)
		require.NoError(t, err, "Expected no error writing buffer [i:%d] %s", i, wanted.desc)
		encoded := buf.Bytes()

		require.Equal(t, len(wanted.rawBytes), len(encoded), "Mismatched packet length [i:%d] %s", i, wanted.desc)
		require.Equal(t, byte(Pubrec<<4), encoded[0], "Mismatched fixed header packets [i:%d] %s", i, wanted.desc)
		require.EqualValues(t, wanted.rawBytes, encoded, "Mismatched byte values [i:%d] %s", i, wanted.desc)

		require.Equal(t, wanted.packet.PacketID, pk.PacketID, "Mismatched Packet ID [i:%d] %s", i, wanted.desc)

	}
}

func BenchmarkPubrecEncode(b *testing.B) {
	pk := new(Packet)
	copier.Copy(pk, expectedPackets[Pubrec][0].packet)

	buf := new(bytes.Buffer)
	for n := 0; n < b.N; n++ {
		pk.PubrecEncode(buf)
	}
}

func TestPubrecDecode(t *testing.T) {
	require.Contains(t, expectedPackets, Pubrec)
	for i, wanted := range expectedPackets[Pubrec] {
		if !decodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(5), Pubrec, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)

		pk := &Packet{FixedHeader: FixedHeader{Type: Pubrec}}
		err := pk.PubrecDecode(wanted.rawBytes[2:]) // Unpack skips fixedheader.

		if wanted.failFirst != nil {
			require.Error(t, err, "Expected error unpacking buffer [i:%d] %s", i, wanted.desc)
			require.True(t, errors.Is(err, wanted.failFirst), "Expected fail state; %v [i:%d] %s", err.Error(), i, wanted.desc)
			continue
		}

		require.NoError(t, err, "Error unpacking buffer [i:%d] %s", i, wanted.desc)

		require.Equal(t, wanted.packet.PacketID, pk.PacketID, "Mismatched Packet ID [i:%d] %s", i, wanted.desc)
	}
}

func BenchmarkPubrecDecode(b *testing.B) {
	pk := &Packet{FixedHeader: FixedHeader{Type: Pubrec}}
	pk.FixedHeader.Decode(expectedPackets[Pubrec][0].rawBytes[0])

	for n := 0; n < b.N; n++ {
		pk.PubrecDecode(expectedPackets[Pubrec][0].rawBytes[2:])
	}
}

func TestPubrelEncode(t *testing.T) {
	require.Contains(t, expectedPackets, Pubrel)
	for i, wanted := range expectedPackets[Pubrel] {
		if !encodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(6), Pubrel, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)
		pk := new(Packet)
		copier.Copy(pk, wanted.packet)

		require.Equal(t, Pubrel, pk.FixedHeader.Type, "Mismatched FixedHeader Type [i:%d] %s", i, wanted.desc)

		buf := new(bytes.Buffer)
		err := pk.PubrelEncode(buf)
		require.NoError(t, err, "Expected no error writing buffer [i:%d] %s", i, wanted.desc)
		encoded := buf.Bytes()

		require.Equal(t, len(wanted.rawBytes), len(encoded), "Mismatched packet length [i:%d] %s", i, wanted.desc)
		if wanted.meta != nil {
			require.Equal(t, byte(Pubrel<<4)|wanted.meta.(byte), encoded[0], "Mismatched fixed header bytes [i:%d] %s", i, wanted.desc)
		} else {
			require.Equal(t, byte(Pubrel<<4), encoded[0], "Mismatched fixed header bytes [i:%d] %s", i, wanted.desc)
		}
		require.EqualValues(t, wanted.rawBytes, encoded, "Mismatched byte values [i:%d] %s", i, wanted.desc)

		require.Equal(t, wanted.packet.PacketID, pk.PacketID, "Mismatched Packet ID [i:%d] %s", i, wanted.desc)
	}
}

func BenchmarkPubrelEncode(b *testing.B) {
	pk := new(Packet)
	copier.Copy(pk, expectedPackets[Pubrel][0].packet)

	buf := new(bytes.Buffer)
	for n := 0; n < b.N; n++ {
		pk.PubrelEncode(buf)
	}
}

func TestPubrelDecode(t *testing.T) {
	require.Contains(t, expectedPackets, Pubrel)
	for i, wanted := range expectedPackets[Pubrel] {
		if !decodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(6), Pubrel, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)

		pk := &Packet{FixedHeader: FixedHeader{Type: Pubrel, Qos: 1}}
		err := pk.PubrelDecode(wanted.rawBytes[2:]) // Unpack skips fixedheader.

		if wanted.failFirst != nil {
			require.Error(t, err, "Expected error unpacking buffer [i:%d] %s", i, wanted.desc)
			require.True(t, errors.Is(err, wanted.failFirst), "Expected fail state; %v [i:%d] %s", err.Error(), i, wanted.desc)
			continue
		}

		require.NoError(t, err, "Error unpacking buffer [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.PacketID, pk.PacketID, "Mismatched Packet ID [i:%d] %s", i, wanted.desc)
	}
}

func BenchmarkPubrelDecode(b *testing.B) {
	pk := &Packet{FixedHeader: FixedHeader{Type: Pubrel, Qos: 1}}
	pk.FixedHeader.Decode(expectedPackets[Pubrel][0].rawBytes[0])

	for n := 0; n < b.N; n++ {
		pk.PubrelDecode(expectedPackets[Pubrel][0].rawBytes[2:])
	}
}

func TestSubackEncode(t *testing.T) {
	require.Contains(t, expectedPackets, Suback)
	for i, wanted := range expectedPackets[Suback] {
		if !encodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(9), Suback, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)
		pk := new(Packet)
		copier.Copy(pk, wanted.packet)

		require.Equal(t, Suback, pk.FixedHeader.Type, "Mismatched FixedHeader Type [i:%d] %s", i, wanted.desc)

		buf := new(bytes.Buffer)
		err := pk.SubackEncode(buf)
		require.NoError(t, err, "Expected no error writing buffer [i:%d] %s", i, wanted.desc)
		encoded := buf.Bytes()

		require.Equal(t, len(wanted.rawBytes), len(encoded), "Mismatched packet length [i:%d] %s", i, wanted.desc)
		if wanted.meta != nil {
			require.Equal(t, byte(Suback<<4)|wanted.meta.(byte), encoded[0], "Mismatched mod fixed header packets [i:%d] %s", i, wanted.desc)
		} else {
			require.Equal(t, byte(Suback<<4), encoded[0], "Mismatched fixed header packets [i:%d] %s", i, wanted.desc)
		}

		require.EqualValues(t, wanted.rawBytes, encoded, "Mismatched byte values [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.PacketID, pk.PacketID, "Mismatched Packet ID [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.ReturnCodes, pk.ReturnCodes, "Mismatched Return Codes [i:%d] %s", i, wanted.desc)
	}
}

func BenchmarkSubackEncode(b *testing.B) {
	pk := new(Packet)
	copier.Copy(pk, expectedPackets[Suback][0].packet)

	buf := new(bytes.Buffer)
	for n := 0; n < b.N; n++ {
		pk.SubackEncode(buf)
	}
}

func TestSubackDecode(t *testing.T) {
	require.Contains(t, expectedPackets, Suback)
	for i, wanted := range expectedPackets[Suback] {
		if !decodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(9), Suback, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)

		pk := &Packet{FixedHeader: FixedHeader{Type: Suback}}
		err := pk.SubackDecode(wanted.rawBytes[2:]) // Unpack skips fixedheader.
		if wanted.failFirst != nil {
			require.Error(t, err, "Expected error unpacking buffer [i:%d] %s", i, wanted.desc)
			require.True(t, errors.Is(err, wanted.failFirst), "Expected fail state; %v [i:%d] %s", err.Error(), i, wanted.desc)
			continue
		}

		require.NoError(t, err, "Error unpacking buffer [i:%d] %s", i, wanted.desc)

		require.Equal(t, wanted.packet.PacketID, pk.PacketID, "Mismatched Packet ID [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.ReturnCodes, pk.ReturnCodes, "Mismatched Return Codes [i:%d] %s", i, wanted.desc)
	}
}

func BenchmarkSubackDecode(b *testing.B) {
	pk := &Packet{FixedHeader: FixedHeader{Type: Suback}}
	pk.FixedHeader.Decode(expectedPackets[Suback][0].rawBytes[0])

	for n := 0; n < b.N; n++ {
		pk.SubackDecode(expectedPackets[Suback][0].rawBytes[2:])
	}
}

func TestSubscribeEncode(t *testing.T) {
	require.Contains(t, expectedPackets, Subscribe)
	for i, wanted := range expectedPackets[Subscribe] {
		if !encodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(8), Subscribe, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)
		pk := new(Packet)
		copier.Copy(pk, wanted.packet)

		require.Equal(t, Subscribe, pk.FixedHeader.Type, "Mismatched FixedHeader Type [i:%d] %s", i, wanted.desc)

		buf := new(bytes.Buffer)
		err := pk.SubscribeEncode(buf)
		encoded := buf.Bytes()

		if wanted.expect != nil {
			require.Error(t, err, "Expected error writing buffer [i:%d] %s", i, wanted.desc)
		} else {
			require.NoError(t, err, "Error writing buffer [i:%d] %s", i, wanted.desc)
			require.Equal(t, len(wanted.rawBytes), len(encoded), "Mismatched packet length [i:%d] %s", i, wanted.desc)
			if wanted.meta != nil {
				require.Equal(t, byte(Subscribe<<4)|wanted.meta.(byte), encoded[0], "Mismatched fixed header bytes [i:%d] %s", i, wanted.desc)
			} else {
				require.Equal(t, byte(Subscribe<<4), encoded[0], "Mismatched fixed header bytes [i:%d] %s", i, wanted.desc)
			}

			require.EqualValues(t, wanted.rawBytes, encoded, "Mismatched byte values [i:%d] %s", i, wanted.desc)
			require.Equal(t, wanted.packet.PacketID, pk.PacketID, "Mismatched Packet ID [i:%d] %s", i, wanted.desc)
			require.Equal(t, wanted.packet.Topics, pk.Topics, "Mismatched Topics slice [i:%d] %s", i, wanted.desc)
			require.Equal(t, wanted.packet.Qoss, pk.Qoss, "Mismatched Qoss slice [i:%d] %s", i, wanted.desc)
		}
	}
}

func BenchmarkSubscribeEncode(b *testing.B) {
	pk := new(Packet)
	copier.Copy(pk, expectedPackets[Subscribe][0].packet)

	buf := new(bytes.Buffer)
	for n := 0; n < b.N; n++ {
		pk.SubscribeEncode(buf)
	}
}

func TestSubscribeDecode(t *testing.T) {
	require.Contains(t, expectedPackets, Subscribe)
	for i, wanted := range expectedPackets[Subscribe] {
		if !decodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(8), Subscribe, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)

		pk := &Packet{FixedHeader: FixedHeader{Type: Subscribe, Qos: 1}}
		err := pk.SubscribeDecode(wanted.rawBytes[2:]) // Unpack skips fixedheader.
		if wanted.failFirst != nil {
			require.Error(t, err, "Expected error unpacking buffer [i:%d] %s", i, wanted.desc)
			require.True(t, errors.Is(err, wanted.failFirst), "Expected fail state; %v [i:%d] %s", err.Error(), i, wanted.desc)
			continue
		}

		require.NoError(t, err, "Error unpacking buffer [i:%d] %s", i, wanted.desc)

		require.Equal(t, wanted.packet.PacketID, pk.PacketID, "Mismatched Packet ID [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.Topics, pk.Topics, "Mismatched Topics slice [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.Qoss, pk.Qoss, "Mismatched Qoss slice [i:%d] %s", i, wanted.desc)
	}
}

func BenchmarkSubscribeDecode(b *testing.B) {
	pk := &Packet{FixedHeader: FixedHeader{Type: Subscribe, Qos: 1}}
	pk.FixedHeader.Decode(expectedPackets[Subscribe][0].rawBytes[0])

	for n := 0; n < b.N; n++ {
		pk.SubscribeDecode(expectedPackets[Subscribe][0].rawBytes[2:])
	}
}

func TestSubscribeValidate(t *testing.T) {
	require.Contains(t, expectedPackets, Subscribe)
	for i, wanted := range expectedPackets[Subscribe] {
		if wanted.group == "validate" || i == 0 {
			pk := wanted.packet
			ok, err := pk.SubscribeValidate()

			if i == 0 {
				require.NoError(t, err, "Subscribe should have validated - error incorrect [i:%d] %s", i, wanted.desc)
				require.Equal(t, Accepted, ok, "Subscribe should have validated - code incorrect [i:%d] %s", i, wanted.desc)
			} else {
				require.Equal(t, Failed, ok, "Subscribe packet didn't validate - code incorrect [i:%d] %s", i, wanted.desc)
				if err != nil {
					require.Equal(t, wanted.expect, err, "Subscribe packet didn't validate - error incorrect [i:%d] %s", i, wanted.desc)
				}
			}
		}
	}
}

func BenchmarkSubscribeValidate(b *testing.B) {
	pk := &Packet{FixedHeader: FixedHeader{Type: Subscribe, Qos: 1}}
	pk.FixedHeader.Decode(expectedPackets[Subscribe][0].rawBytes[0])

	for n := 0; n < b.N; n++ {
		pk.SubscribeValidate()
	}
}

func TestUnsubackEncode(t *testing.T) {
	require.Contains(t, expectedPackets, Unsuback)
	for i, wanted := range expectedPackets[Unsuback] {
		if !encodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(11), Unsuback, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)
		pk := new(Packet)
		copier.Copy(pk, wanted.packet)

		require.Equal(t, Unsuback, pk.FixedHeader.Type, "Mismatched FixedHeader Type [i:%d] %s", i, wanted.desc)

		buf := new(bytes.Buffer)
		err := pk.UnsubackEncode(buf)
		require.NoError(t, err, "Expected no error writing buffer [i:%d] %s", i, wanted.desc)
		encoded := buf.Bytes()

		require.Equal(t, len(wanted.rawBytes), len(encoded), "Mismatched packet length [i:%d] %s", i, wanted.desc)
		if wanted.meta != nil {
			require.Equal(t, byte(Unsuback<<4)|wanted.meta.(byte), encoded[0], "Mismatched mod fixed header packets [i:%d] %s", i, wanted.desc)
		} else {
			require.Equal(t, byte(Unsuback<<4), encoded[0], "Mismatched fixed header packets [i:%d] %s", i, wanted.desc)
		}

		require.EqualValues(t, wanted.rawBytes, encoded, "Mismatched byte values [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.PacketID, pk.PacketID, "Mismatched Packet ID [i:%d] %s", i, wanted.desc)
	}
}

func BenchmarkUnsubackEncode(b *testing.B) {
	pk := new(Packet)
	copier.Copy(pk, expectedPackets[Unsuback][0].packet)

	buf := new(bytes.Buffer)
	for n := 0; n < b.N; n++ {
		pk.UnsubackEncode(buf)
	}
}

func TestUnsubackDecode(t *testing.T) {
	require.Contains(t, expectedPackets, Unsuback)
	for i, wanted := range expectedPackets[Unsuback] {
		if !decodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(11), Unsuback, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)

		pk := &Packet{FixedHeader: FixedHeader{Type: Unsuback}}
		err := pk.UnsubackDecode(wanted.rawBytes[2:]) // Unpack skips fixedheader.
		if wanted.failFirst != nil {
			require.Error(t, err, "Expected error unpacking buffer [i:%d] %s", i, wanted.desc)
			require.True(t, errors.Is(err, wanted.failFirst), "Expected fail state; %v [i:%d] %s", err.Error(), i, wanted.desc)
			continue
		}

		require.NoError(t, err, "Error unpacking buffer [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.PacketID, pk.PacketID, "Mismatched Packet ID [i:%d] %s", i, wanted.desc)
	}
}

func BenchmarkUnsubackDecode(b *testing.B) {
	pk := &Packet{FixedHeader: FixedHeader{Type: Unsuback}}
	pk.FixedHeader.Decode(expectedPackets[Unsuback][0].rawBytes[0])

	for n := 0; n < b.N; n++ {
		pk.UnsubackDecode(expectedPackets[Unsuback][0].rawBytes[2:])
	}
}

func TestUnsubscribeEncode(t *testing.T) {
	require.Contains(t, expectedPackets, Unsubscribe)
	for i, wanted := range expectedPackets[Unsubscribe] {
		if !encodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(10), Unsubscribe, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)
		pk := new(Packet)
		copier.Copy(pk, wanted.packet)

		require.Equal(t, Unsubscribe, pk.FixedHeader.Type, "Mismatched FixedHeader Type [i:%d] %s", i, wanted.desc)

		buf := new(bytes.Buffer)
		err := pk.UnsubscribeEncode(buf)
		encoded := buf.Bytes()
		if wanted.expect != nil {
			require.Error(t, err, "Expected error writing buffer [i:%d] %s", i, wanted.desc)
		} else {
			require.NoError(t, err, "Error writing buffer [i:%d] %s", i, wanted.desc)
			require.Equal(t, len(wanted.rawBytes), len(encoded), "Mismatched packet length [i:%d] %s", i, wanted.desc)
			if wanted.meta != nil {
				require.Equal(t, byte(Unsubscribe<<4)|wanted.meta.(byte), encoded[0], "Mismatched fixed header bytes [i:%d] %s", i, wanted.desc)
			} else {
				require.Equal(t, byte(Unsubscribe<<4), encoded[0], "Mismatched fixed header bytes [i:%d] %s", i, wanted.desc)
			}

			require.NoError(t, err, "Error writing buffer [i:%d] %s", i, wanted.desc)
			require.EqualValues(t, wanted.rawBytes, encoded, "Mismatched byte values [i:%d] %s", i, wanted.desc)

			require.Equal(t, wanted.packet.PacketID, pk.PacketID, "Mismatched Packet ID [i:%d] %s", i, wanted.desc)
			require.Equal(t, wanted.packet.Topics, pk.Topics, "Mismatched Topics slice [i:%d] %s", i, wanted.desc)
		}
	}
}

func BenchmarkUnsubscribeEncode(b *testing.B) {
	pk := new(Packet)
	copier.Copy(pk, expectedPackets[Unsubscribe][0].packet)

	buf := new(bytes.Buffer)
	for n := 0; n < b.N; n++ {
		pk.UnsubscribeEncode(buf)
	}
}

func TestUnsubscribeDecode(t *testing.T) {
	require.Contains(t, expectedPackets, Unsubscribe)
	for i, wanted := range expectedPackets[Unsubscribe] {
		if !decodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(10), Unsubscribe, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)

		pk := &Packet{FixedHeader: FixedHeader{Type: Unsubscribe, Qos: 1}}
		err := pk.UnsubscribeDecode(wanted.rawBytes[2:]) // Unpack skips fixedheader.
		if wanted.failFirst != nil {
			require.Error(t, err, "Expected error unpacking buffer [i:%d] %s", i, wanted.desc)
			require.True(t, errors.Is(err, wanted.failFirst), "Expected fail state; %v [i:%d] %s", err.Error(), i, wanted.desc)
			continue
		}

		require.NoError(t, err, "Error unpacking buffer [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.PacketID, pk.PacketID, "Mismatched Packet ID [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.Topics, pk.Topics, "Mismatched Topics slice [i:%d] %s", i, wanted.desc)
	}
}

func BenchmarkUnsubscribeDecode(b *testing.B) {
	pk := &Packet{FixedHeader: FixedHeader{Type: Unsubscribe, Qos: 1}}
	pk.FixedHeader.Decode(expectedPackets[Unsubscribe][0].rawBytes[0])

	for n := 0; n < b.N; n++ {
		pk.UnsubscribeDecode(expectedPackets[Unsubscribe][0].rawBytes[2:])
	}
}

func TestUnsubscribeValidate(t *testing.T) {
	require.Contains(t, expectedPackets, Unsubscribe)
	for i, wanted := range expectedPackets[Unsubscribe] {
		if wanted.group == "validate" || i == 0 {
			pk := wanted.packet
			ok, err := pk.UnsubscribeValidate()
			if i == 0 {
				require.NoError(t, err, "Unsubscribe should have validated - error incorrect [i:%d] %s", i, wanted.desc)
				require.Equal(t, Accepted, ok, "Unsubscribe should have validated - code incorrect [i:%d] %s", i, wanted.desc)
			} else {
				require.Equal(t, Failed, ok, "Unsubscribe packet didn't validate - code incorrect [i:%d] %s", i, wanted.desc)
				if err != nil {
					require.Equal(t, wanted.expect, err, "Unsubscribe packet didn't validate - error incorrect [i:%d] %s", i, wanted.desc)
				}
			}
		}
	}
}

func BenchmarkUnsubscribeValidate(b *testing.B) {
	pk := &Packet{FixedHeader: FixedHeader{Type: Unsubscribe, Qos: 1}}
	pk.FixedHeader.Decode(expectedPackets[Unsubscribe][0].rawBytes[0])

	for n := 0; n < b.N; n++ {
		pk.UnsubscribeValidate()
	}
}

func TestFormatPacketID(t *testing.T) {
	for _, id := range []uint16{0, 7, 0x100, 0xffff} {
		packet := &Packet{PacketID: id}
		require.Equal(t, fmt.Sprint(id), packet.FormatID())
	}
}
