package generator

import (
	"diablo/core/logging"
	"diablo/core/remote"
	"diablo/core/user"
	"diablo/core/workload"
	"github.com/ethereum/go-ethereum/ethclient"
	"sync"
	"time"
)

type SimpleGenerator struct {
	primary *remote.PrimaryConn
	users   []user.User
	wg      *sync.WaitGroup
	results chan interface{}
	timeout time.Duration
}

func NewSimpleGenerator(primary *remote.PrimaryConn, wk *workload.UserInfoWorkload, timeout time.Duration) (*SimpleGenerator, error) {
	users := make([]user.User, len(wk.UserInfos))
	for i, u := range wk.UserInfos {
		//TODO endpoint
		client, err := ethclient.Dial("ws://127.0.0.1:9000")
		if err != nil {
			return nil, err
		}

		logging.Debugf("assigning account with private key length: %d", len(u.Account.PrivateKey))
		users[i] = &user.SimpleUser{
			Client: client,
			Info:   u,
		}
	}

	return &SimpleGenerator{primary, users, &sync.WaitGroup{}, make(chan interface{}, len(users)), timeout}, nil
}

func (g *SimpleGenerator) Start() error {
	g.wg.Add(len(g.users))
	for _, user := range g.users {
		go user.Run(g.wg, g.results)
	}

	return nil
}

func (g *SimpleGenerator) CollectResults() (user.Results, error) {
	//TODO implement me
	logging.Infof("waiting for users to finish executing")
	g.wg.Wait()
	close(g.results)
	logging.Infof("users finished executing")

	var results user.SimpleResult
	for res := range g.results {
		logging.Infof("received results")
		if userResult, ok := res.(user.UserResult); ok {
			results = append(results, userResult)
		} else {
			logging.Warnf("unknown result type received")
		}
	}

	return &results, nil
}
