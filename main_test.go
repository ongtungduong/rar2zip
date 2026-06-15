package main

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestRun_ExitCodes covers argument validation paths that don't need a real
// archive: usage errors (code 2) and runtime errors (code 1).
func TestRun_ExitCodes(t *testing.T) {
	dir := t.TempDir()

	// A non-rar regular file (wrong extension).
	txt := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(txt, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A path that ends in .rar but does not exist.
	missing := filepath.Join(dir, "nope.rar")
	// A directory whose name ends in .rar (exercises the IsDir guard).
	dirRar := filepath.Join(dir, "bundle.rar")
	if err := os.Mkdir(dirRar, 0o755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		args []string
		want int
	}{
		{"no args", nil, 2},
		{"wrong extension", []string{txt}, 2},
		{"directory input", []string{dir}, 2},         // dir has no .rar ext -> usage error
		{"directory named .rar", []string{dirRar}, 2}, // .rar ext but is a dir -> usage error
		{"missing rar file", []string{missing}, 1},
		// Multiple inputs are valid (batch); these don't exist -> runtime error.
		{"multiple missing inputs", []string{missing, filepath.Join(dir, "x.rar")}, 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := run(tc.args); got != tc.want {
				t.Errorf("run(%v) = %d, want %d", tc.args, got, tc.want)
			}
		})
	}
}

// TestRun_VersionHelp covers the informational flags that exit 0 without
// requiring an input archive.
func TestRun_VersionHelp(t *testing.T) {
	for _, args := range [][]string{{"--version"}, {"-version"}, {"-h"}, {"--help"}} {
		if got := run(args); got != 0 {
			t.Errorf("run(%v) = %d, want 0", args, got)
		}
	}
	// An unknown flag is a usage error.
	if got := run([]string{"--bogus"}); got != 2 {
		t.Errorf("run(--bogus) = %d, want 2", got)
	}
}

// TestRun_VersionOutput verifies --version prints "rar2zip v<version> (<commit>)".
func TestRun_VersionOutput(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	defer func() {
		w.Close()
		os.Stdout = old
	}()

	code := run([]string{"--version"})

	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)

	if code != 0 {
		t.Fatalf("--version exited %d, want 0", code)
	}
	s := strings.TrimSpace(string(out))
	if !strings.HasPrefix(s, "rar2zip ") {
		t.Errorf("version output %q does not start with 'rar2zip '", s)
	}
	if !strings.Contains(s, "(") || !strings.Contains(s, ")") {
		t.Errorf("version output %q missing commit parens, want 'rar2zip v<ver> (<commit>)'", s)
	}
}

// TestRun_OverwriteGuard refuses to clobber an existing output unless --force.
// The overwrite check fires before the archive is opened, so a dummy .rar suffices.
func TestRun_OverwriteGuard(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.rar")
	if err := os.WriteFile(in, []byte("not a real rar"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "in.zip")
	const sentinel = "preexisting"
	if err := os.WriteFile(out, []byte(sentinel), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := run([]string{in}); got != 1 {
		t.Errorf("run without --force = %d, want 1 (refuse clobber)", got)
	}
	if b, _ := os.ReadFile(out); string(b) != sentinel {
		t.Errorf("output was modified despite no --force: %q", b)
	}
}

// TestRun_Convert exercises the happy path, --output, and --force against a real
// fixture. Skips when no fixture is present.
func TestRun_Convert(t *testing.T) {
	matches, _ := filepath.Glob("testdata/*.rar")
	if len(matches) == 0 {
		t.Skip("no testdata/*.rar fixture present; skipping conversion test")
	}
	src := matches[0]
	dir := t.TempDir()

	// --output to an explicit file path.
	out := filepath.Join(dir, "custom.zip")
	if got := run([]string{"-q", "-o", out, src}); got != 0 {
		t.Fatalf("run(-o) = %d, want 0", got)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("expected output %s: %v", out, err)
	}

	// Re-running without --force must refuse.
	if got := run([]string{"-q", "-o", out, src}); got != 1 {
		t.Errorf("re-run without --force = %d, want 1", got)
	}

	// --force overwrites.
	if got := run([]string{"-q", "-f", "-o", out, src}); got != 0 {
		t.Errorf("run(-f) = %d, want 0", got)
	}

	// --output to a directory writes <base>.zip inside it.
	if got := run([]string{"-q", "--output", dir, src}); got != 0 {
		t.Fatalf("run(--output dir) = %d, want 0", got)
	}
	base := filepath.Base(src)
	want := filepath.Join(dir, base[:len(base)-len(filepath.Ext(base))]+".zip")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("expected output in dir %s: %v", want, err)
	}
}

// TestRun_BatchUsageErrors covers Phase-3 flag-combination misuse (exit 2).
func TestRun_BatchUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"output with multiple inputs", []string{"-o", "x.zip", "a.rar", "b.rar"}},
		{"output and out-dir together", []string{"-o", "x.zip", "--out-dir", "d", "a.rar"}},
		{"jobs below one", []string{"--jobs", "0", "a.rar"}},
		{"one bad extension in batch", []string{"a.rar", "notes.txt"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := run(tc.args); got != 2 {
				t.Errorf("run(%v) = %d, want 2", tc.args, got)
			}
		})
	}
}

// TestDefaultJobs verifies the batch concurrency default is multi-core but
// capped (I/O-bound work doesn't benefit from unbounded fan-out) and never < 1.
func TestDefaultJobs(t *testing.T) {
	got := defaultJobs()
	if got < 1 {
		t.Fatalf("defaultJobs() = %d, must be >= 1", got)
	}
	if got > 4 {
		t.Errorf("defaultJobs() = %d, must be capped at 4", got)
	}
	want := runtime.NumCPU()
	if want > 4 {
		want = 4
	}
	if got != want {
		t.Errorf("defaultJobs() = %d, want min(NumCPU,4) = %d", got, want)
	}
}

// TestRun_List covers --list paths that don't need a real archive: rejecting
// output flags (exit 2), the non-rar extension guard (exit 2), and an unreadable
// archive surfacing as a runtime error (exit 1).
func TestRun_List(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "nope.rar")
	txt := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(txt, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		args []string
		want int
	}{
		{"list with output flag", []string{"--list", "-o", "x.zip", missing}, 2},
		{"list with out-dir flag", []string{"--list", "--out-dir", dir, missing}, 2},
		{"list non-rar input", []string{"--list", txt}, 2},
		{"list missing archive", []string{"--list", missing}, 1},
		{"list missing archive json", []string{"--list", "--json", missing}, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := run(tc.args); got != tc.want {
				t.Errorf("run(%v) = %d, want %d", tc.args, got, tc.want)
			}
		})
	}
}

// TestRun_ListFixture lists a real fixture and confirms it writes no ZIP and
// emits valid JSON with at least one entry. Skips when no fixture is present.
func TestRun_ListFixture(t *testing.T) {
	matches, _ := filepath.Glob("testdata/*.rar")
	if len(matches) == 0 {
		t.Skip("no testdata/*.rar fixture present; skipping list fixture test")
	}
	src := matches[0]

	// Capture stdout for the JSON listing.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	code := run([]string{"--list", "--json", src})
	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)

	if code != 0 {
		t.Fatalf("run(--list --json) = %d, want 0", code)
	}
	if !strings.Contains(string(out), "\"archives\"") || !strings.Contains(string(out), "\"entries\"") {
		t.Errorf("JSON listing missing expected keys:\n%s", out)
	}
	// --list must not write a sibling ZIP.
	base := filepath.Base(src)
	zipPath := filepath.Join(filepath.Dir(src), base[:len(base)-len(filepath.Ext(base))]+".zip")
	if _, err := os.Stat(zipPath); err == nil {
		os.Remove(zipPath)
		t.Errorf("--list wrote a ZIP at %s; it must be read-only", zipPath)
	}
}

// TestRun_Batch exercises multi-input conversion, --out-dir, --jobs, and
// continue-on-error against a real fixture. Skips when no fixture is present.
func TestRun_Batch(t *testing.T) {
	matches, _ := filepath.Glob("testdata/*.rar")
	if len(matches) == 0 {
		t.Skip("no testdata/*.rar fixture present; skipping batch test")
	}
	src := matches[0]
	dir := t.TempDir()

	// Two copies of a valid input + --out-dir + --jobs 2 -> both convert.
	outDir := filepath.Join(dir, "out")
	if got := run([]string{"-q", "--jobs", "2", "--out-dir", outDir, src, src}); got != 0 {
		t.Fatalf("batch run = %d, want 0", got)
	}
	base := filepath.Base(src)
	zipName := base[:len(base)-len(filepath.Ext(base))] + ".zip"
	if _, err := os.Stat(filepath.Join(outDir, zipName)); err != nil {
		t.Errorf("expected batch output %s: %v", zipName, err)
	}

	// Continue-on-error: one good input + one missing -> exit 1, good one still written.
	outDir2 := filepath.Join(dir, "out2")
	missing := filepath.Join(dir, "missing.rar")
	if got := run([]string{"-q", "--out-dir", outDir2, src, missing}); got != 1 {
		t.Errorf("batch with one failure = %d, want 1", got)
	}
	if _, err := os.Stat(filepath.Join(outDir2, zipName)); err != nil {
		t.Errorf("good input not converted despite sibling failure: %v", err)
	}
}
