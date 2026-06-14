package convert

import (
	"archive/zip"
	"fmt"
)

// verify reopens the ZIP at path and confirms it contains exactly the expected
// entries (keyed by ZIP name, directories with a trailing slash) and that every
// file entry's uncompressed size matches. It is the self-check behind --verify:
// a corrupt or truncated output is caught before Convert reports success.
func verify(path string, expected map[string]int64) error {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("verify: reopen %q: %w", path, err)
	}
	defer zr.Close()

	if len(zr.File) != len(expected) {
		return fmt.Errorf("verify: entry count mismatch: wrote %d, found %d", len(expected), len(zr.File))
	}

	for _, f := range zr.File {
		wantSize, ok := expected[f.Name]
		if !ok {
			return fmt.Errorf("verify: unexpected entry %q in output", f.Name)
		}
		if int64(f.UncompressedSize64) != wantSize {
			return fmt.Errorf("verify: size mismatch for %q: wrote %d, found %d",
				f.Name, wantSize, f.UncompressedSize64)
		}
	}
	return nil
}
