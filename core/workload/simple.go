package workload

import (
	"diablo/core/behavior"
	"diablo/core/logging"
	"diablo/core/remote"
	"encoding/binary"
	"fmt"
	"io"
	"sync"
	"time"
)

type SimpleCoordinator struct {
	secondaries     []*remote.Secondary
	accounts        []behavior.Account
	userType        string
	app             string
	blockchain      string
	userParams      map[string]interface{}
	emptyResultsGen func() behavior.Results
}

func NewSimpleCoordinator(secondaries []*remote.Secondary, accounts []behavior.Account, userType string, app string, blockchain string, params map[string]interface{}) Coordinator {
	return &SimpleCoordinator{
		secondaries: secondaries,
		accounts:    accounts,
		userType:    userType,
		app:         app,
		blockchain:  blockchain,
		userParams:  params,
	}
}

// SendWorkload implements Coordinator
func (s *SimpleCoordinator) SendWorkload() error {
	wk := make(SimpleWorkload, len(s.accounts))

	logging.Infof("initialize workload")

	//Get specific user initializer
	userTools := Users[s.userType]
	s.emptyResultsGen = userTools.EmptyResults

	var addresses []string
	for _, acc := range s.accounts {
		addresses = append(addresses, acc.Address)
	}

	var err error
	for i, acc := range s.accounts {
		wk[i], err = userTools.Init(s.blockchain, behavior.Config{
			Endpoint:   "ws://127.0.0.1:9000",
			Addresses:  addresses,
			PrivateKey: acc.PrivateKey,
			Address:    acc.Address,
		}, s.userParams)
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
func (s *SimpleCoordinator) CollectResults() behavior.Results {
	finalResults := s.emptyResultsGen()

	for _, sec := range s.secondaries {
		res := s.emptyResultsGen()

		err := res.Decode(sec.Reader())
		if err != nil {
			logging.Errorf("failed to receive results from sec: %s", err.Error())
		}

		finalResults = finalResults.Merge(res)
	}

	return finalResults
}

type SimpleGenerator struct {
	primary *remote.PrimaryConn
	users   []behavior.User
	wg      *sync.WaitGroup
	results chan behavior.Results
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
		make(chan behavior.Results, len(*simpleWk)),
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
func (g *SimpleGenerator) CollectResults() (behavior.Results, error) {
	logging.Infof("waiting for users to finish executing")
	g.wg.Wait()
	close(g.results)
	logging.Infof("users finished executing")

	var results behavior.Results

	for res := range g.results {
		if results == nil {
			results = res
			continue
		}

		if res != nil {
			logging.Infof("merging results")
			results = results.Merge(res)
		}
	}

	return results, nil
}

type SimpleWorkload []behavior.User

// Encode implements Workload
func (w *SimpleWorkload) Encode(dest io.Writer) error {
	err := encodeString(dest, (*w)[0].Name())
	if err != nil {
		return fmt.Errorf("failed to encode usertype: %w", err)
	}

	count := int32(len(*w))
	err = binary.Write(dest, binary.LittleEndian, count)
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
	userType, err := decodeString(src)
	if err != nil {
		return fmt.Errorf("failed to decode usertype: %w", err)
	}

	tools, ok := Users[userType]
	if !ok {
		return fmt.Errorf("unknown user type: %s", userType)
	}

	var count int32
	err = binary.Read(src, binary.LittleEndian, &count)
	if err != nil {
		return err
	}

	*w = make(SimpleWorkload, count)

	for i := int32(0); i < count; i++ {
		userInfo := tools.EmptyUser()
		err = userInfo.Decode(src)
		if err != nil {
			return err
		}
		(*w)[i] = userInfo
	}

	return nil
}
