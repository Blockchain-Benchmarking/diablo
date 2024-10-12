package user

import (
	"io"
	"sync"
)

type Account struct {
	Address    string `yaml:"address"`
	PrivateKey string `yaml:"private"`
}

type User interface {
	Run(wg *sync.WaitGroup, results chan interface{})
	GetParameters(app string, blockchain string) (appParams map[string]interface{}, blockchainParams map[string]interface{}, err error)
	Encode(dest io.Writer) error
	Decode(src io.Reader) error
}

type Results interface {
	Encode(dest io.Writer) error
	Decode(src io.Reader) error
	Merge(other Results) Results
	PrintResult(dest io.Writer) error
}
