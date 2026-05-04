// Package http provides a signed Lovart HTTP client.
package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	nethttp "net/http"
	"time"

	"github.com/aporicho/lovart/internal/auth"
	"github.com/aporicho/lovart/internal/signing"
)

// Client wraps net/http.Client with Lovart auth and signing.
type Client struct {
	http     *nethttp.Client
	creds    *auth.Credentials
	signer   signing.Signer
	offsetMS int64 // server time offset, populated by SyncTime
}

// NewClient creates a Client with Lovart credentials and signer.
func NewClient(creds *auth.Credentials, signer signing.Signer) *Client {
	return &Client{
		http:   &nethttp.Client{Timeout: 120 * time.Second},
		creds:  creds,
		signer: signer,
	}
}

// SetSigner replaces the signer used by this client.
func (c *Client) SetSigner(s signing.Signer) {
	c.signer = s
}

// Do sends a signed request to the given base URL and path.
func (c *Client) Do(ctx context.Context, method, base, path string, body any) (*nethttp.Response, error) {
	url := base + path

	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("http: marshal body: %w", err)
		}
	}

	req, err := nethttp.NewRequestWithContext(ctx, method, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("http: new request: %w", err)
	}

	for k, v := range defaultHeaders {
		req.Header.Set(k, v)
	}

	c.setAuthHeaders(req)
	c.setSignatureHeaders(ctx, req)

	return c.http.Do(req)
}

// PostJSON sends a signed POST and decodes the JSON response into result.
func (c *Client) PostJSON(ctx context.Context, base, path string, body, result any) error {
	resp, err := c.Do(ctx, "POST", base, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if resp.StatusCode >= 400 {
		return fmt.Errorf("http: POST %s returned %d: %s", path, resp.StatusCode, truncateBytes(data, 500))
	}
	if result != nil {
		return json.Unmarshal(data, result)
	}
	return nil
}

// GetJSON sends a signed GET and decodes the JSON response.
func (c *Client) GetJSON(ctx context.Context, base, path string, result any) error {
	resp, err := c.Do(ctx, "GET", base, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if resp.StatusCode >= 400 {
		return fmt.Errorf("http: GET %s returned %d: %s", path, resp.StatusCode, truncateBytes(data, 500))
	}
	return json.Unmarshal(data, result)
}

func (c *Client) setAuthHeaders(req *nethttp.Request) {
	if c.creds == nil {
		return
	}
	if c.creds.Cookie != "" {
		req.Header.Set("Cookie", c.creds.Cookie)
	}
	if c.creds.Token != "" {
		req.Header.Set("token", c.creds.Token)
	}
	if c.creds.CSRF != "" {
		req.Header.Set("X-CSRF-Token", c.creds.CSRF)
	}
}

func (c *Client) setSignatureHeaders(ctx context.Context, req *nethttp.Request) {
	if c.signer == nil {
		return
	}
	timestamp := signing.TimestampNowMS(c.offsetMS)
	reqUUID := signing.RandomHex(32)

	result, err := c.signer.Sign(ctx, signing.SigningPayload{
		Timestamp: timestamp,
		ReqUUID:   reqUUID,
	})
	if err != nil {
		return
	}
	for k, v := range result.Headers() {
		req.Header.Set(k, v)
	}
}

func truncateBytes(b []byte, max int) string {
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "..."
}
