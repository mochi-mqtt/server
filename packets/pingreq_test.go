package packets

import (
	"testing"

	"bytes"

	"github.com/jinzhu/copier"
	"github.com/stretchr/testify/require"
)

func TestPingreqEncode(t *testing.T) {
	require.Contains(t, expectedPackets, Pingreq)
	for i, wanted := range expectedPackets[Pingreq] {

		require.Equal(t, uint8(12), Pingreq, "Incorrect Packet Type [i:%d]", i)

		pk := new(PingreqPacket)
		copier.Copy(pk, wanted.packet.(*PingreqPacket))

		require.Equal(t, Pingreq, pk.Type, "Mismatched Packet Type [i:%d]", i)
		require.Equal(t, Pingreq, pk.FixedHeader.Type, "Mismatched FixedHeader Type [i:%d]", i)

		var b bytes.Buffer
		err := pk.Encode(&b)

		require.NoError(t, err, "Error writing buffer [i:%d]", i)
		require.Equal(t, len(wanted.rawBytes), len(b.Bytes()), "Mismatched packet length [i:%d]", i)
		require.EqualValues(t, wanted.rawBytes, b.Bytes(), "Mismatched byte values [i:%d]", i)
	}
}

func TestPingreqDecode(t *testing.T) {
	pk := newPacket(Pingreq).(*PingreqPacket)

	var b = []byte{}
	err := pk.Decode(b)
	require.NoError(t, err, "Error unpacking buffer")
	require.Empty(t, b)
}

func BenchmarkPingreqDecode(b *testing.B) {
	pk := newPacket(Pingreq).(*PingreqPacket)
	pk.FixedHeader.decode(expectedPackets[Pingreq][0].rawBytes[0])

	for n := 0; n < b.N; n++ {
		pk.Decode(expectedPackets[Pingreq][0].rawBytes[2:])
	}
}
