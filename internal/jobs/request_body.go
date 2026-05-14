package jobs

import "github.com/aporicho/lovart/internal/registry"

func normalizeJobLine(job JobLine) (JobLine, error) {
	body, err := registry.NormalizeRequest(job.Model, job.Body)
	if err != nil {
		return JobLine{}, err
	}
	job.Body = body
	return job, nil
}

func normalizeRemoteRequestBody(request RemoteRequest) (map[string]any, error) {
	return registry.NormalizeRequest(request.Model, request.Body)
}

func requestEffectiveBody(request RemoteRequest) map[string]any {
	if len(request.NormalizedBody) > 0 {
		return request.NormalizedBody
	}
	return request.Body
}
