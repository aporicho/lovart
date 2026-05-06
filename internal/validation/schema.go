// Package validation converts registry and jobs validation results into
// user-facing error codes and recommended actions.
package validation

import (
	"fmt"

	"github.com/aporicho/lovart/internal/errors"
	"github.com/aporicho/lovart/internal/jobs"
	"github.com/aporicho/lovart/internal/registry"
)

// RequestErrorCode returns the public envelope error code for a request
// validation failure.
func RequestErrorCode(result registry.ValidationResult) string {
	for _, issue := range result.Issues {
		if issue.Code == "metadata_missing" {
			return errors.CodeMetadataStale
		}
	}
	return errors.CodeSchemaInvalid
}

// RequestRecommendedActions returns actionable remediation steps for a request
// validation failure.
func RequestRecommendedActions(result registry.ValidationResult) []string {
	for _, issue := range result.Issues {
		switch issue.Code {
		case "metadata_missing":
			return []string{"run `lovart update sync --all`"}
		case "unknown_model":
			return []string{
				"run `lovart models` to list available registry models",
				"run `lovart update sync --metadata-only` if the local model list is stale",
			}
		}
	}
	if result.Model != "" {
		return []string{
			fmt.Sprintf("run `lovart config %s` to inspect supported fields", result.Model),
			"update the request body to match the model schema",
		}
	}
	return []string{"update the request body to match the model schema"}
}

// JobsErrorCode returns the public envelope error code for a jobs validation
// failure.
func JobsErrorCode(err *jobs.ValidationError) string {
	for _, issue := range err.Issues {
		if RequestErrorCode(issue.Validation) == errors.CodeMetadataStale {
			return errors.CodeMetadataStale
		}
	}
	return errors.CodeSchemaInvalid
}

// JobsRecommendedActions returns actionable remediation steps for a jobs
// validation failure.
func JobsRecommendedActions(err *jobs.ValidationError) []string {
	for _, issue := range err.Issues {
		if actions := RequestRecommendedActions(issue.Validation); len(actions) > 0 {
			return actions
		}
	}
	return []string{"update invalid job bodies to match their model schemas"}
}
