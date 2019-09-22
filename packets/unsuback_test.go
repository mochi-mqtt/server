package packets

import (
	"bytes"
	"testing"

	"github.com/jinzhu/copier"
	"github.com/stretchr/testify/require"
)

func TestUnsubackEncode(t *testing.T) {
	require.Contains(t, expectedPackets, Unsuback)
	for i, wanted := range expectedPackets[Unsuback] {
		if !encodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(11), Unsuback, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)
		pk := new(UnsubackPacket)
		copier.Copy(pk, wanted.packet.(*UnsubackPacket))

		require.Equal(t, Unsuback, pk.Type, "Mismatched Packet Type [i:%d] %s", i, wanted.desc)
		require.Equal(t, Unsuback, pk.FixedHeader.Type, "Mismatched FixedHeader Type [i:%d] %s", i, wanted.desc)

		buf := new(bytes.Buffer)
		err := pk.Encode(buf)
		require.NoError(t, err, "Expected no error writing buffer [i:%d] %s", i, wanted.desc)
		encoded := buf.Bytes()

		require.Equal(t, len(wanted.rawBytes), len(encoded), "Mismatched packet length [i:%d] %s", i, wanted.desc)
		if wanted.meta != nil {
			require.Equal(t, byte(Unsuback<<4)|wanted.meta.(byte), encoded[0], "Mismatched mod fixed header packets [i:%d] %s", i, wanted.desc)
		} else {
			require.Equal(t, byte(Unsuback<<4), encoded[0], "Mismatched fixed header packets [i:%d] %s", i, wanted.desc)
		}

		require.EqualValues(t, wanted.rawBytes, encoded, "Mismatched byte values [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*UnsubackPacket).PacketID, pk.PacketID, "Mismatched Packet ID [i:%d] %s", i, wanted.desc)
	}
}

func BenchmarkUnsubackEncode(b *testing.B) {
	pk := new(UnsubackPacket)
	copier.Copy(pk, expectedPackets[Unsuback][0].packet.(*UnsubackPacket))

	buf := new(bytes.Buffer)
	for n := 0; n < b.N; n++ {
		pk.Encode(buf)
	}
}

func TestUnsubackDecode(t *testing.T) {
	require.Contains(t, expectedPackets, Unsuback)
	for i, wanted := range expectedPackets[Unsuback] {

		if !decodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(11), Unsuback, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)

		pk := newPacket(Unsuback).(*UnsubackPacket)
		err := pk.Decode(wanted.rawBytes[2:]) // Unpack skips fixedheader.
		if wanted.failFirst != nil {
			require.Error(t, err, "Expected error unpacking buffer [i:%d] %s", i, wanted.desc)
			require.Equal(t, wanted.failFirst, err.Error(), "Expected fail state; %v [i:%d] %s", err.Error(), i, wanted.desc)
			continue
		}

		require.NoError(t, err, "Error unpacking buffer [i:%d] %s", i, wanted.desc)

		require.Equal(t, wanted.packet.(*UnsubackPacket).PacketID, pk.PacketID, "Mismatched Packet ID [i:%d] %s", i, wanted.desc)
	}
}

func BenchmarkUnsubackDecode(b *testing.B) {
	pk := newPacket(Unsuback).(*UnsubackPacket)
	pk.FixedHeader.decode(expectedPackets[Unsuback][0].rawBytes[0])

	for n := 0; n < b.N; n++ {
		pk.Decode(expectedPackets[Unsuback][0].rawBytes[2:])
	}
}
