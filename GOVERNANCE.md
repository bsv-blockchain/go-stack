# Governance

## Mission

Make the BSV Distributed Applications Stack — across TypeScript, Go, Python, and Rust — easier to maintain, cleaner to read, measurably faster, demonstrably more secure, and boringly reliable. TypeScript is the canonical reference. Specs are the contract.

Full programme: [MBGA.md](https://github.com/bsv-blockchain/ts-stack/blob/main/MBGA.md) (in ts-stack)

## Roles

| Role | Responsibility |
|------|----------------|
| **Stack Architect** | Spec ownership, cross-language consistency, final call on cross-domain boundaries |
| **Domain Leads** (SDK, Wallet, Overlay, Messaging, Network, Helpers) | Parity and conformance across languages within domain; own CODEOWNERS entries |
| **Go Language Lead** | Idiomatic Go quality across all packages in this repo |
| **Security Lead** | Supply chain, threat models, coordinated disclosure, signing infrastructure |
| **QA / Release Engineer** | CI, conformance dashboard, release process, interop matrix |

## Decision Making

- **Day-to-day changes** (bug fixes, tests, docs): single reviewer approval sufficient.
- **New packages or major refactors**: Domain Lead approval required.
- **Cross-domain changes** (touching ≥2 domains): Stack Architect approval required.
- **Breaking changes**: require `BREAKING.md` entry + 60-day deprecation window (security fixes exempt).
- **Spec amendments**: require a spec PR merged before implementation changes.

Disputes: Stack Architect holds final call on cross-domain design; Go Language Lead holds final call on idiomatic code within this repo.

## Versioning and Releases

- **Semver** per module. Monorepo root has no version.
- **Release cadence**: SDKs on demand (reviewed weekly); services monthly; apps independent; security out of band.
- **Breaking changes**: `BREAKING.md` at repo root; minimum 60-day deprecation notice; cross-language parity issues auto-generated.
- **Module versions**: Each Go module in `packages/` or `apps/` is versioned independently via git tags.

## Contribution

### Before You Start

- Check open issues and the [roadmap project](https://github.com/orgs/bsv-blockchain/projects) to avoid duplicate work.
- For large changes, open an issue first to discuss approach.
- Security vulnerabilities: see [SECURITY.md](./SECURITY.md).

### Pull Request Requirements

- [ ] CI green (build, tests)
- [ ] Conformance vectors updated if behaviour changes
- [ ] Regression test added for bug fixes (shared vector if cross-language)
- [ ] `BREAKING.md` updated if breaking (with migration notes)
- [ ] Docs updated if user-facing behaviour changes

### Review SLA

Every PR receives a public review within **5 business days**.

### Good First Issues

Look for `good-first-issue` labels. The easiest entry points:
- Port a single conformance vector from ts-stack.
- Add missing tests to a package below RL3.
- Fix a `go vet` or `staticcheck` warning in a Tier 3 package.

## Parity with ts-stack

| Class | Meaning | Release gate |
|-------|---------|--------------|
| **Required** | Production, cross-language public APIs | Must pass shared vectors; blocks release |
| **Intended** | Planned public support | Status visible; not blocking |
| **Best-effort** | Useful but not critical | Not blocking |
| **Unsupported** | Not implemented or not planned | Explicit in docs |

Cross-language parity SLA: Go within 30 days of TS GA.

## Reliability Levels

Components are rated RL0–RL5. See the MBGA programme doc for the full rubric.

## Code of Conduct

This project follows the [Contributor Covenant v2.1](https://www.contributor-covenant.org/version/2/1/code_of_conduct/).
