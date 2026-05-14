package generation

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aporicho/lovart/internal/http"
)

// SlotError preserves Lovart's slot-gate response for caller classification.
type SlotError struct {
	Code    int
	Message string
}

func (e *SlotError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return fmt.Sprintf("generation: take slot returned code %d: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("generation: take slot returned code %d", e.Code)
}

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
		Code    int    `json:"code"`
		Message string `json:"message"`
		Msg     string `json:"msg"`
	}

	if err := client.PostJSON(ctx, http.WWWBase, path, body, &resp); err != nil {
		return fmt.Errorf("generation: take slot: %w", err)
	}

	if resp.Code != 0 {
		return &SlotError{Code: resp.Code, Message: firstNonEmpty(resp.Message, resp.Msg)}
	}

	return nil
}

// IsConcurrencyLimitError reports whether an error is Lovart's active task cap.
func IsConcurrencyLimitError(err error) bool {
	if err == nil {
		return false
	}
	var slotErr *SlotError
	if errors.As(err, &slotErr) {
		return containsConcurrencyMarker(slotErr.Message)
	}
	return containsConcurrencyMarker(err.Error())
}

func containsConcurrencyMarker(text string) bool {
	lower := strings.ToLower(text)
	for _, marker := range []string{
		"并发上限",
		"同时运行",
		"running tasks",
		"task concurrent",
		"concurrent",
		"concurrency",
		"parallel task",
		"task limit",
		"too many tasks",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
