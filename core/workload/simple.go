package workload

import (
	"diablo/core/behavior"
	"diablo/core/logging"
	"diablo/core/network"
	"fmt"
	"sync"
)

type SimpleCoordinator struct {
	secondaries     []*network.Secondary
	emptyResultsGen func() behavior.Results
	wk              Workload
	results         behavior.Results
}

func (*SimpleCoordinator) Run(secondaries []*network.Secondary, accounts []behavior.Account, user string, blockchain string, userParams map[string]interface{}, _ map[string]interface{}) (behavior.Results, error) {
	logging.Infof("initialize workload")

	//Get specific user initializer
	wk, err := CreateUsersFromAccounts(accounts, user, blockchain, userParams)
	if err != nil {
		return nil, err
	}

	userTools := Users[user]
	c := &SimpleCoordinator{
		secondaries:     secondaries,
		emptyResultsGen: userTools.EmptyResults,
		wk:              wk,
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go c.collectResults(wg)

	//TODO generalize how to distribute workload / add  method to workload interface ?
	numSecondaries := len(secondaries)
	numUsers := len(wk)

	chunkSize := (numUsers + numSecondaries - 1) / numSecondaries

	for i, secondary := range secondaries {
		startIndex := i * chunkSize
		if startIndex >= numUsers {
			break
		}

		endIndex := (i + 1) * chunkSize
		if endIndex > numUsers {
			endIndex = numUsers
		}

		var userChunk Workload
		userChunk = wk[startIndex:endIndex]
		logging.Infof("sending workload of size %d", len(userChunk))
		err = userChunk.Send(secondary.Writer())
		if err != nil {
			return nil, fmt.Errorf("failed to send workload to secondary: %w", err)
		}
		logging.Infof("sent workload")
	}
	//start collect results routing
	wg.Wait()
	logging.Infof("coordinator done")
	return c.results, nil
}

// CollectResults implements Coordinator
func (s *SimpleCoordinator) collectResults(wg *sync.WaitGroup) {
	defer wg.Done()

	finalResults := s.emptyResultsGen()

	for i, sec := range s.secondaries {
		logging.Infof("waiting for result from secondary %d", i)
		res := s.emptyResultsGen()

		err := res.Receive(sec.Reader())
		if err != nil {
			logging.Errorf("failed to receive results from sec: %s", err.Error())
		}

		finalResults = finalResults.Merge(res)
		logging.Infof("merged result from secondary %d", i)
	}

	s.results = finalResults
}

type SimpleGenerator struct {
	users   Workload
	results chan behavior.Results
}

func (*SimpleGenerator) Run(primary *network.PrimaryConn, wg *sync.WaitGroup) {
	defer wg.Done()

	logging.Infof("waiting for workload")
	users := &Workload{}
	err := users.Receive(primary.Reader())
	if err != nil {
		logging.Fatalf("failed to decode workload")
	}

	logging.Infof("received workload of size %d", len(*users))

	gen := &SimpleGenerator{
		*users,
		make(chan behavior.Results, len(*users)),
	}

	userWg := &sync.WaitGroup{}
	results := make(chan behavior.Results, 1)
	go gen.collectResults(userWg, results)

	userWg.Add(len(gen.users))
	for _, user := range gen.users {
		go user.Run(userWg, gen.results)
	}

	res := <-results
	err = res.Send(primary.Writer())
	if err != nil {
		logging.Fatalf("failed to send results to primary")
	}
}

func (g *SimpleGenerator) collectResults(wg *sync.WaitGroup, final chan behavior.Results) {
	logging.Infof("waiting for users to finish executing")
	wg.Wait()
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

	final <- results
}
