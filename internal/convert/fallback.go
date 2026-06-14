package convert

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// fallbackTools lists the external extractors to try, in preference order.
var fallbackTools = []string{"unrar", "7z"}

// lookFallbackTool returns the first available external extractor (name + full
// path), or ("", "") when none is installed.
func lookFallbackTool() (string, string) {
	for _, tool := range fallbackTools {
		if p, err := exec.LookPath(tool); err == nil {
			return tool, p
		}
	}
	return "", ""
}

// convertViaFallback extracts srcRar with a system unrar/7z into a temp dir and
// re-packs that tree into dstZip. It is only reached when the pure-Go decoder
// failed (nativeErr) and the caller opted in via --allow-fallback. When no tool
// is installed it returns an error wrapping nativeErr (the original cause).
func convertViaFallback(srcRar, dstZip string, opts Options, nativeErr error) error {
	tool, toolPath := lookFallbackTool()
	if tool == "" {
		return fmt.Errorf("pure-Go decode failed and no fallback tool (unrar/7z) found: %w", nativeErr)
	}

	workDir, err := os.MkdirTemp("", "rar2zip-fallback-*")
	if err != nil {
		return fmt.Errorf("fallback temp dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	if err := runExtract(toolPath, extractArgs(tool, srcRar, workDir, opts.Password)); err != nil {
		return fmt.Errorf("fallback %s extract failed: %w (pure-Go decode error: %v)", tool, err, nativeErr)
	}
	return zipDir(workDir, dstZip, opts)
}

// extractArgs builds the argument list to extract srcRar into destDir for the
// given tool, running non-interactively (never prompting for a password).
func extractArgs(tool, srcRar, destDir, password string) []string {
	switch tool {
	case "7z":
		args := []string{"x", "-y"}
		if password != "" {
			args = append(args, "-p"+password)
		}
		return append(args, srcRar, "-o"+destDir)
	default: // unrar
		args := []string{"x", "-o+"}
		if password != "" {
			args = append(args, "-p"+password)
		} else {
			args = append(args, "-p-") // never query for a password
		}
		return append(args, srcRar, destDir+string(os.PathSeparator))
	}
}

// runExtract executes the extractor, surfacing its combined output on failure.
func runExtract(toolPath string, args []string) error {
	cmd := exec.Command(toolPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// zipDir packs the tree rooted at srcDir into dstZip using the same atomic
// temp+rename+verify epilogue as the native path.
func zipDir(srcDir, dstZip string, opts Options) error {
	tmp, err := os.CreateTemp(filepath.Dir(dstZip), "."+filepath.Base(dstZip)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp output for %q: %w", dstZip, err)
	}
	tmpName := tmp.Name()
	expected, err := writeZipFromDir(tmp, srcDir, opts)
	if err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	return finalizeOutput(tmpName, dstZip, expected, opts)
}

// writeZipFromDir walks srcDir and writes every file and directory into a new
// ZIP on out, preserving relative paths, mode (sanitized), and mtime. Symlinks
// are neutralized — their target is stored as inert regular-file content — and
// other non-regular entries (devices, pipes, sockets) are skipped. It returns
// the expected name->size map for --verify.
func writeZipFromDir(out *os.File, srcDir string, opts Options) (map[string]int64, error) {
	expected := map[string]int64{}
	zw := zip.NewWriter(out)
	registerCompressor(zw, opts)

	walkErr := filepath.WalkDir(srcDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == srcDir {
			return nil // skip the root itself
		}
		rel, err := filepath.Rel(srcDir, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		info, err := d.Info()
		if err != nil {
			return err
		}

		if d.IsDir() {
			fh := &zip.FileHeader{Name: rel + "/"}
			fh.SetMode(safeMode(info.Mode(), true))
			fh.Modified = info.ModTime()
			if _, err := zw.CreateHeader(fh); err != nil {
				return fmt.Errorf("write zip dir %q: %w", rel, err)
			}
			expected[fh.Name] = 0
			if opts.OnEntry != nil {
				opts.OnEntry(fh.Name)
			}
			return nil
		}

		mode := info.Mode()
		fh := &zip.FileHeader{Name: rel, Method: entryMethod(opts)}
		fh.SetMode(safeMode(mode, false))
		fh.Modified = info.ModTime()

		// Symlink: store its target text as inert content (matches native
		// neutralization), never recreating a link that could escape on extract.
		if mode&fs.ModeSymlink != 0 {
			target, err := os.Readlink(p)
			if err != nil {
				return fmt.Errorf("read symlink %q: %w", rel, err)
			}
			w, err := zw.CreateHeader(fh)
			if err != nil {
				return fmt.Errorf("write zip entry %q: %w", rel, err)
			}
			if _, err := io.WriteString(w, target); err != nil {
				return fmt.Errorf("write symlink target %q: %w", rel, err)
			}
			expected[rel] = int64(len(target))
			if opts.OnEntry != nil {
				opts.OnEntry(rel)
			}
			return nil
		}
		if !mode.IsRegular() {
			return nil // devices/pipes/sockets carry no portable content
		}

		f, err := os.Open(p)
		if err != nil {
			return fmt.Errorf("open extracted file %q: %w", rel, err)
		}
		w, err := zw.CreateHeader(fh)
		if err != nil {
			f.Close()
			return fmt.Errorf("write zip entry %q: %w", rel, err)
		}
		n, err := io.Copy(w, f)
		f.Close()
		if err != nil {
			return fmt.Errorf("copy entry %q: %w", rel, err)
		}
		expected[rel] = n
		if opts.OnEntry != nil {
			opts.OnEntry(rel)
		}
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("finalize zip: %w", err)
	}
	if err := out.Close(); err != nil {
		return nil, fmt.Errorf("close zip: %w", err)
	}
	return expected, nil
}
