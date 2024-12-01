package payment

import (
	"diablo/core/logging"
	"diablo/workload/behavior"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"
)

const STUBBORN_PAYMENT_USER = "stubbornPaymentUser"

// specific transaction or many trnsactions randomly at certain rate
type StubbornPaymentUser struct {
	Implementation string
	Config         behavior.Config
	Timeout        time.Duration

	Stubborn *behavior.StubbornBehavior

	Payments []Info //Chronologically sorted list of interactions to complete or list of interactions to iterate through with below indicated tps

	Random bool //set to true if the interactions should be random (payments must be empty)

	Tps int //0 if interactions should be completed following their schedule

	Duration time.Duration //0 if the user should run as long as possible

	App       PaymentApplication
	restartCh chan StubbornPaymentUser
}

type Info struct {
	Time   int64   `json:"time"`
	Amount float64 `json:"amount"`
	To     string  `json:"to"`
}

func (s *StubbornPaymentUser) New(blockchain string, config behavior.Config, params map[string]interface{}) (behavior.User, error) {
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
		Implementation: blockchain,
		Config:         config,
		Timeout:        timeout,
		Stubborn:       behavior.NewStubbornBehavior(int32(maxRetries)),

		Random:   random,
		Payments: payments,

		Tps: tps,

		Duration:  duration,
		restartCh: make(chan StubbornPaymentUser),
	}

	return u, nil
}

func (s *StubbornPaymentUser) resetParameters(newParameters StubbornPaymentUser) {
	s.Implementation = newParameters.Implementation
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

// TODO defer wg.done from outside ?
func (s *StubbornPaymentUser) Run(wg *sync.WaitGroup, results chan behavior.Result, stop chan struct{}) {
	defer wg.Done()

reset:
	init := PaymentApplications[strings.ToLower(s.Implementation)]
	app, err := init(s.Config)
	if err != nil {
		logging.Errorf("failed to init user app: %s", err.Error())
		return
	}

	s.App = app

	//Scheduled run
	if s.Tps == 0 {
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
				results <- s.executeTransaction(s.Payments[i%len(s.Payments)])
				currentTransaction++
			}
			transactionsWg.Done()
		}()
	}

	for {
		select {
		case <-stop:
			//logging.Infof("waiting for user transactions to finish")
			transactionsWg.Wait()
			//logging.Infof("user transactions done")
			return
		case <-tickerChannel(ticker):
			//logging.Infof("user ran for %s, waiting for user transactions to finish", s.Duration.String())
			transactionsWg.Wait()
			//logging.Infof("user transactions done waiting for stop or restart")
			for {
				select {
				case <-stop:
					logging.Infof("stopping user")
					return
				case newUser := <-s.restartCh:
					//logging.Infof("waiting for transactions to finish to restart user")
					//transactionsWg.Wait()
					s.resetParameters(newUser)
					//logging.Infof("restarting user")
					goto reset
				}
			}
			return
		case newUser := <-s.restartCh:
			//logging.Infof("waiting for transactions to finish to restart user")
			//transactionsWg.Wait() //todo wait for transactions to finish or not ?
			s.resetParameters(newUser)
			//logging.Infof("restarting user")
			goto reset
		case <-time.After(time.Second):
			for i := 0; i < s.Tps; i++ {
				transactionsWg.Add(1)
				go func() {
					if s.Random {
						results <- s.executeTransaction(s.randomTransaction())
					} else {
						results <- s.executeTransaction(s.Payments[i%len(s.Payments)])
						currentTransaction++
					}
					transactionsWg.Done()
				}()
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

func (s *StubbornPaymentUser) Restart(new behavior.User) error {
	newStubbornPaymentUser, ok := new.(*StubbornPaymentUser)
	if !ok {
		return fmt.Errorf("invalid new stubbornPaymentUser")
	}

	s.restartCh <- *newStubbornPaymentUser

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
	}
}

func (s *StubbornPaymentUser) executeTransaction(info Info) behavior.StubbornAction {
	return s.Stubborn.PerformStubbornAction(func() error {
		return s.App.Pay(info.To, info.Amount, s.Timeout)
	})
}
