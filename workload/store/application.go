package store

import (
	"diablo/blockchain"
	"diablo/core/logging"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"os"
	"time"
)

const storeAbiPath = "workload/store/contract/Store.abi"

type Application struct {
	client          blockchain.Blockchain
	contractAddress string
	abi             string
}

func New(implementation string, config blockchain.Config, contractAddress string) (*Application, error) {
	b, ok := blockchain.Blockchains[implementation]
	if !ok {
		return nil, fmt.Errorf("blockchain %s not implemented", implementation)
	}

	client, err := b.New(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s blockchain client: %w", implementation, err)
	}

	abi, err := os.ReadFile(storeAbiPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", storeAbiPath, err)
	}

	return &Application{
		client:          client,
		contractAddress: contractAddress,
		abi:             string(abi),
	}, nil
}

func (s *Application) SetItem(key, value string, timeout time.Duration) error {
	var keyBytes [32]byte
	copy(keyBytes[:], key)

	var valueBytes [32]byte
	copy(valueBytes[:], value)

	logging.Infof("calling set with keybytes: %s, valuebytes: %s", string(keyBytes[:]), string(valueBytes[:]))

	return s.client.SendContractTransaction(s.contractAddress, s.abi, "setItem", timeout, common.RightPadBytes([]byte(key), 32), common.RightPadBytes([]byte(value), 32))
}

func (s *Application) GetItem(key string, timeout time.Duration) (string, error) {
	var keyBytes [32]byte
	copy(keyBytes[:], key)

	var result [32]byte
	logging.Infof("calling items on keybytes: %s", string(keyBytes[:]))
	err := s.client.CallContract(s.contractAddress, s.abi, "items", timeout, &result, common.RightPadBytes([]byte(key), 32))
	if err != nil {
		return "", fmt.Errorf("failed to get item %s: %w", key, err)
	}
	logging.Infof("done with callcontract")

	logging.Infof("got item %s", string(result[:]))

	return string(result[:]), nil
}
