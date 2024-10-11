package workload

import (
	"diablo/core/blockchains"
	"diablo/core/logging"
	"encoding/binary"
	"fmt"
	"github.com/ethereum/go-ethereum/ethclient"
	"io"
	"sync"
	"time"
)

type User interface {
	Run(wg *sync.WaitGroup, results chan interface{})
}

type SimpleUser struct {
	client *ethclient.Client
	info   UserInfo
}

// TODO
func (s *SimpleUser) Run(wg *sync.WaitGroup, results chan interface{}) {
	transactions := 1
	res := make(UserResult, transactions)

	defer func() {
		results <- res
		wg.Done()
	}()

	//TODO add general blockchain interface instead
	for i := 0; i < transactions; i++ {
		res[i] = &Action{}
		res[i].SubmitTime = time.Now().Unix()
		err := blockchains.Transfer(s.client, s.info.Account.Address, s.info.Account.PrivateKey, "865593db1aa261dc707972ea7fbe856b027c7166", 1)
		if err != nil {
			logging.Errorf("transfer error: %s", err.Error())
			res[i].AbortTime = time.Now().Unix()
			res[i].HasError = true
			return
		}
		res[i].CommitTime = time.Now().Unix() //TODO not sure if this is how you get the stats
		//TODO add stubborn behavior
	}
	logging.Infof("user finished all transactions")
}

type UserInfoWorkload struct {
	UserInfos []UserInfo
}

func (w *UserInfoWorkload) Encode(dest io.Writer) error {
	count := int32(len(w.UserInfos))
	err := binary.Write(dest, binary.LittleEndian, count)
	if err != nil {
		return fmt.Errorf("failed to encode UserInfoWorkload count %d: %w", count, err)
	}

	for _, userInfo := range w.UserInfos {
		err = userInfo.Encode(dest)
		if err != nil {
			return fmt.Errorf("failed to encode userInfo: %w", err)
		}
	}

	return nil
}

func (w *UserInfoWorkload) Decode(src io.Reader) error {
	logging.Debugf("decoding user info workload")
	var count int32
	err := binary.Read(src, binary.LittleEndian, &count)
	if err != nil {
		return err
	}

	w.UserInfos = make([]UserInfo, count)

	for i := int32(0); i < count; i++ {
		logging.Debugf("decoding user info")
		userInfo := &UserInfo{}
		err = userInfo.Decode(src)
		if err != nil {
			return err
		}
		logging.Debugf("decodied user info with pk len %d", len(userInfo.Account.PrivateKey))
		w.UserInfos[i] = *userInfo
	}

	return nil
}

type Account struct {
	Address    string `yaml:"address"`
	PrivateKey string `yaml:"private"`
}

type UserInfo struct {
	Account   Account
	Frequency int32
}

func (u *UserInfo) Encode(dest io.Writer) error {
	err := binary.Write(dest, binary.LittleEndian, u.Frequency)
	if err != nil {
		return fmt.Errorf("failed to write UserInfo frequency %d: %w", u.Frequency, err)
	}

	addressLength := int32(len(u.Account.Address))
	err = binary.Write(dest, binary.LittleEndian, addressLength)
	if err != nil {
		return fmt.Errorf("failed to write userInfo Address length: %w", err)
	}

	_, err = dest.Write([]byte(u.Account.Address))
	if err != nil {
		return fmt.Errorf("failed to write userInfo account Address: %w", err)
	}

	privateKeyLength := int32(len(u.Account.PrivateKey))
	err = binary.Write(dest, binary.LittleEndian, privateKeyLength)
	if err != nil {
		return fmt.Errorf("failed to write userInfo account private key length: %w", err)
	}

	_, err = dest.Write([]byte(u.Account.PrivateKey))
	if err != nil {
		return fmt.Errorf("failed to write userInfo account private key: %w", err)
	}

	return nil
}

func (u *UserInfo) Decode(src io.Reader) error {
	err := binary.Read(src, binary.LittleEndian, &u.Frequency)
	if err != nil {
		return err
	}

	var addressLength int32
	err = binary.Read(src, binary.LittleEndian, &addressLength)
	if err != nil {
		return err
	}

	temp := make([]byte, addressLength)
	_, err = io.ReadFull(src, temp)
	if err != nil {
		return err
	}

	u.Account.Address = string(temp)

	var privateKeyLength int32
	err = binary.Read(src, binary.LittleEndian, &privateKeyLength)
	if err != nil {
		return err
	}

	temp = make([]byte, privateKeyLength)
	_, err = io.ReadFull(src, temp)
	if err != nil {
		return err
	}

	u.Account.PrivateKey = string(temp)

	return nil
}
