// Package discovery fetches model lists and schemas from Lovart.
package discovery

import "context"

// ModelInfo is metadata about one generation model.
type ModelInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Type        string `json:"type"`
	VIP         bool   `json:"vip"`
}

// List returns known Lovart generator models.
func List(ctx context.Context, live bool) ([]ModelInfo, error) {
	// TODO: load from ref data or live API
	return nil, nil
}
