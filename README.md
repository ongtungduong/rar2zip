# rar2zip

A small CLI for **macOS** and **Linux** that converts a `.rar` archive into a `.zip`.

Built in Go with a **pure-Go RAR decoder** ([`nwaples/rardecode`](https://github.com/nwaples/rardecode))
and the standard library's `archive/zip`. The result is a single self-contained binary —
no `unrar` / `7z` required on the user's machine.

## Install (from source)

```sh
go build -o bin/rar2zip .
# or
make build
```

## Usage

```sh
rar2zip [flags] <input.rar> [more.rar ...]
```

By default, writes `<input>.zip` next to each input file. Examples:

```sh
rar2zip archive.rar                      # -> archive.zip
rar2zip -o out.zip archive.rar           # custom output file (single input)
rar2zip -o /tmp/ archive.rar             # into a directory -> /tmp/archive.zip
rar2zip -f archive.rar                   # overwrite existing output
rar2zip -q archive.rar                   # no progress output
rar2zip *.rar                            # batch: convert every match
rar2zip --out-dir zips/ a.rar b.rar      # batch outputs into zips/
rar2zip --jobs 4 --out-dir zips/ *.rar   # convert 4 archives concurrently
rar2zip --password secret locked.rar     # password-protected archive
unzip -l archive.zip                      # inspect result
```

Batch runs are **continue-on-error**: a failed archive is reported but doesn't abort
the rest; the process exits non-zero if any conversion failed. Multi-volume sets
(`.part1.rar` / `.r00`) are followed automatically.

### Flags

| Flag | Description |
|------|-------------|
| `-o`, `--output <path>` | Output file, or directory to place `<input>.zip` into. Single input only. Default: sibling `<input>.zip`. |
| `--out-dir <dir>` | Write all outputs into this directory (created if needed). Use for multiple inputs. |
| `-f`, `--force` | Overwrite outputs that already exist (otherwise rar2zip refuses). |
| `-q`, `--quiet` | Suppress progress output (printed to stderr). |
| `--password <pw>` | Password for encrypted archives. |
| `--jobs <n>` | Convert up to `n` archives concurrently (default 1). |
| `--version` | Print version and exit. |
| `-h`, `--help` | Print usage and exit. |

Per-entry **modification time** and **Unix permissions** are preserved, and entries
are Deflate-compressed. Output is written atomically (temp file + rename), so a failed
conversion never clobbers an existing output. Progress is written to stderr so stdout
stays clean for piping.

Exit codes: `0` success · `1` runtime error · `2` usage error.

### Security

Untrusted-archive hardening is in place: entry names are sanitized against
**path traversal (Zip Slip)** and absolute paths, and symlink/device entries are
neutralized (stored as plain content) so they cannot escape the extraction root.

## Scope & limitations

Converts one or more archives, preserving the file/directory tree, with multi-volume
and password support. Not yet supported (see `plans/` for the roadmap):

- Prebuilt binaries / Homebrew / install script (Phase 4 — distribution & CI).
- Compression-level control, stdin/stdout streaming, `unrar` fallback (Phase 5).

RAR is a proprietary *creation* format; this tool only reads RAR and writes ZIP.

## Development

```sh
make test    # go test ./...
make vet     # go vet ./...
make fmt     # gofmt -w .
```

Tests include a fixture-agnostic fidelity check: any `.rar` placed in `testdata/`
is converted and verified entry-by-entry (names + content hashes) against the source.
