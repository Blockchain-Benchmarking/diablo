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
| Custom-script / cell-engine capacity | `!script` | P2PKH input → arbitrary locking-script output + change |

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

## Funding step (one-time, via Metanet Desktop :3321)

Benchmarks must not call a wallet per transaction. Instead, fund a key pool
once:

1. Generate N keys.
2. Broadcast a **fan-out** tx from Metanet Desktop's BRC-100 interface
   (`http://localhost:3321`) splitting coins into many small P2PKH outputs
   across those keys.
3. Record each outpoint in `keys.yaml` (see `configurations/bsv/keys.example.yaml`).

The hot path then signs locally with go-sdk — no wallet round-trip.

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

**Remaining TODO:**

3. **Real cell-engine scripts.** `!script` takes raw locking-script hex. Wire
   the protocol-types cell codec to build the script, and add a second
   interaction type that *spends* these outputs to exercise the OP_CHECKSIG
   verify pipeline end-to-end.
4. **Fee policy.** `feeRate` is a constant; source it from the ARC policy quote.
