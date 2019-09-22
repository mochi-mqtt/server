package packets

import (
	"bytes"
	"testing"

	"github.com/jinzhu/copier"
	"github.com/stretchr/testify/require"
)

func TestPingrespEncode(t *testing.T) {
	require.Contains(t, expectedPackets, Pingresp)
	for i, wanted := range expectedPackets[Pingresp] {

		require.Equal(t, uint8(13), Pingresp, "Incorrect Packet Type [i:%d]", i)

		pk := new(PingrespPacket)
		copier.Copy(pk, wanted.packet.(*PingrespPacket))

		require.Equal(t, Pingresp, pk.Type, "Mismatched Packet Type [i:%d]", i)
		require.Equal(t, Pingresp, pk.FixedHeader.Type, "Mismatched FixedHeader Type [i:%d]", i)

		var b bytes.Buffer
		err := pk.Encode(&b)

		require.NoError(t, err, "Error writing buffer [i:%d]", i)
		require.Equal(t, len(wanted.rawBytes), len(b.Bytes()), "Mismatched packet length [i:%d]", i)
		require.EqualValues(t, wanted.rawBytes, b.Bytes(), "Mismatched byte values [i:%d]", i)
	}
}

func TestPingrespDecode(t *testing.T) {
	pk := newPacket(Pingresp).(*PingrespPacket)

	var b = []byte{}
	err := pk.Decode(b)
	require.NoError(t, err, "Error unpacking buffer")
	require.Empty(t, b)
}

func BenchmarkPingrespDecode(b *testing.B) {
	pk := newPacket(Pingresp).(*PingrespPacket)
	pk.FixedHeader.decode(expectedPackets[Pingresp][0].rawBytes[0])

	for n := 0; n < b.N; n++ {
		pk.Decode(expectedPackets[Pingresp][0].rawBytes[2:])
	}
}
