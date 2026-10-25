package packets

import "testing"

// SubscribeDecodeV5Panic crafts a minimal MQTT v5 SUBSCRIBE payload where the
// topic filter string consumes the buffer exactly to its end, leaving no byte
// for the subscription-options byte. The v5 branch of SubscribeDecode reads that
// byte via a raw index `buf[offset]` (unlike the v4 branch which uses the bounded
// decodeByte), so offset == len(buf) causes an index-out-of-range panic.
func TestSubscribeDecodeV5Panic(t *testing.T) {
	// buf layout:
	//   0x00 0x01       PacketID = 1
	//   0x00            MQTT v5 properties: variable-byte length 0 (empty), 1 byte
	//   0x00 0x00       topic filter string length = 0 (empty filter)
	// After decoding the empty filter, offset == len(buf) == 5, and the v5
	// branch indexes buf[5].
	buf := []byte{0x00, 0x01, 0x00, 0x00, 0x00}

	pk := &Packet{ProtocolVersion: 5}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("PANIC CONFIRMED in SubscribeDecode (v5): %v", r)
		}
	}()

	_ = pk.SubscribeDecode(buf)
	t.Log("no panic")
}
