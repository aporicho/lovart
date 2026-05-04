package generation

import (
	"context"
	"fmt"

	"github.com/aporicho/lovart/internal/http"
)

// TakeSlot reserves a generation slot for the given project and client.
// Must be called before Submit. Requires project_id and cid.
func TakeSlot(ctx context.Context, client *http.Client, projectID, cid string) error {
	if projectID == "" || cid == "" {
		return fmt.Errorf("generation: take slot requires project_id and cid")
	}

	path := "/api/canva/agent-cashier/task/take/slot"
	body := map[string]any{
		"project_id": projectID,
		"cid":        cid,
	}

	var resp struct {
		Code int `json:"code"`
	}

	if err := client.PostJSON(ctx, http.WWWBase, path, body, &resp); err != nil {
		return fmt.Errorf("generation: take slot: %w", err)
	}

	if resp.Code != 0 {
		return fmt.Errorf("generation: take slot returned code %d", resp.Code)
	}

	return nil
}
