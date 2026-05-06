package cli

import (
	"github.com/aporicho/lovart/internal/jobs"
	"github.com/aporicho/lovart/internal/registry"
	sharedvalidation "github.com/aporicho/lovart/internal/validation"
)

func validationErrorCode(result registry.ValidationResult) string {
	return sharedvalidation.RequestErrorCode(result)
}

func validationRecommendedActions(result registry.ValidationResult) []string {
	return sharedvalidation.RequestRecommendedActions(result)
}

func jobValidationErrorCode(err *jobs.ValidationError) string {
	return sharedvalidation.JobsErrorCode(err)
}

func jobValidationRecommendedActions(err *jobs.ValidationError) []string {
	return sharedvalidation.JobsRecommendedActions(err)
}
