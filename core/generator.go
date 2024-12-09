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

const resultsBatchSize = 10

type Generator struct {
	wg      *sync.WaitGroup
	primary *network.PrimaryConn
	execs   map[string]func(msg messaging.Message) error

	start    chan time.Time
	duration time.Duration

	runningUsers map[string]behavior.User

	users chan []behavior.User

	results chan behavior.Result

	stop    chan struct{}
	stopped bool
}

func NewGenerator(primary *network.PrimaryConn, duration string) (*Generator, error) {
	d, err := time.ParseDuration(duration)
	if err != nil {
		return nil, fmt.Errorf("failed to parse duration: %w", err)
	}

	g := &Generator{
		wg:      &sync.WaitGroup{},
		primary: primary,

		duration:     d,
		runningUsers: make(map[string]behavior.User), //user IDs => user
		users:        make(chan []behavior.User),     //TODO why is this channel buffered ?
		results:      make(chan behavior.Result),
		start:        make(chan time.Time, 1),
		stop:         make(chan struct{}),
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

	batches := make(map[string][]behavior.Result) //resultType -> results
	for res := range g.results {
		if batches[res.Type()] == nil {
			batches[res.Type()] = make([]behavior.Result, 0)
		}

		batches[res.Type()] = append(batches[res.Type()], res)
		if len(batches[res.Type()]) >= resultsBatchSize {
			//send results
			buf, err := json.Marshal(batches[res.Type()])
			if err != nil {
				logging.Errorf("Failed to encode results: %v", err)
			}

			err = g.primary.Send(messaging.Results{
				Name:    res.Type(),
				Results: buf,
			})
			if err != nil {
				logging.Errorf("Failed to send results: %s", err.Error())
			}

			//remove
			batches[res.Type()] = make([]behavior.Result, 0)
		}
	}

	//send the remaining
	for t, b := range batches {
		buf, err := json.Marshal(b)
		if err != nil {
			logging.Errorf("failed to marshal results: %s", err.Error())
			continue
		}

		err = g.primary.Send(messaging.Results{Name: t, Results: buf})
		if err != nil {
			logging.Errorf("failed to send results: %s", err.Error())
		}
	}

	err := g.primary.Send(messaging.Stopped{})
	if err != nil {
		logging.Errorf("failed to send stopped message: %s", err.Error())
	}

	g.primary.Conn().Close()
	logging.Infof("results collector done")
}

func (g *Generator) usersRunner() {
	defer g.wg.Done()

	userWg := &sync.WaitGroup{}
	pending := make([]behavior.User, 0)

	once := sync.OnceFunc(func() {
		if g.duration > 0 {
			logging.Infof("start users timer")
			timer := time.NewTimer(g.duration)
			go func() {
				<-timer.C
				logging.Infof("close stop users channel")
				if !g.stopped {
					g.stopped = true
					close(g.stop)
				} else {
					logging.Errorf("already stopped")
				}
			}()
		} else {
			logging.Warnf("duration is %d, no timer started", g.duration)
		}
	})

	//loop:
	for {
		select {
		case <-g.stop:
			logging.Infof("wait for users")
			userWg.Wait()
			close(g.results)
			logging.Infof("users done")
			return
		case startTime := <-g.start:
			logging.Infof("received start time: " + startTime.String())
			for _, user := range pending {
				if existing, running := g.runningUsers[user.ID()]; !running {
					userWg.Add(1)
					g.runningUsers[user.ID()] = user
					once()
					go func() {
						waitingTime := time.Until(startTime)
						if waitingTime > 0 {
							time.Sleep(waitingTime)
						} else {
							logging.Infof("starting users with %s delay", (-1 * waitingTime).String())
						}

						go func() {
							user.Run(g.results, g.stop)
							userWg.Done()
						}()
					}()
				} else {
					err := existing.Restart(behavior.RestartInfo{NewParameters: user, RestartTime: startTime})
					if err != nil {
						logging.Errorf("failed to restart user %s: %s", user.ID(), err.Error())
					}
				}
			}
			pending = make([]behavior.User, 0)
			//logging.Debugf("processed all pending users")

		case users := <-g.users:
			pending = append(pending, users...)
		}
	}
}

func (g *Generator) messagesHandler() {
	defer g.wg.Done()

	for {
		select {
		case <-g.stop:
			logging.Infof("exiting messages handler")
			return
		default:
			msg, err := network.ReadMessageWithTimeout(g.primary.Reader(), 0) //todo
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

func (g *Generator) processUsersMessage(msg messaging.Message) error {
	usersMsg, ok := msg.(*messaging.Users)
	if !ok {
		return errors.New("invalid Users message")
	}
	if len(usersMsg.Users) <= 0 {
		logging.Warnf("received users u.Name()message with no users")
		return nil
	}

	users := make(map[string][]behavior.User)
	for t, buf := range usersMsg.Users {
		userType, ok := Users[t]
		if !ok {
			return fmt.Errorf("unknown user type: %s", t)
		}

		var err error
		users[t], err = userType.UnmarshalUsers(buf)
		if err != nil {
			return err
		}

		//logging.Debugf("sending %d users to runner channel", len(users[t]))
		g.users <- users[t]
		//logging.Debugf("users sent to channel")
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

func (g *Generator) processStopMessage(msg messaging.Message) error {
	logging.Infof("processing stop message")

	_, ok := msg.(*messaging.Stop)
	if !ok {
		return fmt.Errorf("invalid Stop message %v", msg)
	}

	if !g.stopped {
		g.stopped = true
		close(g.users)
		close(g.stop)
	} else {
		logging.Errorf("already stopped")
	}

	return nil
}

func (g *Generator) registerExecs() {
	g.execs = make(map[string]func(msg messaging.Message) error)
	g.execs[messaging.UsersType] = g.processUsersMessage
	g.execs[messaging.StartType] = g.processStartMessage
	g.execs[messaging.StopType] = g.processStopMessage
}
