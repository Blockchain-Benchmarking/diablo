package workload

import (
	"diablo/core/logging"
	"diablo/core/remote"
	"diablo/core/user"
	"encoding/binary"
	"fmt"
	"io"
	"sync"
	"time"
)

type SimpleCoordinator struct {
	secondaries []*remote.Secondary
	accounts    []user.Account
	userType    string
	app         string
	blockchain  string
}

func NewSimpleCoordinator(secondaries []*remote.Secondary, accounts []user.Account, userType string, app string, blockchain string) Coordinator {
	return &SimpleCoordinator{
		secondaries: secondaries,
		accounts:    accounts,
		userType:    userType,
		app:         app,
		blockchain:  blockchain,
	}
}

// SendWorkload implements Coordinator
func (s *SimpleCoordinator) SendWorkload() error {
	wk := make(SimpleWorkload, len(s.accounts))

	logging.Infof("initialize workload")

	addresses := make([]string, len(s.accounts))
	for i, account := range s.accounts {
		addresses[i] = account.Address
	}

	var err error
	for i, account := range s.accounts {
		wk[i], err = Users[s.userType](account, addresses, s.app, s.blockchain)
		if err != nil {
			return fmt.Errorf("failed to create user: %w", err)
		}
	}

	//TODO share between secondaries
	err = wk.Encode(s.secondaries[0].Writer())
	if err != nil {
		return fmt.Errorf("failed to encode workload: %w", err)
	}

	return nil
}

// CollectResults implements Coordinator
func (s *SimpleCoordinator) CollectResults() user.Results {
	finalResults := &user.SimpleResult{}

	for _, sec := range s.secondaries {
		res := &user.SimpleResult{}

		err := res.Decode(sec.Reader())
		if err != nil {
			logging.Errorf("failed to receive results from sec: %s", err.Error())
		}

		if res == nil {
			logging.Errorf("received nil results from sec")
		}

		finalResults = finalResults.Merge(res).(*user.SimpleResult)
	}

	return finalResults
}

type SimpleGenerator struct {
	primary *remote.PrimaryConn
	users   []user.User
	wg      *sync.WaitGroup
	results chan interface{}
	timeout time.Duration
}

func NewSimpleGenerator(primary *remote.PrimaryConn, wk Workload, timeout time.Duration) (Generator, error) {
	simpleWk, ok := wk.(*SimpleWorkload)
	if !ok {
		return nil, fmt.Errorf("expected SimpleWorkload, got something else")
	}

	return &SimpleGenerator{
		primary,
		*simpleWk,
		&sync.WaitGroup{},
		make(chan interface{}, len(*simpleWk)),
		timeout}, nil
}

// Start implements Generator
func (g *SimpleGenerator) Start() error {
	g.wg.Add(len(g.users))
	for _, user := range g.users {
		go user.Run(g.wg, g.results)
	}

	return nil
}

// CollectResults implements Generator
func (g *SimpleGenerator) CollectResults() (user.Results, error) {
	logging.Infof("waiting for users to finish executing")
	g.wg.Wait()
	close(g.results)
	logging.Infof("users finished executing")

	var results user.SimpleResult
	for res := range g.results {
		logging.Infof("received results")
		if userResult, ok := res.(user.SingleUserResult); ok {
			results = append(results, userResult)
		} else {
			logging.Warnf("unknown result type received")
		}
	}

	return &results, nil
}

type SimpleWorkload []user.User

// Encode implements Workload
func (w *SimpleWorkload) Encode(dest io.Writer) error {
	count := int32(len(*w))
	err := binary.Write(dest, binary.LittleEndian, count)
	if err != nil {
		return fmt.Errorf("failed to encode SimpleWorkload count %d: %w", count, err)
	}

	for _, u := range *w {
		err = u.Encode(dest)
		if err != nil {
			return fmt.Errorf("failed to encode userInfo: %w", err)
		}
	}

	return nil
}

// Decode implements Workload
func (w *SimpleWorkload) Decode(src io.Reader) error {
	logging.Debugf("decoding user info workload")
	var count int32
	err := binary.Read(src, binary.LittleEndian, &count)
	if err != nil {
		return err
	}

	*w = make(SimpleWorkload, count)

	for i := int32(0); i < count; i++ {
		logging.Debugf("decoding user info")
		userInfo := &user.StubbornUser{}
		err = userInfo.Decode(src)
		if err != nil {
			return err
		}
		logging.Debugf("decodied user info with pk len %d", len(userInfo.Account.PrivateKey))
		(*w)[i] = userInfo
	}

	return nil
}
