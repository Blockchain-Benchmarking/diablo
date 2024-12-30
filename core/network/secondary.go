package network

import (
	"bufio"
	"diablo/core/logging"
	"diablo/core/messaging"
	"fmt"
	"net"
	"time"
)

type Secondary struct {
	conn   *secondaryConn
	params *messaging.SecondaryInit
}

func NewRemoteSecondary(conn net.Conn, duration time.Duration) (*Secondary, error) {
	var secondary Secondary

	secondary.conn = newSecondaryConn(conn)

	err := secondary.Send(messaging.PrimaryInit{Duration: duration.String()})
	if err != nil {
		return nil, fmt.Errorf("failed to send primary init message: %w", err)
	}

	//Wait for secondary to confirm init
	msg, err := secondary.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read secondary init message: %w", err)
	}

	if msg.Type() != messaging.SecondaryInitType {
		return nil, fmt.Errorf("unexpected message type from secondary, expected %s, got %s", messaging.SecondaryInitType, msg.Type())
	}

	logging.Debugf("received init message from secondary")
	secondary.params = msg.(*messaging.SecondaryInit)

	return &secondary, nil
}

func (s *Secondary) Send(msg messaging.Message) error {
	return SendMessage(s.conn.writer, msg)
}

func (s *Secondary) Read() (messaging.Message, error) {
	return ReadMessage(s.Reader())
}

func (s *Secondary) Tags() []string {
	return s.params.Tags
}

func (s *Secondary) Addr() string {
	return s.conn.addr()
}

func (s *Secondary) Close() error {
	return s.conn.Close()
}

func (s *Secondary) Conn() net.Conn {
	return s.conn.conn
}

func (s *Secondary) Reader() *bufio.Reader {
	return s.conn.reader
}

func (s *Secondary) Writer() *bufio.Writer { return s.conn.writer }

type secondaryConn struct {
	conn   net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
}

func newSecondaryConn(conn net.Conn) *secondaryConn {
	return &secondaryConn{
		conn:   conn,
		reader: bufio.NewReader(conn),
		writer: bufio.NewWriter(conn),
	}
}

func (s *secondaryConn) addr() string {
	return s.conn.RemoteAddr().String()
}

func (s *secondaryConn) Close() error {
	return s.conn.Close()
}
