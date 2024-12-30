package store

import (
	"diablo/blockchain"
	"fmt"
	"os"
	"time"
)

type Application struct {
	client          blockchain.Blockchain
	contractAddress string
	abi             string
}

func New(implementation string, config blockchain.Config, contractAddress string, abiPath string) (*Application, error) {
	b, ok := blockchain.Blockchains[implementation]
	if !ok {
		return nil, fmt.Errorf("blockchain %s not implemented", implementation)
	}

	client, err := b.New(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s blockchain client: %w", implementation, err)
	}

	abi, err := os.ReadFile(abiPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", abiPath, err)
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

	return s.client.SendContractTransaction(s.contractAddress, s.abi, "setItem", timeout, keyBytes, valueBytes)
}

func (s *Application) GetItem(key string, timeout time.Duration) (string, error) {
	var keyBytes [32]byte
	copy(keyBytes[:], key)

	var result [32]byte
	err := s.client.CallContract(s.contractAddress, s.abi, "items", timeout, &result, keyBytes)
	if err != nil {
		return "", fmt.Errorf("failed to get item %s: %w", key, err)
	}

	return string(result[:]), nil
}
