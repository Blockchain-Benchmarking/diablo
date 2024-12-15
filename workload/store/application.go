package store

import (
	"diablo/blockchain"
	"diablo/core/logging"
	"fmt"
	"os"
	"strings"
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
	return s.client.SendContractTransaction(s.contractAddress, s.abi, "setItem", timeout, key, value)
}

func (s *Application) GetItem(key string, timeout time.Duration) (string, error) {
	var keyBytes [32]byte
	copy(keyBytes[:], key)

	var result string
	err := s.client.CallContract(s.contractAddress, s.abi, "items", timeout, &result, key)
	if err != nil {
		return "", fmt.Errorf("failed to get item %s: %w", key, err)
	}

	trimmedResult := strings.TrimRight(result, "\x00")
	fmt.Printf("Decoded result: %s\n", trimmedResult)

	logging.Infof("got item %s", trimmedResult)

	return trimmedResult, nil
}
