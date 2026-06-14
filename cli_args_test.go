package main

import "testing"

func TestParseSize(t *testing.T) {
	cases := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{"0", 0, false},
		{"", 0, false},
		{"500", 500, false},
		{"10K", 10 << 10, false},
		{"2m", 2 << 20, false},
		{"1G", 1 << 30, false},
		{"-5", 0, true},
		{"abc", 0, true},
		{"10X", 0, true},
	}
	for _, c := range cases {
		got, err := parseSize(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("parseSize(%q) = %d, want error", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseSize(%q) error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("parseSize(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestBuildJobs_DetectsDstCollision(t *testing.T) {
	// Two distinct inputs in different dirs collapse to one --out-dir target.
	_, err := buildJobs([]string{"a/x.rar", "b/x.rar"}, "", "out")
	if err == nil {
		t.Fatal("expected a destination-collision error")
	}
}

func TestBuildJobs_NoCollision(t *testing.T) {
	jobs, err := buildJobs([]string{"a/x.rar", "b/y.rar"}, "", "out")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 2 {
		t.Errorf("jobs = %d, want 2", len(jobs))
	}
}

func TestRun_DstCollision(t *testing.T) {
	// Distinct sources that resolve to the same output -> usage error (2),
	// before any conversion runs.
	if got := run([]string{"-q", "--out-dir", "out", "a/x.rar", "b/x.rar"}); got != 2 {
		t.Errorf("run(collision) = %d, want 2", got)
	}
}

func TestRun_InvalidMaxSize(t *testing.T) {
	if got := run([]string{"--max-size", "bogus", "a.rar"}); got != 2 {
		t.Errorf("run(--max-size bogus) = %d, want 2", got)
	}
}

func TestRun_NegativeMaxEntries(t *testing.T) {
	if got := run([]string{"--max-entries", "-1", "a.rar"}); got != 2 {
		t.Errorf("run(--max-entries -1) = %d, want 2", got)
	}
}
