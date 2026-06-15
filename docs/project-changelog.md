# Changelog

Notable changes to rar2zip. Behavior changes that affect observable output or
defaults are called out explicitly.

## Unreleased

### Added
- `--skip-existing` — in a batch, skip inputs whose output already exists
  (reported as skipped, not failed) instead of erroring. The `--json` summary
  gains a `skipped` count. Advisory pre-check, not a collision-safety mechanism.
- `--verbose` — print extra diagnostics to stderr (which decode path ran:
  native vs fallback, and per-archive timing). Suppressed under `--json`/`--quiet`.
- Shell completion scripts for bash, zsh, and fish under `completions/` (also
  bundled in each release archive).
- Contributor docs: `CONTRIBUTING.md` and `docs/TROUBLESHOOTING.md`.
- **Windows support (experimental).** Releases now publish Windows `amd64`/`arm64`
  binaries as `.zip` assets, with a Scoop bucket manifest. A Windows CI row builds
  and vets the code (the test suite is Unix-only — the `--allow-fallback` path
  shells out to `unrar`/`7z` and creates symlinks). See README "Windows (Scoop)".
- **Release signing.** `checksums.txt` is signed with keyless `cosign` (Sigstore,
  via Actions OIDC — no stored keys). `install.sh` verifies the signature when
  `cosign` is installed (authenticity), in addition to the mandatory checksum.
- `--list` — preview an archive's entries (size, modification time, name) without
  converting (read-only; writes no ZIP). Honors `--json`, `--password`, and the
  `--max-entries` cap; rejects `-o`/`--out-dir`. Human output strips control
  characters from entry names to prevent terminal-escape injection from hostile
  archives (`--json` output is already escaped by the encoder).
- Benchmarks for the hot paths (`go test -bench .` in `internal/convert`):
  large-entry streaming write, `--verify` over many entries, and a fixture-gated
  native convert.
- ZIP64 round-trip coverage: entries past the 4 GiB boundary are verified to
  re-pack and `--verify` with correct 64-bit size accounting (exercised on the
  fallback path; gated behind `-short`).

### Changed
- **`--verify` now validates content CRC, not just structure.** It reads every
  entry to force the ZIP reader's CRC32 check, so a same-size-but-corrupted entry
  is caught — previously `--verify` checked only entry count and declared sizes.
- **`--jobs` now defaults to `min(NumCPU, 4)` instead of `1`.** This is a
  behavior change for any invocation with two or more inputs (e.g. a `*.rar`
  glob): the batch now runs **concurrently by default**. Per-archive result
  lines are still emitted in input order, so stdout/stderr stay deterministic;
  the previous live per-job "converting X" start line (which would interleave
  nondeterministically across workers) was removed in favor of the ordered final
  summary. Pass `--jobs 1` to restore fully serial execution.
- Large-entry streaming now uses a pooled 512 KB copy buffer (`io.CopyBuffer`)
  instead of `io.Copy`'s default 32 KB, reducing syscall frequency and
  per-entry allocations on big files (measured ~7% throughput gain and ~60–85%
  fewer bytes allocated on the large-entry write path). Memory stays bounded —
  buffers are pooled and shared across concurrent jobs.

### Hardened
- The `--allow-fallback` path now performs a lightweight free-space pre-check
  before extracting: if the temp filesystem has less free space than the archive
  itself, it fails early with a clear message (pointing at `TMPDIR`) rather than
  extracting partway and hitting an opaque out-of-space error. Extraction stays
  in the system temp dir (it is not relocated onto the output volume). The check
  is a conservative floor, not a full reservation — the external extractor's own
  out-of-space error remains the hard bound. Skipped on platforms without
  `statfs`.

### Cut
- `--encoding` (filename charset override) was evaluated and cut: the pure-Go
  decoder exposes only already-decoded entry names and discards the raw bytes /
  stored codepage, so byte-accurate transcoding is infeasible, and re-decoding
  the decoded string would be both lossy and a path-traversal risk. See the
  "Filename encoding" section in `README.md`.
