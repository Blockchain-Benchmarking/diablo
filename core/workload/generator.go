package workload

import (
	"diablo/core/remote"
	"diablo/core/result"
	"encoding/binary"
	"fmt"
	"github.com/ethereum/go-ethereum/ethclient"
	"io"
)

type Generator interface {
	Start() error
	CollectResults() (result.Result, error)
}

type SimpleGenerator struct {
	primary *remote.PrimaryConn
	users   []User
}

func (g *SimpleGenerator) CollectResults() (result.Result, error) {
	//TODO implement me
	panic("implement me")
}

func NewSimpleGenerator(primary *remote.PrimaryConn, wk *UserInfoWorkload) (*SimpleGenerator, error) {
	//create clients
	users := make([]User, len(wk.UserInfos))
	for i, u := range wk.UserInfos {
		//TODO endpoint
		client, err := ethclient.Dial("127.0.0.1:9000")
		if err != nil {
			return nil, err
		}
		users[i] = &SimpleUser{
			client: client,
			info:   u,
		}
	}

	return &SimpleGenerator{primary, users}, nil
}

func (g *SimpleGenerator) Start() error {
	for _, user := range g.users {
		err := user.Run()
		if err != nil {
			return fmt.Errorf("failed to run user: %w")
		}
	}

	return nil
}

type Workload interface {
	Encode(dest io.Writer) error
	Decode(src io.Reader) error
}

type PaymentWorkload struct {
	Users []User
}

type User interface {
	Run() error
}

type SimpleUser struct {
	client *ethclient.Client
	info   UserInfo
}

// TODO
func (s *SimpleUser) Run() error {
	panic("SimpleUser Run not implemented yet !")
}

// Example workload type
type UserInfoWorkload struct {
	UserInfos []UserInfo
}

// Implementing the Workload interface for UserInfoWorkload
func (w *UserInfoWorkload) Encode(dest io.Writer) error {
	// Write the number of UserInfos
	count := int32(len(w.UserInfos))
	err := binary.Write(dest, binary.LittleEndian, count)
	if err != nil {
		return err
	}

	// Encode each UserInfo
	for _, userInfo := range w.UserInfos {
		err = userInfo.Encode(dest)
		if err != nil {
			return err
		}
	}

	return nil
}

func (w *UserInfoWorkload) Decode(src io.Reader) error {
	// Read the number of UserInfos
	var count int32
	err := binary.Read(src, binary.LittleEndian, &count)
	if err != nil {
		return err
	}

	w.UserInfos = make([]UserInfo, count)

	// Decode each UserInfo
	for i := int32(0); i < count; i++ {
		var userInfo UserInfo
		err = userInfo.Decode(src)
		if err != nil {
			return err
		}
		w.UserInfos[i] = userInfo
	}

	return nil
}

// Define the Account struct
type Account struct {
	address    []byte
	privateKey []byte
}

// UserInfo represents user information with an Account and frequency
type UserInfo struct {
	Account   Account
	Frequency int
}

// Implementing the Encode method for UserInfo
func (u *UserInfo) Encode(dest io.Writer) error {
	// Encode the frequency
	err := binary.Write(dest, binary.LittleEndian, u.Frequency)
	if err != nil {
		return err
	}

	// Encode the Account address
	addressLength := int32(len(u.Account.address))
	err = binary.Write(dest, binary.LittleEndian, addressLength)
	if err != nil {
		return err
	}

	_, err = dest.Write(u.Account.address)
	if err != nil {
		return err
	}

	// Encode the Account private key
	privateKeyLength := int32(len(u.Account.privateKey))
	err = binary.Write(dest, binary.LittleEndian, privateKeyLength)
	if err != nil {
		return err
	}

	_, err = dest.Write(u.Account.privateKey)
	if err != nil {
		return err
	}

	return nil
}

// Implementing the Decode method for UserInfo
func (u *UserInfo) Decode(src io.Reader) error {
	// Decode the frequency
	err := binary.Read(src, binary.LittleEndian, &u.Frequency)
	if err != nil {
		return err
	}

	// Decode the Account address
	var addressLength int32
	err = binary.Read(src, binary.LittleEndian, &addressLength)
	if err != nil {
		return err
	}

	u.Account.address = make([]byte, addressLength)
	_, err = io.ReadFull(src, u.Account.address)
	if err != nil {
		return err
	}

	// Decode the Account private key
	var privateKeyLength int32
	err = binary.Read(src, binary.LittleEndian, &privateKeyLength)
	if err != nil {
		return err
	}

	u.Account.privateKey = make([]byte, privateKeyLength)
	_, err = io.ReadFull(src, u.Account.privateKey)
	if err != nil {
		return err
	}

	return nil
}
