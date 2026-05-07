package devauth

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aporicho/lovart/internal/auth"
)

func captureTargetSession(ctx context.Context, wsURL string) (auth.Session, error) {
	client, err := newCDPClient(ctx, wsURL)
	if err != nil {
		return auth.Session{}, err
	}
	defer client.Close()

	for _, method := range []string{"Network.enable", "Runtime.enable", "Page.enable"} {
		if _, err := client.Call(ctx, method, map[string]any{}); err != nil {
			return auth.Session{}, fmt.Errorf("dev auth: %s: %w", method, err)
		}
	}
	_, _ = client.Call(ctx, "Page.navigate", map[string]any{"url": lovartURL})

	session := auth.Session{}
	ticker := time.NewTicker(750 * time.Millisecond)
	defer ticker.Stop()
	for {
		if session.Cookie != "" && session.CID != "" {
			return session, nil
		}
		select {
		case <-ctx.Done():
			return auth.Session{}, fmt.Errorf("dev auth: capture browser session: %w", ctx.Err())
		case event, ok := <-client.Events():
			if !ok {
				return auth.Session{}, fmt.Errorf("dev auth: Chrome DevTools connection closed")
			}
			applyEventHints(event, &session)
		case <-ticker.C:
			cookieHeader, err := browserCookieHeader(ctx, client)
			if err == nil && cookieHeader != "" {
				session.Cookie = cookieHeader
			}
			hints, err := browserStorageHints(ctx, client)
			if err == nil {
				mergeHints(&session, hints)
			}
		}
	}
}

func applyEventHints(event cdpEvent, session *auth.Session) {
	if event.Method != "Network.requestWillBeSent" && event.Method != "Network.requestWillBeSentExtraInfo" {
		return
	}
	var params struct {
		Request struct {
			URL     string         `json:"url"`
			Headers map[string]any `json:"headers"`
		} `json:"request"`
		Headers map[string]any `json:"headers"`
	}
	if err := json.Unmarshal(event.Params, &params); err != nil {
		return
	}
	if params.Request.URL != "" && !strings.Contains(params.Request.URL, "lovart.ai") {
		return
	}
	mergeHeaderHints(session, params.Request.Headers)
	mergeHeaderHints(session, params.Headers)
}

func mergeHeaderHints(session *auth.Session, headers map[string]any) {
	for key, raw := range headers {
		value, _ := raw.(string)
		if value == "" {
			continue
		}
		switch strings.ToLower(key) {
		case "token", "authorization", "x-auth-token", "x-access-token":
			if session.Token == "" {
				session.Token = value
			}
		case "x-csrf-token", "x-xsrf-token", "csrf-token":
			if session.CSRF == "" {
				session.CSRF = value
			}
		}
	}
}

func browserCookieHeader(ctx context.Context, client *cdpClient) (string, error) {
	raw, err := client.Call(ctx, "Network.getCookies", map[string]any{
		"urls": []string{lovartURL},
	})
	if err != nil {
		return "", err
	}
	var result struct {
		Cookies []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"cookies"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", err
	}
	parts := make([]string, 0, len(result.Cookies))
	for _, cookie := range result.Cookies {
		if cookie.Name != "" {
			parts = append(parts, cookie.Name+"="+cookie.Value)
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, "; "), nil
}

func browserStorageHints(ctx context.Context, client *cdpClient) (map[string]string, error) {
	raw, err := client.Call(ctx, "Runtime.evaluate", map[string]any{
		"expression":    storageHintExpression,
		"returnByValue": true,
	})
	if err != nil {
		return nil, err
	}
	var result struct {
		Result struct {
			Value map[string]string `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result.Result.Value, nil
}

func mergeHints(session *auth.Session, hints map[string]string) {
	if session.Token == "" {
		session.Token = hints["token"]
	}
	if session.CSRF == "" {
		session.CSRF = hints["csrf"]
	}
	if session.ProjectID == "" {
		session.ProjectID = hints["project_id"]
	}
	if session.CID == "" {
		session.CID = firstNonEmptyHint(hints["cid"], hints["webid"], hints["web_id"])
	}
}

func firstNonEmptyHint(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

const storageHintExpression = `(() => {
  const hints = {};
  const put = (name, value) => {
    if (!value || hints[name]) return;
    hints[name] = String(value);
  };
  const inspect = (key, value) => {
    const lower = String(key || "").toLowerCase();
    if (lower === "token" || lower.includes("auth_token") || lower.includes("access_token")) put("token", value);
    if (lower.includes("csrf") || lower.includes("xsrf")) put("csrf", value);
    if (lower === "project_id" || lower === "projectid" || lower.includes("project_id")) put("project_id", value);
    if (lower === "cid") put("cid", value);
    if (lower === "webid") put("webid", value);
    if (lower === "web_id") put("web_id", value);
  };
  const walk = (value) => {
    if (!value || typeof value !== "object") return;
    if (Array.isArray(value)) {
      value.forEach(walk);
      return;
    }
    Object.keys(value).forEach((key) => {
      const item = value[key];
      if (typeof item === "string") inspect(key, item);
      else walk(item);
    });
  };
  const scan = (storage) => {
    if (!storage) return;
    for (let i = 0; i < storage.length; i += 1) {
      const key = storage.key(i) || "";
      const value = storage.getItem(key) || "";
      inspect(key, value);
      if (value && value.length < 50000) {
        try { walk(JSON.parse(value)); } catch (_) {}
      }
    }
  };
  scan(window.localStorage);
  scan(window.sessionStorage);
  return hints;
})()`
