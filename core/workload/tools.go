package workload

import (
	"sync"
)

type Queue struct {
	sync.RWMutex
	users Workload
}

func NewQueue(users Workload) *Queue {
	return &Queue{
		users: users,
	}
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
		return nil, true
	}

	if batch > len(q.users) {
		batch = len(q.users)
	}

	next := q.users[:batch]
	q.users = q.users[batch:]

	return next, len(q.users) == 0
}
