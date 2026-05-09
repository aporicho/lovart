package generation

import (
	"context"
	"fmt"

	"github.com/aporicho/lovart/internal/http"
	"github.com/aporicho/lovart/internal/pricing"
)

// SetMode switches generation mode between fast and relax.
// mode must be "fast" or "relax". "auto" is a no-op.
func SetMode(ctx context.Context, client *http.Client, cid, mode string) error {
	mode, err := pricing.NormalizeMode(mode)
	if err != nil {
		return fmt.Errorf("generation: set mode: %w", err)
	}
	if mode == pricing.ModeAuto {
		return nil
	}

	unlimited := mode == pricing.ModeRelax
	path := "/api/canva/agent-cashier/task/set/unlimited"
	body := map[string]any{
		"unlimited": unlimited,
		"cid":       cid,
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
	}

	if err := client.PostJSON(ctx, http.WWWBase, path, body, &resp); err != nil {
		return fmt.Errorf("generation: set mode: %w", err)
	}

	if resp.Code != 0 {
		return fmt.Errorf("generation: set mode returned code %d", resp.Code)
	}

	return nil
}
