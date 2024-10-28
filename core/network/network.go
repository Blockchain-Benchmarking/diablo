package network

import (
	"bufio"
	"diablo/core/messaging"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

func SendMessage(dst *bufio.Writer, msg messaging.Message) error {
	buf, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	pkt := Packet{
		Type:    msg.Type(),
		Payload: buf,
	}

	pktBytes, err := pkt.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal packet: %w", err)
	}

	pktLength := int32(len(pktBytes))
	err = binary.Write(dst, binary.LittleEndian, pktLength)
	if err != nil {
		return fmt.Errorf("failed to write packet length: %w", err)
	}

	_, err = dst.Write(pktBytes)
	if err != nil {
		return fmt.Errorf("failed to write packet: %w", err)
	}

	err = dst.Flush()
	if err != nil {
		return fmt.Errorf("failed to flush packet: %w", err)
	}

	return nil
}

func ReadMessage(src *bufio.Reader) (messaging.Message, error) {
	var length int32
	err := binary.Read(src, binary.LittleEndian, &length)
	if err != nil {
		return nil, fmt.Errorf("failed to read message length: %w", err)
	}

	packetBytes := make([]byte, length)
	_, err = io.ReadFull(src, packetBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to read full packet: %w", err)
	}

	msg, err := Unmarshal(packetBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal packet: %w", err)
	}

	return msg, nil
}
