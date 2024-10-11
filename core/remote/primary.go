package remote

import (
	"bufio"
	"fmt"
	"net"
)

type PrimaryConn struct {
	conn   net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
}

func NewPrimaryConn(conn net.Conn) *PrimaryConn {
	return &PrimaryConn{
		conn:   conn,
		reader: bufio.NewReader(conn),
		writer: bufio.NewWriter(conn),
	}
}

func (p *PrimaryConn) Init(fromSecondary *MsgSecondaryParameters) (*MsgPrimaryParameters, error) {
	var fromPrimary *MsgPrimaryParameters
	var err error

	fromPrimary, err = DecodeMsgPrimaryParameters(p.reader)
	if err != nil {
		return nil, err
	}

	err = fromSecondary.Encode(p.writer)
	if err != nil {
		return nil, err
	}

	err = p.writer.Flush()
	if err != nil {
		return nil, err
	}

	return fromPrimary, nil
}

func (p *PrimaryConn) Writer() *bufio.Writer {
	return p.writer
}

func (p *PrimaryConn) Reader() *bufio.Reader {
	return p.reader
}

func (p *PrimaryConn) WaitPrepare() (*MsgPrepareDone, error) {
	message, err := DecodeMsgPrepareDone(p.reader)
	if err != nil {
		return nil, fmt.Errorf("failed to decode prepare done message: %w", err)
	}

	return message, nil
}

func (p *PrimaryConn) SyncReady() error {
	var err error

	err = (&MsgPrepareDone{
		Ready: true,
	}).Encode(p.writer)
	if err != nil {
		return err
	}

	return p.writer.Flush()
}

func (p *PrimaryConn) WaitStart() (*MsgStart, error) {
	return DecodeMsgStart(p.reader)
}

func (p *PrimaryConn) Close() error {
	return p.conn.Close()
}
