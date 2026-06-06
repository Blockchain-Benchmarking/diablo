package nbsv

// Direct EF broadcast to an arcade/ARC `/tx` endpoint.
//
// go-sdk's broadcaster.Arc only reports success on a numeric `status:200` ARC
// field, but arcade replies with a txStatus-only body (e.g. {"txStatus":
// "SEEN_ON_NETWORK"}), so it misreads EVERY arcade response as a `[0]` failure
// — which would make Diablo abort every interaction even though the tx landed.
// We POST the extended-format bytes ourselves and read arcade's real txStatus.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	sdktx "github.com/bsv-blockchain/go-sdk/transaction"
)

type arcSubmitResponse struct {
	TxStatus  string `json:"txStatus"`
	Txid      string `json:"txid"`
	Error     string `json:"error"`
	ExtraInfo string `json:"extraInfo"`
}

// broadcastEF submits tx (extended format) to <arcURL>/tx and returns the
// reported txStatus. It errors if the endpoint rejects the tx.
func broadcastEF(client *http.Client, arcURL, apiKey string, tx *sdktx.Transaction) (string, error) {
	ef, err := tx.EF()
	if err != nil {
		return "", fmt.Errorf("build EF: %w", err)
	}

	req, err := http.NewRequest("POST", strings.TrimRight(arcURL, "/")+"/tx",
		bytes.NewReader(ef))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var r arcSubmitResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return "", fmt.Errorf("arc response (HTTP %d): %s", resp.StatusCode,
			truncate(string(body), 160))
	}
	if r.Error != "" || r.TxStatus == "REJECTED" {
		return r.TxStatus, fmt.Errorf("arc rejected: %s %s", r.Error, r.ExtraInfo)
	}
	return r.TxStatus, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
