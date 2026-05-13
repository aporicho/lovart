package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	nethttp "net/http"
	"time"
)

const timeSyncURL = "https://www.lovart.ai/api/www/lovart/time/utc/timestamp"

// SyncTime synchronizes the local clock with the Lovart server using the
// client's credentials (cookie + token required).
func (c *Client) SyncTime(ctx context.Context) error {
	before := time.Now().UnixMilli()

	req, err := nethttp.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s?_t=%d", timeSyncURL, before), nil)
	if err != nil {
		return fmt.Errorf("http: time sync request: %w", err)
	}

	for k, v := range defaultHeaders {
		req.Header.Set(k, v)
	}
	c.setAuthHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("http: time sync request: %w", err)
	}
	defer resp.Body.Close()

	after := time.Now().UnixMilli()
	rtt := after - before

	data, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return fmt.Errorf("http: read time sync response: %w", err)
	}
	if resp.StatusCode == nethttp.StatusUnauthorized {
		return fmt.Errorf("http: time sync unauthorized (HTTP 401): stored Lovart credentials are stale or token/cookie mismatch; run `lovart auth login` (body: %.200s)", string(data))
	}

	var result struct {
		Code int `json:"code"`
		Data struct {
			Timestamp string `json:"timestamp"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("http: parse time sync response: %w (body: %.200s)", err, string(data))
	}

	if result.Code != 0 {
		if result.Code == nethttp.StatusUnauthorized {
			return fmt.Errorf("http: time sync unauthorized (code 401): stored Lovart credentials are stale or token/cookie mismatch; run `lovart auth login` (body: %.200s)", string(data))
		}
		return fmt.Errorf("http: time sync returned code %d (body: %.200s)", result.Code, string(data))
	}

	tsStr := result.Data.Timestamp
	if tsStr == "" || tsStr == "0" {
		return fmt.Errorf("http: time sync returned zero/empty timestamp (body: %.200s)", string(data))
	}

	var serverTS int64
	if _, scanErr := fmt.Sscanf(tsStr, "%d", &serverTS); scanErr != nil {
		return fmt.Errorf("http: parse timestamp %q: %w", tsStr, scanErr)
	}

	c.offsetMS = serverTS - (before + rtt/2)
	return nil
}

// OffsetMS returns the current server time offset in milliseconds.
func (c *Client) OffsetMS() int64 {
	return c.offsetMS
}
