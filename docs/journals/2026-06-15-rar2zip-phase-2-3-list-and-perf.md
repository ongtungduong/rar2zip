# Phase 2 & 3: List Preview & Performance Hardening

**Date**: 2026-06-15 16:45
**Severity**: Medium
**Component**: Archive iteration, copy performance, temp-dir safety
**Status**: Completed

## What Happened

Two rounds of hardening shipped back-to-back: Phase 2 delivered `--list` (read-only archive preview) with a critical encoding trade-off resolved via spike; Phase 3 locked performance with benchmarks, pooled buffers, and space safety checks.

## The Brutal Truth

**Phase 2 was a go/no-go gate that almost failed completely.** The plan initially promised `--encoding` support for legacy-charset filenames (Shift-JIS, GBK, etc.), but a 20-minute spike on `rardecode/v2 v2.2.3` confirmed the library destroys the raw name bytes in-library and exposes only a pre-decoded string. Native transcoding is impossible. This forced a hard cut: encoding was dropped, `--list` became the primary deliverable instead, and we documented the CJK limitation in the README. It stung because we'd already sketched the feature in the plan — but the spike saved us from shipping untested, unverifiable codepage code through the fallback path.

**Phase 3 felt cleaner but had a subtle correctness bet.** Swapping `io.Copy` for `io.CopyBuffer` is a standard perf trick, but the 512 KB pooled buffer had to flow through the `cappedWriter` (which wraps the ZIP writer and enforces an entry-size cap). The cap doesn't implement `ReaderFrom`, so `io.CopyBuffer` still hits the slow path — but that's fine; the pool still saves syscalls and allocs on large entries (7% throughput gain, 60–85% fewer allocations). The test suite proved it doesn't break the cap enforcement.

Defaulting `--jobs` to `min(NumCPU, 4)` is a behavioral change: a glob like `*.rar` now runs concurrently instead of serial, which interleaves stderr output nondeterministically. We removed the live concurrent "converting X" line and kept only the per-job ordered final summary — deterministic output for the common case.

## Technical Details

**Phase 2 — `--list` hardening:**
- `internal/convert/list.go`: read-only iteration over shared `rr.Next()` core (inherits Phase 1 entry-count cap at RT-11).
- `list_output.go`: human table + JSON output; human path sanitizes control runes (C0/C1/DEL) to `?` to prevent terminal-escape injection. JSON is safe via `encoding/json`.
- Spike confirmed: `rardecode.FileHeader.Name` is a pre-decoded string, no raw-bytes or codepage accessor in v2.2.3 (`reader.go:41-56`).
- Encoding cut: fallback `-cp<n>` path unverifiable (no unrar/7z, no legacy-charset fixture). Violates "no simulate/mock" rule.
- Known gap (RT-2): end-to-end `--list` and fidelity tests are fixture-gated and skip in CI (no committed `.rar`; RAR is proprietary, not creatable here). Unit seams via `headerReader` interface cover the logic.

**Phase 3 — performance & space safety:**
- `io.CopyBuffer` with `sync.Pool` (512 KB): ~7% throughput gain on large-entry writes, ~60–85% fewer bytes allocated. Benchmarks committed in `convert_bench_test.go`.
- `--jobs` default: changed 1 → `min(NumCPU, 4)`. Multi-input case (e.g., `*.rar` glob) now concurrent by default. Output determinism preserved by buffering per-job results and flushing in order (dropped the live concurrent line).
- ZIP64 (>4 GiB round-trip): test on fallback path only (sparse file) behind `testing.Short()`; native path has no injectable seam (`writeZip` takes concrete `*rardecode.ReadCloser`). Native-path ZIP64 gap noted (RT-9); would require a real >4 GiB RAR or refactoring `writeZip` to accept an interface.
- Fallback free-space: lightweight pre-check (avail < archive size → early clear error); extraction stays in system temp (cross-platform via `//go:build unix` + stub for Windows). Does not relocate the multi-GB extraction tree.

## What We Tried

**Encoding (Phase 2):**
1. Checked if `rardecode/v2` exposed raw name bytes or codepage APIs — it didn't.
2. Considered native re-decoding of the pre-decoded `hdr.Name` — rejected as unsound (would resurface `/` or `..` after sanitization, a traversal risk).
3. Spike on fallback `-cp<n>` path — environment has no unrar/7z and no legacy-charset fixture to verify against. Cut.

**Copy buffer (Phase 3):**
1. Initial `io.Copy` (32 KB default) measured at ~27 ms/op, 2482 MB/s, 38 KB allocated, 27 allocs.
2. Switched to pooled `io.CopyBuffer` (512 KB) — benchmark showed ~25.2 ms/op, 2665 MB/s, ~5–17 KB allocated, 26 allocs. Verified the cap still holds.

## Root Cause Analysis

**Why encoding failed:** `rardecode/v2` is a read-only decoder that performs Latin-1 coercion and path-separator normalization in-library. The library never exposes raw bytes or codepage info, so downstream apps cannot re-transcode. This is a **library design choice, not an oversight**; the bindings are tight for security. We should have checked the library API surface earlier in planning, but the spike was short and saved us from a half-baked fallback.

**Why space pre-check is lightweight:** The fallback path already extracts the entire archive to a temp directory before zipping; the final ZIP is created atomically on the output filesystem. Exact extracted size is unknowable without decompressing, so we err on the conservative side: if avail < archive size, fail early and clearly. The extractor's ENOSPC (out of disk) is the hard bound; our check just avoids the extract-then-fail loop.

## Lessons Learned

1. **Spike gates features.** `--encoding` looked reasonable in the plan until we tested the library API. A 20-minute spike saved days of chasing a dead-end. Invest in go/no-go spikes for cross-library dependencies.

2. **Fixture gaps block integration tests.** RT-2 (inability to create RAR files) is a hard constraint here. We test the iteration core via seams (`headerReader` interface) and live with fixture-gated skips in CI. Document the gap explicitly so future maintainers don't wonder why tests skip.

3. **Behavioral changes need determinism.** Defaulting `--jobs` to multi-core is a perf win but changes observable output (interleaved stderr). We mitigated by dropping the nondeterministic line and relying on the already-deterministic per-job summary. Documented in `project-changelog.md`.

4. **Pool correctness flows through wrappers.** The `cappedWriter` doesn't implement `ReaderFrom`, so `io.CopyBuffer` doesn't get zero-copy on that path — but that's fine; we still save syscalls and allocs. The test suite proves the cap still catches runaway entries.

5. **Lightweight pre-checks can replace defaults.** Rather than add `--temp-dir` (new API, more UX surface), we kept extraction in system temp and added a cheap free-space floor check. Users who need a custom temp dir already use `TMPDIR` env var.

## Next Steps

- **RT-2 (fixture gap):** Acquiring a small committed `.rar` file out-of-band would activate the currently skipped integration tests for `--list` and conversion fidelity. No hard blocker; the unit seams are solid.
- **RT-9 (native ZIP64):** If native-path ZIP64 coverage becomes critical, refactor `writeZip` to accept an `entryIterator` interface instead of a concrete `*rardecode.ReadCloser`. Lower priority; fallback path is covered.
- **Phase 4–5 (deferred):** Supply-chain hardening and UX/docs polish deferred per user choice; pick up in a follow-on session if needed.
- **Benchmark drift:** Run `go test -bench .` regularly to catch regressions in the hot paths (copy, verify, batch output).
