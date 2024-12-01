package core

import (
	"diablo/core/logging"
	"diablo/core/messaging"
	"diablo/core/network"
	"diablo/workload/behavior"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const usersBatchSize = 200

type Coordinator struct {
	wg    *sync.WaitGroup
	stop  chan struct{}
	execs map[string]func(msg messaging.Message) error

	secondaries map[string]*network.Secondary
	usersTrack  map[string][]behavior.User //secondary -> (userType, list of users)

	results *ResultsCollector

	usersDone chan struct{}
}

func NewCoordinator(secondaries map[string]*network.Secondary) *Coordinator {
	wg := &sync.WaitGroup{}
	c := &Coordinator{
		wg:   wg,
		stop: make(chan struct{}),

		secondaries: secondaries,

		results:    NewResultsCollector(),
		usersDone:  make(chan struct{}),
		usersTrack: make(map[string][]behavior.User),
	}

	c.registerExecs()

	wg.Add(len(secondaries))
	for _, secondary := range secondaries {
		go c.handleGeneratorMessages(secondary)
	}

	return c
}

// SendUsersToGenerators distributes the users equally amongst the generators
func (c *Coordinator) SendUsersToGenerators(users []behavior.User) error {
	perSecondary := len(users) / len(c.secondaries)
	remainder := len(users) % len(c.secondaries)

	i := 0
	for addr, secondary := range c.secondaries {
		n := perSecondary
		if remainder > 0 {
			n++
			remainder--
		}

		usersChunk := users[i : i+n]
		logging.Infof("sending %d users to %s", len(usersChunk), addr)

		usersMap := make(map[string][]behavior.User)
		for _, u := range usersChunk {
			if usersMap[u.Name()] == nil {
				usersMap[u.Name()] = make([]behavior.User, 0)
			}
			usersMap[u.Name()] = append(usersMap[u.Name()], u)
		}

		err := c.SendUsers(secondary, usersMap)
		if err != nil {
			return fmt.Errorf("failed to send users to secondary %s: %w", addr, err)
		}

		i = i + n
	}

	return nil
}

// SendUsers sends the list of users to the given secondary, possibly in multiple messages
func (c *Coordinator) SendUsers(s *network.Secondary, users map[string][]behavior.User) error {
	res := splitUsers(users)
	if len(res) == 0 {
		return fmt.Errorf("no users to send")
	}

	logging.Infof("sending users in %d batches to %s", len(res), s.Addr())

	for _, r := range res {
		buf, err := marshallUsers(r)
		if err != nil {
			return fmt.Errorf("failed to marshal users: %w", err)
		}

		msg := messaging.Users{Users: buf}
		err = s.Send(msg)
		if err != nil {
			return fmt.Errorf("failed to send workload: %w", err)
		}

		if c.usersTrack[s.Addr()] == nil {
			c.usersTrack[s.Addr()] = make([]behavior.User, 0)
		}

		c.usersTrack[s.Addr()] = append(c.usersTrack[s.Addr()], flattenUsersMap(r)...)
	}

	return nil
}

// SendStartToAll sends start or restart signal to all secondaries
func (c *Coordinator) SendStartToAll(startTime time.Time) error {
	logging.Infof("sending start message to secondaries with start time = " + startTime.String())
	for addr, secondary := range c.secondaries {
		err := secondary.Send(messaging.Start{Start: startTime.Unix()})
		if err != nil {
			return fmt.Errorf("failed to send start to %s: %w", addr, err)
		}
	}
	return nil
}

// SendStopToAll signals to the secondaries that no more users will be sent
func (c *Coordinator) SendStopToAll() error {
	logging.Infof("sending stop")

	for addr, secondary := range c.secondaries {
		err := secondary.Send(messaging.Stop{})
		if err != nil {
			return fmt.Errorf("failed to send stop to %s: %w", addr, err)
		}
	}

	return nil
}

func (c *Coordinator) processResultsMessage(msg messaging.Message) error {
	resMsg, ok := msg.(*messaging.Results)
	if !ok {
		return fmt.Errorf("invalid Result message")
	}

	empty, ok := Results[resMsg.Name]
	if !ok {
		return fmt.Errorf("result %s not found", resMsg.Name)
	}

	results, err := empty.UnmarshalResults(resMsg.Results)
	if err != nil {
		return err
	}

	c.results.AddResults(results...)

	return nil
}

func (c *Coordinator) processStoppedMessage(msg messaging.Message) error {
	_, ok := msg.(*messaging.Stopped)
	if !ok {
		return fmt.Errorf("invalid Stopped message")
	}

	c.usersDone <- struct{}{}
	return nil
}

// CollectNewResults returns the new results after the last call to CollectNewResults
func (c *Coordinator) CollectNewResults(earliestSubmit time.Time, latestDone time.Time) []behavior.Result {
	return c.results.CollectNewResults(earliestSubmit, latestDone)
}

// CollectResults returns all the results
func (c *Coordinator) CollectResults() []behavior.Result {
	<-c.usersDone
	logging.Infof("returning with results")
	return c.results.CollectResults()
}

func (c *Coordinator) TotalUsersSent() int {
	sum := 0
	for _, users := range c.usersTrack {
		sum += len(users)
	}

	return sum
}

func (c *Coordinator) Stop() {
	close(c.stop)
	logging.Infof("coordinator waiting for goroutines")
	c.wg.Wait()
	logging.Infof("coordinator done")
}

func (c *Coordinator) handleGeneratorMessages(s *network.Secondary) {
	defer func() {
		logging.Infof("finished with handling gen message")
		c.wg.Done()
	}()

	for {
		select {
		case <-c.stop:
			logging.Infof("stop receiving messages")
			return
		default:
			msg, err := network.ReadMessageWithTimeout(s.Reader(), 0) //todo
			if err != nil {
				if strings.Contains(err.Error(), "EOF") {
					logging.Warnf("EOF from %s", s.Addr())
					return
				} else if errors.Is(err, network.ErrTimeout) || strings.Contains(err.Error(), "timeout") {
					continue
				}
				logging.Fatalf("error receiving message from %s: %s", s.Addr(), err.Error())
				continue
			}

			exec, ok := c.execs[msg.Type()]
			if !ok {
				panic(fmt.Sprintf("unknown workload message type %s", msg.Type()))
			}

			err = exec(msg)
			if err != nil {
				panic(fmt.Sprintf("error executing workload message %s: %s", msg.Type(), err.Error()))
			}
		}
	}
}

func marshallUsers(users map[string][]behavior.User) (map[string][]byte, error) {
	result := make(map[string][]byte)
	for k, v := range users {
		logging.Debugf("marshaling users type: %s", k)
		buf, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("failed to encode workload: %w", err)
		}
		result[k] = buf
	}

	return result, nil
}

func (c *Coordinator) registerExecs() {
	c.execs = make(map[string]func(msg messaging.Message) error)
	c.execs[messaging.ResultsType] = c.processResultsMessage
	c.execs[messaging.StoppedType] = c.processStoppedMessage
}

/**
 * helper functions
 */

func flattenUsersMap(m map[string][]behavior.User) []behavior.User {
	result := make([]behavior.User, 0)
	for _, arr := range m {
		result = append(result, arr...)
	}

	return result
}

type ResultsCollector struct {
	sync.RWMutex
	allResults []behavior.Result
	lastIndex  int
}

func NewResultsCollector() *ResultsCollector {
	return &ResultsCollector{
		allResults: make([]behavior.Result, 0),
		lastIndex:  -1,
	}
}

func (r *ResultsCollector) AddResults(results ...behavior.Result) {
	r.Lock()
	defer r.Unlock()

	r.allResults = append(r.allResults, results...)
}

func (r *ResultsCollector) CollectResults() []behavior.Result {
	r.RLock()
	defer r.RUnlock()

	return r.allResults
}

func (r *ResultsCollector) CollectNewResults(earliestSubmit time.Time, latestDone time.Time) []behavior.Result {
	r.RLock()
	defer r.RUnlock()

	if r.lastIndex >= len(r.allResults) || len(r.allResults) == 0 {
		return []behavior.Result{}
	}

	result := make([]behavior.Result, 0)
	for i := r.lastIndex + 1; i < len(r.allResults); i++ {
		if r.allResults[i].Start().After(earliestSubmit) || r.allResults[i].End().Before(latestDone) {
			result = append(result, r.allResults[i])
		}
	}

	r.lastIndex = len(r.allResults)
	logging.Infof("collecting %d results between %s and %s", len(result), earliestSubmit.String(), latestDone.String())
	return result
}

/**
 * Helper functions
 */

func splitUsers(users map[string][]behavior.User) []map[string][]behavior.User {
	results := make([]map[string][]behavior.User, 0)
	currentBatch := make(map[string][]behavior.User)
	currentBatchSize := 0

	for userType, usersList := range users {
		for len(usersList) > 0 {
			availableSpace := usersBatchSize - currentBatchSize
			if availableSpace <= 0 {
				results = append(results, currentBatch)
				currentBatch = make(map[string][]behavior.User)
				currentBatchSize = 0
				availableSpace = usersBatchSize
			}

			if len(usersList) <= availableSpace {
				currentBatch[userType] = append(currentBatch[userType], usersList...)
				currentBatchSize += len(usersList)
				usersList = nil
			} else {
				currentBatch[userType] = append(currentBatch[userType], usersList[:availableSpace]...)
				currentBatchSize += availableSpace
				usersList = usersList[availableSpace:]
			}
		}
	}

	if currentBatchSize > 0 {
		results = append(results, currentBatch)
	}

	return results
}
