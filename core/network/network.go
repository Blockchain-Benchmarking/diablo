package network

import (
	"bufio"
	"diablo/core/messaging"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

var ErrTimeout = errors.New("timeout")

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

/*func ReceiveWithTimeout(src *bufio.Reader, timeout time.Duration) (messaging.Message, error) {
	lengthCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var msg messaging.Message
	done := make(chan error, 1)

	go func() {
		m, err := ReadMessage(src)
		msg = m
		done <- err
	}()

	select {
	case <-lengthCtx.Done():
		return nil, ErrTimeout
	case err := <-done:
		if err != nil {
			return nil, fmt.Errorf("failed to read message length: %w", err)
		}
	}

	return msg, nil
}*/

func ReadMessageWithTimeout(conn net.Conn, timeout time.Duration) (messaging.Message, error) {
	var length int32

	if timeout != 0 {
		err := conn.SetReadDeadline(time.Now().Add(timeout))
		if err != nil {
			return nil, fmt.Errorf("failed to set read deadline: %w", err)
		}
	}

	src := bufio.NewReader(conn)

	//if timeout == 0 {
	err := binary.Read(src, binary.LittleEndian, &length)
	if err != nil {
		return nil, fmt.Errorf("failed to read message length: %w", err)
	}
	/**} else {
		lengthCtx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		done := make(chan error, 1)

		go func() {
			done <- binary.Read(src, binary.LittleEndian, &length)
		}()

		select {
		case <-lengthCtx.Done():
			return nil, ErrTimeout
		case err := <-done:
			if err != nil {
				return nil, fmt.Errorf("failed to read message length: %w", err)
			}
		}
	}
	*/

	if timeout != 0 {
		err = conn.SetReadDeadline(time.Time{})
		if err != nil {
			return nil, fmt.Errorf("failed to reset read deadline: %w", err)
		}
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
