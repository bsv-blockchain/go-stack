# go-stack

BSV Go monorepo — SDK, wallet toolbox, overlay services, messaging, and supporting infrastructure.

[![CI](https://github.com/bsv-blockchain/go-stack/actions/workflows/ci.yml/badge.svg)](https://github.com/bsv-blockchain/go-stack/actions/workflows/ci.yml)

## Structure

```
packages/sdk/        — Core SDK, transaction building, script templates
packages/wallet/     — Wallet toolbox, 402-pay
packages/overlays/   — Overlay services, discovery, UHRP storage
packages/messaging/  — Message box server, paymail
packages/network/    — Chaintracks, broadcast client, broadcast
packages/middleware/ — BSV auth middleware
conformance/         — Conformance runner (see GO_PLAN.md in ts-stack)
```

**14 modules** across 6 domains. Apps (ARC, Arcade, merkle-service, go-broadcast-app) remain separate repos.

---

## Package Map

### SDK — `packages/sdk/`

| Module | Path |
|--------|------|
| [go-sdk](packages/sdk/go-sdk) | `github.com/bsv-blockchain/go-sdk` |
| [go-subtree](packages/sdk/go-subtree) | `github.com/bsv-blockchain/go-subtree` |
| [go-script-templates](packages/sdk/go-script-templates) | `github.com/bsv-blockchain/go-script-templates` |

### Wallet — `packages/wallet/`

| Module | Path |
|--------|------|
| [go-wallet-toolbox](packages/wallet/go-wallet-toolbox) | `github.com/bsv-blockchain/go-wallet-toolbox` |
| [go-402-pay](packages/wallet/go-402-pay) | `github.com/bsv-blockchain/go-402-pay` |
| [go-402-pay/echo](packages/wallet/go-402-pay/echo) | `github.com/bsv-blockchain/go-402-pay/echo` |
| [go-402-pay/fiber](packages/wallet/go-402-pay/fiber) | `github.com/bsv-blockchain/go-402-pay/fiber` |
| [go-402-pay/gin](packages/wallet/go-402-pay/gin) | `github.com/bsv-blockchain/go-402-pay/gin` |

### Overlays — `packages/overlays/`

| Module | Path |
|--------|------|
| [go-overlay-services](packages/overlays/go-overlay-services) | `github.com/bsv-blockchain/go-overlay-services` |
| [go-overlay-discovery-services](packages/overlays/go-overlay-discovery-services) | `github.com/bsv-blockchain/go-overlay-discovery-services` |
| [go-uhrp-storage-server](packages/overlays/go-uhrp-storage-server) | `github.com/bsv-blockchain/go-uhrp-storage-server` |

### Messaging — `packages/messaging/`

| Module | Path |
|--------|------|
| [go-message-box-server](packages/messaging/go-message-box-server) | `github.com/bsv-blockchain/go-message-box-server` |
| [go-paymail](packages/messaging/go-paymail) | `github.com/bsv-blockchain/go-paymail` |

### Network — `packages/network/`

| Module | Path |
|--------|------|
| [go-chaintracks](packages/network/go-chaintracks) | `github.com/bsv-blockchain/go-chaintracks` |
| [go-broadcast-client](packages/network/go-broadcast-client) | `github.com/bitcoin-sv/go-broadcast-client` |
| [go-broadcast](packages/network/go-broadcast) | `github.com/mrz1836/go-broadcast` |

### Middleware — `packages/middleware/`

| Module | Path |
|--------|------|
| [go-bsv-middleware](packages/middleware/go-bsv-middleware) | `github.com/bsv-blockchain/go-bsv-middleware` |

---

## Development

### Prerequisites

- Go 1.26.0

### Setup

This repo uses a [Go workspace](https://go.dev/ref/mod#workspaces) (`go.work`) to link all modules locally. No changes to import paths are required.

```sh
# Build all packages in the workspace
go build ./...

# Test all packages
go test ./...
```

### Build a specific domain

```sh
go build ./packages/sdk/...
go build ./packages/wallet/...
```

### Working with individual modules

Each subdirectory is a self-contained Go module with its own `go.mod`. You can work inside any module directory independently:

```sh
cd packages/sdk/go-sdk
go test ./...
```

---

## Architecture

Dependencies flow inward toward the SDK:

```
Overlays / Messaging / Middleware
         |
       Wallet
         |
       Network
         |
        SDK
```

Middleware is used by any domain. Conformance tests all domains against shared vector sets.

---

## Updating packages (git subtree)

Each package was originally a standalone repo — commit history is preserved via `git subtree`. To pull upstream changes for a package:

```sh
git subtree pull --prefix=packages/sdk/go-sdk ~/git/go/go-sdk main
```

---

## Conformance

The conformance runner (shared vectors with ts-stack) is planned — see `conformance/README.md` and `GO_PLAN.md` in the ts-stack repo.

---

## License

See individual package directories for license terms.
