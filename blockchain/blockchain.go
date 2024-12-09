package blockchain

import "time"

var Blockchains = map[string]Blockchain{
	"ethereum": &EthereumClient{},
}

type Blockchain interface {
	New(config Config) (Blockchain, error)

	Transfer(amount float64, to string, timeout time.Duration) error
	Balance() (float64, error)

	DeployContract(abi string, bytecode []byte, params ...interface{}) (string, error)
	SendContractTransaction(contractAddress string, abi string, method string, timeout time.Duration, params ...interface{}) error
	CallContract(contractAddress string, abi string, method string, params ...interface{}) (interface{}, error)
}

type Account struct {
	Address    string `yaml:"address"`
	PrivateKey string `yaml:"private"`
}

type Config struct {
	Id         string   `json:"id"`
	Endpoint   string   `json:"endpoint"`
	Addresses  []string `json:"addresses"`
	PrivateKey string   `json:"private_key"`
	Address    string   `json:"address"`
}
