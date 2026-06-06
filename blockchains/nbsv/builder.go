package nbsv

import (
	"bytes"
	"diablo-benchmark/core"
	"fmt"

	sdktx "github.com/bsv-blockchain/go-sdk/transaction"
)

// BlockchainBuilder runs on the Diablo primary. It owns the pre-funded account
// pool and turns scheduled interactions into opaque payloads (the encode step).
type BlockchainBuilder struct {
	logger   core.Logger
	accounts []*account
	used     int
}

func newBuilder(logger core.Logger) *BlockchainBuilder {
	return &BlockchainBuilder{
		logger:   logger,
		accounts: make([]*account, 0),
		used:     0,
	}
}

func (this *BlockchainBuilder) addAccounts(accs []*account) {
	this.accounts = append(this.accounts, accs...)
}

func (this *BlockchainBuilder) CreateAccount(int) (interface{}, error) {
	if this.used >= len(this.accounts) {
		return nil, fmt.Errorf("can only use %d premade accounts",
			len(this.accounts))
	}

	acc := this.accounts[this.used]
	this.used += 1

	return acc, nil
}

func (this *BlockchainBuilder) CreateContract(name string) (interface{}, error) {
	return nil, fmt.Errorf("contracts unsupported on BSV; use the 'script' " +
		"interaction to create custom-script outputs")
}

func (this *BlockchainBuilder) CreateResource(domain string) (core.SampleFactory, bool) {
	return nil, false
}

// reserve pops the next un-reserved UTXO from an account.
//
// THIS IS THE CORE DESIGN SEAM for UTXO benchmarking. Encoding happens once on
// the primary for the whole run, so reservation must be deterministic and
// collision-free: two scheduled interactions must never select the same output
// or they double-spend at trigger time. Today we draw from a finite pre-funded
// pool and error when it is exhausted.
//
// TODO: chain unconfirmed change. Each transfer's change output is itself a
// spendable UTXO whose outpoint is known at encode time (txid = hash of the
// fully-built unsigned tx once inputs/outputs are fixed). Feeding that back
// into the pool lets a single account sustain throughput far beyond its
// pre-funded output count — which is what the high-TPS BSV claim actually
// requires. Until then, pre-split enough outputs to cover the trace.
func (this *BlockchainBuilder) reserve(acc *account) (*utxoRef, error) {
	if acc.cursor >= len(acc.utxos) {
		return nil, fmt.Errorf("account %s UTXO pool exhausted (%d funded): "+
			"pre-split more outputs or implement change-chaining",
			acc.address, len(acc.utxos))
	}

	u := acc.utxos[acc.cursor]
	acc.cursor += 1

	return u, nil
}

// chainChange feeds a transaction's change output back into the account's pool
// so a single account can sustain throughput far past its pre-funded output
// count. The change outpoint is fully determined at encode time: go-sdk signs
// deterministically (RFC6979), so the tx the secondary re-builds + re-signs at
// trigger time is byte-identical — hence the txid computed here is exactly the
// one that lands on chain, and the chained UTXO is real.
//
// The change output is the last output appended by addChangeAndSign; if the fee
// model dropped it (change below dust) there is nothing spendable to chain.
func (this *BlockchainBuilder) chainChange(acc *account, built *sdktx.Transaction) {
	n := len(built.Outputs)
	if n == 0 {
		return
	}
	out := built.Outputs[n-1]
	if !out.Change || out.Satoshis == 0 {
		return
	}

	acc.utxos = append(acc.utxos, &utxoRef{
		txid:     built.TxID().String(),
		vout:     uint32(n - 1),
		lockHex:  out.LockingScript.String(),
		satoshis: out.Satoshis,
	})
}

func (this *BlockchainBuilder) EncodeTransfer(amount int, from, to interface{}, info core.InteractionInfo) ([]byte, error) {
	faccount := from.(*account)
	taccount := to.(*account)

	u, err := this.reserve(faccount)
	if err != nil {
		return nil, err
	}

	tx := newTransferTransaction(faccount.wif, uint64(amount), u,
		taccount.address, faccount.address)

	// Build + sign now (deterministic) to learn the change outpoint and chain
	// it back into the pool. This also surfaces fund-exhaustion at encode time
	// rather than as a broadcast failure on the secondary.
	built, err := tx.getTx()
	if err != nil {
		return nil, err
	}
	this.chainChange(faccount, built)

	var buffer bytes.Buffer
	if err = tx.encode(&buffer); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

func (this *BlockchainBuilder) EncodeInvoke(from, contract interface{}, function string, info core.InteractionInfo) ([]byte, error) {
	return nil, fmt.Errorf("invoke unsupported on BSV; use the 'script' " +
		"interaction for custom-script / cell-engine outputs")
}

func (this *BlockchainBuilder) EncodeInteraction(itype string, expr core.BenchmarkExpression, info core.InteractionInfo) ([]byte, error) {
	switch itype {
	case "script":
		return this.encodeScript(expr, info)
	default:
		return nil, fmt.Errorf("unknown interaction type '%s'", itype)
	}
}

// encodeScript handles a `!script { from, script, satoshis }` interaction: it
// reserves an input from `from` and creates one output carrying the supplied
// locking script. This is the cell-engine / sCrypt capability path.
func (this *BlockchainBuilder) encodeScript(expr core.BenchmarkExpression, info core.InteractionInfo) ([]byte, error) {
	from, err := expr.Field("from").GetResource("account")
	if err != nil {
		return nil, err
	}
	faccount := from.(*account)

	scriptHex, err := expr.Field("script").GetString()
	if err != nil {
		return nil, err
	}

	outSats := 1
	if field, ferr := expr.TryField("satoshis"); ferr == nil {
		outSats, err = field.GetInt()
		if err != nil {
			return nil, err
		}
	}

	u, err := this.reserve(faccount)
	if err != nil {
		return nil, err
	}

	tx := newScriptTransaction(faccount.wif, u, scriptHex, uint64(outSats),
		faccount.address)

	built, err := tx.getTx()
	if err != nil {
		return nil, err
	}
	this.chainChange(faccount, built)

	var buffer bytes.Buffer
	if err = tx.encode(&buffer); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}
