package simple

import (
	"diablo/core/behavior"
	"diablo/core/logging"
	"fmt"
	"time"
)

// TODO move duration ?
func (s *Benchmark) Run(accounts []behavior.Account, _ time.Duration) (behavior.Results, error) {
	wk, err := s.createUsersFromAccounts(accounts, s.Tps)
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

		err = s.coordinator.SendUsers(secondary, usersChunk)
		if err != nil {
			return nil, fmt.Errorf("failed to launch users to secondary %s: %w", addr, err)
		}

		i = i + n
	}

	err = s.coordinator.SendStartToAll(time.Now().Add(5 * time.Second))
	if err != nil {
		return nil, err
	}

	results := s.coordinator.CollectResults() //TODOO
	logging.Infof("benchmark done")
	return results, nil
}
