package payment

import (
	"diablo/core/behavior"
	"diablo/core/logging"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"sync"
	"time"
)

const STUBBORN_PAYMENT_USER = "stubbornPaymentUser"

type StubbornPaymentUser struct {
	// Encodable
	Implementation string
	Config         behavior.Config
	Transactions   int32
	Timeout        time.Duration
	Stubborn       *behavior.StubbornBehavior

	// Init
	App PaymentApplication
}

func NewStubbornPaymentUser(implementation string, config behavior.Config, params map[string]interface{}) (behavior.User, error) {
	transactions, ok := params["transactions"].(int)
	if !ok {
		return nil, fmt.Errorf("params 'transactions' should be specified")
	}

	timeoutString, ok := params["timeout"].(string)
	if !ok {
		return nil, fmt.Errorf("params 'timeout' should be specified")
	}

	timeout, err := time.ParseDuration(timeoutString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse 'timeout' parameter '%s': %w", timeoutString, err)
	}

	maxRetries, ok := params["max_retries"].(int)
	if !ok {
		return nil, fmt.Errorf("params 'maxRetries' should be specified")
	}

	u := &StubbornPaymentUser{
		Implementation: implementation,
		Config:         config,
		Transactions:   int32(transactions),
		Timeout:        timeout,
		Stubborn:       behavior.NewStubbornBehavior(int32(maxRetries)),
	}

	return u, nil
}

func EmptyStubbornPaymentUser() behavior.User {
	return &StubbornPaymentUser{Stubborn: &behavior.StubbornBehavior{}}
}

func EmptyStubbornPaymentResult() behavior.Results {
	return &behavior.StubbornResult{}
}

func (s *StubbornPaymentUser) Name() string {
	return STUBBORN_PAYMENT_USER
}

func (s *StubbornPaymentUser) Run(wg *sync.WaitGroup, results chan behavior.Results) {
	res := make(behavior.SingleUserStubbornResult, s.Transactions)

	defer func() {
		results <- res
		wg.Done()
	}()

	for i := 0; i < int(s.Transactions); i++ {
		res[i] = s.executeTransaction()
	}

	logging.Infof("user finished all Transactions")
}

func (s *StubbornPaymentUser) executeTransaction() *behavior.StubbornAction {
	var to string
	var amount float64

	for to == "" || to == s.Config.Address {
		toIndex := rand.Intn(len(s.App.GetOthers()))
		to = s.App.GetOthers()[toIndex]
	}

	for amount == 0 {
		amount = rand.Float64()
	}

	return s.Stubborn.PerformStubbornAction(func() error {
		return s.App.Pay(to, amount, s.Timeout)
	})
}

// Encode implements User
func (s *StubbornPaymentUser) Encode(dest io.Writer) error {
	err := binary.Write(dest, binary.LittleEndian, s.Transactions)
	if err != nil {
		return fmt.Errorf("failed to write StubbornUser Transactions %d: %w", s.Timeout, err)
	}

	err = binary.Write(dest, binary.LittleEndian, s.Timeout)
	if err != nil {
		return fmt.Errorf("failed to write StubbornUser Timeout %d: %w", s.Timeout, err)
	}

	err = binary.Write(dest, binary.LittleEndian, s.Stubborn.MaxRetries)
	if err != nil {
		return fmt.Errorf("failed to write StubbornUser MaxRetries %d: %w", s.Stubborn.MaxRetries, err)
	}

	appNameLength := int32(len(s.Implementation))
	err = binary.Write(dest, binary.LittleEndian, appNameLength)
	if err != nil {
		return fmt.Errorf("failed to write StubbornUser Implementation length: %w", err)
	}
	_, err = dest.Write([]byte(s.Implementation))
	if err != nil {
		return fmt.Errorf("failed to write Implementation: %w", err)
	}

	err = s.Config.Encode(dest)
	if err != nil {
		return fmt.Errorf("failed to encode Config: %w", err)
	}

	return nil
}

// Decode implements User
func (s *StubbornPaymentUser) Decode(src io.Reader) error {
	err := binary.Read(src, binary.LittleEndian, &s.Transactions)
	if err != nil {
		return fmt.Errorf("failed to read StubbornUser Transactions: %w", err)
	}

	err = binary.Read(src, binary.LittleEndian, &s.Timeout)
	if err != nil {
		return fmt.Errorf("failed to read StubbornUser Timeout: %w", err)
	}

	err = binary.Read(src, binary.LittleEndian, &s.Stubborn.MaxRetries)
	if err != nil {
		return fmt.Errorf("failed to read StubbornUser MaxRetries: %w", err)
	}

	var appNameLength int32
	err = binary.Read(src, binary.LittleEndian, &appNameLength)
	if err != nil {
		return fmt.Errorf("failed to read app name length: %w", err)
	}
	appName := make([]byte, appNameLength)
	_, err = io.ReadFull(src, appName)
	if err != nil {
		return fmt.Errorf("failed to read app name: %w", err)
	}
	s.Implementation = string(appName)

	err = s.Config.Decode(src)
	if err != nil {
		return fmt.Errorf("failed to decode config: %w", err)
	}

	init := PaymentApplications[strings.ToLower(s.Implementation)]
	s.App, err = init(s.Config)
	if err != nil {
		return fmt.Errorf("failed to initialize user application: %w", err)
	}

	return nil
}
