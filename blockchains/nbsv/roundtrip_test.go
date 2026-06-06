package nbsv

// Offline proof that the encode -> ship -> decode -> sign pipeline produces a
// valid, fully-signed BSV transaction without touching a node. This is the
// primary-to-secondary contract exercised end-to-end.

import (
	"bytes"
	"fmt"
	"testing"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"
)

// makeFundedAccount generates a key and a single pre-funded P2PKH UTXO for it.
func makeFundedAccount(t *testing.T, sats uint64) *account {
	t.Helper()

	priv, err := ec.NewPrivateKey()
	if err != nil {
		t.Fatalf("NewPrivateKey: %v", err)
	}

	addr, err := script.NewAddressFromPublicKey(priv.PubKey(), false)
	if err != nil {
		t.Fatalf("NewAddressFromPublicKey: %v", err)
	}

	lock, err := p2pkh.Lock(addr)
	if err != nil {
		t.Fatalf("p2pkh.Lock: %v", err)
	}

	return &account{
		wif:     priv.Wif(),
		priv:    priv,
		address: addr.AddressString,
		lockHex: lock.String(),
		utxos: []*utxoRef{{
			// arbitrary 32-byte outpoint; never broadcast in this test.
			txid:     "a1b2c3d4e5f600112233445566778899aabbccddeeff00112233445566778899",
			vout:     0,
			lockHex:  lock.String(),
			satoshis: sats,
		}},
	}
}

func TestTransferRoundTrip(t *testing.T) {
	from := makeFundedAccount(t, 100000)
	to := makeFundedAccount(t, 0)

	// encode on the "primary"
	encTx := newTransferTransaction(from.wif, 1000, from.utxos[0],
		to.address, from.address)
	var buf bytes.Buffer
	if err := encTx.encode(&buf); err != nil {
		t.Fatalf("encode: %v", err)
	}
	wire := buf.Bytes()

	// type byte must be the transfer discriminator
	if wire[0] != txTypeTransfer {
		t.Fatalf("wire type = %d, want %d", wire[0], txTypeTransfer)
	}

	// decode on the "secondary"
	dec, err := decodeTransaction(bytes.NewBuffer(wire))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	// build + sign
	stx, err := dec.getTx()
	if err != nil {
		t.Fatalf("getTx: %v", err)
	}

	if len(stx.Inputs) != 1 {
		t.Fatalf("inputs = %d, want 1", len(stx.Inputs))
	}
	// payment output + change output
	if len(stx.Outputs) != 2 {
		t.Fatalf("outputs = %d, want 2 (payment + change)", len(stx.Outputs))
	}
	if stx.Outputs[0].Satoshis != 1000 {
		t.Fatalf("payment = %d sats, want 1000", stx.Outputs[0].Satoshis)
	}
	// change must be positive (input 100000 - 1000 - fee)
	if stx.Outputs[1].Satoshis == 0 {
		t.Fatalf("change output is zero; fee/change math is wrong")
	}
	// every input must be signed
	if stx.Inputs[0].UnlockingScript == nil ||
		len(*stx.Inputs[0].UnlockingScript) == 0 {
		t.Fatalf("input 0 has no unlocking script; signing failed")
	}
	if stx.Hex() == "" {
		t.Fatalf("serialized tx is empty")
	}
}

func TestScriptRoundTrip(t *testing.T) {
	from := makeFundedAccount(t, 100000)

	// minimal custom locking script: OP_DROP OP_TRUE (0x75 0x51) over a pushed
	// data item — stands in for a cell-engine / PushDrop output.
	customScriptHex := "047465737475" + "51" // PUSH "test" OP_DROP OP_TRUE

	encTx := newScriptTransaction(from.wif, from.utxos[0], customScriptHex,
		500, from.address)
	var buf bytes.Buffer
	if err := encTx.encode(&buf); err != nil {
		t.Fatalf("encode: %v", err)
	}
	wire := buf.Bytes()

	if wire[0] != txTypeScript {
		t.Fatalf("wire type = %d, want %d", wire[0], txTypeScript)
	}

	dec, err := decodeTransaction(bytes.NewBuffer(wire))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	stx, err := dec.getTx()
	if err != nil {
		t.Fatalf("getTx: %v", err)
	}

	if len(stx.Outputs) != 2 {
		t.Fatalf("outputs = %d, want 2 (script + change)", len(stx.Outputs))
	}
	if stx.Outputs[0].Satoshis != 500 {
		t.Fatalf("script output = %d sats, want 500", stx.Outputs[0].Satoshis)
	}
	if stx.Outputs[0].LockingScript.String() != customScriptHex {
		t.Fatalf("script output locking script = %s, want %s",
			stx.Outputs[0].LockingScript.String(), customScriptHex)
	}
	if stx.Inputs[0].UnlockingScript == nil ||
		len(*stx.Inputs[0].UnlockingScript) == 0 {
		t.Fatalf("input 0 has no unlocking script; signing failed")
	}
}

// TestDeterministicTxid guards the assumption change-chaining relies on: that
// building + signing the same transfer twice yields the SAME txid. If go-sdk
// signing were non-deterministic, the change outpoint the primary chains would
// not match the tx the secondary broadcasts, and the whole chain would be
// spending phantom outputs.
func TestDeterministicTxid(t *testing.T) {
	from := makeFundedAccount(t, 100000)
	to := makeFundedAccount(t, 0)

	tx := newTransferTransaction(from.wif, 1000, from.utxos[0], to.address,
		from.address)

	a, err := tx.getTx()
	if err != nil {
		t.Fatalf("getTx (1): %v", err)
	}
	b, err := tx.getTx()
	if err != nil {
		t.Fatalf("getTx (2): %v", err)
	}

	if a.TxID().String() != b.TxID().String() {
		t.Fatalf("signing is NOT deterministic (%s != %s) — change-chaining "+
			"would chain phantom outpoints", a.TxID().String(),
			b.TxID().String())
	}
}

// TestChangeChaining proves the headline fix: a single funded UTXO sustains
// many transfers because each tx's change is fed back into the pool.
func TestChangeChaining(t *testing.T) {
	from := makeFundedAccount(t, 1000000) // one funded output
	to := makeFundedAccount(t, 0)

	b := newBuilder(nil)
	b.addAccounts([]*account{from, to})

	fromRes, err := b.CreateAccount(0)
	if err != nil {
		t.Fatalf("CreateAccount(from): %v", err)
	}
	toRes, err := b.CreateAccount(0)
	if err != nil {
		t.Fatalf("CreateAccount(to): %v", err)
	}
	facc := fromRes.(*account)

	const n = 8
	for i := 0; i < n; i++ {
		if _, err := b.EncodeTransfer(1000, fromRes, toRes, nil); err != nil {
			t.Fatalf("transfer %d failed with only 1 funded UTXO — change "+
				"not chained: %v", i, err)
		}
	}

	// pool must have grown by n change outputs (1 funded + n chained).
	if len(facc.utxos) != 1+n {
		t.Fatalf("pool size = %d, want %d (1 funded + %d chained change)",
			len(facc.utxos), 1+n, n)
	}

	// chained outpoints must be distinct, non-zero, and monotonically shrinking
	// (each spends the prior change minus payment + fee).
	seen := map[string]bool{}
	prev := uint64(1 << 62)
	for i := 1; i <= n; i++ {
		u := facc.utxos[i]
		if u.satoshis == 0 {
			t.Fatalf("chained utxo %d has 0 sats", i)
		}
		if u.satoshis >= prev {
			t.Fatalf("chained utxo %d sats %d did not shrink (prev %d)",
				i, u.satoshis, prev)
		}
		prev = u.satoshis
		key := fmt.Sprintf("%s:%d", u.txid, u.vout)
		if seen[key] {
			t.Fatalf("duplicate chained outpoint %s", key)
		}
		seen[key] = true
	}
}
