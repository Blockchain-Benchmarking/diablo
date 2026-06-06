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
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"diablo-benchmark/blockchains/nbsv"

	"github.com/bsv-blockchain/go-sdk/transaction/broadcaster"
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
	flag.Parse()

	if *masterWif == "" || *masterUtxo == "" {
		fatal("both -master-wif and -master-utxo are required")
	}
	txid, vout, msats := parseUtxo(*masterUtxo)

	tx, kf, err := nbsv.BuildFundingTx(*masterWif, txid, vout, msats, *accounts, *sats, *mainnet)
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

	arc := &broadcaster.Arc{ApiUrl: *arcURL}
	if _, failure := tx.Broadcast(arc); failure != nil {
		fatal(fmt.Sprintf("broadcast failed [%s]: %s", failure.Code, failure.Description))
	}
	fmt.Printf("\nbroadcast OK via %s — keys.yaml ready.\n", *arcURL)
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
