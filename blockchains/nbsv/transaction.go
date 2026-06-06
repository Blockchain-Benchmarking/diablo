package nbsv

// Wire format + transaction building for the BSV adapter.
//
// Like every Diablo `n*` adapter, encoding happens ONCE on the primary
// (EncodeTransfer / EncodeInteraction produce an opaque []byte), the bytes are
// shipped to the secondaries, and each secondary DecodePayload()s then signs +
// broadcasts at TriggerInteraction time. The wire format below is the contract
// between those two machines.
//
// The signer's WIF is shipped in the payload (the same approach nsolana takes
// with the raw private key) so the secondary can build + sign the transaction
// at trigger time without a wallet round-trip in the hot path.

import (
	"diablo-benchmark/util"
	"encoding/binary"
	"fmt"
	"io"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	sdktx "github.com/bsv-blockchain/go-sdk/transaction"
	feemodel "github.com/bsv-blockchain/go-sdk/transaction/fee_model"
	"github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"
)

const (
	txTypeTransfer uint8 = 0
	txTypeScript   uint8 = 1

	// feeRate is the sats/kB fee rate used when building transactions.
	// TODO: source this from the ARC policy quote (GET /v1/policy) instead of
	// a hard-coded constant so the benchmark tracks real network fees.
	feeRate uint64 = 1
)

// bsvTransaction is the decoded, ready-to-build representation handed to the
// secondary. getTx() reconstructs and signs a concrete go-sdk transaction.
type bsvTransaction interface {
	getTx() (*sdktx.Transaction, error)
}

func decodeTransaction(src io.Reader) (bsvTransaction, error) {
	var txtype uint8

	err := util.NewMonadInputReader(src).
		SetOrder(binary.LittleEndian).
		ReadUint8(&txtype).
		Error()
	if err != nil {
		return nil, err
	}

	switch txtype {
	case txTypeTransfer:
		return decodeTransferTransaction(src)
	case txTypeScript:
		return decodeScriptTransaction(src)
	default:
		return nil, fmt.Errorf("unknown transaction type %d", txtype)
	}
}

// addChangeAndSign appends a change output back to `changeAddr`, lets the
// go-sdk fee model fill in the change value, and signs every input.
func addChangeAndSign(tx *sdktx.Transaction, changeAddr string) error {
	addr, err := script.NewAddressFromString(changeAddr)
	if err != nil {
		return err
	}

	lock, err := p2pkh.Lock(addr)
	if err != nil {
		return err
	}

	tx.AddOutput(&sdktx.TransactionOutput{
		LockingScript: lock,
		Change:        true,
	})

	err = tx.Fee(&feemodel.SatoshisPerKilobyte{Satoshis: feeRate},
		sdktx.ChangeDistributionEqual)
	if err != nil {
		return err
	}

	return tx.Sign()
}

// ---------------------------------------------------------------------------
// transfer: plain P2PKH payment (the payment-throughput claim).
// ---------------------------------------------------------------------------

type transferTransaction struct {
	wif        string
	amount     uint64
	inTxid     string
	inVout     uint32
	inSats     uint64
	inLockHex  string
	toAddr     string
	changeAddr string
}

func newTransferTransaction(wif string, amount uint64, in *utxoRef, toAddr, changeAddr string) *transferTransaction {
	return &transferTransaction{
		wif:        wif,
		amount:     amount,
		inTxid:     in.txid,
		inVout:     in.vout,
		inSats:     in.satoshis,
		inLockHex:  in.lockHex,
		toAddr:     toAddr,
		changeAddr: changeAddr,
	}
}

func (this *transferTransaction) encode(dest io.Writer) error {
	return util.NewMonadOutputWriter(dest).
		SetOrder(binary.LittleEndian).
		WriteUint8(txTypeTransfer).
		WriteUint16(uint16(len(this.wif))).
		WriteString(this.wif).
		WriteUint64(this.amount).
		WriteUint16(uint16(len(this.inTxid))).
		WriteString(this.inTxid).
		WriteUint32(this.inVout).
		WriteUint64(this.inSats).
		WriteUint16(uint16(len(this.inLockHex))).
		WriteString(this.inLockHex).
		WriteUint16(uint16(len(this.toAddr))).
		WriteString(this.toAddr).
		WriteUint16(uint16(len(this.changeAddr))).
		WriteString(this.changeAddr).
		Error()
}

func decodeTransferTransaction(src io.Reader) (*transferTransaction, error) {
	var tx transferTransaction
	var lwif, ltxid, llock, lto, lchg uint16

	// Length prefixes are read into l* and consumed by the following
	// ReadString in the same chain; Go evaluates the operands left-to-right,
	// so each length is populated before its string is read (same idiom as
	// nsolana's decodeTransferTransaction).
	err := util.NewMonadInputReader(src).
		SetOrder(binary.LittleEndian).
		ReadUint16(&lwif).ReadString(&tx.wif, int(lwif)).
		ReadUint64(&tx.amount).
		ReadUint16(&ltxid).ReadString(&tx.inTxid, int(ltxid)).
		ReadUint32(&tx.inVout).
		ReadUint64(&tx.inSats).
		ReadUint16(&llock).ReadString(&tx.inLockHex, int(llock)).
		ReadUint16(&lto).ReadString(&tx.toAddr, int(lto)).
		ReadUint16(&lchg).ReadString(&tx.changeAddr, int(lchg)).
		Error()
	if err != nil {
		return nil, err
	}

	return &tx, nil
}

func (this *transferTransaction) getTx() (*sdktx.Transaction, error) {
	priv, err := ec.PrivateKeyFromWif(this.wif)
	if err != nil {
		return nil, err
	}

	// nil sighash flag defaults to SIGHASH_ALL | FORKID inside the template.
	unlocker, err := p2pkh.Unlock(priv, nil)
	if err != nil {
		return nil, err
	}

	tx := sdktx.NewTransaction()

	err = tx.AddInputFrom(this.inTxid, this.inVout, this.inLockHex,
		this.inSats, unlocker)
	if err != nil {
		return nil, err
	}

	if err = tx.PayToAddress(this.toAddr, this.amount); err != nil {
		return nil, err
	}

	if err = addChangeAndSign(tx, this.changeAddr); err != nil {
		return nil, err
	}

	return tx, nil
}

// ---------------------------------------------------------------------------
// script: create a custom-script output (the cell-engine / sCrypt claim).
//
// This path proves capability no account-model chain can match: producing an
// output guarded by an arbitrary BSV script (e.g. a cell-engine OP_CHECKSIG /
// PushDrop output). For now it spends one P2PKH input and creates one output
// carrying a caller-supplied locking script.
//
// TODO (cell-engine): (a) build the locking script from the cell bytes via the
// protocol-types codec rather than accepting raw hex; (b) provide a matching
// UnlockingScriptTemplate so these outputs can themselves be SPENT in a second
// interaction type, which is what actually exercises the OP_CHECKSIG verify
// pipeline end-to-end.
// ---------------------------------------------------------------------------

type scriptTransaction struct {
	wif        string
	inTxid     string
	inVout     uint32
	inSats     uint64
	inLockHex  string
	outScript  string
	outSats    uint64
	changeAddr string
}

func newScriptTransaction(wif string, in *utxoRef, outScript string, outSats uint64, changeAddr string) *scriptTransaction {
	return &scriptTransaction{
		wif:        wif,
		inTxid:     in.txid,
		inVout:     in.vout,
		inSats:     in.satoshis,
		inLockHex:  in.lockHex,
		outScript:  outScript,
		outSats:    outSats,
		changeAddr: changeAddr,
	}
}

func (this *scriptTransaction) encode(dest io.Writer) error {
	return util.NewMonadOutputWriter(dest).
		SetOrder(binary.LittleEndian).
		WriteUint8(txTypeScript).
		WriteUint16(uint16(len(this.wif))).
		WriteString(this.wif).
		WriteUint16(uint16(len(this.inTxid))).
		WriteString(this.inTxid).
		WriteUint32(this.inVout).
		WriteUint64(this.inSats).
		WriteUint16(uint16(len(this.inLockHex))).
		WriteString(this.inLockHex).
		WriteUint32(uint32(len(this.outScript))).
		WriteString(this.outScript).
		WriteUint64(this.outSats).
		WriteUint16(uint16(len(this.changeAddr))).
		WriteString(this.changeAddr).
		Error()
}

func decodeScriptTransaction(src io.Reader) (*scriptTransaction, error) {
	var tx scriptTransaction
	var lwif, ltxid, llock, lchg uint16
	var lout uint32

	err := util.NewMonadInputReader(src).
		SetOrder(binary.LittleEndian).
		ReadUint16(&lwif).ReadString(&tx.wif, int(lwif)).
		ReadUint16(&ltxid).ReadString(&tx.inTxid, int(ltxid)).
		ReadUint32(&tx.inVout).
		ReadUint64(&tx.inSats).
		ReadUint16(&llock).ReadString(&tx.inLockHex, int(llock)).
		ReadUint32(&lout).ReadString(&tx.outScript, int(lout)).
		ReadUint64(&tx.outSats).
		ReadUint16(&lchg).ReadString(&tx.changeAddr, int(lchg)).
		Error()
	if err != nil {
		return nil, err
	}

	return &tx, nil
}

func (this *scriptTransaction) getTx() (*sdktx.Transaction, error) {
	priv, err := ec.PrivateKeyFromWif(this.wif)
	if err != nil {
		return nil, err
	}

	unlocker, err := p2pkh.Unlock(priv, nil)
	if err != nil {
		return nil, err
	}

	tx := sdktx.NewTransaction()

	err = tx.AddInputFrom(this.inTxid, this.inVout, this.inLockHex,
		this.inSats, unlocker)
	if err != nil {
		return nil, err
	}

	outScript, err := script.NewFromHex(this.outScript)
	if err != nil {
		return nil, err
	}

	tx.AddOutput(&sdktx.TransactionOutput{
		LockingScript: outScript,
		Satoshis:      this.outSats,
	})

	if err = addChangeAndSign(tx, this.changeAddr); err != nil {
		return nil, err
	}

	return tx, nil
}
