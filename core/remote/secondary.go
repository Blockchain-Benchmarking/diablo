package remote

import (
	"bufio"
	"fmt"
	"net"
)

type Secondary struct {
	conn   *secondaryConn
	params *MsgSecondaryParameters
}

func NewRemoteSecondary(conn net.Conn, sysname string, params map[string]string) (*Secondary, error) {
	var secondary Secondary
	var err error

	secondary.conn = newSecondaryConn(conn)

	secondary.params, err = secondary.conn.init(&MsgPrimaryParameters{
		Sysname:     sysname,
		ChainParams: params,
	})

	if err != nil {
		return nil, err
	}

	secondary.params.Tags = append(secondary.params.Tags, secondary.Addr())

	return &secondary, nil
}

func (s *Secondary) Tags() []string {
	return s.params.Tags
}

func (s *Secondary) Ready() error {
	return s.conn.syncReady()
}

func (s *Secondary) Start(duration float64) error {
	return s.conn.sendStart(&MsgStart{
		Duration: duration,
	})
}

func (s *Secondary) Addr() string {
	return s.conn.addr()
}

func (s *Secondary) Close() error {
	return s.conn.Close()
}

func (s *Secondary) Writer() *bufio.Writer {
	return s.conn.writer
}

func (s *Secondary) Reader() *bufio.Reader {
	return s.conn.reader
}

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

func (s *secondaryConn) init(fromPrimary *MsgPrimaryParameters) (*MsgSecondaryParameters, error) {
	var err error

	err = fromPrimary.Encode(s.writer)
	if err != nil {
		return nil, err
	}

	err = s.writer.Flush()
	if err != nil {
		return nil, err
	}

	return DecodeMsgSecondaryParameters(s.reader)
}

func (s *secondaryConn) syncReady() error {
	var err error

	msg := &MsgPrepareDone{
		Ready: false,
	}
	err = msg.Encode(s.writer)
	if err != nil {
		return err
	}

	err = s.writer.Flush()
	if err != nil {
		return err
	}

	ready, err := DecodeMsgPrepareDone(s.reader)
	if err != nil {
		return err
	}

	if !ready.Ready {
		return fmt.Errorf("secondary %s is not ready", s.addr())
	}

	return nil
}

func (s *secondaryConn) sendStart(fromPrimary *MsgStart) error {
	var err error

	err = fromPrimary.Encode(s.writer)
	if err != nil {
		return err
	}

	return s.writer.Flush()
}

func (s *secondaryConn) addr() string {
	return s.conn.RemoteAddr().String()
}

func (s *secondaryConn) Close() error {
	return s.conn.Close()
}
