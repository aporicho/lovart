// Package jobs handles batch generation: JSONL parsing, quoting, running, resuming.
package jobs

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aporicho/lovart/internal/paths"
)

const (
	defaultMode = "auto"
)

var (
	validModes         = map[string]bool{"auto": true, "fast": true, "relax": true}
	quantityBodyFields = map[string]bool{"n": true, "max_images": true, "num_images": true, "count": true}
)

// JobLine is one user-level concept in a jobs.jsonl file.
type JobLine struct {
	Line            int            `json:"-"`
	JobID           string         `json:"job_id"`
	Title           string         `json:"title,omitempty"`
	Model           string         `json:"model"`
	Mode            string         `json:"mode"`
	Outputs         int            `json:"outputs"`
	OutputsExplicit bool           `json:"-"`
	Body            map[string]any `json:"body"`
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
	ProjectID       string
	CID             string
	AllowPaid       bool
	MaxTotalCredits float64
	Wait            bool
	Download        bool
	DownloadDir     string
	TimeoutSeconds  float64
	PollInterval    float64
	Detail          string
	RetryFailed     bool
	Refresh         bool
}

// ParseJobsFile reads and validates a jobs.jsonl file.
func ParseJobsFile(path string) ([]JobLine, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("jobs: read file: %w", err)
	}

	var jobs []JobLine
	seen := map[string]bool{}
	lines := splitLines(string(data))
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			return nil, fmt.Errorf("jobs: parse line %d: %w", i+1, err)
		}

		var j JobLine
		if err := json.Unmarshal([]byte(line), &j); err != nil {
			return nil, fmt.Errorf("jobs: parse line %d: %w", i+1, err)
		}
		if j.Model == "" {
			return nil, fmt.Errorf("jobs: line %d: model is required", i+1)
		}
		if j.Body == nil {
			return nil, fmt.Errorf("jobs: line %d: body must be an object", i+1)
		}
		if j.JobID == "" {
			j.JobID = fmt.Sprintf("line-%06d", i+1)
		}
		if seen[j.JobID] {
			return nil, fmt.Errorf("jobs: line %d: duplicate job_id %q", i+1, j.JobID)
		}
		seen[j.JobID] = true
		if j.Mode == "" {
			j.Mode = defaultMode
		}
		if !validModes[j.Mode] {
			return nil, fmt.Errorf("jobs: line %d: mode must be auto, fast, or relax", i+1)
		}
		_, j.OutputsExplicit = raw["outputs"]
		if !j.OutputsExplicit {
			j.Outputs = 1
		}
		if j.Outputs <= 0 {
			return nil, fmt.Errorf("jobs: line %d: outputs must be > 0", i+1)
		}
		if j.OutputsExplicit {
			if fields := conflictingQuantityFields(j.Body); len(fields) > 0 {
				return nil, fmt.Errorf("jobs: line %d: outputs conflicts with body quantity fields %s", i+1, strings.Join(fields, ", "))
			}
		}
		j.Line = i + 1
		jobs = append(jobs, j)
	}
	return jobs, nil
}

// JobsFileHash returns a stable content hash for a jobs file.
func JobsFileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("jobs: read file: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// DefaultRunDir returns the runtime state directory for a jobs file.
func DefaultRunDir(jobsFile string, outDir string) (string, error) {
	if outDir != "" {
		return outDir, nil
	}
	hash, err := JobsFileHash(jobsFile)
	if err != nil {
		return "", err
	}
	stem := safeStem(filepath.Base(jobsFile))
	return filepath.Join(paths.RunsDir, fmt.Sprintf("%s-%s", stem, hash[:12])), nil
}

func conflictingQuantityFields(body map[string]any) []string {
	var fields []string
	for key := range body {
		if quantityBodyFields[key] {
			fields = append(fields, key)
		}
	}
	sort.Strings(fields)
	return fields
}

func safeStem(name string) string {
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	var b strings.Builder
	for _, r := range stem {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "jobs"
	}
	return out
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
