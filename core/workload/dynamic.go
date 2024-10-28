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
	"sync"
	"sync/atomic"
)

type DynamicCoordinator struct {
	secondaries map[string]*network.Secondary

	wg      *sync.WaitGroup
	results behavior.Results

	queue  *Queue //workload queue
	config DynamicConfig

	msg chan messaging.Message // messages received to be handled

	emptyResultsGen func() behavior.Results
	stop            chan struct{}
}

func (*DynamicCoordinator) Run(secondaries []*network.Secondary, accounts []behavior.Account, userType string, blockchain string, userParams map[string]interface{}, configParams map[string]interface{}) (behavior.Results, error) {
	secMap := make(map[string]*network.Secondary, len(secondaries))
	for _, secondary := range secondaries {
		secMap[secondary.Addr()] = secondary
	}

	users, err := CreateUsersFromAccounts(accounts, userType, blockchain, userParams)
	if err != nil {
		return nil, err
	}

	config, err := ParseDynamicConfig(configParams)
	if err != nil {
		return nil, fmt.Errorf("failed to parse dynamic config: %w", err)
	}

	c := DynamicCoordinator{
		secondaries: secMap,
		wg:          &sync.WaitGroup{},
		stop:        make(chan struct{}),

		msg: make(chan messaging.Message),

		queue:  NewQueue(users),
		config: *config,
	}

	c.wg.Add(len(secondaries))
	for _, secondary := range secondaries {
		go c.handleGeneratorMessages(secondary)
		err = c.sendUsers(secondary.Addr())
		if err != nil {
			return nil, fmt.Errorf("failed to send users: %w", err)
		}
	}

	c.wg.Wait()
	return c.results, nil
}

func (d *DynamicCoordinator) sendUsers(addr string) error {
	secondary := d.secondaries[addr]

	users, empty := d.queue.GetNext(d.config.BatchSize)
	if users == nil {
		return emptyQueueErr
	}

	buf, err := users.encode()
	if err != nil {
		return fmt.Errorf("failed to encode workload: %w", err)
	}

	wk := messaging.Workload{
		Users: buf,
		Done:  empty,
	}

	err = secondary.Send(wk)
	if err != nil {
		return err
	}

	return nil
}

func (d *DynamicCoordinator) handleGeneratorMessages(s *network.Secondary) {
	defer d.wg.Done()

	for {
		select {
		case <-d.stop:
			logging.Infof("stop receiving messages")
			return
		default:
			msg, err := network.ReadMessage(s.Reader()) //todo
			if err != nil {
				logging.Errorf("error receiving message: %s", err.Error())
				return
			}

			logging.Infof("sending msg type to channel %s", msg.Type())
			switch msg.Type() {
			case messaging.MoreType:
				more := msg.(*messaging.More)
				err := d.handleMoreMessage(more)
				if err != nil {
					logging.Errorf("failed to handle More message from %s: %s", more.Source, err.Error())
				}
			case messaging.ResultsType:
				close(d.stop)
				logging.Fatalf("weee received results, should implement")

			default:
				logging.Errorf("unexpected message type: %s", msg.Type())
			}

		}
	}
}

func (d *DynamicCoordinator) handleMoreMessage(msg *messaging.More) error {
	err := d.sendUsers(msg.Source)
	if err != nil && !errors.Is(err, emptyQueueErr) {
		return err
	}

	//if d.queue.Empty() {
	//	return d.sendStop()
	//}

	return nil
}

type DynamicConfig struct {
	InitialLoad int `yaml:"init"`
	BatchSize   int `yaml:"batch"`
}

func ParseDynamicConfig(config map[string]interface{}) (*DynamicConfig, error) {
	initialLoad, ok := config["init"].(int)
	if !ok {
		return nil, fmt.Errorf("`init` parameter should be specified")
	}

	batchSize, ok := config["batch"].(int)
	if !ok {
		return nil, fmt.Errorf("`batch` parameter should be specified")
	}

	return &DynamicConfig{
		InitialLoad: initialLoad,
		BatchSize:   batchSize,
	}, nil
}

/**
 * ----------------------------------
 * -----------Generator--------------
 * ----------------------------------
 */

type DynamicGenerator struct {
	stopCh  chan struct{}
	users   chan behavior.User
	results chan behavior.Results
	current atomic.Int32
	primary *network.PrimaryConn
}

func (*DynamicGenerator) Run(primary *network.PrimaryConn, wg *sync.WaitGroup) {
	defer wg.Done()

	g := DynamicGenerator{
		stopCh:  make(chan struct{}),
		users:   make(chan behavior.User),
		results: make(chan behavior.Results),
		current: atomic.Int32{},
		primary: primary,
	}

	lwg := &sync.WaitGroup{}
	lwg.Add(3)
	go g.handleCoordinatorMessages(lwg)
	go g.collectResults(lwg)
	go g.runUsers(lwg)
	lwg.Wait()
}

func (d *DynamicGenerator) runUsers(wg *sync.WaitGroup) {
	defer wg.Done()

	userWg := &sync.WaitGroup{}
	for user := range d.users {
		userWg.Add(1)
		d.current.Add(1)
		go user.Run(userWg, d.results)
	}

	userWg.Wait()
	close(d.results)
}

func (d *DynamicGenerator) collectResults(wg *sync.WaitGroup) {
	defer wg.Done()

	var results behavior.Results
	for res := range d.results {
		d.current.Add(-1)
		if d.current.Load() == 0 {
			err := d.primary.Send(messaging.More{Source: d.primary.LocalAddr()})
			if err != nil {
				logging.Fatalf("failed to request more users from primary: %s", err.Error())
			}
		}

		if results == nil {
			results = res
			continue
		}

		if res != nil {
			logging.Infof("merging results")
			results = results.Merge(res)
		}
	}

	err := d.reportResults(results)
	if err != nil {
		logging.Errorf("failed to report results to primary: %s", err.Error())
	}
}

func (d *DynamicGenerator) reportResults(results behavior.Results) error {
	buf, err := results.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode results: %w", err)
	}

	err = d.primary.Send(messaging.Results{Results: buf})
	if err != nil {
		return fmt.Errorf("failed to send results to primary: %w", err)
	}

	return nil
}

func (d *DynamicGenerator) handleCoordinatorMessages(wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case <-d.stopCh:
			logging.Infof("stop receiving messages")
			return
		default:
			msg, err := network.ReadMessage(d.primary.Reader()) //todo
			if err != nil {
				logging.Errorf("error receiving message: %s", err.Error())
				return
			}

			logging.Infof("sending msg type to channel %s", msg.Type())
			switch msg.Type() {
			case messaging.WorkloadType:
				more := msg.(*messaging.Workload)
				err := d.handleWorkloadMessage(more)
				if err != nil {
					logging.Errorf("failed to handle Workload message from primary: %s", err.Error())
				}
			default:
				logging.Errorf("unexpected message type: %s", msg.Type())
			}
		}
	}
}

func (d *DynamicGenerator) handleWorkloadMessage(msg *messaging.Workload) error {
	workload := &Workload{}
	err := workload.Receive(bufio.NewReader(bytes.NewReader(msg.Users)))
	if err != nil {
		return fmt.Errorf("failed to decode workload: %w", err)
	}

	for _, u := range *workload {
		d.users <- u
	}

	if msg.Done {
		logging.Infof("received workload message with done flag, stopping reception")
		d.StopReceiving()
	}

	return nil
}

func (d *DynamicGenerator) StopReceiving() {
	close(d.stopCh)
	close(d.users)
}
