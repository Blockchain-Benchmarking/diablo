package workload

import (
	"diablo/core/behavior"
	"errors"
	"fmt"
	"sync"
)

var emptyQueueErr = errors.New("empty queue")

type Queue struct {
	sync.RWMutex
	users Workload
}

func NewQueue(users Workload) *Queue {
	return &Queue{
		users: users,
	}
}

func (q *Queue) Empty() bool {
	q.RLock()
	defer q.RUnlock()
	return len(q.users) == 0
}

func (q *Queue) AddUsers(users Workload) {
	q.Lock()
	defer q.Unlock()
	q.users = append(q.users, users...)
}

func (q *Queue) GetNext(batch int) (Workload, bool) {
	q.Lock()
	defer q.Unlock()

	if len(q.users) == 0 {
		return nil, false
	}

	if batch > len(q.users) {
		batch = len(q.users)
	}

	next := q.users[:batch]
	q.users = q.users[batch:]

	return next, len(q.users) == 0
}

func CreateUsersFromAccounts(accounts []behavior.Account, userType string, blockchain string, userParams map[string]interface{}) (Workload, error) {
	users := make(Workload, len(accounts))

	userTools := Users[userType]

	var addresses []string
	for _, acc := range accounts {
		addresses = append(addresses, acc.Address)
	}

	var err error
	for i, acc := range accounts {
		users[i], err = userTools.Init(blockchain, behavior.Config{
			Endpoint:   "ws://127.0.0.1:9000",
			Addresses:  addresses,
			PrivateKey: acc.PrivateKey,
			Address:    acc.Address,
		}, userParams)
		if err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}
	}

	return users, nil
}
