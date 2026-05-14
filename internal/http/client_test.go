package http

import (
	"context"
	"io"
	nethttp "net/http"
	"strings"
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

func TestClientPrefersCookieAuthHints(t *testing.T) {
	transport := &captureTransport{}
	client := NewClient(&auth.Credentials{
		Cookie: "foo=bar; usertoken=cookie-token; webid=web-123",
		Token:  "header-token",
	}, nil)
	client.http = &nethttp.Client{Transport: transport}

	resp, err := client.Do(context.Background(), nethttp.MethodGet, "https://www.lovart.ai", "/check", nil)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	resp.Body.Close()

	if got := transport.request.Header.Get("token"); got != "cookie-token" {
		t.Fatalf("token header = %q, want cookie token", got)
	}
	if got := transport.request.Header.Get("webid"); got != "web-123" {
		t.Fatalf("webid header = %q, want cookie webid", got)
	}
}

func TestClientUserUUIDComesFromCookieWebID(t *testing.T) {
	client := NewClient(&auth.Credentials{
		Cookie: "foo=bar; usertoken=cookie-token; webid=web-123",
	}, nil)
	if got := client.UserUUID(); got != "web-123" {
		t.Fatalf("UserUUID = %q, want cookie webid", got)
	}
}

func TestSyncTimeUnauthorizedHasActionableError(t *testing.T) {
	client := NewClient(&auth.Credentials{Cookie: "foo=bar", Token: "token"}, nil)
	client.http = &nethttp.Client{Transport: roundTripFunc(func(req *nethttp.Request) (*nethttp.Response, error) {
		return &nethttp.Response{
			StatusCode: nethttp.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"code":401,"msg":"未登录","data":null}`)),
			Header:     make(nethttp.Header),
		}, nil
	})}

	err := client.SyncTime(context.Background())
	if err == nil {
		t.Fatal("SyncTime succeeded, want unauthorized error")
	}
	if !strings.Contains(err.Error(), "token/cookie mismatch") || !strings.Contains(err.Error(), "lovart auth login") {
		t.Fatalf("unauthorized error is not actionable: %v", err)
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

type roundTripFunc func(*nethttp.Request) (*nethttp.Response, error)

func (f roundTripFunc) RoundTrip(req *nethttp.Request) (*nethttp.Response, error) {
	return f(req)
}
