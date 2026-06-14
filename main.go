// Command rar2zip converts one or more .rar archives into .zip archives.
//
// Usage:
//
//	rar2zip [flags] <input.rar> [more.rar ...]
//
// By default each <input>.zip is written alongside its input file.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ongtungduong/rar2zip/internal/convert"
)

// version is overridden at build time via -ldflags (see distribution phase).
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:]))
}

// run executes the CLI and returns a process exit code:
// 0 success, 1 runtime error, 2 usage error.
func run(args []string) int {
	var (
		output      string
		outDir      string
		force       bool
		quiet       bool
		password    string
		jobs        int
		showVersion bool
	)

	fs := flag.NewFlagSet("rar2zip", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&output, "o", "", "write the single output to this file or directory")
	fs.StringVar(&output, "output", "", "write the single output to this file or directory")
	fs.StringVar(&outDir, "out-dir", "", "write all outputs into this directory")
	fs.BoolVar(&force, "f", false, "overwrite outputs that already exist")
	fs.BoolVar(&force, "force", false, "overwrite outputs that already exist")
	fs.BoolVar(&quiet, "q", false, "suppress progress output")
	fs.BoolVar(&quiet, "quiet", false, "suppress progress output")
	fs.StringVar(&password, "password", "", "password for encrypted archives")
	fs.IntVar(&jobs, "jobs", 1, "number of archives to convert concurrently")
	fs.BoolVar(&showVersion, "version", false, "print version and exit")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: rar2zip [flags] <input.rar> [more.rar ...]\n\n"+
			"Convert RAR archives to ZIP. By default writes <input>.zip alongside each input.\n\n"+
			"flags:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp { // -h / --help
			return 0
		}
		return 2
	}

	if showVersion {
		fmt.Printf("rar2zip %s\n", version)
		return 0
	}

	inputs := fs.Args()
	if code := validateArgs(inputs, output, outDir, jobs); code != 0 {
		return code
	}

	if outDir != "" {
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "rar2zip: %v\n", err)
			return 1
		}
	}

	jobList := make([]convert.Job, len(inputs))
	for i, src := range inputs {
		jobList[i] = convert.Job{Src: src, Dst: resolveDst(src, output, outDir)}
	}

	opts := convert.Options{Password: password, Force: force}
	// Per-entry progress only makes sense for a single archive; concurrent
	// batches would interleave. Batches get a per-archive line instead.
	var onStart func(convert.Job)
	if !quiet {
		if len(jobList) == 1 {
			n := 0
			opts.OnEntry = func(name string) {
				n++
				fmt.Fprintf(os.Stderr, "\r\033[K[%d] %s", n, name)
			}
		} else {
			onStart = func(j convert.Job) {
				fmt.Fprintf(os.Stderr, "converting %s\n", j.Src)
			}
		}
	}

	results := convert.RunBatch(jobList, opts, jobs, onStart)
	return report(results, quiet)
}

// validateArgs returns a usage exit code (2) for malformed invocations, or 0.
func validateArgs(inputs []string, output, outDir string, jobs int) int {
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

// report prints per-job outcomes and a batch summary, returning the aggregate
// exit code: 1 if any job failed, else 0.
func report(results []convert.Result, quiet bool) int {
	failed := 0
	for _, r := range results {
		if r.Err != nil {
			failed++
			fmt.Fprintf(os.Stderr, "rar2zip: %s: %v\n", r.Src, r.Err)
			continue
		}
		fmt.Printf("wrote %s\n", r.Dst)
	}

	if len(results) > 1 && !quiet {
		fmt.Fprintf(os.Stderr, "%d succeeded, %d failed\n", len(results)-failed, failed)
	} else if len(results) == 1 && !quiet {
		// Terminate the single-archive progress line.
		fmt.Fprintln(os.Stderr)
	}

	if failed > 0 {
		return 1
	}
	return 0
}
