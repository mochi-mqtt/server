// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package packets

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// The PUBACK/PUBREC/PUBREL/PUBCOMP fast path must produce byte-for-byte what
// the general encoder produces, for every input it accepts — and fall through
// to the general encoder for everything else.
func TestEncodePubAckRelRecCompFastPathMatchesGeneral(t *testing.T) {
	props := map[string]Properties{
		"none":   {},
		"reason": {ReasonString: "nope"},
		"user":   {User: []UserProperty{{Key: "k", Val: "v"}}},
		"both":   {ReasonString: "nope", User: []UserProperty{{Key: "k", Val: "v"}}},
		// Publish properties inherited by buildAck; none are valid on an ack.
		"publish-only": {PayloadFormatFlag: true, PayloadFormat: 1, ContentType: "text/plain", MessageExpiryInterval: 30, TopicAliasFlag: true, TopicAlias: 3},
	}

	var cases int
	for _, typ := range []byte{Puback, Pubrec, Pubrel, Pubcomp} {
		for _, version := range []byte{3, 4, 5} {
			for _, reason := range []byte{0x00, 0x10, 0x80, 0x87, 0x92} {
				for propName, p := range props {
					for _, disallow := range []bool{false, true} {
						for _, maxSize := range []uint32{0, 6} {
							pk := Packet{
								FixedHeader:     FixedHeader{Type: typ, Qos: 1},
								ProtocolVersion: version,
								PacketID:        0xBEEF,
								ReasonCode:      reason,
								Properties:      p,
								Mods:            Mods{DisallowProblemInfo: disallow, MaxSize: maxSize},
							}
							name := fmt.Sprintf("type=%d/v%d/rc=%#x/%s/disallow=%v/max=%d", typ, version, reason, propName, disallow, maxSize)

							fast, general := pk, pk
							var fb, gb bytes.Buffer
							require.NoError(t, fast.encodePubAckRelRecComp(&fb), name)
							require.NoError(t, general.encodePubAckRelRecCompGeneral(&gb), name)
							require.Equal(t, gb.Bytes(), fb.Bytes(), name)
							require.Equal(t, general.FixedHeader.Remaining, fast.FixedHeader.Remaining, name)
							cases++
						}
					}
				}
			}
		}
	}
	require.Equal(t, 4*3*5*5*2*2, cases)
}

func BenchmarkPubackEncodeV5(b *testing.B) {
	pk := Packet{
		FixedHeader:     FixedHeader{Type: Puback},
		ProtocolVersion: 5,
		PacketID:        7,
		// buildAck copies the publish's properties onto the ack.
		Properties: Properties{PayloadFormatFlag: true, PayloadFormat: 1, MessageExpiryInterval: 60},
	}
	var buf bytes.Buffer
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		_ = pk.PubackEncode(&buf)
	}
}
