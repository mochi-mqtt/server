package packets

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

// NewFixedHeader returns a fresh fixedheader for a given packet type.
func NewFixedHeader(packetType byte) FixedHeader {
	fh := FixedHeader{
		Type: packetType,
	}
	if packetType == Pubrel || packetType == Subscribe || packetType == Unsubscribe {
		fh.Qos = 1
	}

	return fh
}

// newPacket returns a packet of a specified packetType.
// this is a convenience package for testing and shouldn't be used for production
// code.
func newPacket(packetType byte) Packet {

	switch packetType {
	case Connect:
		return &ConnectPacket{FixedHeader: FixedHeader{Type: Connect}}
	case Connack:
		return &ConnackPacket{FixedHeader: FixedHeader{Type: Connack}}
	case Publish:
		return &PublishPacket{FixedHeader: FixedHeader{Type: Publish}}
	case Puback:
		return &PubackPacket{FixedHeader: FixedHeader{Type: Puback}}
	case Pubrec:
		return &PubrecPacket{FixedHeader: FixedHeader{Type: Pubrec}}
	case Pubrel:
		return &PubrelPacket{FixedHeader: FixedHeader{Type: Pubrel, Qos: 1}}
	case Pubcomp:
		return &PubcompPacket{FixedHeader: FixedHeader{Type: Pubcomp}}
	case Subscribe:
		return &SubscribePacket{FixedHeader: FixedHeader{Type: Subscribe, Qos: 1}}
	case Suback:
		return &SubackPacket{FixedHeader: FixedHeader{Type: Suback}}
	case Unsubscribe:
		return &UnsubscribePacket{FixedHeader: FixedHeader{Type: Unsubscribe, Qos: 1}}
	case Unsuback:
		return &UnsubackPacket{FixedHeader: FixedHeader{Type: Unsuback}}
	case Pingreq:
		return &PingreqPacket{FixedHeader: FixedHeader{Type: Pingreq}}
	case Pingresp:
		return &PingrespPacket{FixedHeader: FixedHeader{Type: Pingresp}}
	case Disconnect:
		return &DisconnectPacket{FixedHeader: FixedHeader{Type: Disconnect}}
	}
	return nil

}
