package convert

import (
	"archive/zip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// writeFakeTool drops an executable shell script named tool into dir that, when
// run, creates a known file inside the last argument (the extraction dir). It
// returns dir so the caller can prepend it to PATH.
func writeFakeTool(t *testing.T, dir, tool, marker string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake-tool test uses a POSIX shell script")
	}
	script := "#!/bin/sh\n" +
		// The last positional arg is the destination dir; POSIX way to grab it.
		"for dest do :; done\n" +
		"dest=\"${dest%/}\"\n" +
		"mkdir -p \"$dest/sub\"\n" +
		"printf '%s' '" + marker + "' > \"$dest/extracted.txt\"\n" +
		"printf '%s' 'nested' > \"$dest/sub/inner.txt\"\n"
	p := filepath.Join(dir, tool)
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestLookFallbackTool(t *testing.T) {
	dir := t.TempDir()
	writeFakeTool(t, dir, "unrar", "x")
	t.Setenv("PATH", dir)

	name, path := lookFallbackTool()
	if name != "unrar" {
		t.Errorf("tool name = %q, want unrar", name)
	}
	if path == "" {
		t.Error("tool path is empty")
	}
}

func TestLookFallbackTool_None(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // empty dir, no tools
	if name, _ := lookFallbackTool(); name != "" {
		t.Errorf("expected no tool, got %q", name)
	}
}

func TestZipDir(t *testing.T) {
	src := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "sub", "b.txt"), []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(t.TempDir(), "out.zip")
	if err := zipDir(src, dst, Options{Verify: true}); err != nil {
		t.Fatalf("zipDir: %v", err)
	}

	got := map[string]string{}
	zr, err := zip.OpenReader(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		got[f.Name] = string(b)
	}
	if got["a.txt"] != "hello" || got["sub/b.txt"] != "world" {
		t.Errorf("zipped content = %v, want a.txt=hello sub/b.txt=world", got)
	}
}

func TestConvertViaFallback_Integration(t *testing.T) {
	dir := t.TempDir()
	writeFakeTool(t, dir, "unrar", "FROMTOOL")
	// Prepend the fake-tool dir so our unrar wins, but keep the system bins on
	// PATH so the script's own mkdir/printf resolve.
	t.Setenv("PATH", dir+string(os.PathListSeparator)+"/bin:/usr/bin")

	dst := filepath.Join(t.TempDir(), "out.zip")
	// srcRar need not exist; the fake tool ignores it and writes fixed output.
	err := convertViaFallback("whatever.rar", dst, Options{}, errors.New("native decode failed"))
	if err != nil {
		t.Fatalf("convertViaFallback: %v", err)
	}

	zr, err := zip.OpenReader(dst)
	if err != nil {
		t.Fatalf("open result zip: %v", err)
	}
	defer zr.Close()
	var found bool
	for _, f := range zr.File {
		if f.Name == "extracted.txt" {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			rc.Close()
			if string(b) == "FROMTOOL" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected extracted.txt with tool content in fallback output")
	}
}

func TestExtractArgs(t *testing.T) {
	tests := []struct {
		name     string
		tool     string
		password string
		want     []string
	}{
		{"unrar no password", "unrar", "", []string{"x", "-o+", "-p-", "a.rar", "out/"}},
		{"unrar with password", "unrar", "secret", []string{"x", "-o+", "-psecret", "a.rar", "out/"}},
		{"7z no password", "7z", "", []string{"x", "-y", "a.rar", "-oout"}},
		{"7z with password", "7z", "secret", []string{"x", "-y", "-psecret", "a.rar", "-oout"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractArgs(tc.tool, "a.rar", "out", tc.password)
			if strings.Join(got, " ") != strings.Join(tc.want, " ") {
				t.Errorf("extractArgs = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestConvertViaFallback_NoTool_ReturnsNativeError(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // no unrar/7z
	nativeErr := errors.New("native decode failed")
	dst := filepath.Join(t.TempDir(), "out.zip")

	err := convertViaFallback("x.rar", dst, Options{}, nativeErr)
	if err == nil {
		t.Fatal("expected an error when no fallback tool is present")
	}
	if !errors.Is(err, nativeErr) {
		t.Errorf("error %v does not wrap the original native error", err)
	}
}
