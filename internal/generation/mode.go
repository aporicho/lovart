package generation

import (
	"context"
	"fmt"

	"github.com/aporicho/lovart/internal/http"
)

// SetMode switches generation mode between fast and relax.
// mode must be "fast" or "relax". "auto" is a no-op.
func SetMode(ctx context.Context, client *http.Client, cid, mode string) error {
	if mode == "auto" || mode == "" {
		return nil
	}

	unlimited := mode == "relax"
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
