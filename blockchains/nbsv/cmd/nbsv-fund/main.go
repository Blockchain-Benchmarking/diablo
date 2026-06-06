// Command nbsv-fund builds the one-time funding fan-out + keys.yaml for the
// Diablo BSV adapter: it splits a funded master output into one pre-funded
// P2PKH output per benchmark account and writes the matching keys.yaml.
//
//	nbsv-fund -master-wif <wif> -master-utxo <txid>:<vout>:<sats> \
//	    -accounts 8 -sats 100000 -mainnet -out configurations/bsv/keys.yaml
//
// By default it is a DRY RUN: it builds + signs the fan-out and writes keys.yaml
// but does not broadcast. It always prints the raw fan-out tx hex so you can
// broadcast it yourself (e.g. POST to arcade /tx). Pass -broadcast to send it
// via the go-sdk ARC client at -arc-url.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"diablo-benchmark/blockchains/nbsv"

	"gopkg.in/yaml.v3"
)

func main() {
	masterWif := flag.String("master-wif", "", "funded master WIF")
	masterUtxo := flag.String("master-utxo", "", "master outpoint <txid>:<vout>:<sats>")
	accounts := flag.Int("accounts", 8, "number of benchmark accounts to fund")
	sats := flag.Uint64("sats", 100000, "sats per account output")
	mainnet := flag.Bool("mainnet", false, "mainnet address encoding")
	out := flag.String("out", "keys.yaml", "keys.yaml output path")
	doBroadcast := flag.Bool("broadcast", false, "broadcast the fan-out via ARC")
	arcURL := flag.String("arc-url", "https://arcade.gorillapool.io", "ARC/arcade base URL")
	feeRate := flag.Uint64("fee-rate", 100, "fee rate sats/kB (mainnet wants ~50+)")
	flag.Parse()

	if *masterWif == "" || *masterUtxo == "" {
		fatal("both -master-wif and -master-utxo are required")
	}
	txid, vout, msats := parseUtxo(*masterUtxo)

	tx, kf, err := nbsv.BuildFundingTx(*masterWif, txid, vout, msats, *accounts, *sats, *mainnet, *feeRate)
	if err != nil {
		fatal(err.Error())
	}

	data, err := yaml.Marshal(kf)
	if err != nil {
		fatal("marshal keys.yaml: " + err.Error())
	}
	if err := os.WriteFile(*out, data, 0o644); err != nil {
		fatal("write " + *out + ": " + err.Error())
	}

	fmt.Printf("fan-out txid : %s\n", tx.TxID().String())
	fmt.Printf("accounts     : %d x %d sats -> %s\n", *accounts, *sats, *out)
	fmt.Printf("raw tx hex   : %s\n", tx.Hex())

	if !*doBroadcast {
		fmt.Println("\nDRY RUN — keys.yaml written, fan-out NOT broadcast.")
		fmt.Println("Broadcast the raw tx hex above (e.g. POST arcade /tx), or re-run with -broadcast.")
		return
	}

	// Direct EF octet-stream POST to arcade /tx. (go-sdk's broadcaster.Arc only
	// reports success on a numeric `status:200`, but arcade replies with a
	// txStatus-only body, so it misreads every arcade response as a `[0]`
	// failure — we read arcade's real txStatus instead.)
	ef, err := tx.EF()
	if err != nil {
		fatal("build EF: " + err.Error())
	}
	resp, err := http.Post(*arcURL+"/tx", "application/octet-stream", bytes.NewReader(ef))
	if err != nil {
		fatal("POST: " + err.Error())
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var r struct {
		TxStatus  string `json:"txStatus"`
		Status    string `json:"status"`
		Error     string `json:"error"`
		ExtraInfo string `json:"extraInfo"`
	}
	_ = json.Unmarshal(body, &r)
	if r.TxStatus == "REJECTED" || r.Error != "" {
		fatal(fmt.Sprintf("arcade rejected: %s %s", r.Error, r.ExtraInfo))
	}
	fmt.Printf("\nbroadcast OK via %s (txStatus=%s%s) — keys.yaml ready.\n",
		*arcURL, r.TxStatus, r.Status)
}

func parseUtxo(s string) (string, uint32, uint64) {
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		fatal("master-utxo must be <txid>:<vout>:<sats>")
	}
	vout, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		fatal("bad vout: " + err.Error())
	}
	sats, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil {
		fatal("bad sats: " + err.Error())
	}
	return parts[0], uint32(vout), sats
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "nbsv-fund: "+msg)
	os.Exit(1)
}
