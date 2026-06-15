# Contributing to rar2zip

Thanks for your interest in improving rar2zip. It's a small, pure-Go CLI that
converts RAR archives to ZIP with no `unrar` dependency on the default path.

## Development

Requires the Go version in [`go.mod`](go.mod).

```sh
make build   # build ./bin/rar2zip
make test    # go test ./...
make vet     # go vet ./...
make fmt     # gofmt -w .
```

Before opening a PR, make sure all three pass:

```sh
go test ./...
go vet ./...
test -z "$(gofmt -l .)"
```

Some tests are **fixture-gated** and skip when no `testdata/*.rar` is present —
RAR is a proprietary creation format and cannot be generated programmatically,
so binary fixtures are not committed. The core logic is still covered through
interface seams and synthetic inputs. The ZIP64 (>4 GiB) test is heavy and is
skipped under `go test -short`.

## Project layout

- `main.go`, `cli_args.go`, `json_output.go`, `list_output.go` — the CLI (package `main`).
- `internal/convert/` — the conversion engine: native decode, shared emitter
  (bomb caps, dedup), atomic write, `--verify`, `--list`, and the `unrar`/`7z`
  fallback. Keep files focused and under ~200 lines.

## Conventions

- **Commits:** Conventional Commits (`feat:`, `fix:`, `perf:`, `docs:`, `build:`,
  `test:`, `chore:`, `refactor:`). Keep each commit focused.
- **Comments** explain the *why* (invariants, security trade-offs), not the
  origin (no plan/issue codes in code or test names).
- **TDD** for behavior changes: add a failing test first, then the fix.
- **No new runtime dependencies** on the native path — its value is being
  `unrar`-free. The `--allow-fallback` path may shell out to system tools.

## Security

This tool reads untrusted archives. Preserve the hardening already in place:
Zip-Slip / absolute-path sanitization, symlink and non-regular neutralization,
decompression-bomb caps (`--max-size` / `--max-entries`), atomic output, and the
post-sanitize name-collision guard. If you touch the emitter or sanitizer, add a
test proving the invariant still holds. Report sensitive issues privately rather
than in a public issue.
