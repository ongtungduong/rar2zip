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
}

// Convert reads the RAR archive at srcRar and writes an equivalent ZIP archive
// at dstZip. Entries are streamed one at a time so memory stays bounded
// regardless of archive size. Per-entry modification time and Unix mode are
// preserved; entry names are sanitized to defend against Zip Slip. On any
// failure after the output file is created, the partial output is removed.
func Convert(srcRar, dstZip string, opts Options) error {
	if !opts.Force {
		if _, err := os.Stat(dstZip); err == nil {
			return fmt.Errorf("output exists (use -f to overwrite): %s", dstZip)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("stat output %q: %w", dstZip, err)
		}
	}

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
	if err := writeZip(tmp, rr, opts); err != nil {
		_ = tmp.Close()        // no-op if writeZip already closed it
		_ = os.Remove(tmpName) // discard the partial temp; dst is untouched
		return err
	}
	// os.CreateTemp makes the file 0600; match the conventional 0644 for output.
	if err := os.Chmod(tmpName, 0o644); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("set output permissions %q: %w", dstZip, err)
	}
	if err := os.Rename(tmpName, dstZip); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("finalize output %q: %w", dstZip, err)
	}
	return nil
}

// writeZip streams every entry from rr into a new ZIP written to out, closing
// the ZIP writer and out on success so late flush errors surface (rather than
// producing a silently truncated archive).
func writeZip(out *os.File, rr *rardecode.ReadCloser, opts Options) error {
	zw := zip.NewWriter(out)
	for {
		hdr, err := rr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read rar entry: %w", err)
		}

		// Reject any entry that would escape the archive root before it can
		// be written anywhere. rardecode reports names with '/' separators.
		name, err := sanitize(hdr.Name)
		if err != nil {
			return fmt.Errorf("unsafe rar entry %q: %w", hdr.Name, err)
		}

		fh := &zip.FileHeader{Name: name, Method: zip.Deflate}
		fh.SetMode(safeMode(hdr.Mode(), hdr.IsDir))
		if !hdr.ModificationTime.IsZero() {
			fh.Modified = hdr.ModificationTime
		}

		if hdr.IsDir {
			// A trailing slash is what records a directory entry in ZIP.
			fh.Name = strings.TrimRight(name, "/") + "/"
			if _, err := zw.CreateHeader(fh); err != nil {
				return fmt.Errorf("write zip dir %q: %w", hdr.Name, err)
			}
			if opts.OnEntry != nil {
				opts.OnEntry(fh.Name)
			}
			continue
		}

		w, err := zw.CreateHeader(fh)
		if err != nil {
			return fmt.Errorf("write zip entry %q: %w", hdr.Name, err)
		}
		if _, err := io.Copy(w, rr); err != nil {
			return fmt.Errorf("copy entry %q: %w", hdr.Name, err)
		}
		if opts.OnEntry != nil {
			opts.OnEntry(fh.Name)
		}
	}

	if err := zw.Close(); err != nil {
		return fmt.Errorf("finalize zip: %w", err)
	}
	// Close explicitly and check: a late write failure (ENOSPC, network/FUSE)
	// only surfaces here; ignoring it would report success on a truncated archive.
	if err := out.Close(); err != nil {
		return fmt.Errorf("close zip: %w", err)
	}
	return nil
}
