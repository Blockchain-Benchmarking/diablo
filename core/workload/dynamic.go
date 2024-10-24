package workload

import (
	"diablo/core/behavior"
	"diablo/core/network"
	"fmt"
)

type DynamicCoordinator struct {
	secondaries     []*network.Secondary
	queue           *Queue
	emptyResultsGen func() behavior.Results
}

func (d DynamicCoordinator) SendWorkload() error {
	//send initial workload for everyone

	//create goroutine that listens to each of the secondary results channel
	//and fetches the next batch of workload from the queue
	//when the queue is empty, send a signal to all secondaries, exit goroutine and close channels (use waitgroup so that secondary doesn't forget about it)

	//TODO implement me
	panic("implement me")
}

func (d DynamicCoordinator) CollectResults() behavior.Results {
	finalResults := d.emptyResultsGen()

	for {

	}

	//for each collected result, send
}

func NewDynamicCoordinator(secondaries []*network.Secondary, accounts []behavior.Account, userType string, blockchain string, userParams map[string]interface{}, workloadParams map[string]interface{}) (Coordinator, error) {
	users, err := CreateUsersFromAccounts(accounts, userType, blockchain, userParams)
	if err != nil {
		return nil, err
	}

	return &DynamicCoordinator{
		queue:       NewQueue(users),
		secondaries: secondaries,
	}, nil
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
