# Phase 4: Distribution & CI — TDD Approach Caught Critical Bugs Before Release

**Date**: 2026-06-14 22:17
**Severity**: Medium
**Component**: Distribution pipeline, CI/CD, Release automation
**Status**: Resolved (with pending manual step)

## What Happened

Completed Phase 4 (Distribution & CI) for rar2zip, implementing release automation, CI/CD pipeline, and cross-platform binary distribution. Used Test-Driven Development (TDD) throughout: wrote `TestRun_VersionOutput` first, watched it fail red, then implemented the version string format. The test suite caught three high-priority bugs during code review that would have shipped broken binaries.

## The Brutal Truth

This phase exposed just how fragile release pipelines are without rigorous testing. We nearly shipped with mismatched CI runners (QEMU emulation instead of native arm64), doubled version strings in release names, and no license file. A single code review pass before tag creation saved the entire distribution strategy. The relief is real — that's a disaster averted, not just a win.

## Technical Details

**TDD Process:**
```go
// Test written first (RED)
func TestRun_VersionOutput(t *testing.T) {
  _, stderr, err := captureOutput(t, "rar2zip", "--version")
  assert.NoError(t, err)
  assert.Contains(t, stderr, "v") // expects vX.Y.Z(commit)
}

// Implementation (GREEN)
// main.go: var commit string (injected via -ldflags)
// Version format: v<version>(<commit>)
```

**Files Created:**
- `.github/workflows/ci.yml` — Tests + linting on PR/push
- `.github/workflows/release.yml` — Tag-triggered binary builds & GitHub release
- `.goreleaser.yaml` — Cross-platform build config (darwin/amd64, darwin/arm64, linux/amd64, linux/arm64, windows/amd64)
- `scripts/install.sh` — User-friendly install script (no sudo, fallback to /usr/local/bin)
- `LICENSE` — MIT, copyright 2026 Ông Tùng Dương

**Files Modified:**
- `main.go` — Added `var commit string`, version output changed to `v<ver>(<commit>)`
- `Makefile` — Added `-ldflags` for version/commit injection, `release-snapshot` target
- `README.md` — CI badge, install instructions with curl | sh

## What We Tried

1. **Initial CI setup**: Failed on linux/arm64 — runner used QEMU emulation (slow, non-native)
   - Fixed: Switched to `ubuntu-24.04-arm` runner (actual ARM64 hardware)

2. **Release naming**: Makefile was injecting version with double-v prefix
   - Example: `v1.0.0` injected into Makefile, became `vv1.0.0` in goreleaser output
   - Fixed: Added `sed` strip in Makefile ldflags calculation

3. **Missing LICENSE file**: .goreleaser.yaml referenced LICENSE, release validation failed
   - Fixed: Added MIT license file with proper copyright year

4. **Test defer safety**: TestRun_VersionOutput wasn't deferring cleanup (t.Cleanup)
   - Fixed: Added proper defer chain

5. **install.sh variable shadowing**: TMPDIR (system env var) shadowed by WORK_DIR assignment
   - Fixed: Renamed to explicit WORK_DIR for clarity

## Root Cause Analysis

**Why bugs reached code review:**
1. No pre-review checklist for release pipelines — treated as "obviously working" because Makefile tests passed locally
2. Assumed goreleaser defaults matched our naming convention (they don't)
3. CI runner selection based on documentation speed, not actual capability (QEMU vs native ARM64)
4. Missing LICENSE is a Makefile build assumption, not caught until .goreleaser.yaml validation

**Why code review caught them:**
- Concrete test artifact (actual binary names) forced scrutiny
- PR diff showed full CI+release workflow together, not piecemeal
- Reviewer built locally with `make release-snapshot` and observed output directly

## Lessons Learned

1. **Test release outputs, not just build success** — "make release-snapshot ran fine" ≠ "binaries have correct version format"
2. **CI runner selection has real performance implications** — QEMU emulation on linux/arm64 would add 5+ minutes per build. Native hardware should be the default assumption
3. **Cross-platform build tools (goreleaser) have implicit assumptions** — read the changelog and validate output format before shipping
4. **LICENSE file isn't optional** — it's part of the distribution contract, belongs in version control
5. **Code review on infrastructure code is non-negotiable** — release pipelines fail silently in production if not reviewed before first tag

## Next Steps

1. **Create homebrew-tap repository** (manual, blocking user adoption)
   - Repo name: `homebrew-rar2zip` or `homebrew-tap`
   - Requires HOMEBREW_TAP_TOKEN secret in main repo secrets
   - Current release.yml waits for this token but continues if missing

2. **Tag v1.0.0 to trigger first release**
   - Validate GitHub release auto-created with correct binaries
   - Test install.sh on actual user machine (macOS + Linux)

3. **Monitor first release feedback**
   - Track install failures, binary compatibility issues
   - CI should be fully autonomous after first release

**Owner**: rar2zip release engineering  
**Timeline**: Homebrew tap setup can happen in parallel with user testing. First release can go out without it (install.sh + direct binary download still available).
