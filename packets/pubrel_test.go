package packets

import (
	"bytes"
	"testing"

	"github.com/jinzhu/copier"
	"github.com/stretchr/testify/require"
)

func TestPubrelEncode(t *testing.T) {
	require.Contains(t, expectedPackets, Pubrel)
	for i, wanted := range expectedPackets[Pubrel] {
		if !encodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(6), Pubrel, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)
		pk := new(PubrelPacket)
		copier.Copy(pk, wanted.packet.(*PubrelPacket))

		require.Equal(t, Pubrel, pk.Type, "Mismatched Packet Type [i:%d] %s", i, wanted.desc)
		require.Equal(t, Pubrel, pk.FixedHeader.Type, "Mismatched FixedHeader Type [i:%d] %s", i, wanted.desc)

		buf := new(bytes.Buffer)
		err := pk.Encode(buf)
		require.NoError(t, err, "Expected no error writing buffer [i:%d] %s", i, wanted.desc)
		encoded := buf.Bytes()

		require.Equal(t, len(wanted.rawBytes), len(encoded), "Mismatched packet length [i:%d] %s", i, wanted.desc)
		require.Equal(t, byte(Pubrel<<4), encoded[0], "Mismatched fixed header packets [i:%d] %s", i, wanted.desc)
		require.EqualValues(t, wanted.rawBytes, encoded, "Mismatched byte values [i:%d] %s", i, wanted.desc)

		require.Equal(t, wanted.packet.(*PubrelPacket).PacketID, pk.PacketID, "Mismatched Packet ID [i:%d] %s", i, wanted.desc)
	}
}

func BenchmarkPubrelEncode(b *testing.B) {
	pk := new(PubrelPacket)
	copier.Copy(pk, expectedPackets[Pubrel][0].packet.(*PubrelPacket))

	buf := new(bytes.Buffer)
	for n := 0; n < b.N; n++ {
		pk.Encode(buf)
	}
}

func TestPubrelDecode(t *testing.T) {
	require.Contains(t, expectedPackets, Pubrel)
	for i, wanted := range expectedPackets[Pubrel] {
		if !decodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(6), Pubrel, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)

		pk := newPacket(Pubrel).(*PubrelPacket)
		err := pk.Decode(wanted.rawBytes[2:]) // Unpack skips fixedheader.

		if wanted.failFirst != nil {
			require.Error(t, err, "Expected error unpacking buffer [i:%d] %s", i, wanted.desc)
			require.Equal(t, wanted.failFirst, err, "Expected fail state; %v [i:%d] %s", err.Error(), i, wanted.desc)
			continue
		}

		require.NoError(t, err, "Error unpacking buffer [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*PubrelPacket).PacketID, pk.PacketID, "Mismatched Packet ID [i:%d] %s", i, wanted.desc)
	}
}

func BenchmarkPubrelDecode(b *testing.B) {
	pk := newPacket(Pubrel).(*PubrelPacket)
	pk.FixedHeader.decode(expectedPackets[Pubrel][0].rawBytes[0])

	for n := 0; n < b.N; n++ {
		pk.Decode(expectedPackets[Pubrel][0].rawBytes[2:])
	}
}

func TestPubrelValidate(t *testing.T) {
	pk := newPacket(Pubrel).(*PubrelPacket)
	pk.FixedHeader.decode(expectedPackets[Pubrel][0].rawBytes[0])

	b, err := pk.Validate()
	require.NoError(t, err)
	require.Equal(t, Accepted, b)

}

func BenchmarkPubrelValidate(b *testing.B) {
	pk := newPacket(Pubrel).(*PubrelPacket)
	pk.FixedHeader.decode(expectedPackets[Pubrel][0].rawBytes[0])

	for n := 0; n < b.N; n++ {
		pk.Validate()
	}
}
