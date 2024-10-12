package user

import (
	"diablo/core/application"
	"diablo/core/logging"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"sync"
	"time"
)

type StubbornUser struct {
	application.Application
	Account
	App          string
	Blockchain   string
	Timeout      time.Duration
	MaxRetries   int32
	Transactions int32
	Others       []string //known addresses
}

type Action struct {
	StartTime        int64 // 0 if not submitted
	SuccessTime      int64 // 0 if not committed
	FinalFailureTime int64 // 0 if not aborted
	Retries          int32
	HasError         bool
}

type SimpleResult []SingleUserResult
type SingleUserResult []*Action

func CreateStubbornUser(account Account, others []string, app string, blockchain string) (User, error) {
	u := &StubbornUser{
		App:          app,
		Blockchain:   blockchain,
		Account:      account,
		Timeout:      2 * time.Minute,
		MaxRetries:   1,
		Transactions: 2,
		Others:       others,
	}

	return u, nil
}

func (s *StubbornUser) GetParameters(app string, blockchain string) (appParams map[string]interface{}, blockchainParams map[string]interface{}, err error) {
	appParams = make(map[string]interface{})
	blockchainParams = make(map[string]interface{})

	appParams["blockchain"] = blockchain

	switch app {
	case "payment":
		appParams["others"] = s.Others
		switch blockchain {
		case "ethereum":
			blockchainParams["endpoint"] = "ws://127.0.0.1:9000" //TODO
			blockchainParams["privateKey"] = s.PrivateKey
			blockchainParams["address"] = s.Address
		}
	}

	return appParams, blockchainParams, nil
}

// Run implements User
func (s *StubbornUser) Run(wg *sync.WaitGroup, results chan interface{}) {
	appInit, ok := application.Applications[s.App]
	if !ok {
		logging.Fatalf("application %s not implemented", s.App)
	}

	a, b, err := s.GetParameters(s.App, s.Blockchain)
	if err != nil {
		logging.Fatalf("failed to generate parameters for user with app %s and blockchain %s: %s", s.App, s.Blockchain, err.Error())
	}

	s.Application, err = appInit(a, b)
	if err != nil {
		logging.Fatalf("failed to initialize user application %s with blockchain %s: %s", s.App, s.Blockchain, err.Error())
	}

	res := make(SingleUserResult, s.Transactions)

	defer func() {
		results <- res
		wg.Done()
	}()

	for i := 0; i < int(s.Transactions); i++ {
		action := &Action{}
		s.executeTransaction(action)
		res[i] = action
	}

	logging.Infof("user finished all transactions")
}

func (s *StubbornUser) executeTransaction(action *Action) {
	action.StartTime = time.Now().Unix()

	var err error
	for i := 0; i < int(action.Retries)+1; i++ {
		var to string
		var amount float64

		for to == "" || to == s.Address {
			toIndex := rand.Intn(len(s.Others))
			to = s.Others[toIndex]
		}

		for amount == 0 {
			amount = rand.Float64()
		}

		params := map[string]interface{}{
			"to":     to,
			"amount": amount,
		}

		err = s.Execute(params)
		if err == nil {
			action.SuccessTime = time.Now().Unix()
			return
		} else {
			panic(err)
		}
		action.Retries++
	}

	action.FinalFailureTime = time.Now().Unix()
	action.HasError = true
}

// Encode implements User
func (s *StubbornUser) Encode(dest io.Writer) error {
	err := binary.Write(dest, binary.LittleEndian, s.Transactions)
	if err != nil {
		return fmt.Errorf("failed to write StubbornUser Transactions %d: %w", s.Timeout, err)
	}

	err = binary.Write(dest, binary.LittleEndian, s.Timeout)
	if err != nil {
		return fmt.Errorf("failed to write StubbornUser Timeout %d: %w", s.Timeout, err)
	}

	err = binary.Write(dest, binary.LittleEndian, s.MaxRetries)
	if err != nil {
		return fmt.Errorf("failed to write StubbornUser MaxRetries %d: %w", s.MaxRetries, err)
	}

	appLength := int32(len(s.App))
	err = binary.Write(dest, binary.LittleEndian, appLength)
	if err != nil {
		return fmt.Errorf("failed to write app length: %w", err)
	}
	_, err = dest.Write([]byte(s.App))
	if err != nil {
		return fmt.Errorf("failed to write app: %w", err)
	}

	blockchainLength := int32(len(s.Blockchain))
	err = binary.Write(dest, binary.LittleEndian, blockchainLength)
	if err != nil {
		return fmt.Errorf("failed to write Address length: %w", err)
	}
	_, err = dest.Write([]byte(s.Blockchain))
	if err != nil {
		return fmt.Errorf("failed to write Address: %w", err)
	}

	addressLength := int32(len(s.Address))
	err = binary.Write(dest, binary.LittleEndian, addressLength)
	if err != nil {
		return fmt.Errorf("failed to write Address length: %w", err)
	}
	_, err = dest.Write([]byte(s.Address))
	if err != nil {
		return fmt.Errorf("failed to write Address: %w", err)
	}

	privateKeyLength := int32(len(s.PrivateKey))
	err = binary.Write(dest, binary.LittleEndian, privateKeyLength)
	if err != nil {
		return fmt.Errorf("failed to write PrivateKey length: %w", err)
	}
	_, err = dest.Write([]byte(s.PrivateKey))
	if err != nil {
		return fmt.Errorf("failed to write PrivateKey: %w", err)
	}

	othersLength := int32(len(s.Others))
	err = binary.Write(dest, binary.LittleEndian, othersLength)
	if err != nil {
		return fmt.Errorf("failed to write Others slice length: %w", err)
	}
	for _, other := range s.Others {
		otherLength := int32(len(other))
		err = binary.Write(dest, binary.LittleEndian, otherLength)
		if err != nil {
			return fmt.Errorf("failed to write Others entry length: %w", err)
		}
		_, err = dest.Write([]byte(other))
		if err != nil {
			return fmt.Errorf("failed to write Others entry: %w", err)
		}
	}

	return nil
}

// Decode implements User
func (s *StubbornUser) Decode(src io.Reader) error {
	err := binary.Read(src, binary.LittleEndian, &s.Transactions)
	if err != nil {
		return fmt.Errorf("failed to read StubbornUser Transactions: %w", err)
	}

	err = binary.Read(src, binary.LittleEndian, &s.Timeout)
	if err != nil {
		return fmt.Errorf("failed to read StubbornUser Timeout: %w", err)
	}

	err = binary.Read(src, binary.LittleEndian, &s.MaxRetries)
	if err != nil {
		return fmt.Errorf("failed to read StubbornUser MaxRetries: %w", err)
	}

	var appLength int32
	err = binary.Read(src, binary.LittleEndian, &appLength)
	if err != nil {
		return err
	}

	appName := make([]byte, appLength)

	err = binary.Read(src, binary.LittleEndian, &appName)
	if err != nil {
		return err
	}

	s.App = string(appName)

	var blockchainLength int32
	err = binary.Read(src, binary.LittleEndian, &blockchainLength)
	if err != nil {
		return err
	}

	blockchainName := make([]byte, blockchainLength)

	err = binary.Read(src, binary.LittleEndian, &blockchainName)
	if err != nil {
		return err
	}

	s.Blockchain = string(blockchainName)

	var addressLength int32
	err = binary.Read(src, binary.LittleEndian, &addressLength)
	if err != nil {
		return fmt.Errorf("failed to read Address length: %w", err)
	}
	address := make([]byte, addressLength)
	_, err = io.ReadFull(src, address)
	if err != nil {
		return fmt.Errorf("failed to read Address: %w", err)
	}
	s.Address = string(address)

	var privateKeyLength int32
	err = binary.Read(src, binary.LittleEndian, &privateKeyLength)
	if err != nil {
		return fmt.Errorf("failed to read PrivateKey length: %w", err)
	}
	privateKey := make([]byte, privateKeyLength)
	_, err = io.ReadFull(src, privateKey)
	if err != nil {
		return fmt.Errorf("failed to read PrivateKey: %w", err)
	}
	s.PrivateKey = string(privateKey)

	var othersLength int32
	err = binary.Read(src, binary.LittleEndian, &othersLength)
	if err != nil {
		return fmt.Errorf("failed to read Others slice length: %w", err)
	}
	s.Others = make([]string, othersLength)
	for i := int32(0); i < othersLength; i++ {
		var otherLength int32
		err = binary.Read(src, binary.LittleEndian, &otherLength)
		if err != nil {
			return fmt.Errorf("failed to read Others entry length: %w", err)
		}
		other := make([]byte, otherLength)
		_, err = io.ReadFull(src, other)
		if err != nil {
			return fmt.Errorf("failed to read Others entry: %w", err)
		}
		s.Others[i] = string(other)
	}

	return nil
}

// Encode implements Results
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
			if err := binary.Write(dest, binary.LittleEndian, action.StartTime); err != nil {
				return err
			}
			if err := binary.Write(dest, binary.LittleEndian, action.SuccessTime); err != nil {
				return err
			}
			if err := binary.Write(dest, binary.LittleEndian, action.FinalFailureTime); err != nil {
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

// Decode implements Results
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

		userResult := make(SingleUserResult, userResultLength)

		for j := int32(0); j < userResultLength; j++ {
			action := &Action{}

			if err := binary.Read(src, binary.LittleEndian, &action.StartTime); err != nil {
				return fmt.Errorf("failed to read submit time at %d, %d: %w", i, j, err)
			}
			if err := binary.Read(src, binary.LittleEndian, &action.SuccessTime); err != nil {
				return fmt.Errorf("failed to read commit time at %d, %d: %w", i, j, err)
			}
			if err := binary.Read(src, binary.LittleEndian, &action.FinalFailureTime); err != nil {
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

// Merge implements Results
func (s *SimpleResult) Merge(other Results) Results {
	otherSimpleResult, ok := other.(*SimpleResult)
	if !ok {
		panic("tried to merge different result implementations")
	}

	mergedResult := append(*s, *otherSimpleResult...)
	return &mergedResult
}

// PrintResult implements Results
func (s *SimpleResult) PrintResult(dest io.Writer) error {
	buf, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	_, err = dest.Write(buf)
	if err != nil {
		return fmt.Errorf("failed to write result: %w", err)
	}

	return nil
}
