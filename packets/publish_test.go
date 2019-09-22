package packets

import (
	"bytes"
	"testing"

	"github.com/jinzhu/copier"
	"github.com/stretchr/testify/require"
)

func TestPublishEncode(t *testing.T) {
	require.Contains(t, expectedPackets, Publish)
	for i, wanted := range expectedPackets[Publish] {

		if !encodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(3), Publish, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)
		pk := new(PublishPacket)
		copier.Copy(pk, wanted.packet.(*PublishPacket))

		require.Equal(t, Publish, pk.Type, "Mismatched Packet Type [i:%d] %s", i, wanted.desc)
		require.Equal(t, Publish, pk.FixedHeader.Type, "Mismatched FixedHeader Type [i:%d] %s", i, wanted.desc)

		buf := new(bytes.Buffer)
		err := pk.Encode(buf)
		encoded := buf.Bytes()

		if wanted.expect != nil {
			require.Error(t, err, "Expected error writing buffer [i:%d] %s", i, wanted.desc)
		} else {

			// If actualBytes is set, compare mutated version of byte string instead (to avoid length mismatches, etc).
			if len(wanted.actualBytes) > 0 {
				wanted.rawBytes = wanted.actualBytes
			}

			require.Equal(t, len(wanted.rawBytes), len(encoded), "Mismatched packet length [i:%d] %s", i, wanted.desc)
			if wanted.meta != nil {
				require.Equal(t, byte(Publish<<4)|wanted.meta.(byte), encoded[0], "Mismatched fixed header bytes [i:%d] %s", i, wanted.desc)
			} else {
				require.Equal(t, byte(Publish<<4), encoded[0], "Mismatched fixed header bytes [i:%d] %s", i, wanted.desc)
			}

			require.EqualValues(t, wanted.rawBytes, encoded, "Mismatched byte values [i:%d] %s", i, wanted.desc)
			require.Equal(t, wanted.packet.(*PublishPacket).FixedHeader.Qos, pk.FixedHeader.Qos, "Mismatched QOS [i:%d] %s", i, wanted.desc)
			require.Equal(t, wanted.packet.(*PublishPacket).FixedHeader.Dup, pk.FixedHeader.Dup, "Mismatched Dup [i:%d] %s", i, wanted.desc)
			require.Equal(t, wanted.packet.(*PublishPacket).FixedHeader.Retain, pk.FixedHeader.Retain, "Mismatched Retain [i:%d] %s", i, wanted.desc)
			require.Equal(t, wanted.packet.(*PublishPacket).PacketID, pk.PacketID, "Mismatched Packet ID [i:%d] %s", i, wanted.desc)
			require.NoError(t, err, "Expected no error writing buffer [i:%d] %s", i, wanted.desc)
		}
	}
}

func BenchmarkPublishEncode(b *testing.B) {
	pk := new(PublishPacket)
	copier.Copy(pk, expectedPackets[Publish][0].packet.(*PublishPacket))

	buf := new(bytes.Buffer)
	for n := 0; n < b.N; n++ {
		pk.Encode(buf)
	}
}

func TestPublishDecode(t *testing.T) {
	require.Contains(t, expectedPackets, Publish)
	for i, wanted := range expectedPackets[Publish] {
		if !decodeTestOK(wanted) {
			continue
		}

		require.Equal(t, uint8(3), Publish, "Incorrect Packet Type [i:%d] %s", i, wanted.desc)

		pk := newPacket(Publish).(*PublishPacket)
		pk.FixedHeader.decode(wanted.rawBytes[0])

		err := pk.Decode(wanted.rawBytes[2:]) // Unpack skips fixedheader.
		if wanted.failFirst != nil {
			require.Error(t, err, "Expected error unpacking buffer [i:%d] %s", i, wanted.desc)
			require.Equal(t, wanted.failFirst, err.Error(), "Expected fail state; %v [i:%d] %s", err.Error(), i, wanted.desc)
			continue
		}

		require.NoError(t, err, "Error unpacking buffer [i:%d] %s", i, wanted.desc)

		require.Equal(t, wanted.packet.(*PublishPacket).FixedHeader.Qos, pk.FixedHeader.Qos, "Mismatched QOS [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*PublishPacket).FixedHeader.Dup, pk.FixedHeader.Dup, "Mismatched Dup [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*PublishPacket).FixedHeader.Retain, pk.FixedHeader.Retain, "Mismatched Retain [i:%d] %s", i, wanted.desc)
		require.Equal(t, wanted.packet.(*PublishPacket).PacketID, pk.PacketID, "Mismatched Packet ID [i:%d] %s", i, wanted.desc)
	}
}

func BenchmarkPublishDecode(b *testing.B) {
	pk := newPacket(Publish).(*PublishPacket)
	pk.FixedHeader.decode(expectedPackets[Publish][1].rawBytes[0])

	for n := 0; n < b.N; n++ {
		pk.Decode(expectedPackets[Publish][1].rawBytes[2:])
	}
}

func TestPublishCopy(t *testing.T) {
	require.Contains(t, expectedPackets, Publish)
	for i, wanted := range expectedPackets[Publish] {
		if wanted.group == "copy" {

			pk := newPacket(Publish).(*PublishPacket)
			err := pk.Decode(wanted.rawBytes[2:]) // Unpack skips fixedheader.
			require.NoError(t, err, "Error unpacking buffer [i:%d] %s", i, wanted.desc)

			copied := pk.Copy()

			require.Equal(t, byte(0), copied.FixedHeader.Qos, "Mismatched QOS [i:%d] %s", i, wanted.desc)
			require.Equal(t, false, copied.FixedHeader.Dup, "Mismatched Dup [i:%d] %s", i, wanted.desc)
			require.Equal(t, false, copied.FixedHeader.Retain, "Mismatched Retain [i:%d] %s", i, wanted.desc)

			require.Equal(t, pk.Payload, copied.Payload, "Mismatched Payload [i:%d] %s", i, wanted.desc)
			require.Equal(t, pk.TopicName, copied.TopicName, "Mismatched Topic Name [i:%d] %s", i, wanted.desc)

		}
	}
}

func BenchmarkPublishCopy(b *testing.B) {
	pk := newPacket(Publish).(*PublishPacket)
	pk.FixedHeader.decode(expectedPackets[Publish][1].rawBytes[0])

	for n := 0; n < b.N; n++ {
		pk.Copy()
	}
}

func TestPublishValidate(t *testing.T) {
	require.Contains(t, expectedPackets, Publish)
	for i, wanted := range expectedPackets[Publish] {
		if wanted.group == "validate" {

			pk := wanted.packet.(*PublishPacket)
			ok, err := pk.Validate()

			require.Equal(t, Failed, ok, "Publish packet didn't validate - code incorrect [i:%d] %s", i, wanted.desc)
			if err != nil {
				require.Equal(t, wanted.expect, err.Error(), "Publish packet didn't validate - error incorrect [i:%d] %s", i, wanted.desc)
			}

		}
	}
}

func BenchmarkPublishValidate(b *testing.B) {
	pk := newPacket(Publish).(*PublishPacket)
	pk.FixedHeader.decode(expectedPackets[Publish][1].rawBytes[0])

	for n := 0; n < b.N; n++ {
		_, err := pk.Validate()
		if err != nil {
			panic(err)
		}
	}
}
