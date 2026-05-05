package jobs

import (
	"context"

	"github.com/aporicho/lovart/internal/generation"
	"github.com/aporicho/lovart/internal/http"
	"github.com/aporicho/lovart/internal/pricing"
	"github.com/aporicho/lovart/internal/project"
)

// RemoteClient is the jobs package boundary for Lovart network operations.
type RemoteClient interface {
	Quote(ctx context.Context, model string, body map[string]any) (*pricing.QuoteResult, error)
	Submit(ctx context.Context, model string, body map[string]any, opts generation.Options) (*generation.SubmitResult, error)
	FetchTask(ctx context.Context, taskID string) (map[string]any, error)
	AddToCanvas(ctx context.Context, projectID, cid string, images []project.CanvasImage) error
	AddBatchToCanvas(ctx context.Context, projectID, cid string, batch project.CanvasBatch) error
}

type httpRemoteClient struct {
	client *http.Client
}

// NewHTTPRemoteClient adapts the signed HTTP client to batch jobs.
func NewHTTPRemoteClient(client *http.Client) RemoteClient {
	return &httpRemoteClient{client: client}
}

func (c *httpRemoteClient) Quote(ctx context.Context, model string, body map[string]any) (*pricing.QuoteResult, error) {
	return pricing.Quote(ctx, c.client, model, body)
}

func (c *httpRemoteClient) Submit(ctx context.Context, model string, body map[string]any, opts generation.Options) (*generation.SubmitResult, error) {
	return generation.Submit(ctx, c.client, model, body, opts)
}

func (c *httpRemoteClient) FetchTask(ctx context.Context, taskID string) (map[string]any, error) {
	return generation.FetchTask(ctx, c.client, taskID)
}

func (c *httpRemoteClient) AddToCanvas(ctx context.Context, projectID, cid string, images []project.CanvasImage) error {
	return project.AddToCanvas(ctx, c.client, projectID, cid, images)
}

func (c *httpRemoteClient) AddBatchToCanvas(ctx context.Context, projectID, cid string, batch project.CanvasBatch) error {
	return project.AddBatchToCanvas(ctx, c.client, projectID, cid, batch)
}
