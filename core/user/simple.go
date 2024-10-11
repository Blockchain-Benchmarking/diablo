package user

import (
	"diablo/core/applications"
	"diablo/core/logging"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/ethclient"
	"io"
	"sync"
	"time"
)

type SimpleUser struct {
	Client *ethclient.Client
	Info   UserInfo
}

type Action struct {
	SubmitTime int64 // 0 if not submitted
	CommitTime int64 // 0 if not committed
	AbortTime  int64 // 0 if not aborted
	Retries    int32
	HasError   bool
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
		err := applications.Transfer(s.Client, s.Info.Account.Address, s.Info.Account.PrivateKey, "865593db1aa261dc707972ea7fbe856b027c7166", 1)
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

type SimpleResult []UserResult

func (s *SimpleResult) Encode(dest io.Writer) error {
	logging.Infof("writing length %d", int32(len(*s)))
	if err := binary.Write(dest, binary.LittleEndian, int32(len(*s))); err != nil {
		return err
	}

	for _, userResult := range *s {
		if err := binary.Write(dest, binary.LittleEndian, int32(len(userResult))); err != nil {
			return err
		}

		for _, action := range userResult {
			if err := binary.Write(dest, binary.LittleEndian, action.SubmitTime); err != nil {
				return err
			}
			if err := binary.Write(dest, binary.LittleEndian, action.CommitTime); err != nil {
				return err
			}
			if err := binary.Write(dest, binary.LittleEndian, action.AbortTime); err != nil {
				return err
			}
			if err := binary.Write(dest, binary.LittleEndian, action.Retries); err != nil {
				return err
			}
			if err := binary.Write(dest, binary.LittleEndian, action.HasError); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *SimpleResult) Decode(src io.Reader) error {
	var resultLength int32
	if err := binary.Read(src, binary.LittleEndian, &resultLength); err != nil {
		return fmt.Errorf("failed to read result length: %w", err)
	}

	*s = make(SimpleResult, resultLength)

	for i := int32(0); i < resultLength; i++ {
		var userResultLength int32
		if err := binary.Read(src, binary.LittleEndian, &userResultLength); err != nil {
			return fmt.Errorf("failed to read user result length at %d: %w", i, err)
		}

		userResult := make(UserResult, userResultLength)

		for j := int32(0); j < userResultLength; j++ {
			action := &Action{}

			if err := binary.Read(src, binary.LittleEndian, &action.SubmitTime); err != nil {
				return fmt.Errorf("failed to read submit time at %d, %d: %w", i, j, err)
			}
			if err := binary.Read(src, binary.LittleEndian, &action.CommitTime); err != nil {
				return fmt.Errorf("failed to read commit time at %d, %d: %w", i, j, err)
			}
			if err := binary.Read(src, binary.LittleEndian, &action.AbortTime); err != nil {
				return fmt.Errorf("failed to read abort time at %d, %d: %w", i, j, err)
			}
			if err := binary.Read(src, binary.LittleEndian, &action.Retries); err != nil {
				return fmt.Errorf("failed to read retries at %d, %d: %w", i, j, err)
			}
			if err := binary.Read(src, binary.LittleEndian, &action.HasError); err != nil {
				return fmt.Errorf("failed to read has error at %d, %d: %w", i, j, err)
			}

			userResult[j] = action
		}

		(*s)[i] = userResult
	}

	return nil
}

func (s *SimpleResult) Merge(other *SimpleResult) *SimpleResult {
	result := append(*s, *other...)

	return &result
}

type UserResult []*Action

func (s *SimpleResult) PrintResult(dest io.Writer) error {
	buf, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("failed to marshall result: %w", err)
	}

	_, err = dest.Write(buf)
	if err != nil {
		return fmt.Errorf("failed to write result: %w", err)
	}

	return nil
}
