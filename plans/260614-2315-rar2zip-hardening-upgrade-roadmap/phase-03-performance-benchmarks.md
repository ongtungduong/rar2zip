---
phase: 3
title: "Performance & Benchmarks"
status: completed
priority: P2
effort: "1d"
dependencies: [1]
---

# Phase 3: Performance & Benchmarks

## Overview

Lock throughput against regressions with benchmarks, take the high-ROI perf wins (copy buffer,
sensible `--jobs` default), verify ZIP64/>4 GB behavior, and bound the fallback temp-dir cost.

## Requirements

**Functional**
- `--jobs` defaults to a multi-core value for batch (capped) instead of serial 1.
- Fallback extraction fails early/clearly on insufficient temp space and lands temp on a sane filesystem.

**Non-functional**
- Benchmarks exist for the hot paths; ZIP64 (>4 GB entry) is explicitly tested.
- Large-entry throughput improved via larger pooled copy buffer; memory stays bounded.

## Architecture

- **Copy buffer**: replace `io.Copy` with `io.CopyBuffer` using a `sync.Pool` of ~512 KB buffers in the
  shared emitter (from Phase 1). Cuts syscall frequency on large entries; pool avoids per-entry alloc.
- **--jobs default (RT-8 — NOT "single-archive only")**: change to `min(runtime.NumCPU(), 4)`. Output mode in
  `main.go` is keyed on `len(jobList)` (`main.go:113`), NOT `--jobs` — so a `*.rar` glob is already multi-element
  and this change makes that common case run CONCURRENTLY, interleaving `onStart` stderr nondeterministically
  (`batch.go:38-40`). This IS an observable output change for any invocation with ≥2 inputs. Mitigate by
  buffering each job's human output and flushing in job order (deterministic), and document the change in CHANGELOG.
- **ZIP64 (RT-9 — native path has no injectable seam)**: `writeZip` takes a concrete `*rardecode.ReadCloser`
  (`convert.go:127`), so a fake >4 GB sparse reader CANNOT be injected on the native path without a real >4 GB
  RAR (which the project cannot create). Target the FALLBACK path instead: feed a 4 GB+ sparse file into
  `writeZipFromDir` via a temp dir, asserting ZIP64 round-trip + verify accounting past 4 GB. Gate behind
  `testing.Short()`. (Optional larger refactor: add an entry-iterator interface seam to `writeZip` — budget separately.)
- **Fallback temp (RT-S — do NOT relocate the whole extraction tree)**: the FINAL zip-temp already shares the
  output FS (`fallback.go:84`) for atomic rename — keep that. Do NOT move the multi-GB EXTRACTION tree onto the
  output volume (it can fill the user's home disk and needs ~2× headroom = extracted tree + zip). Keep extraction
  in system temp (or add `--temp-dir`); the free-space pre-check must reserve `extracted_size + estimated_zip_size`.

## Related Code Files

- Modify: `internal/convert/emit.go` (CopyBuffer + sync.Pool buffer)
- Modify: `internal/convert/fallback.go` (temp dir location, free-space pre-check)
- Modify: `main.go`, `cli_args.go` (`--jobs` default change + help text)
- Create: `internal/convert/convert_bench_test.go` (benchmarks)
- Add test: `TestConvert_ZIP64` (synthetic >4 GB entry, guarded/short-mode aware)

## Implementation Steps

1. **Benchmarks first** (baseline): `BenchmarkConvertNative`, `BenchmarkWriteZip_LargeEntry`,
   `BenchmarkRunBatch_Parallel`, `BenchmarkVerify_LargeMap`. Capture before-numbers.
2. **Copy buffer**: introduce pooled 512 KB buffer + `io.CopyBuffer` in the emitter; re-bench, record delta.
3. **--jobs default**: change to `min(NumCPU(),4)`; buffer per-job human output and flush in job order so the
   `*.rar`-glob case stays deterministic (RT-8); update help + README; note the behavior change in CHANGELOG.
4. **ZIP64 test (fallback path, RT-9)**: feed a >4 GB sparse file into `writeZipFromDir` via a temp dir behind
   `testing.Short()`; assert round-trip + `verify` accounting past the 4 GB boundary. (Native-path ZIP64 needs a
   real >4 GB RAR — out of reach; note the coverage gap.)
5. **Fallback free-space (RT-S)**: keep EXTRACTION in system temp (or `--temp-dir`); add a `Statfs` pre-check
   reserving `extracted_size + estimated_zip_size`; clear error when insufficient. Do not relocate the extraction tree.
6. Full suite + vet + fmt; include `go test -bench` numbers in the journal/PR.

## Outcome & Decisions (2026-06-15)

**Open questions resolved (user-confirmed):**
1. `--jobs` default → **`min(NumCPU, 4)`** (`maxDefaultJobs`/`defaultJobs` in `main.go`). Modest cap for I/O-bound work.
2. ZIP64 coverage → **fallback path** (cheaper; native seam not built). Native-path ZIP64 gap noted (RT-9).
3. `--temp-dir` → **not added**. Extraction stays in system temp; users redirect via `TMPDIR`. Free-space handling
   is a **lightweight floor** (avail < archive size → early clear error), not a full reservation — exact extracted
   size is unknowable once the native decoder has failed; the extractor's ENOSPC stays the hard bound.

**Benchmark deltas (large-entry write, file-backed, Store, Apple M-series):**
- `io.Copy` (32 KB): ~27.0 ms/op, ~2482 MB/s, 38216 B/op, 27 allocs/op
- `io.CopyBuffer` (512 KB pooled): ~25.2 ms/op, ~2665 MB/s, ~5–17 KB B/op, 26 allocs/op
- ≈7% throughput gain + ~60–85% fewer bytes allocated; memory bounded (pooled, shared across jobs).

**RT-8 output determinism:** chose "ordered final summary only" — the live concurrent per-job "converting X" line
was removed (it interleaved nondeterministically); `report()` already prints per-job results in job order. Documented
in `docs/project-changelog.md`.

## Success Criteria

- [x] Benchmarks committed (`convert_bench_test.go`): large-entry write, `--verify` over many entries, fixture-gated
      native convert; runnable via `go test -bench .`.
- [x] Pooled `CopyBuffer` shows measurable large-entry gain (~7% throughput, ~60–85% fewer allocs — recorded above).
- [x] `--jobs` defaults to `min(NumCPU,4)`; multi-input output stays deterministic (per-job result lines in job order);
      behavior change documented in `docs/project-changelog.md` (RT-8).
- [x] ZIP64 test on the FALLBACK path passes (skips in `-short`), proving >4 GiB round-trip + verify accounting
      (`zip64_test.go`); native-path ZIP64 coverage gap noted (RT-9).
- [x] Fallback EXTRACTION stays in system temp (no relocation, RT-S); lightweight free-space floor errors clearly on
      insufficient space; extractor ENOSPC remains the hard bound.
- [x] `go test ./...`, `go vet`, `gofmt` clean; no regression. Cross-builds (windows/amd64, linux/amd64) green.

## Risk Assessment

- **Changing `--jobs` default alters observed behavior** (interleaved output, CPU use). Mitigation: document
  in CHANGELOG; keep deterministic per-archive output ordering; it's a perf default, not a contract.
- **ZIP64 test cost/time** (large data). Mitigation: sparse/zero source + `-short` skip + CI time budget.
- **Buffer size memory** at high `--jobs`: 512 KB × jobs. Mitigation: pool + cap; negligible at sane jobs.

## Security Considerations

- Free-space pre-check also blunts the fallback half of the decompression-bomb vector (Phase 1 covers native).

## Open Questions

1. `--jobs` default cap: 4, or `NumCPU()` uncapped? (I/O-bound work argues for a modest cap.)
2. ZIP64 test via the fallback path acceptable, or invest in a `writeZip` iterator-interface seam for native
   coverage? (RT-9 — fallback path is the cheaper option.)
3. Add `--temp-dir` now (lets users place extraction on a large volume), or just keep system temp? (RT-S)
