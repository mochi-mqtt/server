package packets

import (
	"bytes"
	"testing"

	"github.com/jinzhu/copier"
	"github.com/stretchr/testify/require"
)

func TestUnsubscribeEncode(t *testing.T) {
	require.Contains(t, expectedPackets, Unsubscribe)
	for i, wanted := range expectedPackets[Unsubscribe] {
		if !encodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(10), Unsubscribe, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)
		pk := new(UnsubscribePacket)
		copier.Copy(pk, wanted.packet.(*UnsubscribePacket))

		require.Equal(t, Unsubscribe, pk.Type, "Mismatched Packet Type [i:%d] %s", i, wanted.desc)
		require.Equal(t, Unsubscribe, pk.FixedHeader.Type, "Mismatched FixedHeader Type [i:%d] %s", i, wanted.desc)

		buf := new(bytes.Buffer)
		err := pk.Encode(buf)
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

			require.Equal(t, wanted.packet.(*UnsubscribePacket).PacketID, pk.PacketID, "Mismatched Packet ID [i:%d] %s", i, wanted.desc)
			require.Equal(t, wanted.packet.(*UnsubscribePacket).Topics, pk.Topics, "Mismatched Topics slice [i:%d] %s", i, wanted.desc)
		}
	}
}

func BenchmarkUnsubscribeEncode(b *testing.B) {
	pk := new(UnsubscribePacket)
	copier.Copy(pk, expectedPackets[Unsubscribe][0].packet.(*UnsubscribePacket))

	buf := new(bytes.Buffer)
	for n := 0; n < b.N; n++ {
		pk.Encode(buf)
	}
}

func TestUnsubscribeDecode(t *testing.T) {
	require.Contains(t, expectedPackets, Unsubscribe)
	for i, wanted := range expectedPackets[Unsubscribe] {

		if !decodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(10), Unsubscribe, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)

		pk := &UnsubscribePacket{FixedHeader: FixedHeader{Type: Unsubscribe, Qos: 1}}
		err := pk.Decode(wanted.rawBytes[2:]) // Unpack skips fixedheader.
		if wanted.failFirst != nil {
			require.Error(t, err, "Expected error unpacking buffer [i:%d] %s", i, wanted.desc)
			require.Equal(t, wanted.failFirst, err, "Expected fail state; %v [i:%d] %s", err.Error(), i, wanted.desc)
			continue
		}

		require.NoError(t, err, "Error unpacking buffer [i:%d] %s", i, wanted.desc)

		require.Equal(t, wanted.packet.(*UnsubscribePacket).PacketID, pk.PacketID, "Mismatched Packet ID [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*UnsubscribePacket).Topics, pk.Topics, "Mismatched Topics slice [i:%d] %s", i, wanted.desc)
	}
}

func BenchmarkUnsubscribeDecode(b *testing.B) {
	pk := &UnsubscribePacket{FixedHeader: FixedHeader{Type: Unsubscribe, Qos: 1}}
	pk.FixedHeader.Decode(expectedPackets[Unsubscribe][0].rawBytes[0])

	for n := 0; n < b.N; n++ {
		pk.Decode(expectedPackets[Unsubscribe][0].rawBytes[2:])
	}
}

func TestUnsubscribeValidate(t *testing.T) {
	require.Contains(t, expectedPackets, Unsubscribe)
	for i, wanted := range expectedPackets[Unsubscribe] {
		if wanted.group == "validate" || i == 0 {
			pk := wanted.packet.(*UnsubscribePacket)
			ok, err := pk.Validate()
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
	pk := &UnsubscribePacket{FixedHeader: FixedHeader{Type: Unsubscribe, Qos: 1}}
	pk.FixedHeader.Decode(expectedPackets[Unsubscribe][0].rawBytes[0])

	for n := 0; n < b.N; n++ {
		pk.Validate()
	}
}
