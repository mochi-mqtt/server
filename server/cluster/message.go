package cluster

import "encoding/json"

const (
	// 0~20 corresponds to mqtt control package types
	MqttPublish = 3
	// 21~
)

type Message struct {
	Type	byte    `json:"type"`
	Data    []byte `json:"data"`
}

func (m *Message) Bytes() []byte {
	data, err := json.Marshal(m)
	if err != nil {
		return []byte("")
	}

	return data
}

func (m *Message) Load(data []byte) error {
	if err := json.Unmarshal(data, m); err != nil {
		return err
	}

	return nil
}
