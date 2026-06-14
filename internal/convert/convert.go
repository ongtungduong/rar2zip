// Package convert performs the core RAR-to-ZIP transformation.
package convert

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/nwaples/rardecode/v2"
)

// Options tunes a Convert run.
type Options struct {
	// OnEntry, if non-nil, is called once per archive entry (files and
	// directories) with the sanitized entry name, for progress reporting.
	OnEntry func(name string)
	// Password, if set, decrypts password-protected RAR archives.
	Password string
	// Force allows overwriting an existing output file. When false, Convert
	// refuses (and does not touch) a destination that already exists.
	Force bool
	// Store writes entries uncompressed (zip.Store) instead of Deflate.
	Store bool
	// Level sets the Deflate compression level. 0 (the zero value) means the
	// stdlib default; 1..9 selects an explicit level (1 = fastest, 9 = best).
	// Ignored when Store is set. For no compression, use Store.
	Level int
	// Verify reopens the finished ZIP and confirms every expected entry is
	// present with a matching uncompressed size before reporting success.
	Verify bool
	// AllowFallback permits shelling out to a system unrar/7z when the pure-Go
	// decoder cannot read the archive. This waives the no-dependency guarantee.
	AllowFallback bool
}

// DefaultLevel is the Level value (zero) that selects the stdlib default
// Deflate compression rather than an explicit 1..9 level.
const DefaultLevel = 0

// Convert reads the RAR archive at srcRar and writes an equivalent ZIP archive
// at dstZip. Entries are streamed one at a time so memory stays bounded
// regardless of archive size. Per-entry modification time and Unix mode are
// preserved; entry names are sanitized to defend against Zip Slip. On any
// failure after the output file is created, the partial output is removed.
//
// When the pure-Go decoder cannot read the archive and opts.AllowFallback is
// set, Convert retries by shelling out to a system unrar/7z (see fallback.go).
func Convert(srcRar, dstZip string, opts Options) error {
	if !opts.Force {
		if _, err := os.Stat(dstZip); err == nil {
			return fmt.Errorf("output exists (use -f to overwrite): %s", dstZip)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("stat output %q: %w", dstZip, err)
		}
	}

	err := convertNative(srcRar, dstZip, opts)
	if err != nil && opts.AllowFallback {
		return convertViaFallback(srcRar, dstZip, opts, err)
	}
	return err
}

// convertNative performs the pure-Go RAR→ZIP conversion (no external tools).
func convertNative(srcRar, dstZip string, opts Options) error {
	var ropts []rardecode.Option
	if opts.Password != "" {
		ropts = append(ropts, rardecode.Password(opts.Password))
	}
	rr, err := rardecode.OpenReader(srcRar, ropts...)
	if err != nil {
		return fmt.Errorf("open rar %q: %w", srcRar, err)
	}
	defer rr.Close()

	// Write to a temporary file in the destination directory, then atomically
	// rename it into place only after a complete archive exists. A failed
	// conversion therefore never truncates or deletes the destination — which a
	// naive create-in-place would do under --force on a mid-stream failure.
	tmp, err := os.CreateTemp(filepath.Dir(dstZip), "."+filepath.Base(dstZip)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp output for %q: %w", dstZip, err)
	}
	tmpName := tmp.Name()
	expected, err := writeZip(tmp, rr, opts)
	if err != nil {
		_ = tmp.Close()        // no-op if writeZip already closed it
		_ = os.Remove(tmpName) // discard the partial temp; dst is untouched
		return err
	}
	return finalizeOutput(tmpName, dstZip, expected, opts)
}

// finalizeOutput chmods the temp ZIP to 0644, atomically renames it into place,
// and (when opts.Verify) reopens it to confirm it matches expected. Any failure
// removes the partial output so a broken ZIP never lands at dstZip. Shared by
// the native and shell-out fallback paths.
func finalizeOutput(tmpName, dstZip string, expected map[string]int64, opts Options) error {
	// os.CreateTemp makes the file 0600; match the conventional 0644 for output.
	if err := os.Chmod(tmpName, 0o644); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("set output permissions %q: %w", dstZip, err)
	}
	if err := os.Rename(tmpName, dstZip); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("finalize output %q: %w", dstZip, err)
	}
	if opts.Verify {
		if err := verify(dstZip, expected); err != nil {
			_ = os.Remove(dstZip) // a ZIP that fails its own check is not a usable output
			return err
		}
	}
	return nil
}

// writeZip streams every entry from rr into a new ZIP written to out, closing
// the ZIP writer and out on success so late flush errors surface (rather than
// producing a silently truncated archive). It returns an expected name->size
// map (files by uncompressed size, directories as 0) that --verify checks the
// finished archive against.
func writeZip(out *os.File, rr *rardecode.ReadCloser, opts Options) (map[string]int64, error) {
	expected := map[string]int64{}
	zw := zip.NewWriter(out)
	registerCompressor(zw, opts)
	for {
		hdr, err := rr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read rar entry: %w", err)
		}

		// Reject any entry that would escape the archive root before it can
		// be written anywhere. rardecode reports names with '/' separators.
		name, err := sanitize(hdr.Name)
		if err != nil {
			return nil, fmt.Errorf("unsafe rar entry %q: %w", hdr.Name, err)
		}

		fh := &zip.FileHeader{Name: name, Method: entryMethod(opts)}
		fh.SetMode(safeMode(hdr.Mode(), hdr.IsDir))
		if !hdr.ModificationTime.IsZero() {
			fh.Modified = hdr.ModificationTime
		}

		if hdr.IsDir {
			// A trailing slash is what records a directory entry in ZIP.
			fh.Name = strings.TrimRight(name, "/") + "/"
			if _, err := zw.CreateHeader(fh); err != nil {
				return nil, fmt.Errorf("write zip dir %q: %w", hdr.Name, err)
			}
			expected[fh.Name] = 0
			if opts.OnEntry != nil {
				opts.OnEntry(fh.Name)
			}
			continue
		}

		w, err := zw.CreateHeader(fh)
		if err != nil {
			return nil, fmt.Errorf("write zip entry %q: %w", hdr.Name, err)
		}
		n, err := io.Copy(w, rr)
		if err != nil {
			return nil, fmt.Errorf("copy entry %q: %w", hdr.Name, err)
		}
		expected[fh.Name] = n
		if opts.OnEntry != nil {
			opts.OnEntry(fh.Name)
		}
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("finalize zip: %w", err)
	}
	// Close explicitly and check: a late write failure (ENOSPC, network/FUSE)
	// only surfaces here; ignoring it would report success on a truncated archive.
	if err := out.Close(); err != nil {
		return nil, fmt.Errorf("close zip: %w", err)
	}
	return expected, nil
}
