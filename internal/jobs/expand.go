package jobs

import (
	"math"

	"github.com/aporicho/lovart/internal/config"
)

// SubRequest represents one atomic Lovart generation API call.
type SubRequest struct {
	Index   int            // 1-based index within the parent job
	N       int            // number of images for this sub-request
	Body    map[string]any // generation parameters (with n/max_images injected)
}

// Expand converts a job's desired output count into a list of atomic sub-requests.
func Expand(model string, outputs int, body map[string]any) ([]SubRequest, error) {
	if outputs <= 0 {
		return nil, nil
	}

	cap := config.OutputCapability(model)

	// Case A: Multi-output model (has n/max_images/num_images field).
	if cap.MultiField != "" {
		return expandMulti(model, outputs, body, cap)
	}

	// Case B: Fixed batch model (e.g., Midjourney outputs 4 per call).
	if cap.IsFixedBatch {
		return expandFixed(outputs, cap.BatchSize, body)
	}

	// Case C: Pure single-image model.
	return expandSingle(outputs, body)
}

// expandMulti handles models with a multi-output field.
func expandMulti(model string, outputs int, body map[string]any, cap *config.OutputCap) ([]SubRequest, error) {
	max := cap.MaxOutputs
	if max <= 0 {
		max = 10 // sensible default if schema doesn't specify
	}

	var subs []SubRequest
	remaining := outputs
	idx := 1

	for remaining > 0 {
		n := remaining
		if n > max {
			n = max
		}

		subBody := copyBody(body)
		subBody[cap.MultiField] = n

		subs = append(subs, SubRequest{
			Index: idx,
			N:     n,
			Body:  subBody,
		})

		remaining -= n
		idx++
	}

	return subs, nil
}

// expandFixed handles fixed-batch models (each API call produces batchSize images).
func expandFixed(outputs int, batchSize int, body map[string]any) ([]SubRequest, error) {
	calls := int(math.Ceil(float64(outputs) / float64(batchSize)))

	var subs []SubRequest
	for i := 0; i < calls; i++ {
		subs = append(subs, SubRequest{
			Index: i + 1,
			N:     1,
			Body:  copyBody(body),
		})
	}
	return subs, nil
}

// expandSingle handles pure single-image models (each API call = 1 image).
func expandSingle(outputs int, body map[string]any) ([]SubRequest, error) {
	var subs []SubRequest
	for i := 0; i < outputs; i++ {
		subs = append(subs, SubRequest{
			Index: i + 1,
			N:     1,
			Body:  copyBody(body),
		})
	}
	return subs, nil
}

// copyBody creates a shallow copy of a body map.
func copyBody(body map[string]any) map[string]any {
	out := make(map[string]any, len(body))
	for k, v := range body {
		out[k] = v
	}
	return out
}
