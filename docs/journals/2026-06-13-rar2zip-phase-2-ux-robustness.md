# rar2zip Phase 2 UX & Robustness Shipped

**Date**: 2026-06-13 23:30
**Severity**: Medium
**Component**: CLI flags, metadata preservation, archive security (Zip Slip)
**Status**: Resolved

## What Happened

Turned the MVP into a safe, polished tool. Added `-o/--output`, `-f/--force`, `-q/--quiet`,
`--version`, `--help`; preserved per-entry mtime + Unix mode with Deflate; added per-entry
progress on stderr; and closed the Phase-1 security gap with a Zip-Slip / absolute-path
sanitizer. Built test-first (TDD), every slice red→green. `go test ./...` green, `go vet` clean,
binary smoke-tested against the real fixture (`zipinfo` confirmed mode + mtime + Deflate).

## The Brutal Truth

The path-name sanitizer was solid first pass — survived 23 adversarial inputs in review.
But code review caught a real gap my own verification missed: **symlink escape**. Sanitizing
the entry *name* isn't enough — preserving `fs.ModeSymlink` makes a compliant extractor
recreate a link whose *body* (the unsanitized target) can point at `/etc/passwd`. The name
check passes, the link still escapes. Classic two-stage bypass.

Fix matched the plan's own wording ("treat symlinks as regular entries"): `safeMode` strips
all non-permission type bits (symlink, device, pipe, socket, setuid), so such entries are
stored as inert file content. Lesson: a name-based traversal defense is incomplete if the
format can carry indirection (symlinks) — sanitize the *effect*, not just the *label*.

## Technical Details

- **CLI parsing**: stayed on stdlib `flag` (KISS) — single command, no subcommand earns cobra.
  Registered short+long aliases by binding both names to the same var.
- **Header preservation**: `zip.CreateHeader` with `Method: zip.Deflate`, `SetMode(safeMode(...))`,
  `Modified` set only when the RAR mtime is non-zero (avoid clobbering with the 1979 default).
- **Security seam**: `sanitize(name)` (path traversal/absolute) + `safeMode(mode, isDir)` (type-bit
  strip) are pure, unit-tested functions — testable without a malicious RAR fixture (RAR creation
  is proprietary, so we can't synthesize one; the security logic lives in unit-testable seams).
- **Progress**: `\r\033[K`-updated line on stderr, keeping stdout pipe-clean.
- **Deferred (YAGNI)**: `--max-size` zip-bomb guard — revisit on real need.

## Files

- `main.go` — flag set, overwrite guard, output resolution, progress wiring.
- `internal/convert/convert.go` — `Convert(src, dst, Options)`, header preservation, sanitize wiring.
- `internal/convert/sanitize.go` — `sanitize` + `safeMode`.
- Tests: `sanitize_test.go`, `safe_mode_test.go`, `convert_test.go` (metadata/progress), `main_test.go`.
- `README.md` — flags, exit codes, security note.

## Unresolved

- No symlink/device-mode RAR fixture exists to integration-test neutralization end-to-end; covered
  by `safeMode` unit tests instead. If a fixture appears, add a conversion-level negative test.
