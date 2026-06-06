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
	"strconv"

	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	sdktx "github.com/bsv-blockchain/go-sdk/transaction"
	feemodel "github.com/bsv-blockchain/go-sdk/transaction/fee_model"
	sighash "github.com/bsv-blockchain/go-sdk/transaction/sighash"
	"github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"
)

const (
	txTypeTransfer  uint8 = 0
	txTypeScript    uint8 = 1
	txTypeCellSpend uint8 = 2
)

// feeRate is the sats/kB fee rate used when building transactions. It is a var,
// not a const, so it can be set from the `fee_rate` setup parameter to match
// the live ARC policy quote (GET /v1/policy reports `miningFee`).
//
// INVARIANT: the primary (encode) and secondaries (trigger) MUST use the same
// feeRate, or the deterministic tx they each build will differ and change-
// chaining will reference phantom outpoints. Both read it from the same
// setup.yaml `parameters:` block via configureFee, so they always agree.
var feeRate uint64 = 1

// configureFee sets feeRate from params["fee_rate"] if present.
func configureFee(params map[string]string) error {
	v, ok := params["fee_rate"]
	if !ok || v == "" {
		return nil
	}
	rate, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid fee_rate '%s': %w", v, err)
	}
	if rate == 0 {
		return fmt.Errorf("fee_rate must be > 0")
	}
	feeRate = rate
	return nil
}

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
	case txTypeCellSpend:
		return decodeCellSpendTransaction(src)
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

// ---------------------------------------------------------------------------
// cell-engine spend: SPEND a cell-token output to exercise OP_CHECKSIG.
//
// The `script`/cell-token create path produces a PushDrop output of the form
//   <cell> OP_DROP <ownerPubKey> OP_CHECKSIG
// — capability no account-model chain can match. Creating it proves nothing
// about VERIFY, though: the OP_CHECKSIG pipeline only runs when the output is
// SPENT. cellSpendTransaction spends one such output (input 0, unlocked by a
// bare signature) plus a P2PKH funding input (input 1) for the fee, so the
// node actually executes the cell's OP_CHECKSIG at validation time.
// ---------------------------------------------------------------------------

// buildCellTokenLock builds <cell> OP_DROP <ownerPub> OP_CHECKSIG — the
// cell-engine / PushDrop locking script, in Go (vs accepting raw hex).
func buildCellTokenLock(cell, ownerPub []byte) (*script.Script, error) {
	s := &script.Script{}
	if err := s.AppendPushData(cell); err != nil {
		return nil, err
	}
	if err := s.AppendOpcodes(script.OpDROP); err != nil {
		return nil, err
	}
	if err := s.AppendPushData(ownerPub); err != nil {
		return nil, err
	}
	if err := s.AppendOpcodes(script.OpCHECKSIG); err != nil {
		return nil, err
	}
	return s, nil
}

// cellPkUnlocker satisfies the go-sdk UnlockingScriptTemplate for a bare
// `<pubkey> OP_CHECKSIG` (P2PK-style) output: the unlocking script is just the
// signature (the pubkey already sits in the locking script). This is what
// drives OP_CHECKSIG verification of a cell-token output.
type cellPkUnlocker struct {
	priv *ec.PrivateKey
	flag sighash.Flag
}

func (this *cellPkUnlocker) Sign(tx *sdktx.Transaction, inputIndex uint32) (*script.Script, error) {
	sh, err := tx.CalcInputSignatureHash(inputIndex, this.flag)
	if err != nil {
		return nil, err
	}

	sig, err := this.priv.Sign(sh)
	if err != nil {
		return nil, err
	}

	buf := append(sig.Serialize(), uint8(this.flag))

	s := &script.Script{}
	if err := s.AppendPushData(buf); err != nil {
		return nil, err
	}
	return s, nil
}

func (this *cellPkUnlocker) EstimateLength(*sdktx.Transaction, uint32) uint32 {
	return 73 // ~72-byte DER sig + sighash flag + push opcode
}

type cellSpendTransaction struct {
	wif         string
	cellTxid    string
	cellVout    uint32
	cellSats    uint64
	cellLockHex string
	fundTxid    string
	fundVout    uint32
	fundSats    uint64
	fundLockHex string
	changeAddr  string
}

func newCellSpendTransaction(wif string, cell *cellTokenRef, fund *utxoRef, changeAddr string) *cellSpendTransaction {
	return &cellSpendTransaction{
		wif:         wif,
		cellTxid:    cell.txid,
		cellVout:    cell.vout,
		cellSats:    cell.satoshis,
		cellLockHex: cell.lockHex,
		fundTxid:    fund.txid,
		fundVout:    fund.vout,
		fundSats:    fund.satoshis,
		fundLockHex: fund.lockHex,
		changeAddr:  changeAddr,
	}
}

func (this *cellSpendTransaction) encode(dest io.Writer) error {
	return util.NewMonadOutputWriter(dest).
		SetOrder(binary.LittleEndian).
		WriteUint8(txTypeCellSpend).
		WriteUint16(uint16(len(this.wif))).WriteString(this.wif).
		WriteUint16(uint16(len(this.cellTxid))).WriteString(this.cellTxid).
		WriteUint32(this.cellVout).
		WriteUint64(this.cellSats).
		WriteUint32(uint32(len(this.cellLockHex))).WriteString(this.cellLockHex).
		WriteUint16(uint16(len(this.fundTxid))).WriteString(this.fundTxid).
		WriteUint32(this.fundVout).
		WriteUint64(this.fundSats).
		WriteUint16(uint16(len(this.fundLockHex))).WriteString(this.fundLockHex).
		WriteUint16(uint16(len(this.changeAddr))).WriteString(this.changeAddr).
		Error()
}

func decodeCellSpendTransaction(src io.Reader) (*cellSpendTransaction, error) {
	var tx cellSpendTransaction
	var lwif, lctxid, lftxid, lflock, lchg uint16
	var lclock uint32

	err := util.NewMonadInputReader(src).
		SetOrder(binary.LittleEndian).
		ReadUint16(&lwif).ReadString(&tx.wif, int(lwif)).
		ReadUint16(&lctxid).ReadString(&tx.cellTxid, int(lctxid)).
		ReadUint32(&tx.cellVout).
		ReadUint64(&tx.cellSats).
		ReadUint32(&lclock).ReadString(&tx.cellLockHex, int(lclock)).
		ReadUint16(&lftxid).ReadString(&tx.fundTxid, int(lftxid)).
		ReadUint32(&tx.fundVout).
		ReadUint64(&tx.fundSats).
		ReadUint16(&lflock).ReadString(&tx.fundLockHex, int(lflock)).
		ReadUint16(&lchg).ReadString(&tx.changeAddr, int(lchg)).
		Error()
	if err != nil {
		return nil, err
	}

	return &tx, nil
}

func (this *cellSpendTransaction) getTx() (*sdktx.Transaction, error) {
	priv, err := ec.PrivateKeyFromWif(this.wif)
	if err != nil {
		return nil, err
	}

	tx := sdktx.NewTransaction()

	// input 0: the cell-token output, unlocked by a bare signature ->
	// OP_CHECKSIG runs at validation.
	cellUnlocker := &cellPkUnlocker{priv: priv, flag: sighash.AllForkID}
	if err = tx.AddInputFrom(this.cellTxid, this.cellVout, this.cellLockHex,
		this.cellSats, cellUnlocker); err != nil {
		return nil, err
	}

	// input 1: P2PKH funding input to cover the fee.
	fundUnlocker, err := p2pkh.Unlock(priv, nil)
	if err != nil {
		return nil, err
	}
	if err = tx.AddInputFrom(this.fundTxid, this.fundVout, this.fundLockHex,
		this.fundSats, fundUnlocker); err != nil {
		return nil, err
	}

	// single change output absorbs cell sats + funding - fee, and signs both
	// inputs (cell via OP_CHECKSIG sig, funding via P2PKH).
	if err = addChangeAndSign(tx, this.changeAddr); err != nil {
		return nil, err
	}

	return tx, nil
}
