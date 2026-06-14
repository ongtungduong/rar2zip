# rar2zip Phase 3 Batch & Advanced Archives Shipped

**Date**: 2026-06-13 23:55
**Severity**: High
**Component**: Batch driver, concurrency, atomic output, password/multi-volume support
**Status**: Resolved

## What Happened

Added batch conversion (multiple inputs, globs), `--out-dir`, `--password`, and `--jobs`
concurrency, plus multi-volume support (automatic via rardecode). Built test-first (TDD),
slice by slice: `Convert` Force/Password → `RunBatch` → `main` rewrite → atomic-output fix.
`go test -race ./...` green, `go vet` clean, binary smoke-tested across batch, `--out-dir`,
`--jobs 2`, continue-on-error, and usage-error paths.

## The Brutal Truth

The headline lesson is a **critical data-loss bug code review caught that my own verification
missed** — and it's a sharp one. I moved the overwrite guard into `Convert` and kept a
`os.Create(dst)` + "remove partial output on error" pattern. Looked clean. But under `--force`,
`os.Create` **truncates the destination immediately**, then a mid-stream failure (corrupt source,
ENOSPC, bad password) hit the cleanup `os.Remove(dst)` — deleting the user's previously-good
output. The nasty part: it's invisible in the obvious tests, because a *non-existent* or
*invalid-from-byte-0* source makes `OpenReader` fail **before** `os.Create`, so dst is never
touched. The bug only fires when the archive opens cleanly and fails partway — exactly the
real-world "re-run my batch with `-f`, one archive got corrupted" scenario, where you'd silently
lose the good zip you already had.

Fix: write to a temp file in the destination dir, `Rename` into place only after a complete
archive exists. Standard pattern, and it collapsed three review findings at once (data loss,
partial-output window, double-close reasoning). Lesson burned in: **"create then write in place"
is never safe for a destination you might be replacing — stage and rename.** And my test for the
force path was too easy (missing source); the regression test now uses a *truncated* fixture to
hit the open-then-fail window deterministically.

## Technical Details

- **Atomic output** (`convert.go`): `os.CreateTemp(dir, "."+base+".*.tmp")` → `writeZip` → `Chmod
  0644` → `os.Rename`. On failure: close + remove temp; dst untouched. Overwrite guard runs before
  `OpenReader`, so a refusal touches nothing.
- **Batch** (`batch.go`): `RunBatch(jobs, opts, maxParallel, onStart)` — semaphore-bounded worker
  pool, results written to per-index slots (race-free), order preserved, continue-on-error.
- **CLI** (`main.go`): `validateArgs` splits usage errors (exit 2, fail fast: bad ext, dir input,
  `-o`+multi, `-o`+`--out-dir`, `--jobs<1`) from runtime job failures (exit 1, continue + summary).
  Single-input keeps per-entry progress; batch uses per-archive lines to avoid concurrent interleave.
- **Password/multi-volume**: threaded to rardecode; rardecode supplies clear `ErrArchiveEncrypted`/
  `ErrBadPassword`/auto-volume-follow. Both are skip-gated in tests — RAR creation is proprietary,
  so no encrypted/multi-volume fixture can be synthesized.

## Files

- `internal/convert/convert.go` — Options{Force,Password}, atomic temp+rename, `writeZip` extracted.
- `internal/convert/batch.go` — `Job`, `Result`, `RunBatch` (new).
- `main.go` — N inputs, `--out-dir`/`--password`/`--jobs`, `validateArgs`, `resolveDst`, `report`.
- Tests: `batch_test.go`, `overwrite_test.go` (incl. C1 regression), `convert_test.go`, `main_test.go`.
- `README.md` — batch usage, flag table, atomic-output note.

## Unresolved

- No encrypted or multi-volume RAR fixture exists to exercise those paths end-to-end (skip-gated).
  If a fixture is added to `testdata/`, the existing tests will pick it up; consider a dedicated
  wrong-password negative test then.
- Interactive no-echo password prompt deferred (KISS) — revisit only if real demand appears.
