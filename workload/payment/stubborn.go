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

type StubbornPaymentUser struct {
	Implementation string
	Config         behavior.Config
	Timeout        time.Duration
	Stubborn       *behavior.StubbornBehavior

	//TODO Scheduler to make users easier to implement

	Schedule *Scheduler
	App      PaymentApplication
}

type Scheduler struct {
	sync.RWMutex
	list    *LinkedRate
	waiting []chan behavior.Rate
}

type LinkedRate struct {
	Rate *behavior.Rate
	Next *LinkedRate
}

func NewScheduler() *Scheduler {
	return &Scheduler{}
}

func (s *Scheduler) Update(new behavior.ScheduleRates) {
	s.Lock()
	defer s.Unlock()

	logging.Infof("updating schedule with %v", new)

	for len(s.waiting) > 0 && len(new.Rates) > 0 {
		promise := s.waiting[0]
		s.waiting = s.waiting[1:]

		promise <- new.Rates[0]
		close(promise)

		new.Rates = new.Rates[1:]
	}

	if s.list == nil && len(new.Rates) > 0 {
		s.list = &LinkedRate{
			Rate: &new.Rates[0],
			Next: nil,
		}
		new.Rates = new.Rates[1:]
	}

	current := s.list
	for current != nil && current.Next != nil {
		current = current.Next
	}

	for _, rate := range new.Rates {
		node := &LinkedRate{
			Rate: &rate,
			Next: nil,
		}
		current.Next = node
		current = node
	}
}

func (s *Scheduler) GetNextRate() (behavior.Rate, <-chan behavior.Rate, bool) {
	s.Lock()
	defer s.Unlock()

	if s.list == nil {
		promise := make(chan behavior.Rate, 1)
		s.waiting = append(s.waiting, promise)
		return behavior.Rate{}, promise, false
	}

	if s.list != nil {
		nextRate := *s.list.Rate
		s.list = s.list.Next

		return nextRate, nil, true
	}

	return behavior.Rate{}, nil, false
}

func (s *StubbornPaymentUser) New(blockchain string, config behavior.Config, params map[string]interface{}) (behavior.User, error) {
	timeoutString, ok := params["timeout"].(string)
	if !ok {
		return nil, fmt.Errorf("params 'timeout' should be specified")
	}

	timeout, err := time.ParseDuration(timeoutString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse 'timeout' parameter '%s': %w", timeoutString, err)
	}

	maxRetries, ok := params["max_attempts"].(int)
	if !ok {
		return nil, fmt.Errorf("params 'maxRetries' should be specified")
	}

	u := &StubbornPaymentUser{
		Implementation: blockchain,
		Config:         config,
		Timeout:        timeout,
		Stubborn:       behavior.NewStubbornBehavior(int32(maxRetries)),

		Schedule: NewScheduler(),
	}

	return u, nil
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

func (s *StubbornPaymentUser) InitApp() error {
	init := PaymentApplications[strings.ToLower(s.Implementation)]
	app, err := init(s.Config)
	if err != nil {
		return err
	}

	s.App = app

	return nil
}

func (s *StubbornPaymentUser) EmptyResult() behavior.Result {
	return &behavior.StubbornAction{}
}

func (s *StubbornPaymentUser) Name() string {
	return STUBBORN_PAYMENT_USER
}

func (s *StubbornPaymentUser) DeliverWorkload(schedule behavior.Schedule) error {
	rates, ok := schedule.(*behavior.ScheduleRates)
	if !ok {
		return fmt.Errorf("schedule type %s is not supported for StubbornPaymentUser", schedule.Name())
	}

	s.Schedule.Update(*rates)
	return nil
}

func tickerChannel(t *time.Ticker) <-chan time.Time {
	if t == nil {
		return nil
	}
	return t.C
}

func (s *StubbornPaymentUser) Run(wg *sync.WaitGroup, results chan behavior.Result, stop chan struct{}) {
	defer func() {
		wg.Done()
	}()

	currentRate, promise, ok := s.Schedule.GetNextRate()
	if !ok {
		currentRate = <-promise
	}

	var ticker *time.Ticker
	if currentRate.Duration == 0 {
		ticker = nil
	} else {
		ticker = time.NewTicker(currentRate.Duration)
	}

	transactionsWg := &sync.WaitGroup{}
	for i := 0; i < currentRate.Tps; i++ {
		transactionsWg.Add(1)
		go func() {
			results <- s.randomTransaction()
			transactionsWg.Done()
		}()
	}

	for {
		select {
		case <-stop:
			logging.Infof("waiting for user transactions to finish ")
			transactionsWg.Wait()
			logging.Infof("user transactions done")
			return
		case <-tickerChannel(ticker):
			//update rate
			logging.Infof("updating user rate")
			currentRate, promise, ok = s.Schedule.GetNextRate()
			if !ok {
			loop:
				for {
					select {
					case <-stop:
						return
					case currentRate = <-promise:
						logging.Infof("received promise rate")
						break loop
					}
				}
			}
			if currentRate.Duration == 0 { //continue till the end
				ticker = nil
			} else {
				ticker = time.NewTicker(currentRate.Duration)
			}
			logging.Infof("new user rate: %d, %s", currentRate.Tps, currentRate.Duration.String())

		case <-time.After(1 * time.Second):
			for i := 0; i < currentRate.Tps; i++ {
				transactionsWg.Add(1)
				go func() {
					results <- s.randomTransaction()
					transactionsWg.Done()
				}()
			}
		}
	}
}

func (s *StubbornPaymentUser) randomTransaction() behavior.StubbornAction {
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
