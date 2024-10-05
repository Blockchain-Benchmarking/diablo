package workload

import (
	"diablo/core/remote"
	"diablo/core/result"
)

var coordinators = map[string]interface{}{
	"simple": &SimpleCoordinator{},
}

type Coordinator interface {
	SendWorkload() error
	CollectResults() result.Result
}

type SimpleCoordinator struct {
	secondaries []*remote.Secondary
	accounts    []Account
}

func New(secondaries []*remote.Secondary, accounts []Account) *SimpleCoordinator {
	return &SimpleCoordinator{
		secondaries: secondaries,
		accounts:    accounts,
	}
}

func (s *SimpleCoordinator) SendWorkload() error {
	workload := UserInfoWorkload{UserInfos: make([]UserInfo, len(s.accounts))}
	for i, account := range s.accounts {
		workload.UserInfos[i] = UserInfo{
			Account:   account,
			Frequency: 1,
		}
	}

	//TODO share between secondaries
	err := workload.Encode(s.secondaries[0].Writer())
	if err != nil {
		return err
	}

	return nil
}

func (s *SimpleCoordinator) CollectResults() result.Result {
	return nil
}

/**
func (this *remoteSecondary) start(duration float64) error {
	return this.conn.sendStart(&msgStart{
		duration: duration,
	})
}

func (this *remoteSecondary) collect() (*SecondaryResult, error) {
	var msgIact *msgResultInteraction
	var result *SecondaryResult
	var client *remoteClient
	var msg msgResult
	var err error
	var ok bool

	result = newSecondaryResult(this.addr(), this.params.tags)

	for {
		Tracef("pull next result from %s", this.addr())
		msg, err = this.conn.pullResult()
		if err != nil {
			return nil, err
		}

		_, ok = msg.(*msgResultDone)
		if ok {
			break
		}

		msgIact, ok = msg.(*msgResultInteraction)
		if ok {
			Tracef("new interaction result for kind %d on "+
				"client %d for %s", msgIact.ikind,
				msgIact.index, this.addr())

			if msgIact.index >= len(this.clients) {
				return nil, fmt.Errorf("invalid client id "+
					"%d for secondary %s", msgIact.index,
					this.addr())
			}

			client = this.clients[msgIact.index]

			if msgIact.ikind >= len(client.kinds) {
				return nil, fmt.Errorf("invalid interaction "+
					"ikind %d for client %d on "+
					"secondary %s", msgIact.ikind,
					msgIact.index, this.addr())
			}

			result.addResult(msgIact.index, client.kind,
				client.kinds[msgIact.ikind],
				msgIact.submitTime, msgIact.commitTime,
				msgIact.abortTime, msgIact.hasError)

			continue
		}

		return nil, fmt.Errorf("not implemented result message %v", msg)
	}

	Tracef("end of results for %s", this.addr())
	return result, nil
}*/
