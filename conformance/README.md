# Conformance

The conformance runner for go-stack is planned as part of the MBGA programme.

See `GO_PLAN.md` in the [ts-stack](https://github.com/bsv-blockchain/ts-stack) repository for the migration plan and current status.

## What goes here

- `runner/` — Go conformance runner (to be migrated from `conformance/runner/go/` in ts-stack)
- `vectors/` — Shared test vectors (fetched from ts-stack CI artifacts)
- `reports/` — Conformance run reports

## Running conformance tests (planned)

```sh
cd conformance/runner
go run . --vectors ../vectors/ --report ../reports/go-report.xml
```

## Status

| Domain | Vectors | Go runner |
|--------|---------|-----------|
| SDK (crypto, script, tx) | in ts-stack | planned |
| Wallet | in ts-stack | planned |
| Overlays | in ts-stack | planned |
| Messaging | in ts-stack | planned |
