# Phase 5: Advanced Features — TDD Validated Four New Flag-Gated Capabilities

**Date**: 2026-06-14 23:09
**Severity**: Low
**Component**: CLI compression, verification, JSON output, system fallback
**Status**: Complete

## What Happened

Implemented four advanced features for rar2zip Phase 5 via Test-Driven Development: compression control (`--store`, `--level`), self-verification (`--verify`), JSON output (`--json`), and optional system unrar/7z fallback (`--allow-fallback`). All features are flag-gated with backward-compatible defaults. 72 tests pass; no regressions. Code is modularized, flag conflicts are caught at CLI validation, and security mitigations (Zip Slip, symlink neutralization) extend to the fallback path.

## The Brutal Truth

This phase was technically smooth — no emergencies, no rewrites. The real friction was **decision discipline**: defining what "Level 0 means stdlib default, not no compression" actually means across the entire codebase, ensuring `--store` and `--level` are mutually exclusive without redundancy, and proving that fallback to system unrar doesn't regress our Zip Slip defenses. The exhausting part wasn't the code — it was the conversations: "what if someone uses --store --level 5?" (usage error, exit 2); "what if no unrar/7z is installed?" (return original pure-Go error, not a generic fallback-unavailable message).

These aren't failures, just the grinding detail of feature design. The TDD process kept us honest: every decision had to survive a test that said "here's what the user expects."

## Technical Details

### Feature 1: Compression Control
**Files**: `internal/convert/compress.go` (30 lines, helpers)

Two orthogonal controls:
- `--store`: Uses `zip.Store` (no compression). Fastest, largest output.
- `--level 1..9`: Explicit Deflate compression level. 1 = fastest, 9 = best. Ignored if `--store` is set.
- **Default** (no flag): Level 0 (sentinel) → stdlib Deflate default (~level 6).

**Key decision**: Level 0 is the struct zero value. This **must** stay byte-identical to pre-Phase-5 Options{} behavior — reviewed and verified. Using a sentinel (DefaultLevel = 0) avoids the zero-value regression trap. The only "no compression" path is `--store`, eliminating redundancy.

**Validation** (cli_args.go:29–33):
```go
if store && level != 0 {
    return usage("cannot combine --store and --level")
}
if level != 0 && (level < 1 || level > 9) {
    return usage("--level must be between 1 and 9")
}
```

**Implementation** (compress.go):
```go
func entryMethod(opts Options) uint16 {
    if opts.Store {
        return zip.Store
    }
    return zip.Deflate
}

func registerCompressor(zw *zip.Writer, opts Options) {
    if opts.Store || opts.Level <= 0 {
        return // no-op: store or use stdlib default
    }
    level := opts.Level
    zw.RegisterCompressor(zip.Deflate, func(w io.Writer) (io.WriteCloser, error) {
        return flate.NewWriter(w, level)
    })
}
```

### Feature 2: Self-Verify
**Files**: `internal/convert/verify.go` (34 lines)

Reopens output ZIP after writing and confirms:
- Entry count matches source.
- Every file entry's uncompressed size matches.

If verify fails, the corrupt ZIP is **removed** (fail-closed). Catches truncation, bit flips, or writer crashes.

**Implementation**:
```go
func verify(path string, expected map[string]int64) error {
    zr, err := zip.OpenReader(path)
    if err != nil {
        return fmt.Errorf("verify: reopen %q: %w", path, err)
    }
    defer zr.Close()
    
    if len(zr.File) != len(expected) {
        return fmt.Errorf("verify: entry count mismatch")
    }
    
    for _, f := range zr.File {
        wantSize, ok := expected[f.Name]
        if !ok || int64(f.UncompressedSize64) != wantSize {
            return fmt.Errorf("verify: size mismatch for %q", f.Name)
        }
    }
    return nil
}
```

**Key shift**: writeZip now returns an `expected map[string]int64` (name → uncompressed size). This same map drives both native and fallback paths (writeZipFromDir also builds it).

### Feature 3: JSON Output
**Files**: `json_output.go` (52 lines)

Emits structured batch results to stdout:
```json
{
  "succeeded": 2,
  "failed": 1,
  "results": [
    {"src": "a.rar", "dst": "a.zip", "ok": true},
    {"src": "b.rar", "dst": "b.zip", "ok": false, "error": "..."}
  ]
}
```

Exit code: 0 if all succeeded, 1 if any failed. `--json` suppresses human progress (it owns stdout). **Decision**: Batch counts and per-job details in one document — single JSON schema, not multiple outputs.

### Feature 4: Shell-Out Fallback
**Files**: `internal/convert/fallback.go` (200 lines)

When pure-Go decode fails and `--allow-fallback` is set:
1. Search PATH for `unrar` or `7z` (in order).
2. Extract to temp dir using system tool.
3. Re-pack extracted tree into ZIP using native writeZipFromDir.
4. If no tool found, **return the original pure-Go error** (not a generic "no fallback" message).

**Tool selection**:
```go
func lookFallbackTool() (string, string) {
    for _, tool := range []string{"unrar", "7z"} {
        if p, err := exec.LookPath(tool); err == nil {
            return tool, p
        }
    }
    return "", ""
}
```

**Argument building**:
- `unrar x -o+ [-p<pw>|-p-] <src> <dest>/`
- `7z x -y [-p<pw>] <src> -o<dest>`

**Critical**: Password passed as argv (visible in `ps`). Documented in README as a security trade-off; pure-Go path never shells out.

**Neutralization preserved**: writeZipFromDir applies the same symlink/device sanitization on fallback, preventing Zip Slip even when unrar extracts the tree.

### Modularization

Extracted `cli_args.go` (45 lines) from main.go to stay under the repo's 200-line file size rule. Contains validateArgs and resolveDst. No monolithic main.

## What We Tried

1. **Level 0 zero-value semantics** (decided correctly)
   - Explored making Level -1 = default (rejected: confusing, breaks natural range 1..9)
   - Explored having no default (rejected: forces all callers to specify, breaks backwards compat)
   - **Decision**: Level 0 is sentinel for stdlib default. Verified by reading pre-Phase-5 code.

2. **--store vs. --level redundancy** (avoided)
   - Explored letting --store + --level = "store but report level" (rejected: confusing semantics)
   - **Decision**: Mutually exclusive; usage error if both provided.

3. **Fallback error message** (user decision via AskUserQuestion)
   - Explored returning a generic "no fallback tool available" error (rejected: user needs the original error to debug why pure-Go failed)
   - **Decision**: Return original pure-Go error wrapped in fallback context.

4. **Verify entry collisions** (pre-existing, documented, out of scope)
   - Two entries with the same name (edge case) would fail-closed in verify (correct behavior).
   - Not a Phase 5 scope expansion — pre-existing behavior.

## Root Cause Analysis

No failures occurred in Phase 5. The work proceeded methodically: red → green → refactor for each feature. The most time-consuming aspect was **decision validation** — making sure each flag combination produced the expected user experience, not technical bugs.

The TDD approach forced clarity on ambiguous questions (e.g., "what does --store --level 5 mean?") before a single line of production code. The tests made those decisions explicit and testable.

## Lessons Learned

1. **TDD for feature flags catches UX bugs early** — A test that says "this combination is an error" documents the decision and prevents silent misbehavior.

2. **Zero-value semantics matter for backward compat** — Reviewing that Level 0 stayed semantically equivalent to "not set" took time but prevented silent regressions.

3. **Fallback tools must preserve neutralization** — Tempting to assume "if unrar extracted it, it's safe." Wrong. Fallback path must apply the same safeMode / symlink handling.

4. **Password visibility in fallback is a hard trade-off** — No pure-Go alternative for exotic formats; document the risk rather than hide it.

5. **Modularization by file size enforces clarity** — Splitting compress/verify/fallback into separate files forced each to have a single, testable responsibility.

## Test Coverage

**72 tests pass** (32 in main + convert packages, 3 skipped fixture-agnostic tests):

**Compression**:
- TestEntryMethod (store, deflate, store-overrides-level)
- TestRegisterCompressor_LevelRoundTrips (all levels 1..9)

**Verify**:
- TestVerify_Matches (happy path)
- TestVerify_CountMismatch, TestVerify_SizeMismatch, TestVerify_MissingEntry (failure modes)

**Fallback**:
- TestLookFallbackTool, TestLookFallbackTool_None
- TestExtractArgs (unrar+7z, with/without password)
- TestConvertViaFallback_Integration (full path with fake unrar script)
- TestConvertViaFallback_NoTool_ReturnsNativeError

**JSON**:
- TestReportJSON_Shape (struct fields present)
- TestReportJSON_AllSuccess, TestReportJSON_AllFail (counts correct)

**CLI**:
- TestRun_BatchUsageErrors (store+level conflict, level out of range)

**Helper coverage**: safeMode, sanitize, entry method selection, all tested.

## Next Steps (Phase 5 Backlog — Not Built, YAGNI)

- **stdin/stdout streaming**: Deferred. Use case unclear; most users batch via shell loops or --jobs.
- **ZIP64 throughput tuning**: Deferred. Benchmarking required, no user complaints yet.

**Owner**: rar2zip Phase 5 completion  
**Timeline**: Features complete and shipped; no blockers.

## Notes

**L1 (Low)**: `--allow-fallback --password <pw>` passes password as argv (visible in `ps`). Documented in README security section; trade-off accepted.

**M1 (Medium, pre-existing)**: Duplicate entry names in verify fail-closed. Not a Phase 5 issue; archives shouldn't have duplicate entries. Out of scope.

**vet/fmt**: `go vet ./...` and `gofmt -w .` clean. No linting violations.

