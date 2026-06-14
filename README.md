# rar2zip

[![CI](https://github.com/ongtungduong/rar2zip/actions/workflows/ci.yml/badge.svg)](https://github.com/ongtungduong/rar2zip/actions/workflows/ci.yml)

A small CLI for **macOS** and **Linux** that converts a `.rar` archive into a `.zip`.

Built in Go with a **pure-Go RAR decoder** ([`nwaples/rardecode`](https://github.com/nwaples/rardecode))
and the standard library's `archive/zip`. The result is a single self-contained binary —
no `unrar` / `7z` required on the user's machine.

## Install

### Homebrew (macOS / Linux)

```sh
brew install ongtungduong/tap/rar2zip
```

### One-line install script

```sh
curl -fsSL https://raw.githubusercontent.com/ongtungduong/rar2zip/main/scripts/install.sh | sh
```

Installs to `~/.local/bin` (if on `$PATH`) or `/usr/local/bin`. Override with `INSTALL_DIR`:

```sh
INSTALL_DIR=/usr/local/bin curl -fsSL https://raw.githubusercontent.com/ongtungduong/rar2zip/main/scripts/install.sh | sh
```

### From source

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
rar2zip --store archive.rar              # no compression (fastest, largest)
rar2zip --level 9 archive.rar            # max Deflate compression
rar2zip --verify archive.rar             # re-open output and validate it
rar2zip --json *.rar                     # machine-readable summary on stdout
rar2zip --allow-fallback exotic.rar      # use system unrar/7z if pure-Go fails
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
| `--store` | Store entries without compression. Mutually exclusive with `--level`. |
| `--level <1..9>` | Deflate compression level (1 = fastest, 9 = best). Default: stdlib default. For no compression use `--store`. |
| `--verify` | Reopen each output ZIP after writing and confirm its entry count and sizes match the source. A ZIP that fails its own check is removed. |
| `--json` | Emit a machine-readable JSON summary on stdout (suppresses the human progress output). |
| `--allow-fallback` | When the pure-Go decoder cannot read an archive, fall back to a system `unrar`/`7z` if installed. Waives the no-dependency guarantee. |
| `--version` | Print version and exit. |
| `-h`, `--help` | Print usage and exit. |

Per-entry **modification time** and **Unix permissions** are preserved, and entries
are Deflate-compressed by default. Output is written atomically (temp file + rename), so a failed
conversion never clobbers an existing output. Progress is written to stderr so stdout
stays clean for piping.

Exit codes: `0` success · `1` runtime error · `2` usage error.

### Security

Untrusted-archive hardening is in place: entry names are sanitized against
**path traversal (Zip Slip)** and absolute paths, and symlink/device entries are
neutralized (stored as plain content) so they cannot escape the extraction root.
The same neutralization is applied on the `--allow-fallback` path.

> **Note:** `--allow-fallback --password <pw>` passes the password as a command-line
> argument to `unrar`/`7z`, which may be visible to other local users via the process
> list (`ps`). The default pure-Go path never shells out and is unaffected.

## Scope & limitations

Converts one or more archives, preserving the file/directory tree, with multi-volume
and password support; supports compression control (`--store`/`--level`), self-verify
(`--verify`), JSON output (`--json`), and an optional system `unrar`/`7z` fallback
(`--allow-fallback`). Not yet supported (see `plans/` for the roadmap):

- stdin/stdout streaming, large-file/ZIP64 throughput tuning.

RAR is a proprietary *creation* format; this tool only reads RAR and writes ZIP.

## Development

```sh
make test    # go test ./...
make vet     # go vet ./...
make fmt     # gofmt -w .
```

Tests include a fixture-agnostic fidelity check: any `.rar` placed in `testdata/`
is converted and verified entry-by-entry (names + content hashes) against the source.
