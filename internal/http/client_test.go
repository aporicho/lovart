package http

import (
	"context"
	"io"
	nethttp "net/http"
	"testing"

	"github.com/aporicho/lovart/internal/auth"
)

func TestClientSetsAuthHeaders(t *testing.T) {
	transport := &captureTransport{}
	client := NewClient(&auth.Credentials{
		Cookie: "foo=bar",
		Token:  "secret-token",
		WebID:  "web-123",
	}, nil)
	client.http = &nethttp.Client{Transport: transport}

	resp, err := client.Do(context.Background(), nethttp.MethodGet, "https://www.lovart.ai", "/check", nil)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	resp.Body.Close()

	if got := transport.request.Header.Get("Cookie"); got != "foo=bar" {
		t.Fatalf("Cookie header = %q", got)
	}
	if got := transport.request.Header.Get("token"); got != "secret-token" {
		t.Fatalf("token header = %q", got)
	}
	if got := transport.request.Header.Get("webid"); got != "web-123" {
		t.Fatalf("webid header = %q", got)
	}
}

type captureTransport struct {
	request *nethttp.Request
}

func (t *captureTransport) RoundTrip(req *nethttp.Request) (*nethttp.Response, error) {
	t.request = req
	return &nethttp.Response{
		StatusCode: nethttp.StatusNoContent,
		Body:       io.NopCloser(nil),
		Header:     make(nethttp.Header),
	}, nil
}
