package jobs

import (
	"fmt"

	"github.com/aporicho/lovart/internal/downloads"
)

// Result renders state at the requested detail level.
func Result(state *RunState, operation string, detail string) *BatchResult {
	if detail == "" {
		detail = "summary"
	}
	summary := Summary(state)
	requests := requestSummaries(state)
	result := &BatchResult{
		Operation:          operation,
		JobsFile:           state.JobsFile,
		JobsFileHash:       state.JobsFileHash,
		RunDir:             state.RunDir,
		StateFile:          state.StateFile,
		Summary:            summary,
		BatchGate:          state.BatchGate,
		Submission:         state.LastSubmission,
		TimedOut:           state.TimedOut,
		TaskCount:          countTasks(requests),
		TaskSampleLimit:    taskSampleLimit,
		Tasks:              taskSamples(requests),
		Failed:             failedRequests(requests),
		Downloads:          allDownloads(state),
		RecommendedActions: recommendedActions(state),
	}
	result.TasksTruncated = result.TaskCount > len(result.Tasks)
	if detail == "requests" || detail == "full" {
		result.RemoteRequests = requests
	}
	if detail == "full" {
		result.Jobs = state.Jobs
	}
	return result
}

func allDownloads(state *RunState) []downloads.FileResult {
	var out []downloads.FileResult
	for _, request := range allRequests(state) {
		out = append(out, request.Downloads...)
	}
	return out
}

func countTasks(requests []RequestSummary) int {
	total := 0
	for _, request := range requests {
		if request.TaskID != "" {
			total++
		}
	}
	return total
}

func taskSamples(requests []RequestSummary) []RequestSummary {
	var tasks []RequestSummary
	for _, request := range requests {
		if request.TaskID != "" {
			tasks = append(tasks, request)
		}
	}
	if len(tasks) <= taskSampleLimit {
		return tasks
	}
	return tasks[:taskSampleLimit]
}

func failedRequests(requests []RequestSummary) []RequestSummary {
	var failed []RequestSummary
	for _, request := range requests {
		if request.Status == StatusFailed {
			failed = append(failed, request)
		}
	}
	return failed
}

func recommendedActions(state *RunState) []string {
	var actions []string
	if countActive(state) > 0 {
		actions = append(actions, fmt.Sprintf("lovart jobs resume %s", state.RunDir))
		actions = append(actions, fmt.Sprintf("lovart jobs status %s --refresh", state.RunDir))
	}
	pending := 0
	failed := 0
	for _, request := range allRequests(state) {
		if request.Status == StatusPending || request.Status == StatusQuoted {
			pending++
		}
		if request.Status == StatusFailed {
			failed++
		}
	}
	if pending > 0 {
		actions = append(actions, fmt.Sprintf("lovart jobs resume %s", state.RunDir))
	}
	if failed > 0 {
		actions = append(actions, fmt.Sprintf("lovart jobs status %s --detail requests", state.RunDir))
	}
	return actions
}
