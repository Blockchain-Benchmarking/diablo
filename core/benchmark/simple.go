package benchmark

import (
	"diablo/core/behavior"
	"diablo/core/logging"
	"diablo/core/network"
	"diablo/core/workload"
	"fmt"
	"time"
)

// Constant load for certain duration
type SimpleBenchmark struct {
	SimpleSetup
	secondaries map[string]*network.Secondary
	coordinator *workload.Coordinator
	results     chan behavior.Results
}

type SimpleSetup struct {
	Blockchain string   `yaml:"blockchain"`
	User       User     `yaml:"user"`
	Tps        int      `yaml:"tps"`
	Endpoints  []string `yaml:"endpoints"`
}

type User struct {
	Name   string                 `yaml:"name"`
	Params map[string]interface{} `yaml:"params"`
}

func NewSimpleBenchmark(setupFile string, secondaries map[string]*network.Secondary) (Benchmark, error) {
	setup := &SimpleSetup{}
	err := ParseSetup(setupFile, setup)
	if err != nil {
		return nil, fmt.Errorf("error parsing setup file: %v", err)
	}

	t, ok := workload.Users[setup.User.Name]
	if !ok {
		return nil, fmt.Errorf("user type '%s' not found", setup.User)
	}

	return &SimpleBenchmark{
		SimpleSetup: *setup,
		secondaries: secondaries,
		coordinator: workload.NewCoordinator(secondaries, t.EmptyResults),
	}, nil
}

func (s *SimpleBenchmark) Run(accounts []behavior.Account, duration time.Duration) (behavior.Results, error) {
	wk, err := s.createUsersFromAccounts(accounts, duration)
	if err != nil {
		return nil, fmt.Errorf("failed to create users: %w", err)
	}

	perSecondary := len(wk) / len(s.secondaries)
	remainder := len(wk) % len(s.secondaries)

	i := 0
	for addr, secondary := range s.secondaries {
		n := perSecondary
		if remainder > 0 {
			n++
			remainder--
		}

		usersChunk := wk[i : i+n]
		logging.Infof("sending %d users to %s", len(usersChunk), addr)

		err = s.coordinator.LaunchUsers(secondary, usersChunk)
		if err != nil {
			return nil, fmt.Errorf("failed to launch users to secondary %s: %w", addr, err)
		}

		i = i + n
	}

	results := s.coordinator.CollectResults()

	s.coordinator.Stop()

	return results, nil
}

func (s *SimpleBenchmark) createUsersFromAccounts(accounts []behavior.Account, duration time.Duration) (workload.Workload, error) {
	users := make(workload.Workload, len(accounts))
	userTools := workload.Users[s.User.Name]

	total := int(float64(s.Tps) * duration.Seconds())
	transactionsPerUser := total / len(accounts)
	remainder := total % len(accounts)

	logging.Debugf("transactions per user: %d, remainder: %d, accounts: %d, tps: %d, duration: %s", transactionsPerUser, remainder, len(accounts), s.Tps, duration.String())

	var addresses []string
	for _, acc := range accounts {
		addresses = append(addresses, acc.Address)
	}

	var err error
	for i, acc := range accounts {
		transactions := transactionsPerUser
		if remainder > 0 {
			transactions++
			remainder--
		}
		users[i], err = userTools.Init(s.Blockchain, behavior.Config{
			Endpoint:   s.Endpoints[i%len(s.Endpoints)],
			Addresses:  addresses,
			PrivateKey: acc.PrivateKey,
			Address:    acc.Address,
		}, transactions, s.User.Params)
		if err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}
	}

	return users, nil
}
