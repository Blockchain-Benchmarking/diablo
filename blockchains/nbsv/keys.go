package nbsv

// Pre-funded key + UTXO pool loader.
//
// Benchmarking must NOT call a wallet per transaction (that would benchmark the
// wallet, not the chain). Instead a one-time funding step — e.g. a fan-out tx
// broadcast from Metanet Desktop's BRC-100 interface on :3321 — splits real
// coins into a pool of pre-funded P2PKH outputs across a generated key set,
// which is exported to a keys.yaml the adapter loads here. The hot path then
// signs locally with go-sdk, no wallet round-trip.

import (
	"fmt"
	"os"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"
	"gopkg.in/yaml.v3"
)

// account is the per-benchmark "resource" handed back by CreateAccount. In the
// UTXO model an account is a signing key plus a pool of spendable outputs.
type account struct {
	wif     string
	priv    *ec.PrivateKey
	address string     // P2PKH address string
	lockHex string     // P2PKH locking script hex (for change / chained spends)
	utxos   []*utxoRef // pre-funded, individually reservable outputs
	cursor  int        // reservation cursor; see BlockchainBuilder.reserve
}

type utxoRef struct {
	txid     string
	vout     uint32
	lockHex  string
	satoshis uint64
}

// keys.yaml schema.
type keyfileUTXO struct {
	Txid     string `yaml:"txid"`
	Vout     uint32 `yaml:"vout"`
	Script   string `yaml:"script"`
	Satoshis uint64 `yaml:"satoshis"`
}

type keyfileAccount struct {
	Wif     string        `yaml:"wif"`
	Address string        `yaml:"address"`
	Utxos   []keyfileUTXO `yaml:"utxos"`
}

type keyfile struct {
	// Mainnet selects address encoding; default false (test/regtest).
	Mainnet  bool             `yaml:"mainnet"`
	Accounts []keyfileAccount `yaml:"accounts"`
}

func loadKeyfile(path string) ([]*account, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var kf keyfile
	if err = yaml.Unmarshal(data, &kf); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	accounts := make([]*account, 0, len(kf.Accounts))
	for i, ka := range kf.Accounts {
		priv, err := ec.PrivateKeyFromWif(ka.Wif)
		if err != nil {
			return nil, fmt.Errorf("account %d: bad WIF: %w", i, err)
		}

		address := ka.Address
		if address == "" {
			addr, err := script.NewAddressFromPublicKey(priv.PubKey(),
				kf.Mainnet)
			if err != nil {
				return nil, fmt.Errorf("account %d: derive address: %w",
					i, err)
			}
			address = addr.AddressString
		}

		addrObj, err := script.NewAddressFromString(address)
		if err != nil {
			return nil, fmt.Errorf("account %d: bad address: %w", i, err)
		}
		lock, err := p2pkh.Lock(addrObj)
		if err != nil {
			return nil, fmt.Errorf("account %d: lock script: %w", i, err)
		}

		acc := &account{
			wif:     ka.Wif,
			priv:    priv,
			address: address,
			lockHex: lock.String(),
		}

		for _, ku := range ka.Utxos {
			lockHex := ku.Script
			if lockHex == "" {
				lockHex = acc.lockHex // default: funded to this account's P2PKH
			}
			acc.utxos = append(acc.utxos, &utxoRef{
				txid:     ku.Txid,
				vout:     ku.Vout,
				lockHex:  lockHex,
				satoshis: ku.Satoshis,
			})
		}

		accounts = append(accounts, acc)
	}

	return accounts, nil
}
