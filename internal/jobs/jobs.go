// Package jobs handles batch generation: parsing JSONL files, quoting, running, resuming.
package jobs

import (
	"encoding/json"
	"fmt"
	"os"
)

// JobLine is one user-level concept in a jobs.jsonl file.
type JobLine struct {
	JobID   string         `json:"job_id"`
	Title   string         `json:"title,omitempty"`
	Model   string         `json:"model"`
	Mode    string         `json:"mode"`
	Outputs int            `json:"outputs"`
	Body    map[string]any `json:"body"`
}

// QuoteSummary reports batch quote results without exposing prompts or full bodies.
type QuoteSummary struct {
	LogicalJobs    int     `json:"logical_jobs"`
	RemoteRequests int     `json:"remote_requests"`
	TotalCredits   float64 `json:"total_credits"`
	PendingQuotes  int     `json:"pending_quote_remote_requests"`
}

// RunSummary reports batch run results.
type RunSummary struct {
	Submitted int    `json:"submitted"`
	Running   int    `json:"running"`
	Completed int    `json:"completed"`
	Failed    int    `json:"failed"`
}

// QuoteOptions configures batch quoting.
type QuoteOptions struct {
	OutDir      string
	Detail      string // summary, requests, full
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
	// Simple line-by-line JSONL parsing
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

// QuoteJobs runs batch quoting.
func QuoteJobs(jobsFile string, opts QuoteOptions) (*QuoteSummary, error) {
	// TODO: implement batch quoting with remote_requests expansion and live pricing
	return &QuoteSummary{}, nil
}

// DryRunJobs validates batch without submitting.
func DryRunJobs(jobsFile string, opts JobsOptions) (map[string]any, error) {
	// TODO: implement dry run
	return nil, nil
}

// RunJobs submits a batch for generation.
func RunJobs(jobsFile string, opts JobsOptions) (*RunSummary, error) {
	// TODO: implement batch run
	return &RunSummary{}, nil
}

// ResumeJobs continues an interrupted batch.
func ResumeJobs(jobsFile string, opts JobsOptions) (*RunSummary, error) {
	// TODO: implement resume
	return &RunSummary{}, nil
}

// StatusJobs reports batch run progress.
func StatusJobs(runDir string, detail string) (map[string]any, error) {
	// TODO: implement status
	return nil, nil
}
