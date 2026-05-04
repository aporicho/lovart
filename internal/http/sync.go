package http

import (
	"context"
	"fmt"

	"github.com/aporicho/lovart/internal/signing"
)

// SyncTime synchronizes the local clock with the Lovart server.
// Must be called once before making signed requests.
func (c *Client) SyncTime(ctx context.Context) error {
	offset, err := signing.SyncTime()
	if err != nil {
		return fmt.Errorf("http: time sync: %w", err)
	}
	c.offsetMS = offset
	return nil
}

// OffsetMS returns the current server time offset in milliseconds.
func (c *Client) OffsetMS() int64 {
	return c.offsetMS
}
