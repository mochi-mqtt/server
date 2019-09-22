package packets

/*



## Packets Funcs

# MQTT 3.1.1
[MQTT-1.4.0-1] The character data in a UTF-8 encoded string MUST be well-formed UTF-8 as defined by the Unicode specification [Unicode] and restated in RFC 3629 [RFC 3629]. In particular this data MUST NOT include encodings of code points between U+D800 and U+DFFF. If a receiver (Server or Client) receives a Control Packet containing ill-formed UTF-8 it MUST close the Network Connection
[MQTT-1.4.0-2] A UTF-8 encoded string MUST NOT include an encoding of the null character U+0000. If a receiver (Server or Client) receives a Control Packet containing U+0000 it MUST close the Network Connection.
[MQTT-1.4.0-3] A UTF-8 encoded sequence 0xEF 0xBB 0xBF is always to be interpreted to mean U+FEFF ("ZERO WIDTH NO-BREAK SPACE") wherever it appears in a string and MUST NOT be skipped over or stripped off by a packet receiver.
[MQTT-2.2.2-1] Where a flag bit is marked as “Reserved” in Table 2.2 - Flag Bits, it is reserved for future use and MUST be set to the value listed in that table
[MQTT-2.2.2-2] If invalid flags are received, the receiver MUST close the Network Connection.
[MQTT-2.3.1-1] SUBSCRIBE, UNSUBSCRIBE, and PUBLISH (in cases where QoS > 0) Control Packets MUST contain a non-zero 16-bit Packet Identifier.
[MQTT-2.3.1-5] A PUBLISH Packet MUST NOT contain a Packet Identifier if its QoS value is set to 0.


## TODO

## Broker tests
// [MQTT-2.3.1-4] The same conditions [MQTT-2.3.1-3] apply to a Server when it sends a PUBLISH with QoS >0.
// [MQTT-2.3.1-2] Each time a Client sends a new packet of one of these types it MUST assign it a currently unused Packet Identifier.
// [MQTT-2.3.1-3] If a Client re-sends a particular Control Packet, then it MUST use the same Packet Identifier in subsequent re-sends of that packet. The Packet Identifier becomes available for reuse after the Client has processed the corresponding acknowledgement packet. In the case of a QoS 1 PUBLISH this is the corresponding PUBACK; in the case of QO2 it is PUBCOMP. For SUBSCRIBE or UNSUBSCRIBE it is the corresponding SUBACK or UNSUBACK.
// [MQTT-2.3.1-6] A PUBACK, PUBREC or PUBREL Packet MUST contain the same Packet Identifier as the PUBLISH Packet that was originally sent.

[MQTT-2.3.1-7]

Similarly to [MQTT-2.3.1-6], SUBACK and UNSUBACK MUST contain the Packet Identifier that was used in the corresponding SUBSCRIBE and UNSUBSCRIBE Packet respectively

[MQTT-3.1.0-1]

After a Network Connection is established by a Client to a Server, the first Packet sent from the Client to the Server MUST be a CONNECT Packet.

[MQTT-3.1.0-2]

The Server MUST process a second CONNECT Packet sent from a Client as a protocol violation and disconnect the Client.

[MQTT-3.1.2-1].

If the protocol name is incorrect the Server MAY disconnect the Client, or it MAY continue processing the CONNECT packet in accordance with some other specification. In the latter case, the Server MUST NOT continue to process the CONNECT packet in line with this specification

[MQTT-3.1.2-2]

The Server MUST respond to the CONNECT Packet with a CONNACK return code 0x01 (unacceptable protocol level) and then disconnect the Client if the Protocol Level is not supported by the Server.

[MQTT-3.1.2-3]

The Server MUST validate that the reserved flag in the CONNECT Control Packet is set to zero and disconnect the Client if it is not zero.

[MQTT-3.1.2-4]

If CleanSession is set to 0, the Server MUST resume communications with the Client based on state from the current Session (as identified by the Client identifier). If there is no Session associated with the Client identifier the Server MUST create a new Session. The Client and Server MUST store the Session after the Client and Server are disconnected.

[MQTT-3.1.2-5]

After the disconnection of a Session that had CleanSession set to 0, the Server MUST store further QoS 1 and QoS 2 messages that match any subscriptions that the client had at the time of disconnection as part of the Session state.

[MQTT-3.1.2-6]

If CleanSession is set to 1, the Client and Server MUST discard any previous Session and start a new one. This Session lasts as long as the Network Connection. State data associated with this Session MUST NOT be reused in any subsequent Session

[MQTT-3.1.2.7]

Retained messages do not form part of the Session state in the Server, they MUST NOT be deleted when the Session ends.

[MQTT-3.1.2-8]

If the Will Flag is set to 1 this indicates that, if the Connect request is accepted,a Will Message MUST be stored on the Server and associated with the Network Connection. The Will Message MUST be published when the Network Connection is subsequently closed unless the Will Message has been deleted by the Server on receipt of a DISCONNECT Packet

[MQTT-3.1.2-9]

If the Will Flag is set to 1, the Will QoS and Will Retain fields in the Connect Flags will be used by the Server, and the Will Topic and Will Message fields MUST be present in the payload.

[MQTT-3.1.2-10]

The Will Message MUST be removed from the stored Session state in the Server once it has been published or the Server has received a DISCONNECT packet from the Client.

[MQTT-3.1.2-11]

If the Will Flag is set to 0 the Will QoS and Will Retain fields in the Connect Flags MUST be set to zero and the Will Topic and Will Message fields MUST NOT be present in the payload

[MQTT-3.1.2-12]

If the Will Flag is set to 0, a Will Message MUST NOT be published when this Network Connection ends.

[MQTT-3.1.2-13]

If the Will Flag is set to 0, then the Will QoS MUST be set to 0 (0x00).

[MQTT-3.1.2-14]

If the Will Flag is set to 1, the value of Will QoS can be 0 (0x00), 1 (0x01), or 2 (0x02). It MUST NOT be 3 (0x03).

[MQTT-3.1.2-15]

If the Will Flag is set to 0, then the Will Retain Flag MUST be set to 0.

[MQTT-3.1.2-16]

If the Will Flag is set to 1 and If Will Retain is set to 0, the Server MUST publish the Will Message as a non-retained message.

[MQTT-3.1.2-17]

If the Will Flag is set to 1 and If Will Retain is set to 1, the Server MUST publish the Will Message as a retained message.



[MQTT-3.1.2-18]

If the User Name Flag is set to 0, a user name MUST NOT be present in the payload.

[MQTT-3.1.2-19]

If the User Name Flag is set to 1, a user name MUST be present in the payload.

[MQTT-3.1.2-20]

If the Password Flag is set to 0, a password MUST NOT be present in the payload.

[MQTT-3.1.2-21]

If the Password Flag is set to 1, a password MUST be present in the payload.

[MQTT-3.1.2-22]

If the User Name Flag is set to 0 then the Password Flag MUST be set to 0.

[MQTT-3.1.2-23]

It is the responsibility of the Client to ensure that the interval between Control Packets being sent does not exceed the Keep Alive value .In the absence of sending any other Control Packets, the Client MUST send a PINGREQ Packet.

[MQTT-3.1.2-24]

If the Keep Alive value is non-zero and the Server does not receive a Control Packet from the Client within one and a half times the Keep Alive time period, it MUST disconnect the Network Connection to the Client as if the network had failed.

[MQTT-3.1.3-1]

These fields, if present, MUST appear in the order Client Identifier, Will Topic, Will Message, User Name, Password.

[MQTT-3.1.3-2]

The ClientId MUST be used by Clients and by Servers to identify state that they hold relating to this MQTT connection between the Client and the Server

[MQTT-3.1.3-3]

The Client Identifier (ClientId) MUST be present and MUST be the first field in the CONNECT packet payload.

[MQTT-3.1.3-4]

The ClientId MUST be a UTF-8 encoded string as defined in Section 1.5.3..

[MQTT-3.1.3-5]

The Server MUST allow ClientIds which are between 1 and 23 UTF-8 encoded bytes in length, and that contain only the characters

"0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

[MQTT-3.1.3-6]

A Server MAY allow a Client to supply a ClientId that has a length of zero bytes. However if it does so the Server MUST treat this as a special case and assign a unique ClientId to that Client. It MUST then process the CONNECT packet as if the Client had provided that unique ClientId.

[MQTT-3.1.3-7]

If the Client supplies a zero-byte ClientId, the Client MUST also set CleanSession to 1.

[MQTT-3.1.3-8]

If the Client supplies a zero-byte ClientId with CleanSession set to 0, the Server MUST respond to the CONNECT Packet with a CONNACK return code 0x02 (Identifier rejected) and then close the Network Connection.

[MQTT-3.1.3-9]

If the Server rejects the ClientId it MUST respond to the CONNECT Packet with a CONNACK return code 0x02 (Identifier rejected) and then close the Network Connection.

[MQTT-3.1.3-10]

The WillTopic MUST be a UTF-8 encoded string as defined in Section ‎1.5.3.

[MQTT-3.1.3-11]

User Name MUST be a UTF-8 encoded string as defined in Section ‎1.5.3.

[MQTT-3.1.4-1]

The Server MUST validate that the CONNECT Packet conforms to section 3.1 and close the Network Connection without sending a CONNACK if it does not conform.

[MQTT-3.1.4-2]

If the ClientId represents a Client already connected to the Server then the Server MUST disconnect the existing Client.

[MQTT-3.1.4-3]

If CONNECT validation is successful the Server MUST perform the processing of CleanSession MUST that is described in section 3.1.2.4.

[MQTT-3.1.4-4]

If CONNECT validation is successful the Server MUST acknowledge the CONNECT Packet with a CONNACK Packet containing a zero return code

[MQTT-3.1.4-5]

If the Server rejects the CONNECT, it MUST NOT process any data sent by the Client after the CONNECT Packet.

[MQTT-3.2.0-1]

The first packet sent from the Server to the Client MUST be a CONNACK Packet.

[MQTT-3.2.2-1]

If the Server accepts a connection with CleanSession set to 1, the Server MUST set Session Present to 0 in the CONNACK packet in addition to setting a zero return code in the CONNACK packet.

[MQTT-3.2.2-2]

If the Server accepts a connection with CleanSession set to 0, the value set in Session Present depends on whether the Server already has stored Session state for the supplied client ID. If the Server has stored Session state, it MUST set Session Present to 1 in the CONNACK packet.

[MQTT-3.2.2-3]

If the Server does not have stored Session state, it MUST set Session Present to 0 in the CONNACK packet. This is in addition to setting a zero return code in the CONNACK packet.

[MQTT-3.2.2-4]

If a server sends a CONNACK packet containing a non-zero return code it MUST set Session Present to 0.

[MQTT-3.2.2-5]

If a server sends a CONNACK packet containing a non-zero return code it MUST then close the Network Connection.

[MQTT-3.2.2-6]

If none of the return codes listed in Table 3.1 – Connect Return code valuesare deemed applicable, then the Server MUST close the Network Connection without sending a CONNACK.

[MQTT-3.3.2-1]

The Topic Name MUST be present as the first field in the PUBLISH Packet Variable header. It MUST be a UTF-8 encoded string.

[MQTT-3.3.2-2]

The Topic Name in the PUBLISH Packet MUST NOT contain wildcard characters.

[MQTT-3.3.1-1]

The DUP flag MUST be set to 1 by the Client or Server when it attempts to re-deliver a PUBLISH Packet.

[MQTT-3.3.1-2]

The DUP flag MUST be set to 0 for all QoS 0 messages

[MQTT-3.3.1-3]

The value of the DUP flag from an incoming PUBLISH packet is not propagated when the PUBLISH Packet is sent to subscribers by the Server. The DUP flag in the outgoing PUBLISH packet is set independently to the incoming PUBLISH packet, its value MUST be determined solely by whether the outgoing PUBLISH packet is a retransmission.

[MQTT-3.3.1-4]

A PUBLISH Packet MUST NOT have both QoS bits set to 1. If a Server or Client receives a PUBLISH Packet which has both QoS bits set to 1 it MUST close the Network Connection

[MQTT-3.3.1-5]

If the RETAIN flag is set to 1, in a PUBLISH Packet sent by a Client to a Server, the Server MUST store the Application Message and its QoS, so that it can be delivered to future subscribers whose subscriptions match its topic name.

[MQTT-3.3.1-6]

When a new subscription is established, the last retained message, if any, on each matching topic name MUST be sent to the subscriber.

[MQTT-3.3.1-7]

If the Server receives a QoS 0 message with the RETAIN flag set to 1 it MUST discard any message previously retained for that topic. It SHOULD store the new QoS 0 message as the new retained message for that topic, but MAY choose to discard it at any time - if this happens there will be no retained message for that topic.

[MQTT-3.3.1-8]

When sending a PUBLISH Packet to a Client the Server MUST set the RETAIN flag to 1 if a message is sent as a result of a new subscription being made by a Client.

[MQTT-3.3.1-9]

It MUST set the RETAIN flag to 0 when a PUBLISH Packet is sent to a Client because it matches an established subscription regardless of how the flag was set in the message it received

[MQTT-3.3.1-10]

A PUBLISH Packet with a RETAIN flag set to 1 and a payload containing zero bytes will be processed as normal by the Server and sent to Clients with a subscription matching the topic name. Additionally any existing retained message with the same topic name MUST be removed and any future subscribers for the topic will not receive a retained message.

[MQTT-3.3.1-11]

A zero byte retained message MUST NOT be stored as a retained message on the Server.

[MQTT-3.3.1-12]

If the RETAIN flag is 0, in a PUBLISH Packet sent by a Client to a Server, the Server MUST NOT store the message and MUST NOT remove or replace any existing retained message.

[MQTT-3-8.3-4]

The Server MUST treat a SUBSCRIBE packet as malformed and close the Network Connection if any of Reserved bits in the payload are non-zero, or QoS is not 0,1 or 2.

[MQTT-3.8.4-1]

When the Server receives a SUBSCRIBE Packet from a Client, the Server MUST respond with a SUBACK Packet.

[MQTT-3.8.4-2]

The SUBACK Packet MUST have the same Packet Identifier as the SUBSCRIBE Packet.

[MQTT-3.8.4-3]

A subscribe request which contains a Topic Filter that is identical to an existing Subscription’s Topic Filter completely replaces that existing Subscription with a new Subscription. The Topic Filter in the new Subscription will be identical to that in the previous Subscription, although its maximum QoS value could be different. Any existing retained messages matching the Topic Filter are re-sent, but the flow of publications is not interrupted.

[MQTT-3.8.4-4]

If a Server receives a SUBSCRIBE packet that contains multiple Topic Filters it MUST handle that packet as if it had received a sequence of multiple SUBSCRIBE packets, except that it combines their responses into a single SUBACK response.

[MQTT-3.8.4-5]

The SUBACK Packet sent by the Server to the Client MUST contain a return code for each Topic Filter/QoS pair. This return code MUST either show the maximum QoS that was granted for that Subscription or indicate that the subscription failed.

[MQTT-3.8.4-6]

The Server might grant a lower maximum QoS than the subscriber requested. The QoS of Payload Messages sent in response to a Subscription MUST be the minimum of the QoS of the originally published message and the maximum QoS granted by the Server. The server is permitted to send duplicate copies of a message to a subscriber in the case where the original message was published with QoS 1 and the maximum QoS granted was QoS 0.

[MQTT-3.9.3-1]

The order of return codes in the SUBACK Packet MUST match the order of Topic Filters in the SUBSCRIBE Packet.

[MQTT-3.9.3-2]

SUBACK return codes other than 0x00, 0x01, 0x02 and 0x80 are reserved and MUST NOT be used.

[MQTT-3.10.1-1]

Bits 3,2,1 and 0 of the fixed header of the UNSUBSCRIBE Control Packet are reserved and MUST be set to 0,0,1 and 0 respectively. The Server MUST treat any other value as malformed and close the Network Connection.

[MQTT-3.10.3-1

The Topic Filters in an UNSUBSCRIBE packet MUST be UTF-8 encoded strings as defined in Section 1.5.3, packed contiguously

[MQTT-3.10.3-2]

The Payload of an UNSUBSCRIBE packet MUST contain at least one Topic Filter. An UNSUBSCRIBE packet with no payload is a protocol violation.

[MQTT-3.10.4-1]

The Topic Filter (whether containing a wild-card or not) supplied in an UNSUBSCRIBE packet MUST be compared byte-for-byte with the current set of Topic Filters held by the Server for the Client. If any filter matches exactly then it is deleted, otherwise no additional processing occurs.

[MQTT-3.10.4-2]

The Server sends an UNSUBACK Packet to the Client in response to an UNSUBSCRIBE Packet, The Server MUST stop adding any new messages for delivery to the Client.

[MQTT-3.10.4-3]

The Server sends an UNSUBACK Packet to the Client in response to an UNSUBSCRIBE Packet, The Server MUST complete the delivery of any QoS 1 or QoS 2 messages which it has started to send to the Client.

[MQTT-3.10.4-4]

The Server sends an UNSUBACK Packet to the Client in response to an UNSUBSCRIBE Packet, The Server MUST send an UNSUBACK packet. The UNSUBACK Packet MUST have the same Packet Identifier as the UNSUBSCRIBE Packet.

[MQTT-3.10.4-5]

Even where no Topic Filters are deleted, the Server MUST respond with an UNSUBACK.

[MQTT-3.10.4-6]

If a Server receives an UNSUBSCRIBE packet that contains multiple Topic Filters it MUST handle that packet as if it had received a sequence of multiple UNSUBSCRIBE packets, except that it sends just one UNSUBACK response.

[MQTT-3.12.4-1]

The Server MUST send a PINGRESP Packet in response to a PINGREQ packet.

[MQTT-3.14.1-1]

The Server MUST validate that reserved bits are set to zero in DISCONNECT Control Packet, and disconnect the Client if they are not zero.

[MQTT-3.14.4-1]

After sending a DISCONNECT Packet the Client MUST close the Network Connection.

[MQTT-3.14.4-2]

After sending a DISCONNECT Packet the Client MUST NOT send any more Control Packets on that Network Connection.

[MQTT-3.14.4-3]

On receipt of DISCONNECT the Server MUST discard any Will Message associated with the current connection without publishing it, as described in Section 3.1.2.5

[MQTT-4.1.0.1]

The Client and Server MUST store Session state for the entire duration of the Session.

[MQTT-4.1.0-2]

A Session MUST last at least as long it has an active Network Connection.

[MQTT-4.3.1.1]



In the QoS 0 delivery protocol, the Sender

·         MUST send a PUBLISH packet with QoS=0, DUP=0.

[MQTT-4.3.2.1]



In the QoS 1 delivery protocol, the Sender

·         MUST assign an unused Packet Identifier each time it has a new Application Message to publish.

·         MUST send a PUBLISH Packet containing this Packet Identifier with QoS=1, DUP=0.

·         MUST treat the PUBLISH Packet as "unacknowledged" until it has received the corresponding PUBACK packet from the receiver. See Section 4.4 for a discussion of unacknowledged messages.

[MQTT-4.3.2.2]



In the QoS 1 delivery protocol, the Receiver

MUST respond with a PUBACK Packet containing the Packet Identifier from the incoming PUBLISH Packet, having accepted ownership of the Application Message
After it has sent a PUBACK Packet the Receiver MUST treat any incoming PUBLISH packet that contains the same Packet Identifier as being a new publication, irrespective of the setting of its DUP flag.
[MQTT-4.3.3-1]



In the QoS 2 delivery protocol, the Sender

MUST assign an unused Packet Identifier when it has a new Application Message to publish.
MUST send a PUBLISH packet containing this Packet Identifier with QoS=2, DUP=0.
MUST treat the PUBLISH packet as "unacknowledged" until it has received the corresponding PUBREC packet from the receiver. See Section 4.4 for a discussion of unacknowledged messages.
MUST send a PUBREL packet when it receives a PUBREC packet from the receiver. This PUBREL packet MUST contain the same Packet Identifier as the original PUBLISH packet.
MUST treat the PUBREL packet as "unacknowledged" until it has received the corresponding PUBCOMP packet from the receiver.
MUST NOT re-send the PUBLISH once it has sent the corresponding PUBREL packet.


[MQTT-4.3.3-2]



In the QoS 2 delivery protocol, the Receiver

MUST respond with a PUBREC containing the Packet Identifier from the incoming PUBLISH Packet, having accepted ownership of the Application Message.
Until it has received the corresponding PUBREL packet, the Receiver MUST acknowledge any subsequent PUBLISH packet with the same Packet Identifier by sending a PUBREC. It MUST NOT cause duplicate messages to be delivered to any onward recipients in this case.
MUST respond to a PUBREL packet by sending a PUBCOMP packet containing the same Packet Identifier as the PUBREL.
After it has sent a PUBCOMP, the receiver MUST treat any subsequent PUBLISH packet that contains that Packet Identifier as being a new publication.
[MQTT-4.4.0-1]

When a Client reconnects with CleanSession set to 0, both the Client and Server MUST re-send any unacknowledged PUBLISH Packets (where QoS > 0) and PUBREL Packets using their original Packet Identifiers.

[MQTT-4.5.0-1]

When a Server takes ownership of an incoming Application Message it MUST add it to the Session state of those clients that have matching Subscriptions. Matching rules are defined in Section ‎4.7.

[MQTT-4.5.0-2]

The Client MUST acknowledge any Publish Packet it receives according to the applicable QoS rules regardless of whether it elects to process the Application Message that it contains.

[MQTT-4.6.0-1]

When it re-sends any PUBLISH packets, it MUST re-send them in the order in which the original PUBLISH packets were sent (this applies to QoS 1 and QoS 2 messages).

[MQTT-4.6.0-2]

Client MUST send PUBACK packets in the order in which the corresponding PUBLISH packets were received (QoS 1 messages).

[MQTT-4.6.0-3]

Client MUST send PUBREC packets in the order in which the corresponding PUBLISH packets were received (QoS 2 messages).

[MQTT-4.6.0-4]

Client MUST send PUBREL packets in the order in which the corresponding PUBREC packets were received (QoS 2 messages).

[MQTT-4.6.0-5]

A Server MUST by default treat each Topic as an "Ordered Topic". It MAY provide an administrative or other mechanism to allow one or more Topics to be treated as an "Unordered Topic".

[MQTT-4.6.0-6]

When a Server processes a message that has been published to an Ordered Topic, it MUST follow the rules listed above when delivering messages to each of its subscribers. In addition it MUST send PUBLISH packets to consumers (for the same Topic and QoS) in the order that they were received from any given Client.

[MQTT-4.7.1-1]

The wildcard characters can be used in Topic Filters, but MUST NOT be used within a Topic Name.

[MQTT-4.7.1-2]

The multi-level wildcard character MUST be specified either on its own or following a topic level separator. In either case it MUST be the last character specified in the Topic Filter.

[MQTT-4.7.1-3]

The single-level wildcard can be used at any level in the Topic Filter, including first and last levels. Where it is used it MUST occupy an entire level of the filter.

[MQTT-4.7.2-1]

The Server MUST NOT match Topic Filters starting with a wildcard character (# or +) with Topic Names beginning with a $ character.

[MQTT-4.7.3-1]

All Topic Names and Topic Filters MUST be at least one character long.

[MQTT-4.7.3-2]

Topic Names and Topic Filters MUST NOT include the null character (Unicode U+0000).

[MQTT-4.7.3-3]

Topic Names and Topic Filters are UTF-8 encoded strings, they MUST NOT encode to more than 65535 bytes.

[MQTT-4.7.3-4]

When it performs subscription matching the Server MUST NOT perform any normalization of Topic Names or Topic Filters, or any modification or substitution of unrecognized characters

[MQTT-4.8.0-1]

Unless stated otherwise, if either the Server or Client encounters a protocol violation, it MUST close the Network Connection on which it received that Control Packet which caused the protocol violation.

[MQTT-4.8.0-2]

If the Client or Server encounters a Transient Error while processing an inbound Control Packet it MUST close the Network Connection on which it received that Control Packet.

[MQTT-6.0.0.1]

MQTT Control Packets MUST be sent in WebSocket binary data frames. If any other type of data frame is received the recipient MUST close the Network Connection.

[MQTT-6.0.0.2]

A single WebSocket data frame can contain multiple or partial MQTT Control Packets. The receiver MUST NOT assume that MQTT Control Packets are aligned on WebSocket frame boundaries].

[MQTT-6.0.0.3]

The client MUST include “mqtt” in the list of WebSocket Sub Protocols it offers.

[MQTT-6.0.0.4]

The WebSocket Sub Protocol name selected and returned by the server MUST be “mqtt”.

[MQTT-7.0.0-1]

A Server that both accepts inbound connections and establishes outbound connections to other Servers MUST conform as both an MQTT Client and MQTT Server.

[MQTT-7.0.0-2]

Conformant implementations MUST NOT require the use of any extensions defined outside of this specification in order to interoperate with any other conformant implementation.

[MQTT-7.1.1-1]

A conformant Server MUST support the use of one or more underlying transport protocols that provide an ordered, lossless, stream of bytes from the Client to Server and Server to Client.

[MQTT-7.1.2-1]

A conformant Client MUST support the use of one or more underlying transport protocols that provide an ordered, lossless, stream of bytes from the Client to Server and Server to Client.
*/
