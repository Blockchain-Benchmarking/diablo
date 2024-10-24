package workload

import (
	"diablo/core/behavior"
	"fmt"
	"sync"
)

type Queue struct {
	sync.Mutex
	users []behavior.User
}

func NewQueue(users []behavior.User) *Queue {
	return &Queue{
		users: users,
	}
}

func (q *Queue) AddUsers(users ...behavior.User) {
	q.Lock()
	defer q.Unlock()
	q.users = append(q.users, users...)
}

func (q *Queue) GetNext(batch int) []behavior.User {
	q.Lock()
	defer q.Unlock()

	next := q.users[:batch]
	q.users = q.users[batch:]

	return next
}

func CreateUsersFromAccounts(accounts []behavior.Account, userType string, blockchain string, userParams map[string]interface{}) ([]behavior.User, error) {
	var users []behavior.User

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
