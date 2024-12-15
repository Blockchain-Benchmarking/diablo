package store

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

const STUBBORN_STORE_USER = "stubbornStoreUser"

const (
	Read = iota
	Write
)

type StubbornStoreUser struct {
	Implementation string
	Config         blockchain.Config

	ContractAddress string

	//TODO put all these in stubborn behavior ?
	Actions  []Info
	Random   bool
	Tps      int
	Duration time.Duration
	Timeout  time.Duration
	Stubborn *behavior.StubbornBehavior

	App       *Application
	restartCh chan behavior.RestartInfo
}

type Info struct {
	Time  int64
	Key   string
	Value string
	Type  int
}

func (s *StubbornStoreUser) New(implementation string, config blockchain.Config, params map[string]interface{}) (behavior.User, error) {
	contractAddress, ok := params["contractAddress"].(string)
	if !ok {
		return nil, fmt.Errorf("param 'contractAddress' is required")
	}

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

	actions, ok := params["actions"].([]Info)
	if !ok {
		actions = make([]Info, 0)
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

	u := &StubbornStoreUser{
		Implementation:  implementation,
		Config:          config,
		ContractAddress: contractAddress,
		Timeout:         timeout,
		Stubborn:        behavior.NewStubbornBehavior(int32(maxRetries)),

		Actions: actions,
		Random:  random,

		Tps: tps,

		Duration:  duration,
		restartCh: make(chan behavior.RestartInfo),
	}

	return u, nil
}

func (s *StubbornStoreUser) resetParameters(newParameters StubbornStoreUser) {
	if s.Implementation != newParameters.Implementation {
		s.App = nil
		s.Implementation = newParameters.Implementation
	}
	s.Config = newParameters.Config
	s.Timeout = newParameters.Timeout
	s.Stubborn = newParameters.Stubborn
	s.Random = newParameters.Random
	s.Actions = newParameters.Actions
	s.Tps = newParameters.Tps
	s.Duration = newParameters.Duration
}

func (s *StubbornStoreUser) Empty() behavior.User {
	return &StubbornStoreUser{Stubborn: &behavior.StubbornBehavior{}}
}

func (s *StubbornStoreUser) UnmarshalUsers(buf []byte) ([]behavior.User, error) {
	var res []*StubbornStoreUser
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

func (s *StubbornStoreUser) ID() string {
	return s.Config.Id
}

func (s *StubbornStoreUser) EmptyResult() behavior.Result {
	return &behavior.StubbornAction{}
}

func (s *StubbornStoreUser) Name() string {
	return STUBBORN_STORE_USER
}

func tickerChannel(t *time.Ticker) <-chan time.Time {
	if t == nil {
		return nil
	}
	return t.C
}

// TODO defer wg.done from outside ?
func (s *StubbornStoreUser) Run(results chan behavior.Result, stop chan struct{}) {
	s.restartCh = make(chan behavior.RestartInfo)
	interval := time.Second

reset:
	if s.App == nil {
		var err error
		s.App, err = New(s.Implementation, s.Config, s.ContractAddress)
		if err != nil {
			logging.Errorf("failed to init user app: %s", err.Error())
			return
		}
	}

	//Scheduled run
	if len(s.Actions) != 0 {
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
				results <- s.executeTransaction(s.Actions[currentTransaction%len(s.Actions)])
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
			logging.Infof("ticker, waiting for transactions")
			transactionsWg.Wait()
			logging.Infof("all transactions done")
			for {
				select {
				case <-stop:
					logging.Infof("stopping user")
					return
				case info := <-s.restartCh:
					transactionsWg.Wait()
					s.resetParameters(*info.NewParameters.(*StubbornStoreUser))
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
			s.resetParameters(*info.NewParameters.(*StubbornStoreUser))
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
						//increment own counter
						results <- s.executeTransaction(s.randomTransaction())
					} else {
						results <- s.executeTransaction(s.Actions[currentTransaction%len(s.Actions)])
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

func (s *StubbornStoreUser) runSchedule(results chan behavior.Result) {
	//Execute all transactions sequentially once and return
	for _, info := range s.Actions {
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

func (s *StubbornStoreUser) Restart(info behavior.RestartInfo) error {
	_, ok := info.NewParameters.(*StubbornStoreUser)
	if !ok {
		return fmt.Errorf("invalid new stubbornPaymentUser")
	}

	s.restartCh <- info

	return nil
}

func (s *StubbornStoreUser) randomTransaction() Info {
	info := Info{
		Key:   s.ID(),
		Value: "",
		Type:  rand.Intn(2),
	}

	if info.Type == Write {
		info.Value = randomString(2)
	}

	return info
}

func (s *StubbornStoreUser) executeTransaction(info Info) behavior.StubbornAction {
	var actionType string
	if info.Type == Read {
		actionType = "read"
	} else if info.Type == Write {
		actionType = "write"
	}

	return s.Stubborn.PerformStubbornAction(func() error {
		if info.Type == Read {
			logging.Infof("calling get item on %s", info.Key)
			_, err := s.App.GetItem(info.Key, s.Timeout)
			return err
		}

		logging.Infof("calling set item on %s to %s", info.Key, info.Value)

		err := s.App.SetItem(info.Key, info.Value, s.Timeout)
		if err != nil {
			return err
		}

		logging.Infof("after set, calling get item on %s", info.Key)

		res, err := s.App.GetItem(info.Key, s.Timeout)
		if err != nil {
			return err
		}

		logging.Infof("got item %s after setting it to %s", res, info.Value)

		return nil
	}, actionType)
}

var letters = []rune("abcdefghijklmnopqrstuvwxyz")

func randomString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
