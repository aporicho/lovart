package config

import "github.com/aporicho/lovart/internal/registry"

// OutputCap describes a model's multi-output generation capability.
type OutputCap = registry.OutputCap

// OutputCapability returns the output capability for a model.
func OutputCapability(model string) *OutputCap {
	cap := registry.OutputCapability(model)
	return &cap
}
