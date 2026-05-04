// Package discovery fetches model lists and schemas from Lovart.
package discovery

import (
	"context"
	"fmt"

	"github.com/aporicho/lovart/internal/http"
)

// ModelInfo is metadata about one generation model.
type ModelInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Type        string `json:"type"`
	VIP         bool   `json:"vip"`
}

// generatorListResponse mirrors the Lovart LGW API response envelope.
type generatorListResponse struct {
	Code    int `json:"code"`
	Message string `json:"message"`
	Data struct {
		Items []ModelInfo `json:"items"`
	} `json:"data"`
}

// List returns known Lovart generator models.
// When live is true, fetches from the LGW API; otherwise returns nil.
func List(ctx context.Context, client *http.Client, live bool) ([]ModelInfo, error) {
	if !live {
		return nil, nil
	}

	path := "/v1/generator/list?biz_type=16"

	var resp generatorListResponse
	if err := client.GetJSON(ctx, http.LGWBase, path, &resp); err != nil {
		return nil, fmt.Errorf("discovery: fetch generator list: %w", err)
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("discovery: LGW returned code %d: %s", resp.Code, resp.Message)
	}

	return resp.Data.Items, nil
}

// GetSchema fetches the OpenAPI schema for a model from the LGW API.
func GetSchema(ctx context.Context, client *http.Client, model string) (map[string]any, error) {
	path := "/v1/generator/schema?biz_type=16"

	var resp struct {
		Data map[string]any `json:"data"`
		Code int            `json:"code"`
	}
	if err := client.GetJSON(ctx, http.LGWBase, path, &resp); err != nil {
		return nil, fmt.Errorf("discovery: fetch schema: %w", err)
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("discovery: LGW schema returned code %d", resp.Code)
	}
	return resp.Data, nil
}
