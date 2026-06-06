package nbsv

import (
	"bytes"
	"diablo-benchmark/core"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bsv-blockchain/go-sdk/transaction/broadcaster"
)

// BlockchainClient runs on a Diablo secondary. It decodes payloads shipped from
// the primary and, at trigger time, signs + broadcasts them.
type BlockchainClient struct {
	logger      core.Logger
	broadcaster *broadcaster.Arc
	confirmer   transactionConfirmer
	httpClient  *http.Client
}

func newClient(logger core.Logger, bcast *broadcaster.Arc, confirmer transactionConfirmer) *BlockchainClient {
	return &BlockchainClient{
		logger:      logger,
		broadcaster: bcast,
		confirmer:   confirmer,
		httpClient:  &http.Client{Timeout: 20 * time.Second},
	}
}

func (this *BlockchainClient) DecodePayload(encoded []byte) (interface{}, error) {
	buffer := bytes.NewBuffer(encoded)

	tx, err := decodeTransaction(buffer)
	if err != nil {
		return nil, err
	}

	this.logger.Tracef("decode transaction %p", tx)

	return tx, nil
}

func (this *BlockchainClient) TriggerInteraction(iact core.Interaction) error {
	tx := iact.Payload().(bsvTransaction)

	stx, err := tx.getTx()
	if err != nil {
		return err
	}

	txid := stx.TxID().String()
	this.logger.Tracef("submit transaction %s", txid)

	iact.ReportSubmit()

	// Direct EF POST (see broadcast.go for why go-sdk's Arc.Broadcast can't be
	// used against arcade).
	status, err := broadcastEF(this.httpClient, this.broadcaster.ApiUrl,
		this.broadcaster.ApiKey, stx)
	if err != nil {
		iact.ReportAbort()
		return fmt.Errorf("broadcast %s failed: %w", txid, err)
	}
	this.logger.Tracef("submitted %s (%s)", txid, status)

	return this.confirmer.confirm(iact, txid)
}

// transactionConfirmer decides when an interaction has committed, firing
// ReportCommit() so Diablo records its end-to-end latency.
type transactionConfirmer interface {
	confirm(iact core.Interaction, txid string) error
}

// immediateConfirmer treats a successful ARC broadcast as commit — i.e. it
// measures mempool ACCEPTANCE latency. Fast, but not the honest end-to-end
// number. Use `confirm=mined` (pollStatusConfirmer) for block-inclusion latency.
type immediateConfirmer struct {
	logger core.Logger
}

func (this *immediateConfirmer) confirm(iact core.Interaction, txid string) error {
	this.logger.Tracef("seen on network: %s", txid)
	iact.ReportCommit()
	return nil
}

// newConfirmer picks the confirmation policy from `params["confirm"]`:
//
//	immediate (default) — commit on ARC acceptance (mempool latency)
//	mined               — commit on block inclusion (honest end-to-end latency)
//
// For `mined`, the status endpoint prefix defaults to arcade's root `/tx/`;
// override with params["status_prefix"] = "/v1/tx/" for a classic-ARC endpoint.
func newConfirmer(logger core.Logger, bcast *broadcaster.Arc, params map[string]string) transactionConfirmer {
	if params["confirm"] != "mined" {
		return &immediateConfirmer{logger: logger}
	}

	prefix := params["status_prefix"]
	if prefix == "" {
		prefix = "/tx/" // arcade root; classic ARC would be /v1/tx/
	}
	interval := 10 * time.Second
	if v, ok := params["confirm_interval"]; ok {
		if d, err := time.ParseDuration(v); err == nil {
			interval = d
		}
	}
	base := strings.TrimRight(bcast.ApiUrl, "/")

	return &pollStatusConfirmer{
		logger:   logger,
		statusURL: func(txid string) string {
			return base + prefix + txid
		},
		interval: interval,
		client:   &http.Client{Timeout: 15 * time.Second},
		pendings: make(map[string]*pendingConfirm),
	}
}

// pollStatusConfirmer fires ReportCommit only when a tx is MINED.
//
// Note vs nsolana: BSV (Teranode) blocks can hold millions of txs, so scanning
// every block's txids — nsolana's approach — is infeasible. Instead one shared
// background loop polls each pending tx's status endpoint (arcade/ARC reports a
// txStatus that advances to MINED with a blockHeight). confirm() blocks the
// trigger goroutine until its tx mines, exactly like the suite's other
// confirmers, so Diablo records true block-inclusion latency.
type pollStatusConfirmer struct {
	logger    core.Logger
	statusURL func(txid string) string
	interval  time.Duration
	client    *http.Client

	mu       sync.Mutex
	pendings map[string]*pendingConfirm
	running  bool
}

type pendingConfirm struct {
	iact core.Interaction
	done chan struct{}
}

func (this *pollStatusConfirmer) confirm(iact core.Interaction, txid string) error {
	p := &pendingConfirm{iact: iact, done: make(chan struct{})}

	this.mu.Lock()
	this.pendings[txid] = p
	if !this.running {
		this.running = true
		go this.run()
	}
	this.mu.Unlock()

	<-p.done // block until the background loop sees this tx mined
	return nil
}

func (this *pollStatusConfirmer) run() {
	for {
		time.Sleep(this.interval)

		this.mu.Lock()
		txids := make([]string, 0, len(this.pendings))
		for txid := range this.pendings {
			txids = append(txids, txid)
		}
		this.mu.Unlock()

		for _, txid := range txids {
			if !this.isMined(txid) {
				continue
			}
			this.mu.Lock()
			p := this.pendings[txid]
			delete(this.pendings, txid)
			this.mu.Unlock()
			if p != nil {
				this.logger.Tracef("mined: %s", txid)
				p.iact.ReportCommit()
				close(p.done)
			}
		}
	}
}

// isMined reports whether the status endpoint shows the tx in a block.
func (this *pollStatusConfirmer) isMined(txid string) bool {
	resp, err := this.client.Get(this.statusURL(txid))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	var s struct {
		TxStatus    string `json:"txStatus"`
		BlockHeight uint64 `json:"blockHeight"`
	}
	if err := json.Unmarshal(body, &s); err != nil {
		return false
	}
	return s.TxStatus == "MINED" || s.BlockHeight > 0
}
