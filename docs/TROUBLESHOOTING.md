# Troubleshooting

## "pure-Go decode failed and no fallback tool (unrar/7z) found"

The bundled pure-Go decoder could not read the archive (older/exotic RAR variant,
or a corrupt file). Install `unrar` or `7z` and retry with `--allow-fallback`:

```sh
rar2zip --allow-fallback archive.rar
```

> **Security:** `--allow-fallback` extracts the *entire* archive to a temp
> directory with the external tool **before** rar2zip's `--max-size` /
> `--max-entries` caps apply — so the fallback path is **not** bomb-bounded. Use
> it only on archives you trust. (It does perform a lightweight free-space
> pre-check and keeps extraction in the system temp dir; redirect with `TMPDIR`.)

## Filenames look garbled (mojibake) — CJK / legacy charsets

There is **no encoding override**. The pure-Go decoder returns entry names
already decoded and discards the raw bytes / stored codepage, so a legacy-charset
name (Shift-JIS, GBK, Big5, …) cannot be re-interpreted after the fact. Names the
archive stored in a non-Unicode codepage may appear garbled. To get correct
names, extract with a tool that takes a source-codepage flag and re-zip:

```sh
unrar x -cp936 archive.rar   # then zip the extracted tree
```

See the "Filename encoding" section in the README for the full rationale.

## It asks for / fails on a password

Encrypted archives need `--password`:

```sh
rar2zip --password 'secret' locked.rar
```

> **Security:** with `--allow-fallback`, the password is passed as a command-line
> argument to `unrar`/`7z` and may be visible to other local users via the
> process list (`ps`). The default pure-Go path never shells out and is unaffected.

## macOS: "cannot be opened because the developer cannot be verified" (Gatekeeper)

A binary downloaded outside Homebrew is quarantined. Remove the quarantine
attribute (after you trust the source — verify the checksum/signature first):

```sh
xattr -d com.apple.quarantine ./rar2zip
```

Installing via Homebrew or the install script avoids this.

## The download won't verify

`install.sh` requires `sha256sum` or `shasum`; a missing hash tool is a hard
error (no silent unverified install). Install one, or — only if you understand
the risk and are not piping from the network — bypass with `SKIP_CHECKSUM=1`.
If `cosign` is installed, the script also verifies the release signature.

## "output exists (use -f to overwrite)"

rar2zip refuses to clobber an existing output. Choose one:

- `--force` — overwrite it.
- `--skip-existing` — skip that input (useful for re-running a batch); the
  skipped count appears in the summary (and in `--json`).

## A conversion is slow / I want to see what it's doing

Use `--verbose` for per-archive timing and which decode path (native vs
fallback) was used, and `--jobs N` to tune batch concurrency (default
`min(NumCPU, 4)`).
