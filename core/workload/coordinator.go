package workload

import (
	"bufio"
	"bytes"
	"diablo/core/behavior"
	"diablo/core/logging"
	"diablo/core/messaging"
	"diablo/core/network"
	"errors"
	"fmt"
	"strings"
	"sync"
)

type Coordinator struct {
	wg    *sync.WaitGroup
	stop  chan struct{}
	execs map[string]func(msg messaging.Message) error

	secondaries map[string]*network.Secondary

	emptyResult  func() behavior.Results
	results      behavior.Results
	usersDone    chan struct{}
	currentUsers int
}

func NewCoordinator(secondaries map[string]*network.Secondary, emptyResult func() behavior.Results) *Coordinator {
	wg := &sync.WaitGroup{}
	c := &Coordinator{
		wg:   wg,
		stop: make(chan struct{}, 1),

		secondaries: secondaries,

		currentUsers: 0,
		emptyResult:  emptyResult,
		results:      emptyResult(),
		usersDone:    make(chan struct{}),
	}

	c.registerExecs()

	wg.Add(len(secondaries))
	for _, secondary := range secondaries {
		go c.handleGeneratorMessages(secondary)
	}

	return c
}

func (c *Coordinator) LaunchUsers(s *network.Secondary, users Workload) error {
	buf, err := users.encode()
	if err != nil {
		return fmt.Errorf("failed to encode workload: %w", err)
	}

	msg := messaging.Workload{Users: buf}
	err = s.Send(msg)
	if err != nil {
		return fmt.Errorf("failed to send workload: %w", err)
	}

	c.currentUsers += len(users)
	return nil
}

func (c *Coordinator) processResultsMessage(msg messaging.Message) error {
	res, ok := msg.(*messaging.Results)
	if !ok {
		return fmt.Errorf("invalid Results message")
	}

	if c.currentUsers <= 0 {
		return errors.New("received unexpected result")
	}

	results := c.emptyResult()
	err := results.Decode(bufio.NewReader(bytes.NewReader(res.Results)))
	if err != nil {
		return fmt.Errorf("failed to decode results: %w", err)
	}

	c.results = c.results.Merge(results)
	c.currentUsers--

	if c.currentUsers == 0 {
		c.usersDone <- struct{}{}
	}

	return nil
}

// CollectResults returns the results of the launched users after the last call to CollectResults
func (c *Coordinator) CollectResults() behavior.Results {
	<-c.usersDone
	return c.results
}

func (c *Coordinator) Stop() {
	close(c.stop)
	for addr, s := range c.secondaries {
		logging.Infof("sending stop signal to %s", addr)
		err := s.Send(messaging.Stop{})
		if err != nil {
			logging.Errorf("failed to send stop signal to %s: %v", addr, err)
		}
	}
	c.wg.Wait()
}

func (c *Coordinator) handleGeneratorMessages(s *network.Secondary) {
	defer c.wg.Done()

	for {
		select {
		case <-c.stop:
			logging.Infof("stop receiving messages")
			return
		default:
			msg, err := network.ReadMessage(s.Reader()) //todo
			if err != nil {
				if strings.Contains(err.Error(), "EOF") {
					logging.Warnf("EOF from %s", s.Addr())
					return
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

func (c *Coordinator) registerExecs() {
	c.execs = make(map[string]func(msg messaging.Message) error)
	c.execs[messaging.ResultsType] = c.processResultsMessage
}
