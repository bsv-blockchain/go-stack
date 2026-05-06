# Conformance

The conformance runner for go-stack is being migrated from `conformance/runner/go/` in ts-stack into this repo as part of the MBGA programme.

See [`PLAN_GO.md`](https://github.com/bsv-blockchain/mbga/blob/main/PLAN_GO.md) in the `mbga` repo for the migration plan, runner architecture, and corpus consumption strategy. The shared vector corpus, schema, and CLI contract live in [ts-stack](https://github.com/bsv-blockchain/ts-stack/tree/main/conformance).

## Layout

- `runner/` — Go conformance runner (in flight)
- `vendor/vectors/` — Shared test vectors fetched from ts-stack `conformance-vectors` CI artifact (gitignored)
- `reports/` — Conformance run reports (gitignored)

## Running conformance tests

```sh
# 1. Fetch the latest ts-stack conformance corpus (one-time per branch)
scripts/fetch-vectors.sh

# 2. Run the Go runner
cd conformance/runner
go run . --vectors ../vendor/vectors --report ../reports/go-results.json --verbose
```

CLI contract (binding across all language runners — see [ts-stack VECTOR-FORMAT.md](https://github.com/bsv-blockchain/ts-stack/blob/main/conformance/VECTOR-FORMAT.md)):

| Flag | Effect |
|------|--------|
| `--validate-only` | Schema-validate vectors without executing |
| `--filter <glob>` | Run only vector files matching the glob (e.g. `sdk.keys.*`) |
| `--report <path>` | Write JSON summary report |
| `--verbose` | Per-vector pass/fail lines |

| Exit code | Meaning |
|----|---------|
| 0 | All vectors passed |
| 1 | One or more vectors failed |
| 2 | Schema validation error |

## Status

| Domain | Vectors (ts-stack) | Go runner |
|--------|-------------------|-----------|
| SDK (crypto, script, tx) | published | porting from legacy go runner (195/216 baseline) |
| Wallet (BRC-100) | published | planned |
| Wallet (BRC-29) | published | planned |
| Messaging (BRC-31) | published | planned |
| Overlays / boundary specs | published (OpenAPI) | out of scope (Schemathesis) |
| Regressions | 12 vectors | partial (intended-fail until parity ships) |
