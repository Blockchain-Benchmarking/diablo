package network

import (
	"diablo/core/messaging"
	"encoding/json"
	"fmt"
)

type Packet struct {
	Type    string
	Payload json.RawMessage
}

func (p *Packet) Marshal() ([]byte, error) {
	return json.Marshal(p.Payload)
}

func Unmarshal(data []byte) (messaging.Message, error) {
	p := &Packet{}
	err := json.Unmarshal(data, p)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal packet: %w", err)
	}

	m, ok := messaging.Messages[p.Type]
	if !ok {
		return nil, fmt.Errorf("unknown message type %q", p.Type)
	}

	msg := m.Empty()
	err = json.Unmarshal(p.Payload, &msg)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	return msg, nil
}
