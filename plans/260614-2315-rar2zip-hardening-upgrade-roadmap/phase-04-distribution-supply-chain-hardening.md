---
phase: 4
title: "Distribution & Supply-Chain Hardening"
status: completed
priority: P2
effort: "0.5-1d"
dependencies: []
---

# Phase 4: Distribution & Supply-Chain Hardening

## Overview

Reduce release blast-radius and broaden reach: scope down the Homebrew tap token, pin Actions to
commit SHAs, make install.sh checksum-verification fatal, and add Windows builds + Scoop.

## Requirements

**Functional**
- Windows amd64/arm64 binaries built and published; Scoop manifest available.
- `install.sh` refuses to install unverified when no checksum tool is present (no silent downgrade).

**Non-functional**
- `HOMEBREW_TAP_TOKEN` is least-privilege (single repo, `contents:write`).
- All GitHub Actions pinned to immutable commit SHAs.

## Architecture

- **goreleaser**: add `windows` to GOOS matrix; add a Scoop manifest block (goreleaser `scoops:`); keep
  darwin/linux as-is. Optionally nfpm deb/rpm — DEFERRED unless requested (kept out of scope).
- **CI/release pins (RT-10 — pin the WHOLE release supply chain, not just checkout/setup-go)**: replace `@vN`
  action tags with `owner/action@<sha>  # vN` pins INCLUDING `goreleaser/goreleaser-action` (`release.yml:33`);
  AND pin the goreleaser binary to an exact `version:` (not the floating `~> v2`, `release.yml:34`), verifying
  its checksum. Enable Dependabot for actions.
- **Token scope (RT-10)**: (a) switch `HOMEBREW_TAP_TOKEN` to a fine-grained PAT (single `homebrew-tap` repo,
  `contents:write`); (b) **also** narrow the workflow `permissions:` to job-level least privilege — the release
  job needs `contents: write`, everything else defaults to read (`release.yml:8-9`); (c) **revoke the old broad
  `gho_` token** as an explicit step — switching the secret does NOT revoke the prior credential.
- **install.sh (RT-N)**: make missing `sha256sum`/`shasum` FATAL. State plainly that a self-fetched
  `checksums.txt` is an INTEGRITY (not authenticity) control. Add SIGNATURE verification (goreleaser cosign/
  minisign signing of `checksums.txt`); verify signature then checksum. Document `SKIP_CHECKSUM=1` as
  "disables integrity verification — never use with piped installs".

## Related Code Files

- Modify: `.goreleaser.yaml` (windows GOOS, scoops block)
- Modify: `.github/workflows/ci.yml`, `release.yml` (SHA-pin actions; optionally windows in CI matrix)
- Modify: `scripts/install.sh` (fatal checksum when no tool / SKIP_CHECKSUM opt-out)
- Create: `.github/dependabot.yml` (github-actions ecosystem)
- Modify: `README.md` (Windows/Scoop install; token-scope note)
- Create (separate repo, manual): Scoop bucket entry if not using goreleaser's tap push

## Implementation Steps

1. Add `windows` to goreleaser builds + a Windows CI test row (RT-Q); reconcile per-OS mode-bit expectations in
   fidelity tests (rardecode synthesizes Windows modes differently than Unix `safeMode`). Verify
   `goreleaser build --snapshot --clean` produces win binaries.
2. Add goreleaser `scoops:` manifest targeting the tap (or a `scoop-bucket` repo); document install.
3. SHA-pin ALL actions in `ci.yml` + `release.yml` INCLUDING `goreleaser-action`; pin goreleaser binary `version:`
   to an exact tag + verify checksum; add `.github/dependabot.yml` for github-actions (RT-10).
4. Switch `HOMEBREW_TAP_TOKEN` to a fine-grained PAT; narrow workflow `permissions:` to job-level; **revoke the
   old broad `gho_` token** (explicit step); document scope in README (RT-10).
5. `install.sh`: fatal checksum when no hash tool; add signature (cosign/minisign) verify-then-checksum; document
   `SKIP_CHECKSUM=1` warning (RT-N).
6. Tag a test pre-release (e.g. `v0.2.0-rc1`) to validate the full matrix (incl. Windows + signing) before a real tag.

## Outcome & Decisions (2026-06-15)

**Open questions resolved (user-confirmed):**
1. Windows IS a supported target now → **build + Scoop + CI build-only** (experimental). Full Windows test row deferred
   (fallback/symlink tests are Unix-only; build-tagging them was judged not worth it this round, RT-Q).
2. deb/rpm via nfpm → **deferred** (YAGNI, unchanged).
3. Windows CI → **build-only** row on `windows-latest` (vet + compile; test step skipped via `if: matrix.os != 'windows'`).

**Signing (RT-N):** keyless **cosign** signs `checksums.txt` in goreleaser (Actions OIDC, `id-token: write`, no stored
keys). `install.sh` makes the checksum **fatal** (integrity) and verifies the cosign signature **only if cosign is
present** (best-effort authenticity) — mandatory cosign would break installs for users without it. `SKIP_CHECKSUM=1`
documented as unsafe.

**Action pins (RT-10):** all actions pinned to immutable SHAs with `# vX.Y.Z` comments —
`actions/checkout@34e1148 # v4.3.1`, `actions/setup-go@40f1582 # v5.6.0`,
`goreleaser/goreleaser-action@e435ccd # v6.4.0`, `sigstore/cosign-installer@7e8b541 # v3.10.1`. goreleaser binary
pinned to exact `version: "v2.16.0"` (was floating `~> v2`). `.github/dependabot.yml` added (github-actions + gomod,
weekly) so pins don't go stale. Workflow `permissions:` narrowed: both workflows default to `contents: read`; the
release job opts into `contents: write` + `id-token: write` only.

**Local validation:** `go build ./...` (incl. windows cross-build via CI), `sh -n scripts/install.sh`, ruby YAML
parse of all four YAML files, `goreleaser check` (valid; only the intentional `brews` deprecation), and
`goreleaser build --snapshot` (produced all 6 targets incl. `windows_amd64`/`windows_arm64`).

**Review-found fix (Critical):** the install.sh checksum step originally piped `grep | sha256sum -c -`. A pipe takes
sha256sum's exit status, and `sha256sum -c` on EMPTY stdin exits 0 — so a missing/renamed entry or a checksums.txt
served as an HTML error page would SILENTLY pass, defeating the FATAL-verification goal. Replaced with an exact awk
second-field match that errors on no-match. Verified across 4 cases (match→pass, tamper→fail, no-entry→fatal,
html-page→fatal). Also added an empty-`VERSION` guard after the latest-release lookup.

## Maintainer Runbook (cannot be done from this repo — requires GitHub account/secret access)

These items need GitHub UI / account access and are NOT executable from code; do them before/at the next release:

1. **Create a fine-grained PAT** scoped to ONLY the `homebrew-tap` repo with `contents: write`; store it as the
   `HOMEBREW_TAP_TOKEN` repo secret. (Same for a `scoop-bucket` repo → `SCOOP_TAP_TOKEN`, if publishing Scoop.)
2. **Revoke the old broad `gho_` token** — switching the secret does NOT revoke the prior credential. Delete it in
   GitHub → Settings → Developer settings → Tokens.
3. **Create the `scoop-bucket` repo** under `ongtungduong` (goreleaser pushes the manifest there when `SCOOP_TAP_TOKEN`
   is set; otherwise it skips, mirroring brews).
4. **Pre-release dry-run (step 6):** tag `v0.2.0-rc1` to validate the full matrix — Windows artifacts, cosign
   signatures (`checksums.txt.sig`/`.pem`), and tap pushes — before a real tag.

## Success Criteria

- [x] Release config publishes Windows amd64/arm64 (`.zip`) binaries; Windows **marked experimental** with a
      build-only CI row (RT-Q); Scoop documented in README.
- [x] ALL Actions pinned to SHAs INCLUDING `goreleaser-action` (+ `cosign-installer`); goreleaser binary pinned to
      exact `v2.16.0`; `.github/dependabot.yml` configured (RT-10).
- [x] Workflow `permissions:` narrowed to job-level (`contents: read` default; release job adds `contents: write` +
      `id-token: write`). Fine-grained PAT + **old-token revocation** are maintainer-runbook items (account access
      required — documented above, cannot be done from code) (RT-10).
- [x] `install.sh` missing-hash-tool is FATAL; cosign signature of `checksums.txt` verified best-effort when present
      (integrity always + authenticity when cosign installed); `SKIP_CHECKSUM=1` documented as unsafe (RT-N).
- [~] CI config green locally (`goreleaser check`, snapshot build). Full pre-release dry-run (Windows artifacts +
      live signatures + tap pushes) is a maintainer step — see runbook (cannot tag a real release from here).

## Risk Assessment

- **Windows path/permission semantics** differ (no Unix mode, `\` separators). Mitigation: the tool only
  READS RAR + WRITES ZIP names with `/`; verify `safeMode`/sanitize behave on Windows; add a windows CI row.
- **SHA-pinning maintenance burden.** Mitigation: Dependabot auto-PRs updates.
- **Token rotation** could break releases if misconfigured. Mitigation: test with a pre-release tag first.

## Security Considerations

- This phase IS the supply-chain hardening: least-privilege token, immutable action pins, mandatory
  checksum verification — directly addresses security-audit findings #4, #5, #6.

## Open Questions

1. Is Windows now a supported target (the original plan said "not a target now")? Confirm before building.
2. deb/rpm via nfpm — in scope now or deferred? (Defaulting to deferred per YAGNI.)
3. Add a Windows row to CI test matrix, or build-only (no test run) on Windows?
