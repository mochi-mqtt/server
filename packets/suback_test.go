package packets

import (
	"bytes"
	"testing"

	"github.com/jinzhu/copier"
	"github.com/stretchr/testify/require"
)

func TestSubackEncode(t *testing.T) {
	require.Contains(t, expectedPackets, Suback)
	for i, wanted := range expectedPackets[Suback] {

		if !encodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(9), Suback, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)
		pk := new(SubackPacket)
		copier.Copy(pk, wanted.packet.(*SubackPacket))

		require.Equal(t, Suback, pk.Type, "Mismatched Packet Type [i:%d] %s", i, wanted.desc)
		require.Equal(t, Suback, pk.FixedHeader.Type, "Mismatched FixedHeader Type [i:%d] %s", i, wanted.desc)

		buf := new(bytes.Buffer)
		err := pk.Encode(buf)
		require.NoError(t, err, "Expected no error writing buffer [i:%d] %s", i, wanted.desc)
		encoded := buf.Bytes()

		require.Equal(t, len(wanted.rawBytes), len(encoded), "Mismatched packet length [i:%d] %s", i, wanted.desc)
		if wanted.meta != nil {
			require.Equal(t, byte(Suback<<4)|wanted.meta.(byte), encoded[0], "Mismatched mod fixed header packets [i:%d] %s", i, wanted.desc)
		} else {
			require.Equal(t, byte(Suback<<4), encoded[0], "Mismatched fixed header packets [i:%d] %s", i, wanted.desc)
		}

		require.EqualValues(t, wanted.rawBytes, encoded, "Mismatched byte values [i:%d] %s", i, wanted.desc)

		require.Equal(t, wanted.packet.(*SubackPacket).PacketID, pk.PacketID, "Mismatched Packet ID [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*SubackPacket).ReturnCodes, pk.ReturnCodes, "Mismatched Return Codes [i:%d] %s", i, wanted.desc)
	}
}

func BenchmarkSubackEncode(b *testing.B) {
	pk := new(SubackPacket)
	copier.Copy(pk, expectedPackets[Suback][0].packet.(*SubackPacket))

	buf := new(bytes.Buffer)
	for n := 0; n < b.N; n++ {
		pk.Encode(buf)
	}
}

func TestSubackDecode(t *testing.T) {
	require.Contains(t, expectedPackets, Suback)
	for i, wanted := range expectedPackets[Suback] {

		if !decodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(9), Suback, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)

		pk := newPacket(Suback).(*SubackPacket)
		err := pk.Decode(wanted.rawBytes[2:]) // Unpack skips fixedheader.
		if wanted.failFirst != nil {
			require.Error(t, err, "Expected error unpacking buffer [i:%d] %s", i, wanted.desc)
			require.Equal(t, wanted.failFirst, err.Error(), "Expected fail state; %v [i:%d] %s", err.Error(), i, wanted.desc)
			continue
		}

		require.NoError(t, err, "Error unpacking buffer [i:%d] %s", i, wanted.desc)

		require.Equal(t, wanted.packet.(*SubackPacket).PacketID, pk.PacketID, "Mismatched Packet ID [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*SubackPacket).ReturnCodes, pk.ReturnCodes, "Mismatched Return Codes [i:%d] %s", i, wanted.desc)
	}
}

func BenchmarkSubackDecode(b *testing.B) {
	pk := newPacket(Suback).(*SubackPacket)
	pk.FixedHeader.decode(expectedPackets[Suback][0].rawBytes[0])

	for n := 0; n < b.N; n++ {
		pk.Decode(expectedPackets[Suback][0].rawBytes[2:])
	}
}
