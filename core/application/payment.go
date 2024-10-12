package application

import (
	"context"
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

var PaymentBlockchains = map[string]func(params map[string]interface{}) (PaymentBlockchain, error){
	"ethereum": NewEthereum,
}

type PaymentBlockchain interface {
	Transfer(to string, amount float64) error
}

type PaymentApp struct {
	blockchain PaymentBlockchain
	others     []string
}

func CreatePaymentApp(appParams map[string]interface{}, blockchainParams map[string]interface{}) (Application, error) {
	p := &PaymentApp{}
	if others, ok := appParams["others"].([]string); ok {
		p.others = others
	} else {
		return nil, fmt.Errorf("missing or invalid others parameter")
	}

	if blockchain, ok := appParams["blockchain"].(string); ok {
		init, ok := PaymentBlockchains[blockchain]
		if !ok {
			return nil, fmt.Errorf("payment application does not implement %s blockchain", blockchain)
		}

		var err error
		p.blockchain, err = init(blockchainParams)
		if err != nil {
			return nil, fmt.Errorf("failed to init blockchain: %w", err)
		}
	} else {
		return nil, fmt.Errorf("missing or invalid blockchain parameter")
	}

	return p, nil
}

func (p *PaymentApp) Execute(params map[string]interface{}) error {
	to, ok := params["to"].(string)
	if !ok {
		return fmt.Errorf("missing or invalid parameter \"to\"")
	}

	amount, ok := params["amount"].(float64)
	if !ok {
		return fmt.Errorf("missing or invalid parameter \"amount\"")
	}

	return p.blockchain.Transfer(to, amount)
}

type Ethereum struct {
	client     *ethclient.Client
	privateKey string
	address    string
}

func NewEthereum(params map[string]interface{}) (PaymentBlockchain, error) {
	endpoint, ok := params["endpoint"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid parameter \"endpoint\"")
	}

	privateKey, ok := params["privateKey"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid parameter \"privateKey\"")
	}

	address, ok := params["address"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid parameter \"address\"")
	}

	logging.Infof("dialing client at %s", endpoint)
	client, err := ethclient.Dial(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum client %s: %v", endpoint, err)
	}

	return &Ethereum{
		client:     client,
		privateKey: privateKey,
		address:    address,
	}, nil
}

// Transfer implements PaymentBlockchain
func (e *Ethereum) Transfer(to string, amount float64) error {
	logging.Infof("transfering %f from %s to %s", amount, e.address, to)
	privateKey, err := crypto.HexToECDSA(e.privateKey)
	if err != nil {
		return fmt.Errorf("failed to convert private key: %w", err)
	}

	address := common.HexToAddress(e.address)
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
	timeout := 2 * time.Minute
	start := time.Now()

	logging.Infof("waiting for receipt " + hash.String())

	//TODO
	for {
		time.Sleep(1 * time.Second)

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
