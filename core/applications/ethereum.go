package applications

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

// ethereum transfer
func Transfer(client *ethclient.Client, addressBytes string, privateKeyBytes string, to string, amount float64) error {
	privateKey, err := crypto.HexToECDSA(privateKeyBytes)
	if err != nil {
		return fmt.Errorf("failed to convert private key: %w", err)
	}

	address := common.HexToAddress(addressBytes)
	toAddress := common.HexToAddress(to)
	nonce, err := client.PendingNonceAt(context.Background(), address)
	if err != nil {
		return fmt.Errorf("failed to get nonce: %w", err)
	}

	logging.Infof("received nonce " + strconv.Itoa(int(nonce)))

	gasPrice, err := client.SuggestGasPrice(context.Background())
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

	chainID, err := client.NetworkID(context.Background())
	if err != nil {
		return err
	}

	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
	if err != nil {
		return err
	}

	logging.Debugf("sending transaction")
	err = client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		return err
	}

	hash := signedTx.Hash()
	timeout := 2 * time.Minute
	start := time.Now()

	logging.Infof("waiting for receipt " + hash.String())

	for {
		time.Sleep(1 * time.Second)

		receipt, err := client.TransactionReceipt(context.Background(), hash)
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
