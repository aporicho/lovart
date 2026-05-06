package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLoginServerAcceptsValidStateOnce(t *testing.T) {
	server := &LoginServer{
		state:     "state-123",
		expiresAt: time.Now().Add(time.Minute),
		result:    make(chan Session, 1),
		cancelled: make(chan struct{}, 1),
	}

	body := map[string]any{"state": "bad", "cookie": "c"}
	data, _ := json.Marshal(body)
	resp := callLoginComplete(server, data)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("invalid state status = %d, want 403", resp.Code)
	}

	body["state"] = server.State()
	data, _ = json.Marshal(body)
	resp = callLoginComplete(server, data)
	if resp.Code != http.StatusOK {
		t.Fatalf("valid state status = %d, want 200", resp.Code)
	}

	select {
	case session := <-server.Result():
		if session.Cookie != "c" {
			t.Fatalf("session = %#v", session)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for login result")
	}

	resp = callLoginComplete(server, data)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("reused state status = %d, want 403", resp.Code)
	}
}

func callLoginComplete(server *LoginServer, data []byte) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/lovart/auth/complete", bytes.NewReader(data))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.handleComplete(recorder, request)
	return recorder
}
