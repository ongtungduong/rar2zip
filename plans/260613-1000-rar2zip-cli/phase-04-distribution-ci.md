---
phase: 4
title: "Distribution & CI"
status: completed
priority: P2
effort: "1d"
dependencies: [1]
---

# Phase 4: Distribution & CI

## Overview

Make the binary easy to get and keep quality gated. Can run in parallel with Phase 2/3 since it
only needs a buildable Phase-1 binary.

## Requirements

**Functional**
- CI on push/PR: `go vet`, `go test ./...`, `gofmt -l` check, build matrix (darwin/linux × amd64/arm64).
- Tagged releases publish prebuilt binaries (and checksums) to GitHub Releases.
- Homebrew install: `brew install ongtungduong/tap/rar2zip`.
- One-line install script: `curl -fsSL .../install.sh | sh` (downloads correct asset for OS/arch).

**Non-functional**
- Reproducible-ish builds with version/commit embedded via `-ldflags -X`.

## Architecture

- `goreleaser` config builds the matrix, generates archives + checksums, creates the GitHub release, and updates the Homebrew tap formula.
- GitHub Actions: `ci.yml` (test/lint/build on PR) + `release.yml` (on tag → goreleaser).

## Related Code Files

- Create: `.github/workflows/ci.yml`, `.github/workflows/release.yml`.
- Create: `.goreleaser.yaml`.
- Create: `scripts/install.sh`.
- Modify: `main.go` (version/commit ldflags vars), `README.md` (install instructions, badges).
- Create (separate repo): `homebrew-tap` with the generated formula.

## Implementation Steps

1. Add `ci.yml` (vet, test, gofmt, build matrix).
2. Add `var version, commit string` set via ldflags; wire `--version`.
3. Add `.goreleaser.yaml`; test locally with `goreleaser release --snapshot --clean`.
4. Create `homebrew-tap` repo + token; enable brew publish in goreleaser.
5. Add `install.sh` (detect uname/arch, fetch matching release asset, verify checksum).
6. Cut `v0.1.0` tag → verify release artifacts + `brew install` + install script end-to-end.

## Success Criteria

- [x] CI green on PRs; release workflow produces signed/checksummed binaries for all targets.
- [ ] `brew install ongtungduong/tap/rar2zip` installs a working binary. *(requires creating `homebrew-tap` repo + `HOMEBREW_TAP_TOKEN` secret)*
- [x] `curl ... | sh` installs a working binary on macOS and Linux.
- [x] `rar2zip --version` prints the released version + commit.

## Risk Assessment

- Code signing / Gatekeeper on macOS — unsigned binaries may warn; document the workaround or add notarization later.
- Secrets management — GitHub token scope for tap publishing; keep least-privilege.
