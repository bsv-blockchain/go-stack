# AGENTS.md — ARC Codebase Guide

ARC is a microservice-based Bitcoin SV transaction processor (Go 1.25).
Core services: API (Echo), Metamorph, BlockTx, Callbacker. Communication via NATS + gRPC. Data in PostgreSQL + Redis.

## Build / Lint / Test Commands

This project uses [Task](https://taskfile.dev) (v3) instead of Make. See `Taskfile.yml`.

```sh
task deps              # go mod download
task build             # go build ./...
task lint              # golangci-lint run -v ./...
task lint_fix          # golangci-lint run -v ./... --fix
task test              # go test -parallel=8 -coverprofile=./cov.out -covermode=atomic -race ./... -coverpkg ./...
task test_short        # same as above with -short (skips integration tests)
task gen               # regenerate protobuf code (all 4 proto files)
task gen_go            # go generate ./... (mocks via moq, etc.)
task api               # regenerate REST API code from OpenAPI spec (pkg/api/arc.yaml)
```

### Running a Single Test

```sh
# Single test function
go test -race -v -run TestFunctionName ./internal/package/...

# Single test with sub-test
go test -race -v -run TestFunctionName/sub_test_name ./internal/package/...

# Single package
go test -race -v ./internal/metamorph/...

# Skip integration tests (which need Docker/Postgres)
go test -race -short -v -run TestFunctionName ./internal/package/...
```

### CI Checks (what PR validation runs)

- `go vet ./...`
- `gofmt -s` — checked via `git diff --exit-code` (no goimports)
- `go generate ./...` — verified unchanged (mocks + API code must be committed)
- `golangci-lint run -v ./...` (v2.5.0, config in `.golangci.yml`)
- Unit tests with race detection + coverage
- E2E tests in Docker (`--tags=e2e`)

## Code Style Guidelines

### Imports

Three groups separated by blank lines, alphabetically sorted within each:

```go
import (
    // 1. Standard library
    "context"
    "errors"
    "fmt"
    "log/slog"

    // 2. Third-party packages
    "github.com/labstack/echo/v4"
    "google.golang.org/grpc"

    // 3. Internal packages (github.com/bitcoin-sv/arc/...)
    "github.com/bitcoin-sv/arc/internal/blocktx/store"
    "github.com/bitcoin-sv/arc/internal/global"
)
```

Use named aliases only when package names would collide: `sdkTx "github.com/bsv-blockchain/go-sdk/transaction"`.

### Formatting

- **gofmt -s** is the only enforced formatter. No goimports.
- No line-length limit is enforced.
- Pre-commit hooks: trailing whitespace, end-of-file newline, no merge conflict markers.

### Naming Conventions

| Element | Convention | Example |
|---|---|---|
| Files | `snake_case.go` | `processor_helpers.go`, `nats_jetstream_client.go` |
| Packages | `snake_case` (project convention) | `k8s_watcher`, `blocktx_api`, `node_client` |
| Exported constants | `PascalCase` | `SubmitTxTopic`, `DustLimit` |
| Unexported constants | `camelCase` | `maxInputsDefault`, `clearCacheInterval` |
| Interfaces | Noun-based; `I` suffix when a concrete struct has the same name | `Store`, `Sender`, `ProcessorI`, `PeerI` |
| Sentinel errors | `ErrPascalCase` | `ErrCacheNotFound`, `ErrBlockAlreadyExists` |
| Mock files | `mocks/` subdir, `*_mock.go` | `mocks/arc_client_mock.go` |
| Mock generate files | `*_mocks.go` in source pkg | `broadcaster_mocks.go` (contains `//go:generate moq` directives) |
| Enum constants | `iota` with typed base | `NoneFeeValidation`, `StandardFeeValidation` |

### Error Handling

- **Sentinel errors** in `var` blocks: `var ErrXxx = errors.New("description")`
- **Wrapping**: prefer `errors.Join(ErrSentinel, err)` over `fmt.Errorf`. When `fmt.Errorf` is used, the convention is `%v` (not `%w`).
- **Checking**: `errors.Is(err, ErrSentinel)` and `errors.As(err, &target)`
- **Custom error types** are rare; the main one is `validator.Error` which attaches an API status code.

```go
// Wrapping pattern
return errors.Join(ErrFailedToInsertBlock, err)
return errors.Join(ErrInvalidInput, fmt.Errorf("block height: %d", block.Height), err)
```

### Logging

Uses standard library `log/slog` with structured key-value pairs. Logger is injected via constructors and enriched with `.With()`:

```go
p.logger = logger.With(slog.String("module", "processor"))
p.logger.Info("Processed block", slog.String("hash", h), slog.Uint64("height", n))
p.logger.Error("operation failed", slog.String("err", err.Error()))
```

Errors are logged as `slog.String("err", err.Error())`, not as native error types.

### Constructors and Lifecycle

- **Constructor**: `NewXxx(logger *slog.Logger, deps..., opts ...func(*T)) (*T, error)`
- **Functional options**: `WithBatchSize(n int) func(*Broadcaster)` — applied in constructor loop.
- Options may be in a separate `*_opts.go` file.
- **Lifecycle**: `Start() error` to launch goroutines, `Shutdown()` calls `cancelAll()` + `wg.Wait()`.

### Context

- First parameter in functions, or stored in struct for long-running processors.
- Created via `context.WithCancel(context.Background())` in constructors.
- Checked in `select` loops: `case <-p.ctx.Done(): return`.
- `context.WithTimeout` for request-scoped deadlines.

### Testing

- **External test package** is standard: `package metamorph_test`. Internal package used only to test unexported functions.
- **Table-driven tests**: slice named `tt`, loop var `tc` (or `tests`/`tt`).
- **Assertions**: `testify/require` is dominant (fatal on failure). `testify/assert` used occasionally for non-fatal checks.
- **Mocks**: generated by `moq` into `mocks/` subdirs. Initialize inline: `&mocks.StoreMock{GetFunc: func(...) {...}}`.
- **Verify mock calls**: `require.Equal(t, 1, len(mock.GetCalls()))`.
- **Test comments**: `// given`, `// when`, `// then` pattern.
- **Integration tests**: in `integration_test/` subdirs, require Docker (use `dockertest/v3`).
- **E2E tests**: in `test/` dir, built with `-tags=e2e`.
- **Logger in tests**: `slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))`.

### Commit Messages

Conventional Commits enforced by pre-commit hook:

```
<type>(<scope>): <description>
```

Types: `feat`, `fix`, `chore`, `deps`, `sec`, `refactor`, `docs`, `build`, `ci`, `test`
Scope (optional): ticket ID like `ARCO-001`
Example: `feat(ARCO-001): add new feature to the project`

### Code Generation

- **Protobuf**: `task gen` — 4 proto files in `internal/*/api/` and `pkg/message_queue/`
- **Mocks**: `task gen_go` (runs `go generate ./...`) — uses `moq` v0.5.3
- **API**: `task api` — generates from OpenAPI spec `pkg/api/arc.yaml` via `oapi-codegen`
- Generated code (`*.pb.go`, `*_mock.go`, `pkg/api/arc.go`) **must be committed**.

### Key Dependencies

| Purpose | Package |
|---|---|
| HTTP framework | `github.com/labstack/echo/v4` |
| gRPC | `google.golang.org/grpc` + `protobuf` |
| Database | `pgx/v5`, `sqlx`, `golang-migrate` |
| Message queue | `nats.go` (JetStream + Core) |
| Cache | `go-redis/v8`, `go-cache` |
| Config | `viper` + `cobra` |
| Telemetry | OpenTelemetry (traces + metrics) |
| Testing | `testify`, `dockertest/v3`, `moq` |
