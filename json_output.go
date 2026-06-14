package main

import (
	"encoding/json"
	"io"

	"github.com/ongtungduong/rar2zip/internal/convert"
)

// jsonResult is the machine-readable form of one conversion outcome.
type jsonResult struct {
	Src   string `json:"src"`
	Dst   string `json:"dst"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// jsonSummary is the top-level --json document.
type jsonSummary struct {
	Succeeded int          `json:"succeeded"`
	Failed    int          `json:"failed"`
	Results   []jsonResult `json:"results"`
}

// reportJSON writes a JSON summary of the batch to w and returns the aggregate
// exit code: 1 if any job failed, else 0.
func reportJSON(results []convert.Result, w io.Writer) int {
	summary := jsonSummary{Results: make([]jsonResult, 0, len(results))}
	for _, r := range results {
		jr := jsonResult{Src: r.Src, Dst: r.Dst, OK: r.Err == nil}
		if r.Err != nil {
			jr.Error = r.Err.Error()
			summary.Failed++
		} else {
			summary.Succeeded++
		}
		summary.Results = append(summary.Results, jr)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	// Encode failure is effectively impossible for this struct + an os.Stdout
	// writer, but surface it as a runtime error rather than reporting success.
	if err := enc.Encode(summary); err != nil {
		return 1
	}
	if summary.Failed > 0 {
		return 1
	}
	return 0
}
