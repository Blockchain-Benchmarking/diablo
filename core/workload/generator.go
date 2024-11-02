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
	"time"
)

type Generator struct {
	wg      *sync.WaitGroup
	primary *network.PrimaryConn
	execs   map[string]func(msg messaging.Message) error

	start    chan time.Time
	duration time.Duration
	users    chan behavior.User
	results  chan behavior.Results
	stop     chan struct{}
}

func NewGenerator(primary *network.PrimaryConn, duration string) (*Generator, error) {
	d, err := time.ParseDuration(duration)
	if err != nil {
		return nil, fmt.Errorf("failed to parse duration: %w", err)
	}

	g := &Generator{
		wg:      &sync.WaitGroup{},
		primary: primary,

		duration: d,
		users:    make(chan behavior.User, 100000),
		results:  make(chan behavior.Results),
		start:    make(chan time.Time, 1),
		stop:     make(chan struct{}),
	}

	g.registerExecs()

	return g, nil
}

func (g *Generator) Run(wg *sync.WaitGroup) {
	defer wg.Done()

	g.wg.Add(3)
	go g.resultsCollector()
	go g.usersRunner()
	go g.messagesHandler()
	g.wg.Wait()

	logging.Infof("generator done")
}

func (g *Generator) resultsCollector() {
	defer g.wg.Done()

	for res := range g.results {
		logging.Infof("sending result to primary")
		buf, err := res.Encode()
		if err != nil {
			logging.Errorf("Failed to encode results: %v", err)
		}

		err = g.primary.Send(messaging.Results{Results: buf})
		if err != nil {
			logging.Errorf("Failed to send results: %v", err)
		}
	}

	logging.Infof("results collector done")
}

func (g *Generator) usersRunner() {
	defer g.wg.Done()

	userWg := &sync.WaitGroup{}

	once := sync.OnceFunc(func() {
		logging.Infof("start users timer")
		timer := time.NewTimer(g.duration)
		go func() {
			<-timer.C
			logging.Infof("close stop users channel")
			close(g.stop)
		}()
	})

	startTime := <-g.start
	logging.Infof("received start time: " + startTime.String())
	waitingTime := time.Until(startTime)

	if waitingTime > 0 {
		logging.Infof(waitingTime.String() + " until start")
		time.Sleep(waitingTime)
		logging.Infof("starting users runner")
	} else {
		logging.Infof("starting users runner with %s delay", (-1 * waitingTime).String())
	}

loop:
	for {
		select {
		case <-g.stop:
			break loop
		case user := <-g.users:
			once()
			userWg.Add(1)
			go user.Run(userWg, g.results, g.stop)
		}
	}

	logging.Infof("wait for users")
	userWg.Wait()
	close(g.results)
	logging.Infof("users done")
	return
}

func (g *Generator) messagesHandler() {
	defer g.wg.Done()

	for {
		select {
		case <-g.stop:
			return
		default:
			msg, err := network.ReadMessageWithTimeout(g.primary.Conn(), 3*time.Second) //todo
			if err != nil {
				if strings.Contains(err.Error(), "EOF") {
					logging.Warnf("EOF from primary")
					return
				} else if errors.Is(err, network.ErrTimeout) || strings.Contains(err.Error(), "timeout") {
					continue
				}
				logging.Fatalf("error receiving message from: %s", err.Error())
				continue
			}

			logging.Infof("received %s message", msg.Type())

			/**
			if msg.Type() == messaging.StopType {
				close(g.users)
				return
			}*/

			exec, ok := g.execs[msg.Type()]
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

func (g *Generator) processWorkloadMessage(msg messaging.Message) error {
	workloadMsg, ok := msg.(*messaging.Workload)
	if !ok {
		return errors.New("invalid Workload message")
	}

	if len(workloadMsg.Users) > 0 {
		workload := &Workload{}
		err := workload.Receive(bufio.NewReader(bytes.NewReader(workloadMsg.Users)))
		if err != nil {
			return fmt.Errorf("failed to decode workload: %w", err)
		}

		//TODO

		for _, u := range *workload {
			g.users <- u
		}
	}

	return nil
}

func (g *Generator) processStartMessage(msg messaging.Message) error {
	logging.Infof("processing start message")

	startMsg, ok := msg.(*messaging.Start)
	if !ok {
		return fmt.Errorf("invalid Start message %v", msg)
	}

	g.start <- time.Unix(startMsg.Start, 0)
	logging.Infof("finished processing start message")
	return nil
}

func (g *Generator) registerExecs() {
	g.execs = make(map[string]func(msg messaging.Message) error)
	g.execs[messaging.WorkloadType] = g.processWorkloadMessage
	g.execs[messaging.StartType] = g.processStartMessage
}
