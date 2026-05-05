package jobs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/aporicho/lovart/internal/metadata"
	"github.com/aporicho/lovart/internal/pricing"
)

const (
	stateFileName = "jobs_state.json"

	StatusPending    = "pending"
	StatusQuoted     = "quoted"
	StatusSubmitted  = "submitted"
	StatusRunning    = "running"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
	StatusSkipped    = "skipped"
	StatusDownloaded = "downloaded"
)

// RunState persists all local batch execution state.
type RunState struct {
	Version      int        `json:"version"`
	JobsFile     string     `json:"jobs_file"`
	JobsFileHash string     `json:"jobs_file_hash"`
	RunDir       string     `json:"run_dir"`
	StateFile    string     `json:"state_file"`
	ProjectID    string     `json:"project_id,omitempty"`
	CID          string     `json:"cid,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	BatchGate    *BatchGate `json:"batch_gate,omitempty"`
	TimedOut     bool       `json:"timed_out,omitempty"`
	Jobs         []JobState `json:"jobs"`
}

// JobState records one logical user job and all concrete remote requests.
type JobState struct {
	Line           int             `json:"line"`
	JobID          string          `json:"job_id"`
	Title          string          `json:"title,omitempty"`
	Model          string          `json:"model"`
	Mode           string          `json:"mode"`
	Outputs        int             `json:"outputs"`
	Body           map[string]any  `json:"body"`
	Status         string          `json:"status"`
	RemoteRequests []RemoteRequest `json:"remote_requests"`
	Errors         []JobError      `json:"errors,omitempty"`
}

// RemoteRequest is one atomic Lovart task submission.
type RemoteRequest struct {
	RequestID   string               `json:"request_id"`
	JobID       string               `json:"job_id"`
	Title       string               `json:"title,omitempty"`
	Model       string               `json:"model"`
	Mode        string               `json:"mode"`
	Index       int                  `json:"index"`
	OutputCount int                  `json:"output_count"`
	Body        map[string]any       `json:"body"`
	Status      string               `json:"status"`
	Quote       *pricing.QuoteResult `json:"quote,omitempty"`
	TaskID      string               `json:"task_id,omitempty"`
	Response    map[string]any       `json:"response,omitempty"`
	Task        map[string]any       `json:"task,omitempty"`
	Artifacts   []map[string]any     `json:"artifacts,omitempty"`
	Attempts    int                  `json:"attempts"`
	Errors      []JobError           `json:"errors,omitempty"`
	UpdatedAt   time.Time            `json:"updated_at,omitempty"`
}

// JobError records a structured per-request failure.
type JobError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// BatchResult is the command-facing batch output.
type BatchResult struct {
	Operation          string           `json:"operation"`
	JobsFile           string           `json:"jobs_file,omitempty"`
	JobsFileHash       string           `json:"jobs_file_hash,omitempty"`
	RunDir             string           `json:"run_dir"`
	StateFile          string           `json:"state_file"`
	Summary            BatchSummary     `json:"summary"`
	BatchGate          *BatchGate       `json:"batch_gate,omitempty"`
	TimedOut           bool             `json:"timed_out,omitempty"`
	TaskCount          int              `json:"task_count"`
	TaskSampleLimit    int              `json:"task_sample_limit,omitempty"`
	TasksTruncated     bool             `json:"tasks_truncated,omitempty"`
	Tasks              []RequestSummary `json:"tasks,omitempty"`
	RemoteRequests     []RequestSummary `json:"remote_requests,omitempty"`
	Failed             []RequestSummary `json:"failed,omitempty"`
	Jobs               []JobState       `json:"jobs,omitempty"`
	RecommendedActions []string         `json:"recommended_actions,omitempty"`
}

// BatchSummary is a compact state summary.
type BatchSummary struct {
	LogicalJobs                  int            `json:"logical_jobs"`
	RemoteRequests               int            `json:"remote_requests"`
	RequestedOutputs             int            `json:"requested_outputs"`
	StatusCounts                 map[string]int `json:"status_counts"`
	RemoteStatusCounts           map[string]int `json:"remote_status_counts"`
	TotalCredits                 float64        `json:"total_credits"`
	PaidRemoteRequests           int            `json:"paid_remote_requests"`
	ZeroCreditRemoteRequests     int            `json:"zero_credit_remote_requests"`
	UnknownPricingRemoteRequests int            `json:"unknown_pricing_remote_requests"`
	FailedJobs                   int            `json:"failed_jobs"`
	Complete                     bool           `json:"complete"`
}

// RequestSummary is the compact public view of a remote request.
type RequestSummary struct {
	JobID         string    `json:"job_id"`
	Title         string    `json:"title,omitempty"`
	RequestID     string    `json:"request_id"`
	Model         string    `json:"model"`
	Mode          string    `json:"mode"`
	OutputCount   int       `json:"output_count"`
	Status        string    `json:"status"`
	TaskID        string    `json:"task_id,omitempty"`
	RemoteStatus  string    `json:"remote_status,omitempty"`
	ArtifactCount int       `json:"artifact_count"`
	QuoteCredits  float64   `json:"quote_credits,omitempty"`
	LastError     *JobError `json:"last_error,omitempty"`
}

// NewRunState builds an unsaved state from parsed jobs.
func NewRunState(jobsFile string, lines []JobLine, opts JobsOptions) (*RunState, error) {
	runDir, err := DefaultRunDir(jobsFile, opts.OutDir)
	if err != nil {
		return nil, err
	}
	hash, err := JobsFileHash(jobsFile)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	state := &RunState{
		Version:      1,
		JobsFile:     jobsFile,
		JobsFileHash: hash,
		RunDir:       runDir,
		StateFile:    filepath.Join(runDir, stateFileName),
		ProjectID:    opts.ProjectID,
		CID:          opts.CID,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	for _, line := range lines {
		job, err := newJobState(line)
		if err != nil {
			return nil, err
		}
		state.Jobs = append(state.Jobs, job)
	}
	RefreshStatuses(state)
	return state, nil
}

func newJobState(line JobLine) (JobState, error) {
	job := JobState{
		Line:    line.Line,
		JobID:   line.JobID,
		Title:   line.Title,
		Model:   line.Model,
		Mode:    line.Mode,
		Outputs: line.Outputs,
		Body:    copyBody(line.Body),
		Status:  StatusPending,
	}
	if !line.OutputsExplicit && hasQuantityField(line.Body) {
		job.Outputs = inferredOutputCount(line.Body)
		job.RemoteRequests = append(job.RemoteRequests, RemoteRequest{
			RequestID:   fmt.Sprintf("%s-%03d", line.JobID, 1),
			JobID:       line.JobID,
			Title:       line.Title,
			Model:       line.Model,
			Mode:        line.Mode,
			Index:       1,
			OutputCount: job.Outputs,
			Body:        copyBody(line.Body),
			Status:      StatusPending,
		})
		return job, nil
	}

	subs, err := Expand(line.Model, line.Outputs, line.Body)
	if err != nil {
		return JobState{}, err
	}
	for _, sub := range subs {
		request := RemoteRequest{
			RequestID:   fmt.Sprintf("%s-%03d", line.JobID, sub.Index),
			JobID:       line.JobID,
			Title:       line.Title,
			Model:       line.Model,
			Mode:        line.Mode,
			Index:       sub.Index,
			OutputCount: sub.N,
			Body:        copyBody(sub.Body),
			Status:      StatusPending,
		}
		job.RemoteRequests = append(job.RemoteRequests, request)
	}
	return job, nil
}

func hasQuantityField(body map[string]any) bool {
	return len(conflictingQuantityFields(body)) > 0
}

func inferredOutputCount(body map[string]any) int {
	for _, key := range []string{"n", "max_images", "num_images", "count"} {
		if n, ok := intValue(body[key]); ok && n > 0 {
			return n
		}
	}
	return 1
}

func intValue(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		if v == float64(int(v)) {
			return int(v), true
		}
	}
	return 0, false
}

// SaveState writes the state atomically.
func SaveState(state *RunState) error {
	state.UpdatedAt = time.Now().UTC()
	return metadata.WriteJSONAtomic(state.StateFile, state, 0644)
}

// LoadState reads a saved run state.
func LoadState(runDir string) (*RunState, error) {
	data, err := os.ReadFile(filepath.Join(runDir, stateFileName))
	if err != nil {
		return nil, fmt.Errorf("jobs: read state: %w", err)
	}
	var state RunState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("jobs: parse state: %w", err)
	}
	if state.RunDir == "" {
		state.RunDir = runDir
	}
	if state.StateFile == "" {
		state.StateFile = filepath.Join(runDir, stateFileName)
	}
	return &state, nil
}

// ExistingStateHasRemoteTasks reports whether a run dir contains submitted tasks.
func ExistingStateHasRemoteTasks(runDir string) bool {
	state, err := LoadState(runDir)
	if err != nil {
		return false
	}
	for _, job := range state.Jobs {
		for _, request := range job.RemoteRequests {
			if request.TaskID != "" {
				return true
			}
		}
	}
	return false
}

// RefreshStatuses derives logical job statuses from remote request statuses.
func RefreshStatuses(state *RunState) {
	for i := range state.Jobs {
		state.Jobs[i].Status = summarizeJobStatus(state.Jobs[i].RemoteRequests)
	}
}

func summarizeJobStatus(requests []RemoteRequest) string {
	if len(requests) == 0 {
		return StatusPending
	}
	counts := map[string]int{}
	for _, request := range requests {
		counts[request.Status]++
	}
	if counts[StatusFailed] > 0 {
		return StatusFailed
	}
	if counts[StatusRunning] > 0 {
		return StatusRunning
	}
	if counts[StatusSubmitted] > 0 {
		return StatusSubmitted
	}
	if counts[StatusPending] > 0 {
		return StatusPending
	}
	if counts[StatusQuoted] > 0 {
		return StatusQuoted
	}
	if counts[StatusCompleted] == len(requests) || counts[StatusDownloaded] == len(requests) {
		return StatusCompleted
	}
	return StatusCompleted
}

// Summary returns a compact aggregate state.
func Summary(state *RunState) BatchSummary {
	RefreshStatuses(state)
	summary := BatchSummary{
		StatusCounts:       map[string]int{},
		RemoteStatusCounts: map[string]int{},
		Complete:           len(state.Jobs) > 0,
	}
	for _, job := range state.Jobs {
		summary.LogicalJobs++
		summary.RequestedOutputs += job.Outputs
		summary.StatusCounts[job.Status]++
		if job.Status == StatusFailed {
			summary.FailedJobs++
		}
		if !terminalStatus(job.Status) {
			summary.Complete = false
		}
		for _, request := range job.RemoteRequests {
			summary.RemoteRequests++
			summary.RemoteStatusCounts[request.Status]++
			if request.Quote == nil {
				summary.UnknownPricingRemoteRequests++
			} else {
				summary.TotalCredits += request.Quote.Price
				if request.Quote.Price > 0 {
					summary.PaidRemoteRequests++
				} else {
					summary.ZeroCreditRemoteRequests++
				}
			}
		}
	}
	return summary
}

func terminalStatus(status string) bool {
	return status == StatusCompleted || status == StatusDownloaded || status == StatusFailed || status == StatusSkipped
}

func activeStatus(status string) bool {
	return status == StatusSubmitted || status == StatusRunning
}

func failedWithoutTask(request RemoteRequest) bool {
	return request.Status == StatusFailed && request.TaskID == ""
}

func addRequestError(request *RemoteRequest, code, message string, details map[string]any) {
	request.Errors = append(request.Errors, JobError{Code: code, Message: message, Details: details})
	request.UpdatedAt = time.Now().UTC()
}

func lastRequestError(request RemoteRequest) *JobError {
	if len(request.Errors) == 0 {
		return nil
	}
	err := request.Errors[len(request.Errors)-1]
	return &err
}

func allRequests(state *RunState) []RemoteRequest {
	var requests []RemoteRequest
	for _, job := range state.Jobs {
		requests = append(requests, job.RemoteRequests...)
	}
	return requests
}

func requestSummaries(state *RunState) []RequestSummary {
	var out []RequestSummary
	for _, request := range allRequests(state) {
		out = append(out, summarizeRequest(request))
	}
	return out
}

func summarizeRequest(request RemoteRequest) RequestSummary {
	summary := RequestSummary{
		JobID:         request.JobID,
		Title:         request.Title,
		RequestID:     request.RequestID,
		Model:         request.Model,
		Mode:          request.Mode,
		OutputCount:   request.OutputCount,
		Status:        request.Status,
		TaskID:        request.TaskID,
		ArtifactCount: len(request.Artifacts),
		LastError:     lastRequestError(request),
	}
	if request.Quote != nil {
		summary.QuoteCredits = request.Quote.Price
	}
	if request.Task != nil {
		if status, _ := request.Task["status"].(string); status != "" {
			summary.RemoteStatus = status
		}
	}
	return summary
}
