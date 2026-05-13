package auth

import (
	"context"
	"strconv"
	"time"

	lovarterrors "github.com/aporicho/lovart/internal/errors"
)

// BrowserOpenFunc opens a browser URL for the login flow.
type BrowserOpenFunc func(string) error

// BrowserLoginOptions controls the browser-extension auth login flow.
type BrowserLoginOptions struct {
	Timeout              time.Duration
	Ports                []int
	OpenBrowser          BrowserOpenFunc
	RequireBrowserOpened bool
	BeforeOpen           func(BrowserLoginResult)
	OnOpenError          func(BrowserLoginResult)
}

// BrowserLoginResult contains only safe login metadata.
type BrowserLoginResult struct {
	Authenticated bool     `json:"authenticated"`
	Pending       bool     `json:"pending,omitempty"`
	Status        Status   `json:"status"`
	LoginURL      string   `json:"login_url,omitempty"`
	CallbackPort  int      `json:"callback_port"`
	ExpiresAt     string   `json:"expires_at"`
	OpenedBrowser bool     `json:"opened_browser"`
	OpenError     string   `json:"open_error,omitempty"`
	NextSteps     []string `json:"next_steps"`
}

// RunBrowserExtensionLogin starts a local callback, opens Lovart, waits for the
// connector extension callback, and saves the resulting session.
func RunBrowserExtensionLogin(ctx context.Context, opts BrowserLoginOptions) (BrowserLoginResult, error) {
	server, cancel, done, result, err := openBrowserLogin(ctx, opts)
	if err != nil {
		return result, err
	}
	defer cancel()
	defer func() {
		closeCtx, closeCancel := context.WithTimeout(context.Background(), time.Second)
		defer closeCancel()
		_ = server.Close(closeCtx)
	}()

	select {
	case session := <-server.Result():
		session.Source = LoginSourceBrowserExtension
		if err := SaveSession(session); err != nil {
			return result, lovarterrors.Internal("save auth session", map[string]any{"error": err.Error()})
		}
		result.Authenticated = true
		result.Status = GetStatus()
		return result, nil
	case <-server.Cancelled():
		return result, lovarterrors.InputError("auth login cancelled", nil)
	case <-done:
		return result, lovarterrors.New(lovarterrors.CodeTimeout, "auth login timed out", map[string]any{
			"recommended_actions": []string{"rerun `lovart auth login`", "run `lovart dev auth-login` for developer browser capture"},
		})
	}
}

// StartBrowserExtensionLogin starts a local callback and returns immediately
// with a browser URL. The callback continues in the background until the
// extension completes, cancels, or the timeout expires.
func StartBrowserExtensionLogin(ctx context.Context, opts BrowserLoginOptions) (BrowserLoginResult, error) {
	server, cancel, done, result, err := openBrowserLogin(ctx, opts)
	if err != nil {
		return result, err
	}
	result.Pending = true
	result.NextSteps = []string{
		"open login_url in Chrome with Lovart Connector loaded",
		"click Connect in the Lovart Connector page prompt",
		"call lovart_auth_status to confirm authenticated=true",
	}

	go func() {
		defer cancel()
		defer func() {
			closeCtx, closeCancel := context.WithTimeout(context.Background(), time.Second)
			defer closeCancel()
			_ = server.Close(closeCtx)
		}()

		select {
		case session := <-server.Result():
			session.Source = LoginSourceBrowserExtension
			_ = SaveSession(session)
		case <-server.Cancelled():
		case <-done:
		}
	}()

	return result, nil
}

func openBrowserLogin(ctx context.Context, opts BrowserLoginOptions) (*LoginServer, context.CancelFunc, <-chan struct{}, BrowserLoginResult, error) {
	if opts.Timeout <= 0 {
		return nil, nil, nil, BrowserLoginResult{}, lovarterrors.InputError("timeout must be positive", nil)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	loginCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	server, err := StartLoginServer(loginCtx, LoginServerOptions{Ports: opts.Ports, Timeout: opts.Timeout})
	if err != nil {
		cancel()
		return nil, nil, nil, BrowserLoginResult{}, lovarterrors.Internal("start auth login server", map[string]any{"error": err.Error()})
	}

	loginURL := "https://www.lovart.ai/?lovart_cli_auth=1&port=" + strconv.Itoa(server.Port())
	result := BrowserLoginResult{
		LoginURL:     loginURL,
		CallbackPort: server.Port(),
		ExpiresAt:    server.ExpiresAt().Format(time.RFC3339),
		Status:       GetStatus(),
		NextSteps: []string{
			"lovart doctor",
			"lovart project list",
			"lovart project select <project_id>",
		},
	}
	if opts.OpenBrowser != nil {
		if opts.BeforeOpen != nil {
			opts.BeforeOpen(result)
		}
		if err := opts.OpenBrowser(loginURL); err != nil {
			result.OpenError = err.Error()
			if opts.OnOpenError != nil {
				opts.OnOpenError(result)
			}
			if opts.RequireBrowserOpened {
				closeCtx, closeCancel := context.WithTimeout(context.Background(), time.Second)
				defer closeCancel()
				_ = server.Close(closeCtx)
				cancel()
				return nil, nil, nil, result, lovarterrors.InputError("open browser for auth login", map[string]any{
					"error":     err.Error(),
					"login_url": loginURL,
				})
			}
		} else {
			result.OpenedBrowser = true
		}
	}
	return server, cancel, loginCtx.Done(), result, nil
}
