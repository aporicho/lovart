package jobs

import (
	"fmt"

	"github.com/aporicho/lovart/internal/registry"
)

// JobValidationIssue binds a schema validation result to a JSONL job line.
type JobValidationIssue struct {
	Line       int                       `json:"line"`
	JobID      string                    `json:"job_id,omitempty"`
	Model      string                    `json:"model"`
	Validation registry.ValidationResult `json:"validation"`
}

// ValidationError reports one or more invalid job bodies.
type ValidationError struct {
	Issues []JobValidationIssue `json:"issues"`
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("jobs: %d job(s) failed schema validation", len(e.Issues))
}

// PrepareJobsFile parses a JSONL job file and validates every request body.
func PrepareJobsFile(path string) ([]JobLine, *ValidationError, error) {
	lines, err := ParseJobsFile(path)
	if err != nil {
		return nil, nil, err
	}
	return lines, ValidateJobLines(lines), nil
}

// ValidateJobLines validates all job bodies against the runtime registry.
func ValidateJobLines(lines []JobLine) *ValidationError {
	reg, err := registry.Load()
	if err != nil {
		return &ValidationError{Issues: []JobValidationIssue{{
			Line:  0,
			Model: "",
			Validation: registry.ValidationResult{
				OK:    false,
				Model: "",
				Issues: []registry.ValidationIssue{{
					Path:    "$",
					Code:    "metadata_missing",
					Message: err.Error(),
				}},
			},
		}}}
	}

	var issues []JobValidationIssue
	for _, line := range lines {
		result := reg.ValidateRequest(line.Model, line.Body)
		if result.OK {
			continue
		}
		issues = append(issues, JobValidationIssue{
			Line:       line.Line,
			JobID:      line.JobID,
			Model:      line.Model,
			Validation: result,
		})
	}
	if len(issues) == 0 {
		return nil
	}
	return &ValidationError{Issues: issues}
}
