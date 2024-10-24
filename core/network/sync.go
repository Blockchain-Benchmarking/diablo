package network

import (
	"encoding/binary"
	"errors"
	"fmt"
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

type MsgPrimaryParameters struct {
	Sysname     string
	ChainParams map[string]string
	MaxDelay    float64
	MaxSkew     float64
}

func DecodeMsgPrimaryParameters(src io.Reader) (*MsgPrimaryParameters, error) {
	var buf []byte = make([]byte, 255)
	var this MsgPrimaryParameters
	var key, value string
	var err error
	var i, n, k, v int

	_, err = io.ReadFull(src, buf[:1])
	if err != nil {
		return nil, err
	}

	n = int(buf[0])

	_, err = io.ReadFull(src, buf[:n])
	if err != nil {
		return nil, err
	}

	this.Sysname = string(buf[:n])

	_, err = io.ReadFull(src, buf[:1])
	if err != nil {
		return nil, err
	}

	n = int(buf[0])

	this.ChainParams = make(map[string]string, n)

	for i = 0; i < n; i++ {
		_, err = io.ReadFull(src, buf[:2])
		if err != nil {
			return nil, err
		}

		k = int(buf[0])
		v = int(buf[1])

		_, err = io.ReadFull(src, buf[:k])
		if err != nil {
			return nil, err
		}

		key = string(buf[:k])

		_, err = io.ReadFull(src, buf[:v])
		if err != nil {
			return nil, err
		}

		value = string(buf[:v])

		this.ChainParams[key] = value
	}

	err = binary.Read(src, binary.LittleEndian, &this.MaxDelay)
	if err != nil {
		return nil, err
	}

	err = binary.Read(src, binary.LittleEndian, &this.MaxSkew)
	if err != nil {
		return nil, err
	}

	return &this, nil
}

func (m *MsgPrimaryParameters) Encode(dest io.Writer) error {
	var key, value string
	var buf []byte
	var err error

	if len(m.Sysname) > 255 {
		return fmt.Errorf("interface name '%s' too long (%d bytes)",
			m.Sysname, len(m.Sysname))
	}

	if len(m.ChainParams) > 255 {
		return fmt.Errorf("too many chain parameters (%d)",
			len(m.ChainParams))
	}

	for key, value = range m.ChainParams {
		if len(key) > 255 {
			return fmt.Errorf("chain parameter name '%s' too "+
				"long (%d bytes)", key, len(key))
		}

		if len(value) > 255 {
			return fmt.Errorf("chain parameter value '%s' too "+
				"long (%d bytes)", value, len(value))
		}
	}

	buf = make([]byte, 2)

	buf[0] = uint8(len(m.Sysname))
	_, err = dest.Write(buf[:1])
	if err != nil {
		return err
	}

	_, err = io.WriteString(dest, m.Sysname)
	if err != nil {
		return err
	}

	buf[0] = uint8(len(m.ChainParams))
	_, err = dest.Write(buf[:1])
	if err != nil {
		return err
	}

	for key, value = range m.ChainParams {
		buf[0] = uint8(len(key))
		buf[1] = uint8(len(value))
		_, err = dest.Write(buf)
		if err != nil {
			return err
		}

		_, err = io.WriteString(dest, key)
		if err != nil {
			return err
		}

		_, err = io.WriteString(dest, value)
		if err != nil {
			return err
		}
	}

	err = binary.Write(dest, binary.LittleEndian, m.MaxDelay)
	if err != nil {
		return err
	}

	err = binary.Write(dest, binary.LittleEndian, m.MaxSkew)
	if err != nil {
		return err
	}

	return nil
}

type MsgSecondaryParameters struct {
	Tags []string
}

func DecodeMsgSecondaryParameters(src io.Reader) (*MsgSecondaryParameters, error) {
	var buf []byte = make([]byte, 255)
	var this MsgSecondaryParameters
	var err error
	var i, n int

	_, err = io.ReadFull(src, buf[:1])
	if err != nil {
		return nil, err
	}

	this.Tags = make([]string, int(buf[0]))

	for i = range this.Tags {
		_, err = io.ReadFull(src, buf[:1])
		if err != nil {
			return nil, err
		}

		n = int(buf[0])

		_, err = io.ReadFull(src, buf[:n])
		if err != nil {
			return nil, err
		}

		this.Tags[i] = string(buf[:n])
	}

	return &this, nil
}

func (m *MsgSecondaryParameters) Encode(dest io.Writer) error {
	var buf []byte
	var tag string
	var err error

	if len(m.Tags) > 255 {
		return fmt.Errorf("too many tags (%d)", len(m.Tags))
	}

	for _, tag = range m.Tags {
		if len(tag) > 255 {
			return fmt.Errorf("tag '%s' too long (%d bytes)", tag,
				len(tag))
		}
	}

	buf = make([]byte, 1)

	buf[0] = uint8(len(m.Tags))
	_, err = dest.Write(buf)
	if err != nil {
		return err
	}

	for _, tag = range m.Tags {
		buf[0] = uint8(len(tag))
		_, err = dest.Write(buf)
		if err != nil {
			return err
		}

		_, err = io.WriteString(dest, tag)
		if err != nil {
			return err
		}
	}

	return nil
}
