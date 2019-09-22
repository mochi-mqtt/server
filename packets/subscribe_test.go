package packets

import (
	"bytes"
	"testing"

	"github.com/jinzhu/copier"
	"github.com/stretchr/testify/require"
)

func TestSubscribeEncode(t *testing.T) {
	require.Contains(t, expectedPackets, Subscribe)
	for i, wanted := range expectedPackets[Subscribe] {
		if !encodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(8), Subscribe, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)
		pk := new(SubscribePacket)
		copier.Copy(pk, wanted.packet.(*SubscribePacket))

		require.Equal(t, Subscribe, pk.Type, "Mismatched Packet Type [i:%d] %s", i, wanted.desc)
		require.Equal(t, Subscribe, pk.FixedHeader.Type, "Mismatched FixedHeader Type [i:%d] %s", i, wanted.desc)

		buf := new(bytes.Buffer)
		err := pk.Encode(buf)
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
			require.Equal(t, wanted.packet.(*SubscribePacket).PacketID, pk.PacketID, "Mismatched Packet ID [i:%d] %s", i, wanted.desc)
			require.Equal(t, wanted.packet.(*SubscribePacket).Topics, pk.Topics, "Mismatched Topics slice [i:%d] %s", i, wanted.desc)
			require.Equal(t, wanted.packet.(*SubscribePacket).Qoss, pk.Qoss, "Mismatched Qoss slice [i:%d] %s", i, wanted.desc)
		}

	}
}

func BenchmarkSubscribeEncode(b *testing.B) {
	pk := new(SubscribePacket)
	copier.Copy(pk, expectedPackets[Subscribe][0].packet.(*SubscribePacket))

	buf := new(bytes.Buffer)
	for n := 0; n < b.N; n++ {
		pk.Encode(buf)
	}
}

func TestSubscribeDecode(t *testing.T) {
	require.Contains(t, expectedPackets, Subscribe)
	for i, wanted := range expectedPackets[Subscribe] {

		if !decodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(8), Subscribe, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)

		pk := newPacket(Subscribe).(*SubscribePacket)
		err := pk.Decode(wanted.rawBytes[2:]) // Unpack skips fixedheader.
		if wanted.failFirst != nil {
			require.Error(t, err, "Expected error unpacking buffer [i:%d] %s", i, wanted.desc)
			require.Equal(t, wanted.failFirst, err.Error(), "Expected fail state; %v [i:%d] %s", err.Error(), i, wanted.desc)
			continue
		}

		require.NoError(t, err, "Error unpacking buffer [i:%d] %s", i, wanted.desc)

		require.Equal(t, wanted.packet.(*SubscribePacket).PacketID, pk.PacketID, "Mismatched Packet ID [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*SubscribePacket).Topics, pk.Topics, "Mismatched Topics slice [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*SubscribePacket).Qoss, pk.Qoss, "Mismatched Qoss slice [i:%d] %s", i, wanted.desc)
	}
}

func BenchmarkSubscribeDecode(b *testing.B) {
	pk := newPacket(Subscribe).(*SubscribePacket)
	pk.FixedHeader.decode(expectedPackets[Subscribe][0].rawBytes[0])

	for n := 0; n < b.N; n++ {
		pk.Decode(expectedPackets[Subscribe][0].rawBytes[2:])
	}
}
