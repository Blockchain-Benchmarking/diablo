package messaging

import (
	"encoding/binary"
	"errors"
	"io"
)

type MsgPrepareDone struct {
	Ready bool
}

func DecodeMsgPrepareDone(src io.Reader) (*MsgPrepareDone, error) {
	var this MsgPrepareDone
	var readyByte byte

	err := binary.Read(src, binary.LittleEndian, &readyByte)
	if err != nil {
		return nil, err
	}

	if readyByte == 1 {
		this.Ready = true
	} else if readyByte == 0 {
		this.Ready = false
	} else {
		return nil, errors.New("invalid value for Ready field")
	}

	return &this, nil
}

func (m *MsgPrepareDone) Encode(dest io.Writer) error {
	var readyByte byte
	if m.Ready {
		readyByte = 1
	} else {
		readyByte = 0
	}

	return binary.Write(dest, binary.LittleEndian, readyByte)
}

type MsgStart struct {
	Duration float64
}

func DecodeMsgStart(src io.Reader) (*MsgStart, error) {
	var this MsgStart
	var err error

	err = binary.Read(src, binary.LittleEndian, &this.Duration)
	if err != nil {
		return nil, err
	}

	return &this, nil
}

func (m *MsgStart) Encode(dest io.Writer) error {
	return binary.Write(dest, binary.LittleEndian, m.Duration)
}
