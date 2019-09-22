package packets

import (
	"bytes"
	"testing"

	"github.com/jinzhu/copier"
	"github.com/stretchr/testify/require"
)

func TestDisconnectEncode(t *testing.T) {
	require.Contains(t, expectedPackets, Disconnect)
	for i, wanted := range expectedPackets[Disconnect] {
		require.Equal(t, uint8(14), Disconnect, "Incorrect Packet Type [i:%d]", i)

		pk := new(DisconnectPacket)
		copier.Copy(pk, wanted.packet.(*DisconnectPacket))

		require.Equal(t, Disconnect, pk.Type, "Mismatched Packet Type [i:%d]", i)
		require.Equal(t, Disconnect, pk.FixedHeader.Type, "Mismatched FixedHeader Type [i:%d]", i)

		buf := new(bytes.Buffer)
		err := pk.Encode(buf)
		require.NoError(t, err, "Expected no error writing buffer [i:%d] %s", i, wanted.desc)
		encoded := buf.Bytes()

		require.Equal(t, len(wanted.rawBytes), len(encoded), "Mismatched packet length [i:%d]", i)
		require.EqualValues(t, wanted.rawBytes, encoded, "Mismatched byte values [i:%d]", i)
	}
}

func BenchmarkDisconnectEncode(b *testing.B) {
	pk := new(DisconnectPacket)
	copier.Copy(pk, expectedPackets[Disconnect][0].packet.(*DisconnectPacket))

	buf := new(bytes.Buffer)
	for n := 0; n < b.N; n++ {
		pk.Encode(buf)
	}
}

func TestDisconnectDecode(t *testing.T) {
	pk := newPacket(Disconnect).(*DisconnectPacket)

	var b = []byte{}
	err := pk.Decode(b)
	require.NoError(t, err, "Error unpacking buffer")
	require.Empty(t, b)
}

func BenchmarkDisconnectDecode(b *testing.B) {
	pk := newPacket(Disconnect).(*DisconnectPacket)
	pk.FixedHeader.decode(expectedPackets[Disconnect][0].rawBytes[0])

	for n := 0; n < b.N; n++ {
		pk.Decode(expectedPackets[Disconnect][0].rawBytes[2:])
	}
}

func TestDisconnectValidate(t *testing.T) {
	pk := newPacket(Disconnect).(*DisconnectPacket)
	pk.FixedHeader.decode(expectedPackets[Disconnect][0].rawBytes[0])

	b, err := pk.Validate()
	require.NoError(t, err)
	require.Equal(t, Accepted, b)

}

func BenchmarkDisconnectValidate(b *testing.B) {
	pk := newPacket(Disconnect).(*DisconnectPacket)
	pk.FixedHeader.decode(expectedPackets[Disconnect][0].rawBytes[0])

	for n := 0; n < b.N; n++ {
		pk.Validate()
	}
}
