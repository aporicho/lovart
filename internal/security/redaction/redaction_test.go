package redaction

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestRedactStringRemovesSensitiveFragments(t *testing.T) {
	secrets := []string{
		"bearer-secret-123",
		"cookie-secret-123",
		"set-cookie-secret-123",
		"token-secret-123",
		"csrf-secret-123",
		"xsrf-secret-123",
		"user-token-secret-123",
		"session-secret-123",
	}
	input := strings.Join([]string{
		"Authorization: Bearer bearer-secret-123",
		"Cookie: lovart_session=cookie-secret-123; theme=light",
		"Set-Cookie: lovart_session=set-cookie-secret-123; HttpOnly",
		"api_token=token-secret-123",
		"csrf=csrf-secret-123",
		"xsrfToken=xsrf-secret-123",
		"https://example.test/callback?usertoken=user-token-secret-123&next=/",
		"session_id=session-secret-123",
	}, "\n")

	redacted := RedactString(input)
	assertNoSecrets(t, redacted, secrets...)
	if !strings.Contains(redacted, Replacement) {
		t.Fatalf("redacted string does not contain replacement marker: %q", redacted)
	}
}

func TestRedactErrorRemovesSensitiveFragments(t *testing.T) {
	secret := "error-token-secret-123"
	redacted := RedactError(errors.New("request failed: Authorization: Bearer " + secret))
	if redacted == nil {
		t.Fatal("expected redacted error")
	}
	assertNoSecrets(t, redacted.Error(), secret)
}

func TestRedactValueRecursesThroughMapsAndSlices(t *testing.T) {
	secrets := []string{
		"header-cookie-secret-123",
		"nested-token-secret-123",
		"slice-session-secret-123",
		"query-user-token-secret-123",
	}
	input := map[string]any{
		"headers": map[string]any{
			"Set-Cookie": "session=header-cookie-secret-123",
		},
		"items": []any{
			map[string]any{"token": "nested-token-secret-123"},
			"session=slice-session-secret-123",
			"https://example.test/?usertoken=query-user-token-secret-123",
		},
	}

	redacted := RedactValue(input)
	assertNoSecrets(t, fmt.Sprintf("%v", redacted), secrets...)
}

func TestRedactMapRedactsSensitiveKeys(t *testing.T) {
	secret := "direct-token-secret-123"
	redacted := RedactMap(map[string]any{"accessToken": secret})
	assertNoSecrets(t, fmt.Sprintf("%v", redacted), secret)
	if redacted["accessToken"] != Replacement {
		t.Fatalf("expected sensitive key value to be replaced, got %#v", redacted["accessToken"])
	}
}

func assertNoSecrets(t *testing.T, value string, secrets ...string) {
	t.Helper()
	for _, secret := range secrets {
		if strings.Contains(value, secret) {
			t.Fatalf("secret %q leaked in %q", secret, value)
		}
	}
}
