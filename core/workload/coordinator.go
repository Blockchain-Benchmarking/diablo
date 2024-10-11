package workload

import (
	"diablo/core/logging"
	"diablo/core/remote"
	"fmt"
)

var coordinators = map[string]interface{}{
	"simple": &SimpleCoordinator{},
}

type SimpleCoordinator struct {
	secondaries []*remote.Secondary
	accounts    []Account
}

func NewSimpleCoordinator(secondaries []*remote.Secondary, accounts []Account) *SimpleCoordinator {
	return &SimpleCoordinator{
		secondaries: secondaries,
		accounts:    accounts,
	}
}

func (s *SimpleCoordinator) SendWorkload() error {
	logging.Debugf("sending workload")
	workload := UserInfoWorkload{UserInfos: make([]UserInfo, len(s.accounts))}
	for i, account := range s.accounts {
		workload.UserInfos[i] = UserInfo{
			Account:   account,
			Frequency: 1,
		}
	}

	logging.Debugf("encoding workload with %d users", len(workload.UserInfos))
	//TODO share between secondaries
	err := workload.Encode(s.secondaries[0].Writer())
	if err != nil {
		return fmt.Errorf("failed to encode workload: %w", err)
	}

	return nil
}

func (s *SimpleCoordinator) CollectResults() Results {
	finalResults := &SimpleResult{}

	for _, sec := range s.secondaries {
		res := &SimpleResult{}

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
