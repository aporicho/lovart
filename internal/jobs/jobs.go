// Package jobs handles batch generation: JSONL parsing, quoting, running, resuming.
package jobs

import (
	"encoding/json"
	"fmt"
	"os"
)

// JobLine is one user-level concept in a jobs.jsonl file.
type JobLine struct {
	Line    int            `json:"-"`
	JobID   string         `json:"job_id"`
	Title   string         `json:"title,omitempty"`
	Model   string         `json:"model"`
	Mode    string         `json:"mode"`
	Outputs int            `json:"outputs"`
	Body    map[string]any `json:"body"`
}

// RunSummary reports batch run results.
type RunSummary struct {
	Submitted int `json:"submitted"`
	Running   int `json:"running"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
}

// QuoteOptions configures batch quoting.
type QuoteOptions struct {
	OutDir      string
	Detail      string
	Concurrency int
	Limit       string
	AllRequests bool
	Refresh     bool
}

// JobsOptions configures batch run/resume.
type JobsOptions struct {
	OutDir          string
	AllowPaid       bool
	MaxTotalCredits float64
	Wait            bool
	Download        bool
	DownloadDir     string
	TimeoutSeconds  float64
	PollInterval    float64
	Detail          string
	RetryFailed     bool
}

// ParseJobsFile reads and validates a jobs.jsonl file.
func ParseJobsFile(path string) ([]JobLine, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("jobs: read file: %w", err)
	}

	var jobs []JobLine
	lines := splitLines(string(data))
	for i, line := range lines {
		if line == "" {
			continue
		}
		var j JobLine
		if err := json.Unmarshal([]byte(line), &j); err != nil {
			return nil, fmt.Errorf("jobs: parse line %d: %w", i+1, err)
		}
		if j.Model == "" {
			return nil, fmt.Errorf("jobs: line %d: model is required", i+1)
		}
		if j.Outputs <= 0 {
			return nil, fmt.Errorf("jobs: line %d: outputs must be > 0", i+1)
		}
		j.Line = i + 1
		jobs = append(jobs, j)
	}
	return jobs, nil
}

func splitLines(s string) []string {
	var lines []string
	current := ""
	for _, c := range s {
		if c == '\n' {
			lines = append(lines, current)
			current = ""
		} else if c != '\r' {
			current += string(c)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}
