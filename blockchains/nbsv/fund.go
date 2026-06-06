package nbsv

// Funding-pool builder. Produces the one-time fan-out tx that splits a funded
// master output into a pre-funded P2PKH pool — one output per benchmark account
// — and the matching keys.yaml. Because change-chaining (see builder.go) lets a
// single funded UTXO sustain an account's whole trace, ONE output per account
// is enough; the trace's change feeds itself.
//
// The cmd/nbsv-fund wrapper turns this into a CLI (build + write yaml +
// optionally broadcast via arcade).

import (
	"fmt"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	sdktx "github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"
)

// buildFundingTx fans `masterTxid:masterVout` (value masterSats, P2PKH to the
// master key) out to `n` freshly generated accounts, `satsPerAccount` each,
// with change back to the master. It returns the signed tx and the keyfile that
// records each account's WIF + its funded outpoint (the fan-out txid, vout i).
func BuildFundingTx(masterWif, masterTxid string, masterVout uint32, masterSats uint64, n int, satsPerAccount uint64, mainnet bool, feeRatePerKB uint64) (*sdktx.Transaction, *keyfile, error) {
	if n <= 0 {
		return nil, nil, fmt.Errorf("need at least one account")
	}
	if feeRatePerKB > 0 {
		feeRate = feeRatePerKB // package var; addChangeAndSign reads it
	}

	masterPriv, err := ec.PrivateKeyFromWif(masterWif)
	if err != nil {
		return nil, nil, fmt.Errorf("master wif: %w", err)
	}
	masterAddr, err := script.NewAddressFromPublicKey(masterPriv.PubKey(), mainnet)
	if err != nil {
		return nil, nil, err
	}
	masterLock, err := p2pkh.Lock(masterAddr)
	if err != nil {
		return nil, nil, err
	}

	tx := sdktx.NewTransaction()

	unlock, err := p2pkh.Unlock(masterPriv, nil)
	if err != nil {
		return nil, nil, err
	}
	if err = tx.AddInputFrom(masterTxid, masterVout, masterLock.String(),
		masterSats, unlock); err != nil {
		return nil, nil, err
	}

	kf := &keyfile{Mainnet: mainnet}
	for i := 0; i < n; i++ {
		priv, err := ec.NewPrivateKey()
		if err != nil {
			return nil, nil, err
		}
		addr, err := script.NewAddressFromPublicKey(priv.PubKey(), mainnet)
		if err != nil {
			return nil, nil, err
		}
		if err = tx.PayToAddress(addr.AddressString, satsPerAccount); err != nil {
			return nil, nil, err
		}
		kf.Accounts = append(kf.Accounts, keyfileAccount{
			Wif:     priv.Wif(),
			Address: addr.AddressString,
			Utxos: []keyfileUTXO{{
				Vout:     uint32(i), // account i is output i (change is last)
				Satoshis: satsPerAccount,
			}},
		})
	}

	// change back to master + fee + sign the input.
	if err = addChangeAndSign(tx, masterAddr.AddressString); err != nil {
		return nil, nil, fmt.Errorf("fund: %w (master output too small for %d x %d sats + fee?)", err, n, satsPerAccount)
	}

	txid := tx.TxID().String()
	for i := range kf.Accounts {
		kf.Accounts[i].Utxos[0].Txid = txid
	}

	return tx, kf, nil
}
