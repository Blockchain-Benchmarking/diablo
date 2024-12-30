package payment

import (
	"diablo/blockchain"
	"diablo/core/logging"
	"diablo/workload/behavior"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

const STUBBORN_PAYMENT_USER = "stubbornPaymentUser"

type StubbornPaymentUser struct {
	Implementation string
	Config         blockchain.Config
	Timeout        time.Duration

	Stubborn *behavior.StubbornBehavior

	Payments []Info

	Random bool

	Tps int

	Duration time.Duration

	App       *Application
	restartCh chan behavior.RestartInfo
}

type Info struct {
	Time   int64   `json:"time"`
	Amount float64 `json:"amount"`
	To     string  `json:"to"`
	ID     string  `json:"id"`
}

func (s *StubbornPaymentUser) New(implementation string, config blockchain.Config, params map[string]interface{}) (behavior.User, error) {
	timeoutString, ok := params["timeout"].(string)
	if !ok {
		return nil, fmt.Errorf("params 'timeout' should be specified")
	}

	timeout, err := time.ParseDuration(timeoutString)
	if err != nil {
		return nil, fmt.Errorf("invalid timeout format: %s", timeoutString)
	}

	maxRetries, ok := params["max_attempts"].(int)
	if !ok {
		return nil, fmt.Errorf("params 'maxRetries' should be specified")
	}

	random, ok := params["random"].(bool)
	if !ok {
		return nil, fmt.Errorf("params 'random' should be specified")
	}

	payments, ok := params["payments"].([]Info)
	if !ok {
		payments = make([]Info, 0)
	}

	tps, ok := params["tps"].(int)
	if !ok {
		return nil, fmt.Errorf("params 'tps' should be specified")
	}

	duration := time.Duration(0)
	durationString, ok := params["duration"].(string)
	if ok {
		duration, err = time.ParseDuration(durationString)
		if err != nil {
			return nil, fmt.Errorf("invalid duration format: %s", durationString)
		}
	}

	u := &StubbornPaymentUser{
		Implementation: implementation,
		Config:         config,
		Timeout:        timeout,
		Stubborn:       behavior.NewStubbornBehavior(int32(maxRetries)),

		Random:   random,
		Payments: payments,

		Tps: tps,

		Duration:  duration,
		restartCh: make(chan behavior.RestartInfo),
	}

	return u, nil
}

func (s *StubbornPaymentUser) resetParameters(newParameters StubbornPaymentUser) {
	if s.Implementation != newParameters.Implementation {
		s.App = nil
		s.Implementation = newParameters.Implementation
	}
	s.Config = newParameters.Config
	s.Timeout = newParameters.Timeout
	s.Stubborn = newParameters.Stubborn
	s.Random = newParameters.Random
	s.Payments = newParameters.Payments
	s.Tps = newParameters.Tps
	s.Duration = newParameters.Duration
}

func (s *StubbornPaymentUser) Empty() behavior.User {
	return &StubbornPaymentUser{Stubborn: &behavior.StubbornBehavior{}}
}

func (s *StubbornPaymentUser) UnmarshalUsers(buf []byte) ([]behavior.User, error) {
	var res []*StubbornPaymentUser
	err := json.Unmarshal(buf, &res)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal stubborn payment users: %w", err)
	}

	users := make([]behavior.User, len(res))
	for i, user := range res {
		users[i] = user
	}

	return users, nil
}

func (s *StubbornPaymentUser) ID() string {
	return s.Config.Id
}

func (s *StubbornPaymentUser) EmptyResult() behavior.Result {
	return &behavior.StubbornAction{}
}

func (s *StubbornPaymentUser) Name() string {
	return STUBBORN_PAYMENT_USER
}

func tickerChannel(t *time.Ticker) <-chan time.Time {
	if t == nil {
		return nil
	}
	return t.C
}

func (s *StubbornPaymentUser) Run(results chan behavior.Result, stop chan struct{}) {
	s.restartCh = make(chan behavior.RestartInfo)
	interval := time.Second

reset:
	if s.App == nil {
		var err error
		s.App, err = NewPaymentApplication(s.Config, s.Implementation)
		if err != nil {
			logging.Errorf("failed to init user app: %s", err.Error())
			return
		}
	}

	if len(s.Payments) != 0 {
		s.runSchedule(results)
		return
	}

	var ticker *time.Ticker
	if s.Duration > 0 {
		ticker = time.NewTicker(s.Duration)
	}

	currentTransaction := 0
	transactionsWg := &sync.WaitGroup{}
	for i := 0; i < s.Tps; i++ {
		transactionsWg.Add(1)
		go func() {
			if s.Random {
				results <- s.executeTransaction(s.randomTransaction())
			} else {
				results <- s.executeTransaction(s.Payments[currentTransaction%len(s.Payments)])
				currentTransaction++
			}
			transactionsWg.Done()
		}()
	}

	for {
		select {
		case <-stop:
			transactionsWg.Wait()
			return
		case <-tickerChannel(ticker):
			transactionsWg.Wait()
			for {
				select {
				case <-stop:
					return
				case info := <-s.restartCh:
					transactionsWg.Wait()
					s.resetParameters(*info.NewParameters.(*StubbornPaymentUser))
					waitingTime := time.Until(info.RestartTime)
					if waitingTime > 0 {
						time.Sleep(waitingTime)
					} else {
						logging.Infof("starting user with %s delay", (-1 * waitingTime).String())
					}
					goto reset
				}
			}
		case info := <-s.restartCh:
			transactionsWg.Wait()
			s.resetParameters(*info.NewParameters.(*StubbornPaymentUser))
			waitingTime := time.Until(info.RestartTime)
			if waitingTime > 0 {
				time.Sleep(waitingTime)
			} else {
				logging.Warnf("starting user with %s delay", (-1 * waitingTime).String())
			}
			goto reset
		case <-time.After(interval):
			interval = time.Second

			st := time.Now()
			for i := 0; i < s.Tps; i++ {
				transactionsWg.Add(1)
				go func() {
					if s.Random {
						results <- s.executeTransaction(s.randomTransaction())
					} else {
						results <- s.executeTransaction(s.Payments[currentTransaction%len(s.Payments)])
						currentTransaction++
					}
					transactionsWg.Done()
				}()
			}

			interval = time.Duration(math.Max(float64(time.Second-time.Since(st)), 0))
			if interval == 0 {
				logging.Warnf("sending transactions takes too much time")
			}
		}
	}
}

func (s *StubbornPaymentUser) runSchedule(results chan behavior.Result) {
	//Execute all transactions sequentially once and return
	for _, info := range s.Payments {
		wait := time.Unix(info.Time, 0).Sub(time.Now())
		if wait > 0 {
			time.Sleep(wait)
		}

		if wait < 0 {
			logging.Warnf("executing transaction with %s delay", wait.String())
		}

		results <- s.executeTransaction(info)
	}
}

func (s *StubbornPaymentUser) Restart(info behavior.RestartInfo) error {
	_, ok := info.NewParameters.(*StubbornPaymentUser)
	if !ok {
		return fmt.Errorf("invalid new stubbornPaymentUser")
	}

	s.restartCh <- info

	return nil
}

func (s *StubbornPaymentUser) randomTransaction() Info {
	var to string
	var amount float64

	for to == "" || to == s.Config.Address {
		toIndex := rand.Intn(len(s.App.GetOthers()))
		to = s.App.GetOthers()[toIndex]
	}

	for amount == 0 {
		amount = rand.Float64()
	}

	return Info{
		Amount: amount,
		To:     to,
		ID:     s.ID(),
	}
}

func (s *StubbornPaymentUser) executeTransaction(info Info) behavior.StubbornAction {
	return s.Stubborn.PerformStubbornAction(func() error {
		return s.App.Pay(info.To, info.Amount, s.Timeout)
	}, "transfer", info.ID)
}

func (s *StubbornPaymentUser) ContractPaths() map[string]behavior.ContractInfo {
	return nil
}
