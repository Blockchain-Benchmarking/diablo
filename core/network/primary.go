package network

import (
	"bufio"
	"diablo/core/messaging"
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

func (p *PrimaryConn) Send(msg messaging.Message) error {
	return SendMessage(p.writer, msg)
}

func (p *PrimaryConn) Read() (messaging.Message, error) {
	return ReadMessage(p.reader)
}

func (p *PrimaryConn) Writer() *bufio.Writer {
	return p.writer
}

func (p *PrimaryConn) Reader() *bufio.Reader {
	return p.reader
}

func (p *PrimaryConn) LocalAddr() string {
	return p.conn.LocalAddr().String()
}
