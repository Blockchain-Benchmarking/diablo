package user

import (
	"sync"
)

type Account struct {
	Address    string `yaml:"address"`
	PrivateKey string `yaml:"private"`
}

type User interface {
	Run(wg *sync.WaitGroup, results chan interface{})
}
