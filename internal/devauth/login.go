// Package devauth implements the developer browser-capture login flow.
package devauth

import (
	"context"
	"fmt"
	"time"

	"github.com/aporicho/lovart/internal/auth"
	lovarthttp "github.com/aporicho/lovart/internal/http"
	"github.com/aporicho/lovart/internal/project"
	"github.com/aporicho/lovart/internal/signing"
)

const (
	DefaultDebugPort = 47831
	DefaultTimeout   = 90 * time.Second
	lovartURL        = "https://www.lovart.ai/"
)

// Options configures developer auth capture.
type Options struct {
	Timeout       time.Duration
	DebugPort     int
	RestartChrome bool
}

// Result is the safe JSON payload returned by the developer auth command.
type Result struct {
	Authenticated        bool     `json:"authenticated"`
	Source               string   `json:"source"`
	ProjectID            string   `json:"project_id"`
	ProjectName          string   `json:"project_name,omitempty"`
	ProjectContextReady  bool     `json:"project_context_ready"`
	BrowserRestarted     bool     `json:"browser_restarted"`
	ProjectCount         int      `json:"project_count"`
	RecommendedNextSteps []string `json:"recommended_next_steps"`
}

// CaptureInfo reports safe browser-capture metadata.
type CaptureInfo struct {
	BrowserRestarted bool
}

// VerifyInfo reports safe verification metadata.
type VerifyInfo struct {
	ProjectName  string
	ProjectCount int
}

// Capturer obtains a browser session without exposing it to stdout.
type Capturer interface {
	Capture(ctx context.Context, opts Options) (auth.Session, CaptureInfo, error)
}

// Verifier completes and validates a captured session.
type Verifier interface {
	Verify(ctx context.Context, session auth.Session) (auth.Session, VerifyInfo, error)
}

// Run captures, validates, and persists developer browser auth.
func Run(ctx context.Context, opts Options) (Result, error) {
	return runWith(ctx, opts, BrowserCapturer{}, ProjectVerifier{})
}

func runWith(ctx context.Context, opts Options, capturer Capturer, verifier Verifier) (Result, error) {
	opts = normalizeOptions(opts)
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	session, captureInfo, err := capturer.Capture(ctx, opts)
	if err != nil {
		return Result{}, err
	}
	session.Source = auth.LoginSourceDevBrowserCapture

	completed, verifyInfo, err := verifier.Verify(ctx, session)
	if err != nil {
		return Result{}, err
	}
	completed.Source = auth.LoginSourceDevBrowserCapture
	if completed.ProjectID == "" || completed.CID == "" {
		return Result{}, fmt.Errorf("dev auth: project context incomplete after verification")
	}
	if err := auth.SaveSession(completed); err != nil {
		return Result{}, fmt.Errorf("dev auth: save session: %w", err)
	}

	return Result{
		Authenticated:       true,
		Source:              completed.Source,
		ProjectID:           completed.ProjectID,
		ProjectName:         verifyInfo.ProjectName,
		ProjectContextReady: true,
		BrowserRestarted:    captureInfo.BrowserRestarted,
		ProjectCount:        verifyInfo.ProjectCount,
		RecommendedNextSteps: []string{
			"lovart doctor",
			"lovart project current",
			"lovart generate <model> --prompt <text>",
		},
	}, nil
}

func normalizeOptions(opts Options) Options {
	if opts.Timeout <= 0 {
		opts.Timeout = DefaultTimeout
	}
	if opts.DebugPort <= 0 {
		opts.DebugPort = DefaultDebugPort
	}
	if !opts.RestartChrome {
		opts.RestartChrome = true
	}
	return opts
}

// ProjectVerifier validates the captured session against Lovart project APIs.
type ProjectVerifier struct{}

// Verify validates credentials, selects a project, and requires CID/webid.
func (ProjectVerifier) Verify(ctx context.Context, session auth.Session) (auth.Session, VerifyInfo, error) {
	if session.Cookie == "" && session.Token == "" {
		return auth.Session{}, VerifyInfo{}, fmt.Errorf("dev auth: browser capture did not find cookie or token")
	}
	if session.CID == "" {
		return auth.Session{}, VerifyInfo{}, fmt.Errorf("dev auth: browser capture did not find cid/webid")
	}

	signer, err := signing.NewSigner()
	if err != nil {
		return auth.Session{}, VerifyInfo{}, fmt.Errorf("dev auth: load signer: %w", err)
	}
	client := lovarthttp.NewClient(&auth.Credentials{
		Cookie: session.Cookie,
		Token:  session.Token,
		CSRF:   session.CSRF,
		WebID:  session.CID,
	}, signer)
	if err := client.SyncTime(ctx); err != nil {
		return auth.Session{}, VerifyInfo{}, fmt.Errorf("dev auth: verify time sync: %w", err)
	}
	projects, err := project.List(ctx, client)
	if err != nil {
		return auth.Session{}, VerifyInfo{}, fmt.Errorf("dev auth: verify projects: %w", err)
	}
	if len(projects) == 0 {
		return auth.Session{}, VerifyInfo{}, fmt.Errorf("dev auth: account has no Lovart projects")
	}

	selected := projects[0]
	if session.ProjectID != "" {
		if found, ok := project.FindByID(projects, session.ProjectID); ok {
			selected = *found
		}
	}
	session.ProjectID = selected.ID
	return session, VerifyInfo{ProjectName: selected.Name, ProjectCount: len(projects)}, nil
}
