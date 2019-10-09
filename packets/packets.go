package packets

import (
	"bytes"
)

// All of the valid packet types and their packet identifier.
const (
	Reserved    byte = iota
	Connect          // 1
	Connack          // 2
	Publish          // 3
	Puback           // 4
	Pubrec           // 5
	Pubrel           // 6
	Pubcomp          // 7
	Subscribe        // 8
	Suback           // 9
	Unsubscribe      // 10
	Unsuback         // 11
	Pingreq          // 12
	Pingresp         // 13
	Disconnect       // 14
)

// Names is a map that provides human-readable names for the different
// MQTT packet types based on their ids.
var Names = map[byte]string{
	0:  "RESERVED",
	1:  "CONNECT",
	2:  "CONNACK",
	3:  "PUBLISH",
	4:  "PUBACK",
	5:  "PUBREC",
	6:  "PUBREL",
	7:  "PUBCOMP",
	8:  "SUBSCRIBE",
	9:  "SUBACK",
	10: "UNSUBSCRIBE",
	11: "UNSUBACK",
	12: "PINGREQ",
	13: "PINGRESP",
	14: "DISCONNECT",
}

// Packet is the base interface that all MQTT packets must implement.
type Packet interface {

	// Encode encodes a packet into a byte buffer.
	Encode(*bytes.Buffer) error

	// Decode decodes a byte array into a packet struct.
	Decode([]byte) error

	// Validate the packet. Returns a error code and error if not valid.
	Validate() (byte, error)
}
