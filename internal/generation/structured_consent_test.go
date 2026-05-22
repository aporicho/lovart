package generation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/aporicho/lovart/internal/auth"
	lovarthttp "github.com/aporicho/lovart/internal/http"
	"github.com/aporicho/lovart/internal/paths"
)

func TestPreflightStructuredPaidConsentMissingConfirmPaidBlocks(t *testing.T) {
	server := newGenerationConsentServer(t)
	client := newGenerationConsentClient(server)

	result, err := Preflight(context.Background(), client, "openai/gpt-image-2", map[string]any{"prompt": "test"}, Options{
		Mode:       "auto",
		AllowPaid:  true,
		MaxCredits: 20,
	})
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	if result.CanSubmit {
		t.Fatalf("missing structured consent allowed submit: %#v", result)
	}
	assertRecommendedActionContains(t, result.RecommendedActions, "structured_consent_missing_confirm_paid")
	if server.submitCalls != 0 || server.slotCalls != 0 {
		t.Fatalf("preflight performed submit side effects: submits=%d slots=%d", server.submitCalls, server.slotCalls)
	}
}

func TestPreflightStructuredPaidConsentMaxCreditsMismatchBlocks(t *testing.T) {
	server := newGenerationConsentServer(t)
	client := newGenerationConsentClient(server)

	result, err := Preflight(context.Background(), client, "openai/gpt-image-2", map[string]any{"prompt": "test"}, validConsentOptions(func(opts *Options) {
		opts.ConfirmedMaxCredits = 10
	}))
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	if result.CanSubmit {
		t.Fatalf("max credits mismatch allowed submit: %#v", result)
	}
	assertRecommendedActionContains(t, result.RecommendedActions, "structured_consent_max_total_credits_mismatch")
}

func TestPreflightStructuredPaidConsentModelModeOutputsMismatchBlocks(t *testing.T) {
	tests := []struct {
		name string
		mut  func(*Options)
		code string
	}{
		{name: "model", mut: func(opts *Options) { opts.ConfirmedModel = "other-model" }, code: "structured_consent_model_mismatch"},
		{name: "mode", mut: func(opts *Options) { opts.ConfirmedMode = "fast" }, code: "structured_consent_mode_mismatch"},
		{name: "outputs", mut: func(opts *Options) { opts.ConfirmedOutputs = 1 }, code: "structured_consent_outputs_mismatch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newGenerationConsentServer(t)
			client := newGenerationConsentClient(server)
			result, err := Preflight(context.Background(), client, "openai/gpt-image-2", map[string]any{"prompt": "test", "n": 2}, validConsentOptions(tt.mut))
			if err != nil {
				t.Fatalf("Preflight: %v", err)
			}
			if result.CanSubmit {
				t.Fatalf("%s mismatch allowed submit: %#v", tt.name, result)
			}
			assertRecommendedActionContains(t, result.RecommendedActions, tt.code)
		})
	}
}

func TestSubmitStructuredPaidConsentMissingConfirmPaidBlocksBeforeRemoteSubmit(t *testing.T) {
	server := newGenerationConsentServer(t)
	client := newGenerationConsentClient(server)

	_, err := Submit(context.Background(), client, "openai/gpt-image-2", map[string]any{"prompt": "test"}, Options{
		Mode:               "auto",
		AllowPaid:          true,
		MaxCredits:         20,
		ProjectID:          "project-1",
		ConfirmedProjectID: "project-1",
		CID:                "cid-1",
	})
	if err == nil {
		t.Fatal("Submit succeeded without structured consent")
	}
	var consentErr *StructuredConsentError
	if !errors.As(err, &consentErr) {
		t.Fatalf("error type = %T, want StructuredConsentError: %v", err, err)
	}
	if consentErr.Code != "structured_consent_missing_confirm_paid" {
		t.Fatalf("code = %q", consentErr.Code)
	}
	if server.submitCalls != 0 || server.slotCalls != 0 {
		t.Fatalf("submit gate caused side effects before denial: submits=%d slots=%d", server.submitCalls, server.slotCalls)
	}
}

func TestSubmitStructuredPaidConsentValidAllowsExistingPath(t *testing.T) {
	server := newGenerationConsentServer(t)
	client := newGenerationConsentClient(server)

	result, err := Submit(context.Background(), client, "openai/gpt-image-2", map[string]any{"prompt": "test", "n": 2}, validConsentOptions(func(opts *Options) {
		opts.ProjectID = "project-1"
		opts.ConfirmedProjectID = "project-1"
		opts.CID = "cid-1"
	}))
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if result.TaskID != "task-123" || result.Status != "submitted" {
		t.Fatalf("result = %#v", result)
	}
	if server.quoteCalls != 1 || server.slotCalls != 1 || server.submitCalls != 1 {
		t.Fatalf("calls quote=%d slot=%d submit=%d", server.quoteCalls, server.slotCalls, server.submitCalls)
	}
}

func validConsentOptions(mut func(*Options)) Options {
	opts := Options{
		Mode:                    "auto",
		AllowPaid:               true,
		MaxCredits:              20,
		ConfirmPaid:             true,
		ConfirmedMaxCredits:     20,
		ConfirmedModel:          "openai/gpt-image-2",
		ConfirmedMode:           "auto",
		ConfirmedOutputs:        2,
		ConfirmedRemoteRequests: 1,
	}
	if mut != nil {
		mut(&opts)
	}
	return opts
}

func assertRecommendedActionContains(t *testing.T, actions []string, want string) {
	t.Helper()
	for _, action := range actions {
		if bytes.Contains([]byte(action), []byte(want)) {
			return
		}
	}
	t.Fatalf("recommended actions %#v do not contain %q", actions, want)
}

type generationConsentServer struct {
	quoteCalls  int
	slotCalls   int
	submitCalls int
}

func newGenerationConsentServer(t *testing.T) *generationConsentServer {
	t.Helper()
	setupGenerationConsentRegistry(t)
	return &generationConsentServer{}
}

func (s *generationConsentServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/v1/generator/pricing":
		s.quoteCalls++
		writeGenerationConsentJSON(w, map[string]any{"code": 0, "data": map[string]any{
			"price":   12.5,
			"balance": 40,
			"price_detail": map[string]any{
				"unit_price":     12.5,
				"unit_count":     1,
				"unit_name":      "image",
				"total_price":    12.5,
				"search_key":     "openai/gpt-image-2",
				"generator_name": "openai/gpt-image-2",
			},
		}})
	case "/api/canva/agent-cashier/task/take/slot":
		s.slotCalls++
		writeGenerationConsentJSON(w, map[string]any{"code": 0})
	case "/v1/generator/tasks":
		s.submitCalls++
		writeGenerationConsentJSON(w, map[string]any{"code": 0, "data": map[string]any{"generator_task_id": "task-123"}})
	default:
		w.WriteHeader(http.StatusNotFound)
		writeGenerationConsentJSON(w, map[string]any{"code": 404, "message": r.URL.Path})
	}
}

func writeGenerationConsentJSON(w http.ResponseWriter, value any) {
	_ = json.NewEncoder(w).Encode(value)
}

func newGenerationConsentClient(handler http.Handler) *lovarthttp.Client {
	client := lovarthttp.NewClient(&auth.Credentials{Cookie: "lovart=test", Token: "token", WebID: "web-123"}, nil)
	client.SetHTTPClient(&http.Client{Transport: generationConsentRoundTripper{handler: handler}, Timeout: 5 * time.Second})
	return client
}

type generationConsentRoundTripper struct {
	handler http.Handler
}

func (t generationConsentRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	body, err := cloneGenerationConsentBody(req.Body)
	if err != nil {
		return nil, err
	}
	clone := req.Clone(req.Context())
	clone.Body = io.NopCloser(bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	t.handler.ServeHTTP(recorder, clone)
	return recorder.Result(), nil
}

func cloneGenerationConsentBody(body io.ReadCloser) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	defer body.Close()
	return io.ReadAll(body)
}

func setupGenerationConsentRegistry(t *testing.T) {
	t.Helper()
	t.Cleanup(paths.Reset)
	dir := t.TempDir()
	t.Setenv("LOVART_HOME", dir)
	paths.Reset()
	if err := os.MkdirAll(paths.MetadataDir, 0755); err != nil {
		t.Fatal(err)
	}
	list := []byte(`{"items":[{"name":"openai/gpt-image-2","display_name":"GPT Image 2","type":"image"}]}`)
	schema := []byte(`{
  "paths": {
    "/openai/gpt-image-2": {
      "post": {
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/GPTImage2Request"}
            }
          }
        }
      }
    }
  },
  "components": {
    "schemas": {
      "GPTImage2Request": {
        "required": ["prompt"],
        "properties": {
          "prompt": {"type": "string"},
          "size": {"type": "string", "default": "2048*2048"},
          "quality": {"type": "string", "default": "low"},
          "n": {"type": "integer", "default": 1, "minimum": 1, "maximum": 10}
        }
      }
    }
  }
}`)
	if err := os.WriteFile(paths.GeneratorListFile, list, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.GeneratorSchemaFile, schema, 0644); err != nil {
		t.Fatal(err)
	}
}
