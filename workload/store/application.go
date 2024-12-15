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
	return s.client.SendContractTransaction(s.contractAddress, s.abi, "setItem", timeout, common.HexToHash("abc"), common.HexToHash("def"))
}

func (s *Application) GetItem(key string) (string, error) {
	logging.Infof("in getitem")
	var keyBytes [32]byte
	copy(keyBytes[:], "abc")

	result, err := s.client.CallContract(s.contractAddress, s.abi, "items", keyBytes)
	if err != nil {
		return "", fmt.Errorf("failed to get item %s: %w", key, err)
	}

	str := result.(string)
	logging.Infof("got item %s", str)

	return str, nil
}
