package packets

import (
	"bytes"
	"testing"

	"github.com/jinzhu/copier"
	"github.com/stretchr/testify/require"
)

func TestPubackEncode(t *testing.T) {
	require.Contains(t, expectedPackets, Puback)
	for i, wanted := range expectedPackets[Puback] {
		if !encodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(4), Puback, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)
		pk := new(PubackPacket)
		copier.Copy(pk, wanted.packet.(*PubackPacket))

		require.Equal(t, Puback, pk.Type, "Mismatched Packet Type [i:%d] %s", i, wanted.desc)
		require.Equal(t, Puback, pk.FixedHeader.Type, "Mismatched FixedHeader Type [i:%d] %s", i, wanted.desc)

		var b bytes.Buffer
		err := pk.Encode(&b)

		encoded := b.Bytes()

		require.Equal(t, len(wanted.rawBytes), len(encoded), "Mismatched packet length [i:%d] %s", i, wanted.desc)
		require.Equal(t, byte(Puback<<4), encoded[0], "Mismatched fixed header packets [i:%d] %s", i, wanted.desc)

		require.NoError(t, err, "Error writing buffer [i:%d] %s", i, wanted.desc)
		require.EqualValues(t, wanted.rawBytes, encoded, "Mismatched byte values [i:%d] %s", i, wanted.desc)

		require.Equal(t, wanted.packet.(*PubackPacket).PacketID, pk.PacketID, "Mismatched Packet ID [i:%d] %s", i, wanted.desc)
	}
}

func TestPubackDecode(t *testing.T) {
	require.Contains(t, expectedPackets, Puback)
	for i, wanted := range expectedPackets[Puback] {

		if !decodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(4), Puback, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)

		pk := newPacket(Puback).(*PubackPacket)
		err := pk.Decode(wanted.rawBytes[2:]) // Unpack skips fixedheader.

		if wanted.failFirst != nil {
			require.Error(t, err, "Expected error unpacking buffer [i:%d] %s", i, wanted.desc)
			require.Equal(t, wanted.failFirst, err.Error(), "Expected fail state; %v [i:%d] %s", err.Error(), i, wanted.desc)
			continue
		}

		require.NoError(t, err, "Error unpacking buffer [i:%d] %s", i, wanted.desc)

		require.Equal(t, wanted.packet.(*PubackPacket).PacketID, pk.PacketID, "Mismatched Packet ID [i:%d] %s", i, wanted.desc)
	}
}

func BenchmarkPubackDecode(b *testing.B) {
	pk := newPacket(Puback).(*PubackPacket)
	pk.FixedHeader.decode(expectedPackets[Puback][0].rawBytes[0])

	for n := 0; n < b.N; n++ {
		pk.Decode(expectedPackets[Puback][0].rawBytes[2:])
	}
}
