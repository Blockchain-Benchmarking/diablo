package simple

import (
	"diablo/core/behavior"
	"diablo/core/benchmark"
	"diablo/core/logging"
	"diablo/core/network"
	"diablo/core/workload"
	"fmt"
)

// Constant load for certain duration
type Benchmark struct {
	Setup
	secondaries map[string]*network.Secondary
	coordinator *workload.Coordinator
	results     chan behavior.Results
}

type Setup struct {
	Blockchain string   `yaml:"blockchain"`
	User       User     `yaml:"user"`
	Tps        int      `yaml:"tps"`
	Endpoints  []string `yaml:"endpoints"`
}

type User struct {
	Name   string                 `yaml:"name"`
	Params map[string]interface{} `yaml:"params"`
}

func NewSimpleBenchmark(setupFile string, secondaries map[string]*network.Secondary) (benchmark.Benchmark, error) {
	setup := &Setup{}
	err := benchmark.ParseSetup(setupFile, setup)
	if err != nil {
		return nil, fmt.Errorf("error parsing setup file: %v", err)
	}

	t, ok := workload.Users[setup.User.Name]
	if !ok {
		return nil, fmt.Errorf("user type '%s' not found", setup.User)
	}

	return &Benchmark{
		Setup:       *setup,
		secondaries: secondaries,
		coordinator: workload.NewCoordinator(secondaries, t.EmptyResults),
	}, nil
}

func (s *Benchmark) StopCoordinator() {
	s.coordinator.Stop()
}

func (s *Benchmark) createUsersFromAccounts(accounts []behavior.Account, tps int) (workload.Workload, error) {
	users := make(workload.Workload, len(accounts))
	userTools := workload.Users[s.User.Name]

	tpsPerUser := tps / len(users)
	remainder := tps % len(users)

	logging.Debugf("tps per user: %d, remainder: %d, accounts: %d, total tps: %d", tpsPerUser, remainder, len(accounts), s.Tps)

	var addresses []string
	for _, acc := range accounts {
		addresses = append(addresses, acc.Address)
	}

	var err error
	for i, acc := range accounts {
		userTps := tpsPerUser
		if remainder > 0 {
			userTps++
			remainder--
		}
		users[i], err = userTools.Init(s.Blockchain, behavior.Config{
			Endpoint:   s.Endpoints[i%len(s.Endpoints)],
			Addresses:  addresses,
			PrivateKey: acc.PrivateKey,
			Address:    acc.Address,
		}, userTps, s.User.Params)
		if err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}
	}

	return users, nil
}
