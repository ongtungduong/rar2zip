---
title: rar2zip CLI — RAR to ZIP converter for macOS & Linux
description: >-
  Cross-platform CLI that converts .rar archives to .zip. Go single static
  binary, pure-Go RAR decoder (no system unrar dependency).
status: in-progress
priority: P2
branch: ''
tags:
  - cli
  - go
  - archive
  - rar
  - zip
blockedBy: []
blocks: []
created: '2026-06-13T03:04:52.365Z'
createdBy: 'ck:plan'
source: skill
---

# rar2zip CLI — RAR to ZIP converter for macOS & Linux

## Overview

`rar2zip` is a single-binary CLI for macOS and Linux that converts `.rar` archives into `.zip`.
Built in **Go**, decoding RAR with the **pure-Go `github.com/nwaples/rardecode/v2`** library and
re-packing with the stdlib **`archive/zip`**. No system `unrar`/`7z` dependency — the binary is
self-contained, so `brew install` / `curl | sh` "just works".

**Core flow:** open RAR → stream each entry → write into a new ZIP (no full extraction to disk).

**Roadmap is incremental:** Phase 1 ships a usable MVP (`rar2zip foo.rar` → `foo.zip`). Each later
phase is independently shippable and adds UX, batch handling, distribution, then advanced options.
Phases 2–5 are deliberately lighter-detail; re-plan a phase with `/ck:plan` when you reach it.

## Tech Stack

| Concern | Choice | Notes |
|---------|--------|-------|
| Language | Go (>= 1.22) | Single static binary, fast startup, trivial cross-compile |
| RAR decode | `github.com/nwaples/rardecode/v2` | Pure-Go; reads RAR4 & RAR5; multi-volume & password support |
| ZIP write | stdlib `archive/zip` | No external dep |
| CLI parsing | stdlib `flag` (MVP) → `spf13/cobra` (Phase 2 if subcommands needed) | Start minimal |
| Build/dist | `goreleaser` + GitHub Actions + Homebrew tap (Phase 4) | |

## Phases

| Phase | Name | Status | Ships |
|-------|------|--------|-------|
| 1 | [MVP Core Conversion](./phase-01-mvp-core-conversion.md) | ✅ Completed | `rar2zip <in.rar>` → `<in>.zip` |
| 2 | [UX & Robustness](./phase-02-ux-robustness.md) | ✅ Completed | flags, overwrite/progress, mode+mtime preserved, errors |
| 3 | [Batch & Advanced Archives](./phase-03-batch-advanced-archives.md) | ✅ Completed | multiple inputs, globs, multi-volume, password |
| 4 | [Distribution & CI](./phase-04-distribution-ci.md) | ✅ Completed | CI, releases, Homebrew, install script |
| 5 | [Advanced Features](./phase-05-advanced-features.md) | Pending | compression level, large-file streaming tuning, optional shell-out fallback |

## Dependencies

- External Go module: `github.com/nwaples/rardecode/v2` (verify exact API at implementation: `OpenReader`, `(*ReadCloser).Next()`, `FileHeader{Name, IsDir, ModificationTime, Mode()}`).
- No cross-plan dependencies (greenfield repo).

## Project Layout (target)

```
rar2zip/
├── go.mod / go.sum
├── main.go                       # entry: parse args, call convert
├── internal/
│   └── convert/
│       └── convert.go            # Convert(srcRar, dstZip) — core rar→zip stream
├── Makefile                      # build / test / lint helpers
├── README.md
└── docs/                         # project docs (per repo rules)
```

## Out of Scope (MVP)

- Creating RAR archives (RAR creation is proprietary — never feasible).
- GUI. Windows builds (can add later via Go cross-compile; not a target now).
- zip→rar (reverse) direction.
