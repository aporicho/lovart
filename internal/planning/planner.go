// Package planning generates non-submitting quality/cost/speed routes.
package planning

// Route is one configuration path for generation.
type Route struct {
	ID                 string   `json:"id"`
	Model              string   `json:"model"`
	Mode               string   `json:"mode"`
	BodyPatch          map[string]any `json:"body_patch,omitempty"`
	ZeroCredit         bool     `json:"zero_credit"`
	RequiresPaid       bool     `json:"requires_paid_confirmation"`
	QualityScore       float64  `json:"quality_score"`
	CostScore          float64  `json:"cost_score"`
	SpeedScore         float64  `json:"speed_score"`
}

// PlanResult contains quality_best, cost_best, and speed_best routes.
type PlanResult struct {
	QualityBest *Route   `json:"quality_best,omitempty"`
	CostBest    *Route   `json:"cost_best,omitempty"`
	SpeedBest   *Route   `json:"speed_best,omitempty"`
	Candidates  []Route  `json:"candidates,omitempty"`
}

// Plan generates route candidates without submitting.
func Plan(model *string, intent string, body map[string]any) (*PlanResult, error) {
	// TODO: discover models, enumerate config candidates, quote each
	return &PlanResult{}, nil
}
