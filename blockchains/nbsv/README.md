# `nbsv` — Bitcoin SV adapter for Diablo

Drives **Bitcoin SV (Teranode + arcade)** with Diablo's realistic application
traces so BSV throughput/latency claims can be measured on the same neutral,
peer-reviewed methodology as every other chain in the suite.

This is the **first UTXO-model adapter** in Diablo — every shipped adapter
(Ethereum, Solana, Algorand, Diem) is account-model, so this models UTXO
selection from scratch rather than cribbing an existing template.

## What it proves

| Claim | Interaction | Path |
|---|---|---|
| Payment throughput | `!transfer` | P2PKH input → P2PKH payment + change |
| Custom-script capacity | `!script` | P2PKH input → arbitrary locking-script output + change |
| Cell-engine output | `!celltoken` | builds `<cell> OP_DROP <pub> OP_CHECKSIG`, records its outpoint |
| Cell-engine VERIFY | `!cellspend` | spends a cell-token (bare-sig unlock) → runs OP_CHECKSIG at validation |

Run the **same** trace (NASDAQ / Uber / FIFA) the EuroSys'23 paper ran against
the other chains, on comparable hardware, to produce a directly comparable
cross-chain table. The credibility comes from the neutral harness — so the
adapter should be **upstreamed** to `Blockchain-Benchmarking/diablo`, not run
as a private fork.

## Architecture (mirrors the `n*` adapters)

- `interface.go` — `Builder` (primary) loads the key pool from `env keys=...`;
  `Client` (secondary) wires an ARC broadcaster at the endpoint URL.
- `builder.go` — account pool, `CreateAccount`, `EncodeTransfer`,
  `EncodeInteraction("script")`, and **`reserve()` — the UTXO reservation seam**.
- `transaction.go` — wire `encode`/`decode` + `getTx()` build/sign via go-sdk.
- `client.go` — `DecodePayload` + `TriggerInteraction` (broadcast + confirm).
- `keys.go` — `keys.yaml` loader (pre-funded WIF + UTXO pool).

Encoding runs **once on the primary**, producing opaque bytes shipped to the
secondaries, each of which decodes, signs, and broadcasts at trigger time. The
signer WIF travels in the payload (same approach as nsolana's raw private key).

## Funding step (one-time) — `nbsv-fund`

Benchmarks must not call a wallet per transaction. Fund a key pool once with the
bundled tool: it generates N keys, builds the fan-out splitting a funded master
output into one pre-funded P2PKH output per account, and writes `keys.yaml`.
Change-chaining (see `builder.go`) means **one output per account suffices** —
the trace's change feeds itself.

```sh
go run ./blockchains/nbsv/cmd/nbsv-fund \
    -master-wif <funded-master-WIF> \
    -master-utxo <txid>:<vout>:<sats> \
    -accounts 8 -sats 100000 -mainnet \
    -out configurations/bsv/keys.yaml -broadcast -arc-url https://arcade.gorillapool.io
```

Without `-broadcast` it's a **dry run**: it writes `keys.yaml` and prints the raw
fan-out tx hex for you to broadcast yourself (e.g. `POST` to arcade `/tx`). Fund
the master output first (any wallet — e.g. Metanet Desktop's BRC-100 interface on
`http://localhost:3321`). The hot path then signs locally with go-sdk — no wallet
round-trip.

## Run

```sh
# primary
diablo primary --env keys=configurations/bsv/keys.yaml \
    configurations/bsv/setup.yaml configurations/bsv/benchmark.yaml
# secondaries point at the arcade/ARC endpoint from setup.yaml
```

## Offline test

```sh
go test ./blockchains/nbsv/ -run RoundTrip -v
```

Proves the encode → decode → sign pipeline yields a valid, fully-signed tx
(payment + script) with correct change math, without a node.

## Status

**Done:**

1. **Change-chaining (the headline throughput fix).** Each transfer/script tx
   is built + signed at encode time (go-sdk signs deterministically — RFC6979,
   asserted by `TestDeterministicTxid`), so its change outpoint is known and fed
   back into the account pool (`chainChange`). A single funded UTXO now sustains
   an unbounded chain of transfers — `TestChangeChaining` drives 8 transfers off
   one funded output. Because signing is deterministic, the tx the secondary
   re-signs at trigger time is byte-identical, so the chained outpoint is real.
2. **Mined-confirmation latency.** `confirm=mined` selects `pollStatusConfirmer`,
   which blocks each interaction until its tx shows MINED (a block height) on the
   arcade/ARC status endpoint — honest end-to-end latency. (BSV blocks are far
   too large to scan txid-by-txid like nsolana, so this polls per-tx status
   instead.) Default remains `immediate` (mempool-acceptance latency).

```sh
# honest block-inclusion latency:
diablo primary --param confirm=mined ...
```

3. **Cell-engine scripts + VERIFY.** `buildCellTokenLock` assembles a
   `<cell> OP_DROP <pub> OP_CHECKSIG` output in Go (`!celltoken`), and
   `!cellspend` spends it with a bare-signature unlock (`cellPkUnlocker`), so
   the node executes OP_CHECKSIG at validation — capability no account-model
   chain has. `TestCellTokenSpendRoundTrip` asserts the cell input carries a
   bare sig (≈73 B), not P2PKH's sig+pubkey (≈107 B).
4. **Configurable fee.** `feeRate` is set from the `fee_rate` setup parameter
   (sats/kB) to match the live ARC policy quote (`GET /v1/policy` → `miningFee`).
   Primary + secondaries read the same param, so the deterministic txs agree.

## Remaining TODO

- Auto-source `fee_rate` from a one-shot `/v1/policy` fetch at run start (still
  pinned for the run so determinism holds), instead of the operator pasting it.
- Build the cell bytes via the protocol-types codec rather than raw hex.
- A live primary/secondary run against arcade (offline tests only so far).
