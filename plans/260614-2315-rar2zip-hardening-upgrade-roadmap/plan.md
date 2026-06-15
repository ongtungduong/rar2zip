---
title: "rar2zip Hardening & Upgrade Roadmap"
description: >-
  Audit-driven roadmap to fix correctness/security defects and upgrade rar2zip
  from a solid MVP into a robust, production-grade RAR→ZIP tool. Sourced from a
  5-dimension audit (features, performance, logic, architecture, security).
status: pending
priority: P1
branch: "main"
tags: [cli, go, hardening, security, performance]
blockedBy: []
blocks: []
created: "2026-06-14T16:22:55.275Z"
createdBy: "ck:plan"
source: skill
---

# rar2zip Hardening & Upgrade Roadmap

## Overview

A comprehensive audit (security, performance, logic, architecture, features) found the
codebase **well-built** — solid Zip-Slip defense, atomic writes, clean concurrency — but
surfaced two genuine correctness bugs, one exploitable resource-exhaustion gap, a real
CJK data-loss issue, and a near-zero CI coverage problem (no committed fixture). This
roadmap fixes the must-fix defects first, then layers on robustness, performance, and polish.

Phases are ordered by severity. **Phase 1 is mandatory before any release**; later phases are
independently shippable. Each phase ends green (`go test ./...`, `go vet`, `gofmt`) and follows
TDD where behavior changes.

## Audit Source

Reports under `plans/reports/` (260614-23xx): security, performance, architecture/logic, features/UX.

## Phases

| Phase | Name | Priority | Status |
|-------|------|----------|--------|
| 1 | [Correctness & Safety Hardening](./phase-01-correctness-safety-hardening.md) | P1 | ✅ Completed |
| 2 | [Real-World Robustness](./phase-02-real-world-robustness.md) | P2 | ✅ Completed |
| 3 | [Performance & Benchmarks](./phase-03-performance-benchmarks.md) | P2 | ✅ Completed |
| 4 | [Distribution & Supply-Chain Hardening](./phase-04-distribution-supply-chain-hardening.md) | P2 | ✅ Completed |
| 5 | [UX & Docs Polish](./phase-05-ux-docs-polish.md) | P3 | Pending |

## Out of Scope (YAGNI — defer until demand)

- Public library promotion of `internal/convert` (move is trivial when a real consumer appears).
- Docker image, winget, asdf/mise, deb/rpm (revisit if user base grows).
- Per-file include/exclude globs, path flattening, stdout streaming, comment preservation.
- Parallel compression within a single archive (breaks the streaming model; KISS).

## Dependencies

- Supersedes the Phase-5 backlog of `plans/260613-1000-rar2zip-cli` (stdin/stdout, ZIP64 tuning).
  ZIP64 verification is folded into Phase 3 here. The old plan's frontmatter now records `supersededBy`.
- External: `github.com/nwaples/rardecode/v2` — confirmed to expose ONLY a pre-decoded `Name string`
  (no raw bytes / codepage). This makes native-path encoding transcoding infeasible (see Phase 2 / RT-1).

## Red Team Review

### Session — 2026-06-14
**Findings:** 15 (15 accepted, 0 rejected) + 3 lower-priority noted
**Severity breakdown:** 5 Critical, 5 High, 5 Medium
**Method:** 3 hostile reviewers (Security Adversary, Failure Mode Analyst, Assumption Destroyer), each
requiring `file:line` codebase evidence. All findings passed the evidence filter.

| # | Finding | Severity | Disposition | Applied To |
|---|---------|----------|-------------|------------|
| RT-1 | Native encoding transcoding infeasible (rardecode exposes only decoded `Name`) | Critical | Accept | Phase 2 (rescoped to fallback-only/cut; `--list` primary; P1→P2) |
| RT-2 | Fixture creation has no path (RAR creation is proprietary) | Critical | Accept | Phase 1 (step-0 provenance gate) |
| RT-3 | Emitter refactor would drop symlink/non-regular handling | Critical | Accept | Phase 1 (behavior-neutral refactor, own commit, preserve handling) |
| RT-4 | `io.LimitReader` truncates silently instead of erroring | Critical | Accept | Phase 1 (counting reader that errors; no-output-on-trip) |
| RT-5 | Map-keyed fidelity test can't catch dedup data loss | Critical | Accept | Phase 1 (add count/slice assertion) |
| RT-6 | Fallback bomb open until Phase 3 but Phase 1 is release-gating | High | Accept | Phase 1 (bring forward bound OR doc unsafe-vs-untrusted) |
| RT-7 | Fail-closed dedup may break legit multi-volume archives | High | Accept | Phase 1 (experiment first; reject only post-sanitize collisions) |
| RT-8 | `--jobs` default change alters multi-glob output (not "single-archive only") | High | Accept | Phase 3 (per-job buffering; CHANGELOG note) |
| RT-9 | ZIP64 test can't inject sparse reader on native path | High | Accept | Phase 3 (test via fallback path; note native gap) |
| RT-10 | Supply chain incomplete (GITHUB_TOKEN scope, goreleaser pin, token revocation) | High | Accept | Phase 4 (full pinning + job perms + revoke step) |
| RT-11 | `--list` bypasses MaxEntries cap | Medium | Accept | Phase 1/2 (cap in iteration core, not emit step) |
| RT-12 | Inter-job dst-collision TOCTOU under concurrent default | Medium | Accept | Phase 1 (collision detection in job-builder) |
| RT-13 | Argv hardening misdiagnosed (`--` terminator + `@listfile`, not abs-path) | Medium | Accept | Phase 1 (real-invariant test) |
| RT-14 | Bomb defaults arbitrary + default-on breaks backward compat | Medium | Accept | Phase 1 (opt-in/warn-only default) |
| RT-15 | `supersede` claim never wired into old plan | Medium | Accept | old plan.md (`supersededBy` added) |
| RT-N | install.sh checksum is integrity-not-authenticity | Low | Accept | Phase 4 (add signature) |
| RT-S | Phase 3 temp relocation doubles output-FS footprint | Medium | Accept | Phase 3 (keep extraction in system temp) |
| RT-Q | Windows mode-fidelity divergence asserted without test | Medium | Accept | Phase 4 (Windows CI row / mark experimental) |

### Whole-Plan Consistency Sweep
Re-read `plan.md` + all five phase files after applying findings. Checks passed:
- No surviving "native transcoding" as a deliverable (only infeasibility statements); `encoding.go` removed from
  Related Code Files; Phase 2 priority P2 consistent in frontmatter, table, and RT-1 row.
- No "single-archive unchanged" stale claim in Phase 3; `--jobs` output-change acknowledged everywhere.
- No "io.LimitReader-style" implementation language remains; bomb limit is a counting-reader-that-errors throughout.
- `10 GB / 100k` appears only as the explicitly-unjustified value flagged by RT-14 (opt-in/warn-only decision pending).
- Supersession reciprocated: old plan carries `supersededBy`; new plan Dependencies references it.
- **No unresolved contradictions.** Open questions remain (fixture provenance, cap defaults, encoding go/no-go,
  Windows CI) — these are decisions to make at implementation step 0, not plan contradictions.
