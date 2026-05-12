# BSV SDK Conformance Test Runner (Go)

Runs the conformance vector suite against the Go SDK.

## Requirements

- Go 1.25+
- `go-stack` workspace checkout, including `packages/sdk/go-sdk`

## Build

```sh
cd conformance/runner
go mod tidy
go build ./...
```

## Usage

```sh
# Run all vendored vectors
go run . --vectors ../vendor/vectors

# Specify a custom vectors directory
go run . --vectors /path/to/vectors

# Validate JSON format only (no execution)
go run . --validate-only --vectors ../vendor/vectors

# Write a JSON summary report
go run . --vectors ../vendor/vectors --report ../reports/go-results.json

# Write a JUnit XML report
go run . --vectors ../vendor/vectors --junit-report ../reports/go-results.xml
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--vectors <dir>` | `../vendor/vectors` | Directory to search recursively for `*.json` vector files |
| `--filter <glob>` | _(none)_ | Run matching vector file IDs, vector IDs, or paths |
| `--report <path>` | _(none)_ | Write JSON summary report to this path |
| `--junit-report <path>` | _(none)_ | Write JUnit XML report to this path |
| `--validate-only` | false | Parse and validate JSON format; skip execution |
| `--verbose` | false | Print per-vector pass/fail/skip lines |

## Exit codes

- `0` - all executed vectors passed (skipped vectors do not count as failures)
- `1` - one or more vectors failed, or a fatal error occurred
- `2` - schema validation error

## Implemented categories

| Category | Status |
|----------|--------|
| `sdk.crypto.sha256` | Implemented (single and double hash) |
| `sdk.crypto.ripemd160` | Implemented |
| `sdk.crypto.hash160` | Implemented |
| `sdk.crypto.hmac` | Implemented (HMAC-SHA256 and HMAC-SHA512) |
| `sdk.crypto.ecdsa` | Partially implemented |
| `sdk.crypto.aes` | Implemented |
| `sdk.crypto.ecies` | Partially implemented |
| `sdk.compat.signature` | Implemented |
| `sdk.compat.bsm` | Implemented |
| `sdk.keys.key-derivation` | Implemented |
| `sdk.keys.private-key` | Implemented |
| `sdk.keys.public-key` | Implemented |
| `sdk.scripts.evaluation` | Partially implemented |
| `sdk.transactions.merkle-path` | Implemented |
| `sdk.transactions.serialization` | Implemented |
| Selected regressions | Implemented |
| All others | Skipped with `not-implemented` status |
