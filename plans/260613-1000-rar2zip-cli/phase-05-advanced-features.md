---
phase: 5
title: "Advanced Features"
status: in-progress
priority: P3
effort: "1-2d"
dependencies: [3]
---

# Phase 5: Advanced Features

## Overview

Optional power-user features. Strictly YAGNI-gated — build only what real usage demands.

## Delivered

Four features shipped (TDD, all flag-gated, defaults preserve Phase 1–2 behavior):

- [x] **Compression control** — `--store` (zip.Store) and `--level 1..9` (Deflate level).
      `internal/convert/compress.go`. `--store`+`--level` is a usage error.
- [x] **Self-verify** — `--verify` reopens the output ZIP and checks entry count + sizes;
      a failed check removes the output. `internal/convert/verify.go`.
- [x] **JSON output** — `--json` emits `{succeeded,failed,results[]}` on stdout. `json_output.go`.
- [x] **Shell-out fallback** — `--allow-fallback` uses system `unrar`/`7z` when the pure-Go
      decoder fails; returns the original Go error if no tool is found.
      `internal/convert/fallback.go`. Symlink/Zip-Slip neutralization preserved.

Remaining backlog (not built): stdin/stdout streaming, large-file/ZIP64 throughput tuning.

## Candidate Features (pick on demand)

- **Compression control:** `--level 0..9` and `--store` (no compression) mapping to `zip.Deflate` / `zip.Store`.
- **Shell-out fallback:** if the pure-Go decoder can't read an archive (exotic RAR5/encryption), optionally fall back to system `unrar`/`7z` when present (`--allow-fallback`). Restores the "hybrid" option deferred from initial scope.
- **Large-file tuning:** buffered/streamed copy sizing; report throughput; ensure ZIP64 for >4 GB entries (stdlib handles ZIP64 automatically — verify).
- **Stdin/stdout streaming:** `rar2zip - < in.rar > out.zip` for pipelines (note: RAR needs seeking, so this may require buffering to a temp file).
- **Self-verify:** after writing, reopen the ZIP and confirm entry count/sizes match the source.
- **Logging/JSON output:** `--json` summary for scripting.

## Architecture Notes

- Each feature is additive behind a flag; keep `Convert` core stable, extend via `Options`.
- Shell-out fallback must be opt-in and clearly logged (changes the "no system dependency" guarantee).

## Implementation Approach

Re-plan with `/ck:plan` when a specific feature is prioritized — do not pre-build the whole list.

## Success Criteria (per chosen feature)

- [ ] Feature is flag-gated, documented, and tested.
- [ ] Defaults preserve Phase 1–2 behavior (no regressions).
- [ ] `go test ./...` green.

## Risk Assessment

- Scope creep — this phase is a backlog, not a commitment. Promote items only with a concrete need.
- Fallback reintroduces an external dependency — gate behind explicit flag + clear messaging.
