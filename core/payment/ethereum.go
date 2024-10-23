package payment

import (
	"context"
	"diablo/core/behavior"
	"diablo/core/logging"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"math/big"
	"strconv"
	"time"
)

type EthereumPaymentApplication struct {
	client *ethclient.Client
	behavior.Config
}

// NewEthereumPaymentApplication creates a new ethereum payment application instance
func NewEthereumPaymentApplication(config behavior.Config) (PaymentApplication, error) {
	app := &EthereumPaymentApplication{
		Config: config,
	}

	var err error
	app.client, err = ethclient.Dial(config.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create Ethereum client: %v", err)
	}

	return app, nil
}

func (e *EthereumPaymentApplication) Pay(to string, amount float64, timeout time.Duration) error {
	logging.Infof("transfering %f from %s to %s", amount, e.Address, to)
	privateKey, err := crypto.HexToECDSA(e.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to convert private key: %w", err)
	}

	address := common.HexToAddress(e.Address)
	toAddress := common.HexToAddress(to)

	logging.Infof("getting nonce at %s", address.String())
	nonce, err := e.client.PendingNonceAt(context.Background(), address)
	if err != nil {
		return fmt.Errorf("failed to get nonce: %w", err)
	}

	logging.Infof("received nonce " + strconv.Itoa(int(nonce)))

	gasPrice, err := e.client.SuggestGasPrice(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get gas price: %w", err)
	}

	gasLimit := uint64(21000)
	//1 eth
	value := big.NewInt(1000000000000000000)

	tx := types.NewTx(
		&types.LegacyTx{
			Nonce:    nonce,
			GasPrice: gasPrice,
			Gas:      gasLimit,
			To:       &toAddress,
			Value:    value,
		},
	)

	chainID, err := e.client.NetworkID(context.Background())
	if err != nil {
		return err
	}

	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
	if err != nil {
		return err
	}

	logging.Debugf("sending transaction")
	err = e.client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		return err
	}

	hash := signedTx.Hash()
	start := time.Now()

	logging.Infof("waiting for receipt " + hash.String())

	for {
		time.Sleep(time.Millisecond)

		receipt, err := e.client.TransactionReceipt(context.Background(), hash)
		if err == nil {
			logging.Infof("transaction status: %d", receipt.Status)
			break
		}

		if errors.Is(err, ethereum.NotFound) {
			if time.Since(start) > timeout {
				return errors.New("transaction not mined within timeout")
			}
			continue
		}

		return fmt.Errorf("failed to retrieve receipt: %w", err)
	}

	return nil
}

func (e *EthereumPaymentApplication) Balance() (float64, error) {
	balance, err := e.client.BalanceAt(context.Background(), common.HexToAddress(e.Address), nil)
	if err != nil {
		return 0, fmt.Errorf("failed to get balance: %w", err)
	}

	res, _ := balance.Float64()
	return res, nil
}

func (e *EthereumPaymentApplication) GetOthers() []string {
	others := make([]string, 0)
	for _, add := range e.Config.Addresses {
		if add != e.Address {
			others = append(others, add)
		}
	}

	return others
}
