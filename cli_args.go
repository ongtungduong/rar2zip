package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// validateArgs returns a usage exit code (2) for malformed invocations, or 0.
func validateArgs(inputs []string, output, outDir string, jobs int, store bool, level int) int {
	usage := func(format string, a ...any) int {
		fmt.Fprintf(os.Stderr, "rar2zip: "+format+"\n", a...)
		return 2
	}
	if len(inputs) == 0 {
		fmt.Fprintln(os.Stderr, "usage: rar2zip [flags] <input.rar> [more.rar ...]")
		return 2
	}
	if output != "" && outDir != "" {
		return usage("-o/--output and --out-dir are mutually exclusive")
	}
	if output != "" && len(inputs) > 1 {
		return usage("-o/--output targets a single file; use --out-dir for multiple inputs")
	}
	if jobs < 1 {
		return usage("--jobs must be >= 1")
	}
	if store && level != 0 {
		return usage("cannot combine --store and --level")
	}
	if level != 0 && (level < 1 || level > 9) {
		return usage("--level must be between 1 and 9 (use --store for no compression)")
	}
	for _, src := range inputs {
		if !strings.EqualFold(filepath.Ext(src), ".rar") {
			return usage("input must be a .rar file: %s", src)
		}
		if fi, err := os.Stat(src); err == nil && fi.IsDir() {
			return usage("input is a directory, not a .rar file: %s", src)
		}
	}
	return 0
}

// resolveDst computes the destination ZIP path for one input. --out-dir places
// <base>.zip in that directory; -o names a file (or, if it is an existing
// directory, places <base>.zip inside it); otherwise the output is the sibling
// <input>.zip.
func resolveDst(src, output, outDir string) string {
	base := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src)) + ".zip"
	switch {
	case outDir != "":
		return filepath.Join(outDir, base)
	case output != "":
		if fi, err := os.Stat(output); err == nil && fi.IsDir() {
			return filepath.Join(output, base)
		}
		return output
	default:
		return filepath.Join(filepath.Dir(src), base)
	}
}
