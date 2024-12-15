package blockchain

import (
	"context"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"math"
	"math/big"
	"strings"
	"sync"
	"time"
)

type EthereumClient struct {
	client     *ethclient.Client
	nonce      uint64
	nonceMutex sync.Mutex

	Config
}

func (e *EthereumClient) New(config Config) (Blockchain, error) {
	client, err := ethclient.Dial(config.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to dial endpoint %s: %w", config.Endpoint, err)
	}

	nonce, err := client.PendingNonceAt(context.Background(), common.HexToAddress(config.Address))
	if err != nil {
		return nil, fmt.Errorf("failed to get pending nonce of account %s: %w", config.Address, err)
	}

	return &EthereumClient{
		client: client,
		nonce:  nonce,
		Config: config,
	}, nil
}

func (e *EthereumClient) Transfer(amount float64, to string, timeout time.Duration) error {
	privateKey, err := crypto.HexToECDSA(e.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to convert private key: %w", err)
	}

	address := common.HexToAddress(e.Address)
	toAddress := common.HexToAddress(to)

	value := new(big.Int)
	value.SetString(fmt.Sprintf("%.0f", amount*math.Pow(10, 18)), 10)

	balance, err := e.client.BalanceAt(context.Background(), address, nil)
	if err != nil {
		return fmt.Errorf("failed to get account balance: %w", err)
	}

	gasLimit := uint64(21000)
	gasPrice, err := e.client.SuggestGasPrice(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get gas price: %w", err)
	}

	gasCost := new(big.Int).Mul(gasPrice, big.NewInt(int64(gasLimit)))
	totalCost := new(big.Int).Add(value, gasCost)

	if balance.Cmp(totalCost) < 0 {
		return fmt.Errorf("insufficient funds: balance=%s, required=%s", balance.String(), totalCost.String())
	}

	e.nonceMutex.Lock()

	unlock := func() {
		e.nonce++
		e.nonceMutex.Unlock()
	}

	tx := types.NewTx(
		&types.LegacyTx{
			Nonce:    e.nonce,
			GasPrice: gasPrice,
			Gas:      gasLimit,
			To:       &toAddress,
			Value:    value,
		},
	)

	chainID, err := e.client.NetworkID(context.Background())
	if err != nil {
		unlock()
		return err
	}

	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
	if err != nil {
		unlock()
		return err
	}

	err = e.client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		unlock()
		return err
	}

	unlock()

	hash := signedTx.Hash()
	_, err = e.waitForReceipt(hash, timeout)
	if err != nil {
		return err
	}

	return nil
}

func (e *EthereumClient) Balance() (float64, error) {
	balance, err := e.client.BalanceAt(context.Background(), common.HexToAddress(e.Address), nil)
	if err != nil {
		return 0, fmt.Errorf("failed to get balance: %w", err)
	}

	res, _ := balance.Float64()
	return res, nil
}

func (e *EthereumClient) DeployContract(abiString string, bytecode []byte, timeout time.Duration, params ...interface{}) (string, error) {
	privateKey, err := crypto.HexToECDSA(e.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("failed to parse private key: %w", err)
	}

	networkID, err := e.client.NetworkID(context.Background())
	if err != nil {
		return "", fmt.Errorf("failed to fetch network ID: %w", err)
	}

	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, networkID)
	if err != nil {
		return "", fmt.Errorf("failed to create transactor: %w", err)
	}

	gasPrice, err := e.client.SuggestGasPrice(context.Background())
	if err != nil {
		return "", fmt.Errorf("failed to suggest gas price: %w", err)
	}

	auth.GasPrice = gasPrice
	auth.GasLimit = uint64(3000000)

	parsedABI, err := abi.JSON(strings.NewReader(abiString))
	if err != nil {
		return "", fmt.Errorf("failed to parse ABI: %w", err)
	}

	address, tx, _, err := bind.DeployContract(auth, parsedABI, bytecode, e.client, params...)
	if err != nil {
		return "", fmt.Errorf("failed to deploy contract: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	receipt, err := bind.WaitMined(ctx, e.client, tx)
	if err != nil {
		return "", fmt.Errorf("failed to wait for transaction to be mined: %w", err)
	}

	if receipt.Status != 1 {
		return "", fmt.Errorf("contract deployment transaction failed, receipt status is %d", receipt.Status)
	}

	return address.Hex(), nil
}

func (e *EthereumClient) SendContractTransaction(contractAddress string, abiString string, method string, timeout time.Duration, params ...interface{}) error {
	parsedABI, err := abi.JSON(strings.NewReader(abiString))
	if err != nil {
		return fmt.Errorf("failed to parse ABI: %w", err)
	}

	data, err := parsedABI.Pack(method, params...)
	if err != nil {
		return fmt.Errorf("failed to pack parameters: %w", err)
	}

	balance, err := e.client.BalanceAt(context.Background(), common.HexToAddress(e.Config.Address), nil)
	if err != nil {
		return fmt.Errorf("failed to get account balance: %w", err)
	}

	gasLimit := uint64(30000)
	gasPrice, err := e.client.SuggestGasPrice(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get gas price: %w", err)
	}

	gasCost := new(big.Int).Mul(gasPrice, big.NewInt(int64(gasLimit)))

	if balance.Cmp(gasCost) < 0 {
		return fmt.Errorf("insufficient funds: balance=%s, required=%s", balance.String(), gasCost.String())
	}

	e.nonceMutex.Lock()

	defer func() {
		e.nonce++
		e.nonceMutex.Unlock()
	}()

	to := common.HexToAddress(contractAddress)

	tx := types.NewTx(
		&types.LegacyTx{
			Nonce:    e.nonce,
			GasPrice: gasPrice,
			Gas:      gasLimit,
			To:       &to,
			Value:    big.NewInt(0),
			Data:     data,
		},
	)

	chainID, err := e.client.NetworkID(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get network ID: %w", err)
	}

	pk, err := crypto.HexToECDSA(e.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), pk)
	if err != nil {
		return fmt.Errorf("failed to sign transaction: %w", err)
	}

	err = e.client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		return fmt.Errorf("failed to send transaction: %w", err)
	}

	hash := signedTx.Hash()
	_, err = e.waitForReceipt(hash, timeout)
	if err != nil {
		return err
	}

	return nil
}

func (e *EthereumClient) CallContract(contractAddress string, abiString string, method string, timeout time.Duration, result interface{}, params ...interface{}) error {
	parsedABI, err := abi.JSON(strings.NewReader(abiString))
	if err != nil {
		return fmt.Errorf("failed to parse ABI: %w", err)
	}

	data, err := parsedABI.Pack(method, params...)
	if err != nil {
		return fmt.Errorf("failed to pack parameters: %w", err)
	}

	toAddress := common.HexToAddress(contractAddress)
	msg := ethereum.CallMsg{
		To:   &toAddress,
		Data: data,
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err = e.client.Client().CallContext(ctx, &result, "eth_call", toCallArg(msg), "latest")
	if err != nil {
		return fmt.Errorf("eth_call failed: %w", err)
	}

	return nil
}

func (e *EthereumClient) waitForReceipt(txHash common.Hash, timeout time.Duration) (*types.Receipt, error) {
	var receipt *types.Receipt
	var err error

	start := time.Now()

	for {
		time.Sleep(time.Millisecond)

		receipt, err = e.client.TransactionReceipt(context.Background(), txHash)
		if err == nil {
			break
		}

		if errors.Is(err, ethereum.NotFound) {
			if time.Since(start) > timeout {
				return nil, TimeoutError
			}
			continue
		}

		return nil, fmt.Errorf("failed to retrieve receipt: %w", err)
	}

	return receipt, nil
}

// taken from the ethclient packet with "data" field instead of "input"
// TODO add link to issue
func toCallArg(msg ethereum.CallMsg) interface{} {
	arg := map[string]interface{}{
		"from": msg.From,
		"to":   msg.To,
	}
	if len(msg.Data) > 0 {
		arg["data"] = hexutil.Bytes(msg.Data)
	}
	if msg.Value != nil {
		arg["value"] = (*hexutil.Big)(msg.Value)
	}
	if msg.Gas != 0 {
		arg["gas"] = hexutil.Uint64(msg.Gas)
	}
	if msg.GasPrice != nil {
		arg["gasPrice"] = (*hexutil.Big)(msg.GasPrice)
	}
	if msg.GasFeeCap != nil {
		arg["maxFeePerGas"] = (*hexutil.Big)(msg.GasFeeCap)
	}
	if msg.GasTipCap != nil {
		arg["maxPriorityFeePerGas"] = (*hexutil.Big)(msg.GasTipCap)
	}
	if msg.AccessList != nil {
		arg["accessList"] = msg.AccessList
	}
	if msg.BlobGasFeeCap != nil {
		arg["maxFeePerBlobGas"] = (*hexutil.Big)(msg.BlobGasFeeCap)
	}
	if msg.BlobHashes != nil {
		arg["blobVersionedHashes"] = msg.BlobHashes
	}
	return arg
}
