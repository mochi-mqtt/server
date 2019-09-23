package packets

import (
	"bytes"
	"testing"

	"github.com/jinzhu/copier"
	"github.com/stretchr/testify/require"
)

func TestConnackEncode(t *testing.T) {
	require.Contains(t, expectedPackets, Connack)
	for i, wanted := range expectedPackets[Connack] {

		if !encodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(2), Connack, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)
		pk := new(ConnackPacket)
		copier.Copy(pk, wanted.packet.(*ConnackPacket))

		require.Equal(t, Connack, pk.Type, "Mismatched Packet Type [i:%d] %s", i, wanted.desc)
		require.Equal(t, Connack, pk.FixedHeader.Type, "Mismatched FixedHeader Type [i:%d] %s", i, wanted.desc)

		buf := new(bytes.Buffer)
		err := pk.Encode(buf)
		require.NoError(t, err, "Expected no error writing buffer [i:%d] %s", i, wanted.desc)
		encoded := buf.Bytes()

		require.Equal(t, len(wanted.rawBytes), len(encoded), "Mismatched packet length [i:%d] %s", i, wanted.desc)
		require.Equal(t, byte(Connack<<4), encoded[0], "Mismatched fixed header packets [i:%d] %s", i, wanted.desc)
		require.EqualValues(t, wanted.rawBytes, encoded, "Mismatched byte values [i:%d] %s", i, wanted.desc)

		require.Equal(t, wanted.packet.(*ConnackPacket).ReturnCode, pk.ReturnCode, "Mismatched return code [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*ConnackPacket).SessionPresent, pk.SessionPresent, "Mismatched session present bool [i:%d] %s", i, wanted.desc)
	}
}

func BenchmarkConnackEncode(b *testing.B) {
	pk := new(ConnackPacket)
	copier.Copy(pk, expectedPackets[Connack][0].packet.(*ConnackPacket))

	buf := new(bytes.Buffer)
	for n := 0; n < b.N; n++ {
		pk.Encode(buf)
	}
}

func TestConnackDecode(t *testing.T) {
	require.Contains(t, expectedPackets, Connack)
	for i, wanted := range expectedPackets[Connack] {

		if !decodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(2), Connack, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)

		pk := newPacket(Connack).(*ConnackPacket)
		err := pk.Decode(wanted.rawBytes[2:]) // Unpack skips fixedheader.
		if wanted.failFirst != nil {
			require.Error(t, err, "Expected error unpacking buffer [i:%d] %s", i, wanted.desc)
			require.Equal(t, wanted.failFirst, err, "Expected fail state; %v [i:%d] %s", err.Error(), i, wanted.desc)
			continue
		}

		require.NoError(t, err, "Error unpacking buffer [i:%d] %s", i, wanted.desc)

		require.Equal(t, wanted.packet.(*ConnackPacket).ReturnCode, pk.ReturnCode, "Mismatched return code [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*ConnackPacket).SessionPresent, pk.SessionPresent, "Mismatched session present bool [i:%d] %s", i, wanted.desc)
	}
}

func BenchmarkConnackDecode(b *testing.B) {
	pk := newPacket(Connack).(*ConnackPacket)
	pk.FixedHeader.decode(expectedPackets[Connack][0].rawBytes[0])

	for n := 0; n < b.N; n++ {
		pk.Decode(expectedPackets[Connack][0].rawBytes[2:])
	}
}

func TestConnackValidate(t *testing.T) {
	pk := newPacket(Connack).(*ConnackPacket)
	pk.FixedHeader.decode(expectedPackets[Connack][0].rawBytes[0])

	b, err := pk.Validate()
	require.NoError(t, err)
	require.Equal(t, Accepted, b)

}

func BenchmarkConnackValidate(b *testing.B) {
	pk := newPacket(Connack).(*ConnackPacket)
	pk.FixedHeader.decode(expectedPackets[Connack][0].rawBytes[0])

	for n := 0; n < b.N; n++ {
		pk.Validate()
	}
}
