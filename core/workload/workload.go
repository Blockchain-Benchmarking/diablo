package workload

import (
	"bufio"
	"bytes"
	"diablo/core/behavior"
	"diablo/core/logging"
	"diablo/core/payment"
	"encoding/binary"
	"fmt"
	"io"
)

type UserTools struct {
	Init         func(implementation string, config behavior.Config, transactions int, params map[string]interface{}) (behavior.User, error)
	EmptyUser    func() behavior.User
	EmptyResults func() behavior.Results
}

var Users = map[string]UserTools{
	payment.STUBBORN_PAYMENT_USER: {
		Init:         payment.NewStubbornPaymentUser,
		EmptyUser:    payment.EmptyStubbornPaymentUser,
		EmptyResults: payment.EmptyStubbornPaymentResult,
	},
}

type Tuple struct {
	Coordinator
	Generator
}

type Workload []behavior.User

func (w *Workload) encode() ([]byte, error) {
	var buf bytes.Buffer

	err := encodeString(&buf, (*w)[0].Name())
	if err != nil {
		return nil, fmt.Errorf("failed to encode usertype: %w", err)
	}

	count := int32(len(*w))
	err = binary.Write(&buf, binary.LittleEndian, count)
	if err != nil {
		return nil, fmt.Errorf("failed to encode Workload count %d: %w", count, err)
	}

	for _, u := range *w {
		err = u.Encode(&buf)
		if err != nil {
			return nil, fmt.Errorf("failed to encode userInfo: %w", err)
		}
	}

	return buf.Bytes(), nil
}

func (w *Workload) Send(dest *bufio.Writer) error {
	buf, err := w.encode()
	if err != nil {
		return fmt.Errorf("failed to encode workload: %w", err)
	}

	_, err = dest.Write(buf)
	if err != nil {
		return fmt.Errorf("failed to write workload: %w", err)
	}

	err = dest.Flush()
	if err != nil {
		return fmt.Errorf("failed to flush writer: %w", err)
	}

	return nil
}

func (w *Workload) Receive(src *bufio.Reader) error {
	userType, err := decodeString(src)
	if err != nil {
		return fmt.Errorf("failed to decode usertype: %w", err)
	}

	tools, ok := Users[userType]
	if !ok {
		return fmt.Errorf("unknown user type: %s", userType)
	}

	var count int32
	err = binary.Read(src, binary.LittleEndian, &count)
	if err != nil {
		return err
	}

	logging.Infof("received %d %s users", count, userType)

	*w = make(Workload, count)

	for i := int32(0); i < count; i++ {
		userInfo := tools.EmptyUser()
		err = userInfo.Decode(src)
		if err != nil {
			return err
		}
		(*w)[i] = userInfo
	}

	return nil
}

func encodeString(dest io.Writer, str string) error {
	length := int32(len(str))
	if err := binary.Write(dest, binary.LittleEndian, length); err != nil {
		return err
	}

	if _, err := dest.Write([]byte(str)); err != nil {
		return err
	}

	return nil
}

func decodeString(src io.Reader) (string, error) {
	var length int32
	if err := binary.Read(src, binary.LittleEndian, &length); err != nil {
		return "", err
	}

	buf := make([]byte, length)
	if _, err := io.ReadFull(src, buf); err != nil {
		return "", err
	}

	return string(buf), nil
}
