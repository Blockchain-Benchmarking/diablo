package coordinator

import (
	"diablo/core/logging"
	"diablo/core/remote"
	"diablo/core/user"
	"diablo/core/workload"
	"fmt"
)

type SimpleCoordinator struct {
	secondaries []*remote.Secondary
	accounts    []user.Account
}

func NewSimpleCoordinator(secondaries []*remote.Secondary, accounts []user.Account) *SimpleCoordinator {
	return &SimpleCoordinator{
		secondaries: secondaries,
		accounts:    accounts,
	}
}

func (s *SimpleCoordinator) SendWorkload() error {
	logging.Debugf("sending workload")
	wk := workload.UserInfoWorkload{UserInfos: make([]user.UserInfo, len(s.accounts))}
	for i, account := range s.accounts {
		wk.UserInfos[i] = user.UserInfo{
			Account:   account,
			Frequency: 1,
		}
	}

	logging.Debugf("encoding workload with %d users", len(wk.UserInfos))
	//TODO share between secondaries
	err := wk.Encode(s.secondaries[0].Writer())
	if err != nil {
		return fmt.Errorf("failed to encode workload: %w", err)
	}

	return nil
}

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

		finalResults = finalResults.Merge(res)
	}

	return finalResults
}
