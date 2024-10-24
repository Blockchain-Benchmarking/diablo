package workload

import (
	"diablo/core/behavior"
	"diablo/core/network"
	"diablo/core/payment"
	"encoding/binary"
	"io"
	"time"
)

var Workloads = map[string]Tuple{
	"simple": {
		NewSimpleCoordinator,
		NewSimpleGenerator,
	},

	"dynamic:": {
		NewDynamicCoordinator,
		NewDynamicGenerator,
	},
}

type UserTools struct {
	Init         func(implementation string, config behavior.Config, params map[string]interface{}) (behavior.User, error)
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
	NewCoordinator func(secondaries []*network.Secondary, accounts []behavior.Account, user string, blockchain string, params map[string]interface{}) Coordinator
	NewGenerator   func(primary *network.PrimaryConn, wk Workload, timeout time.Duration) (Generator, error)
}

type Generator interface {
	Start() error
	CollectResults() (behavior.Results, error)
}

type Coordinator interface {
	SendWorkload() error
	CollectResults() behavior.Results
}

type Workload interface {
	Encode(dest io.Writer) error
	Decode(src io.Reader) error
}

func encodeString(dest io.Writer, str string) error {
	// First, write the length of the string as int32
	length := int32(len(str))
	if err := binary.Write(dest, binary.LittleEndian, length); err != nil {
		return err
	}

	// Then, write the string bytes
	if _, err := dest.Write([]byte(str)); err != nil {
		return err
	}

	return nil
}

func decodeString(src io.Reader) (string, error) {
	// Read the length of the string
	var length int32
	if err := binary.Read(src, binary.LittleEndian, &length); err != nil {
		return "", err
	}

	// Read the string bytes
	buf := make([]byte, length)
	if _, err := io.ReadFull(src, buf); err != nil {
		return "", err
	}

	return string(buf), nil
}
