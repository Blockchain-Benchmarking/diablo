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
	"time"
)

type Generator struct {
	wg      *sync.WaitGroup
	primary *network.PrimaryConn
	execs   map[string]func(msg messaging.Message) error

	duration time.Duration
	users    chan behavior.User
	results  chan behavior.Results
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
		users:    make(chan behavior.User),
		results:  make(chan behavior.Results),
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

	for user := range g.users {
		//TODO add timer / duration
		userWg.Add(1)
		go user.Run(userWg, g.results)
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
		msg, err := network.ReadMessage(g.primary.Reader()) //todo
		if err != nil {
			logging.Errorf("error receiving message: %s", err.Error())
			return
		}

		logging.Infof("received message type %s", msg.Type())

		if msg.Type() == messaging.StopType {
			close(g.users)
			return
		}

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

		for _, u := range *workload {
			g.users <- u
		}
	}

	return nil
}

func (g *Generator) registerExecs() {
	g.execs = make(map[string]func(msg messaging.Message) error)
	g.execs[messaging.WorkloadType] = g.processWorkloadMessage
}
