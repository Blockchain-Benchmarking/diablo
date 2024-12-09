package payment

import (
	"diablo/blockchain"
	"fmt"
	"time"
)

type Application struct {
	client blockchain.Blockchain
	blockchain.Config
}

func NewPaymentApplication(config blockchain.Config, blockchainName string) (*Application, error) {
	b, ok := blockchain.Blockchains[blockchainName]
	if !ok {
		return nil, fmt.Errorf("blockchain %s not implemented", blockchainName)
	}

	client, err := b.New(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create blockchain client: %w", err)
	}

	return &Application{
		client: client,
		Config: config,
	}, nil
}

func (a *Application) Pay(to string, amount float64, timeout time.Duration) error {
	return a.client.Transfer(amount, to, timeout)
}

func (a *Application) Balance() (float64, error) {
	return a.client.Balance()
}

func (a *Application) GetOthers() []string {
	others := make([]string, 0)
	for _, add := range a.Config.Addresses {
		if add != a.Address {
			others = append(others, add)
		}
	}

	return others
}
