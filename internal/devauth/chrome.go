package devauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/aporicho/lovart/internal/auth"
)

const discoveryBase = "http://127.0.0.1:%d"

// BrowserCapturer opens Chrome with DevTools enabled and captures Lovart auth.
type BrowserCapturer struct {
	HTTPClient *http.Client
	Runner     CommandRunner
}

// CommandRunner runs browser-management commands.
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) error
	Start(ctx context.Context, name string, args ...string) error
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args ...string) error {
	return exec.CommandContext(ctx, name, args...).Run()
}

func (execRunner) Start(ctx context.Context, name string, args ...string) error {
	return exec.CommandContext(ctx, name, args...).Start()
}

// Capture returns a complete-enough browser session for verification.
func (c BrowserCapturer) Capture(ctx context.Context, opts Options) (auth.Session, CaptureInfo, error) {
	if runtime.GOOS != "darwin" {
		return auth.Session{}, CaptureInfo{}, fmt.Errorf("dev auth: unsupported platform %s", runtime.GOOS)
	}
	runner := c.Runner
	if runner == nil {
		runner = execRunner{}
	}
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 2 * time.Second}
	}

	info := CaptureInfo{}
	if opts.RestartChrome {
		_ = runner.Run(ctx, "osascript", "-e", `quit app "Google Chrome"`)
		info.BrowserRestarted = true
		time.Sleep(1200 * time.Millisecond)
	}
	if err := runner.Start(ctx, "open", "-na", "Google Chrome", "--args",
		fmt.Sprintf("--remote-debugging-port=%d", opts.DebugPort),
		"--remote-allow-origins=*",
		lovartURL,
	); err != nil {
		return auth.Session{}, info, fmt.Errorf("dev auth: start Chrome: %w", err)
	}

	target, err := waitForLovartTarget(ctx, httpClient, opts.DebugPort)
	if err != nil {
		return auth.Session{}, info, err
	}
	session, err := captureTargetSession(ctx, target.WebSocketDebuggerURL)
	if err != nil {
		return auth.Session{}, info, err
	}
	return session, info, nil
}

type chromeTarget struct {
	ID                   string `json:"id"`
	Type                 string `json:"type"`
	URL                  string `json:"url"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

func waitForLovartTarget(ctx context.Context, client *http.Client, port int) (chromeTarget, error) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		target, err := lovartTarget(ctx, client, port)
		if err == nil && target.WebSocketDebuggerURL != "" {
			return target, nil
		}
		select {
		case <-ctx.Done():
			return chromeTarget{}, fmt.Errorf("dev auth: wait for Chrome DevTools target: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func lovartTarget(ctx context.Context, client *http.Client, port int) (chromeTarget, error) {
	targets, err := listTargets(ctx, client, port)
	if err != nil {
		return chromeTarget{}, err
	}
	for _, target := range targets {
		if target.Type == "page" && strings.Contains(target.URL, "lovart.ai") {
			return target, nil
		}
	}
	target, err := openTarget(ctx, client, port, lovartURL)
	if err != nil {
		return chromeTarget{}, err
	}
	return target, nil
}

func listTargets(ctx context.Context, client *http.Client, port int) ([]chromeTarget, error) {
	var targets []chromeTarget
	if err := getJSON(ctx, client, fmt.Sprintf(discoveryBase+"/json/list", port), &targets); err != nil {
		return nil, err
	}
	return targets, nil
}

func openTarget(ctx context.Context, client *http.Client, port int, targetURL string) (chromeTarget, error) {
	endpoint := fmt.Sprintf(discoveryBase+"/json/new?%s", port, url.QueryEscape(targetURL))
	var target chromeTarget
	if err := requestJSON(ctx, client, http.MethodGet, endpoint, &target); err == nil {
		return target, nil
	}
	if err := requestJSON(ctx, client, http.MethodPut, endpoint, &target); err != nil {
		return chromeTarget{}, err
	}
	return target, nil
}

func getJSON(ctx context.Context, client *http.Client, endpoint string, out any) error {
	return requestJSON(ctx, client, http.MethodGet, endpoint, out)
}

func requestJSON(ctx context.Context, client *http.Client, method, endpoint string, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("dev auth: Chrome DevTools %s returned %d", endpoint, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
