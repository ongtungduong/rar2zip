---
phase: 2
title: "Real-World Robustness"
status: completed
priority: P2
effort: "1d"
dependencies: [1]
---

# Phase 2: Real-World Robustness

> **Red-team rescope (RT-1).** `rardecode/v2 FileHeader` exposes only a pre-decoded `Name string` — raw
> name bytes are destroyed in-library (Latin-1 coercion + `\`→`/` rewrite). **Byte-accurate native-path
> transcoding is INFEASIBLE.** Encoding is therefore demoted from a P1 feature to a best-effort,
> fallback-only capability gated behind a go/no-go spike. **`--list` is now the primary, guaranteed
> deliverable**; the phase priority drops to P2 accordingly.

## Overview

Deliver `--list` (preview an archive without converting) and, only if the spike clears it, a best-effort
encoding override for legacy-charset filenames via the external `unrar -cp<codepage>` fallback path.

## Requirements

**Functional**
- `--list` prints the archive's entries (name, size, mtime, dir flag) WITHOUT converting; honors `--json`.
  Bounded by the Phase-1 entry-count cap (placed in the iteration core, RT-11).
- **(Conditional on spike)** `--encoding <codepage>` improves legacy-charset entry names via the fallback
  `unrar -cp<n>` / `7z` codepage option — documented as best-effort and NOT a security boundary. If the spike
  shows no viable path, this requirement is CUT and the CJK limitation is documented instead.

**Non-functional**
- `--list` and encoding are opt-in: behavior unchanged when neither flag is given.
- No native-path name re-decoding of already-decoded strings (unsound — see RT-1).

## Architecture

- **Step 1 is a HARD go/no-go gate (RT-1), executed before any encoding code.** Confirm directly against
  `rardecode/v2 v2.2.3` that no raw-name-bytes / codepage accessor exists (v1 had none; verify v2). Expected
  outcome: native transcoding infeasible. Decision: route encoding through the FALLBACK only (tell external
  `unrar`/`7z` the source codepage via `-cp<n>`), or CUT encoding and document the limitation. Do NOT decode
  the already-decoded `hdr.Name` bytes — that is lossy and a traversal risk (RT-1 / sanitize ordering below).
- **--list (primary)**: a read-only archive walk over the `rr.Next()` iteration core (shared with `Convert`,
  and already carrying the Phase-1 entry-count cap). Emits `[]EntryInfo` (name, size, modtime, isDir); never
  allocates unboundedly. `main.go` prints a human table; `--json` emits structured output.

## Related Code Files

- Create: `internal/convert/list.go` (read-only entry listing; reuses the iteration core)
- Modify: `main.go`, `cli_args.go` (`--list`, and `--encoding` only if spike clears it)
- Modify: `json_output.go` (listing JSON shape) or new `list_output.go`
- Modify: `internal/convert/fallback.go` (codepage flag on extractArgs) — ONLY if encoding kept via fallback
- Modify: `README.md` (`--list` docs; encoding section or CJK-limitation note per spike outcome)
- Add tests: list output (human + JSON), `--list` entry-cap bound; fallback codepage (if kept)
- NOT created: `internal/convert/encoding.go` / native transcode helper (infeasible per RT-1)

## Implementation Steps

1. **--list FIRST (primary deliverable, TDD):** `internal/convert/list.go` walks entries read-only over the
   shared iteration core (with the Phase-1 entry-count cap), returns `[]EntryInfo`. `main.go` prints a human
   table; `--json` emits structured list. Test human + JSON; test that the entry-count cap bounds `--list`.
2. **Encoding go/no-go spike (RT-1):** verify `rardecode/v2 v2.2.3` exposes no raw-name/codepage accessor.
   Record the finding. If infeasible (expected), proceed to step 3a; else reconsider.
3a. **(If spike clears a fallback path)** add `--encoding <codepage>` that passes `-cp<n>` (unrar) / codepage
    (7z) on the FALLBACK path only; document as best-effort, lossy, NOT a security boundary. Test against a
    real legacy-charset archive if one can be sourced.
3b. **(If no viable path)** CUT `--encoding`; document the CJK limitation in README + TROUBLESHOOTING.
4. Update README: `--list` examples; encoding section per 3a/3b outcome.
5. Full suite + vet + fmt green.

## Success Criteria

- [x] `--list archive.rar` previews contents without writing a ZIP; `--list --json` emits structured output;
      `--list` is bounded by the Phase-1 entry-count cap (test-proven — `TestListEntries_EntryCap`, no OOM).
- [x] Encoding spike outcome documented in this phase (native transcoding infeasibility **confirmed**).
- [x] `--encoding` is cut and the CJK limitation is documented (README "Filename encoding") — no dangling criterion.
- [x] `go test ./...`, `go vet`, `gofmt` clean; no regression.

## Risk Assessment

- **Encoding may be wholly infeasible (RT-1).** Mitigation: spike gates it; `--list` carries the phase regardless.
- **Native re-decode of `hdr.Name` is unsound + a traversal risk.** Mitigation: explicitly forbidden — encoding
  only via the external tool's own codepage flag, where bytes are decoded once by the tool.
- **`--list` unbounded allocation on a crafted archive (RT-11).** Mitigation: entry-count cap lives in the
  iteration core (Phase 1), so the lister inherits it.

## Security Considerations

- `--list` reads untrusted archives; it must NOT bypass the Phase-1 entry-count cap (RT-11) — the cap belongs
  in the shared `rr.Next()` iteration core, not the write emitter.
- No native-path name re-decoding: re-interpreting an already-decoded `hdr.Name` through a charset decoder can
  resurface a `/` or `..` AFTER `sanitize` ran (sanitize's backslash-normalize happens once, `sanitize.go:18`).
  Encoding is delegated to the external tool's `-cp` so decode happens before rar2zip ever sanitizes.

## Spike Outcome & Decisions (2026-06-15)

**Step 2 — encoding go/no-go (RT-1): CONFIRMED INFEASIBLE → `--encoding` CUT (plan 3b).**
- `rardecode/v2 v2.2.3 FileHeader` (`reader.go:41-56`) exposes only a pre-decoded `Name string`
  and no raw-name-bytes / codepage / charset accessor. Verified directly against the module cache.
  Native byte-accurate transcoding is impossible — the raw bytes are gone before rar2zip sees them.
- Fallback `-cp<n>` path is **unverifiable in this environment**: no `unrar`/`7z` installed
  (`command -v` all miss) and no legacy-charset (Shift-JIS/GBK) `.rar` fixture sourceable (Open Q2).
  Shipping untested codepage code would violate the "real, verified code — no simulate/mock" rule.
- **Decision (user-confirmed):** cut `--encoding`; document the CJK/legacy-charset limitation in
  README ("Filename encoding" subsection). `internal/convert/encoding.go` not created (per plan).

**Step 1 — `--list` format (Open Q3): human table + `--json`** (the plan's recommended option).
Hardening added during review: the human table replaces control runes (C0/C1/DEL) in entry names
with `?` so a hostile archive cannot smuggle ANSI escapes / CR / NUL into the operator's terminal
(`list_output.go: displaySafe`; `--json` is already safe via `encoding/json`). Printable UTF-8
(incl. CJK) is preserved — covered by `TestPrintList_StripsControlChars`.

**Known CI gap (RT-2):** the end-to-end no-ZIP-written guarantee (`TestRun_ListFixture`) and the
conversion fidelity tests are fixture-gated and currently SKIP — no committed `.rar` exists and none
can be generated here (proprietary format; no encoder; no `unrar`/`7z`/`rar`/libarchive-write). Unit
seams cover the cap, field mapping, error, and formatter logic. Committing a small real `.rar` fixture
(sourced out-of-band) would activate the skipped integration tests for both `--list` and conversion.

**Fixture constraint (RT-2):** RAR cannot be created programmatically (proprietary format; rardecode
is decode-only; no Go encoder; libarchive/bsdtar cannot write RAR either). The read-only iteration
core is therefore tested through a `headerReader` interface seam driven by a fake (`list_test.go`),
not a committed `.rar`. The end-to-end `--list` path (`main_test.go: TestRun_ListFixture`) skips when
`testdata/*.rar` is absent, matching the existing fidelity test's design.

## Resolved Questions

1. Does `rardecode/v2 v2.2.3` expose ANY raw-name/codepage accessor? **No** — confirmed (RT-1). Encoding cut.
2. Can a real legacy-charset `.rar` be sourced to test the fallback? **No** in this environment. Encoding cut.
3. `--list` default format? **Human table + `--json`.** Implemented.
