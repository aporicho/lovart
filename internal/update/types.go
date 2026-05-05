package update

import (
	"context"
	"errors"
	nethttp "net/http"
	"time"

	"github.com/aporicho/lovart/internal/metadata"
	"github.com/aporicho/lovart/internal/signing"
)

const defaultCanvasURL = "https://www.lovart.ai/canvas"

// HTTPDoer is the subset of net/http.Client used by update.
type HTTPDoer interface {
	Do(*nethttp.Request) (*nethttp.Response, error)
}

// Service provides update operations. Tests can inject HTTP and time.
type Service struct {
	HTTP      HTTPDoer
	CanvasURL string
	Now       func() time.Time
}

// DriftResult shows what changed between runtime cache and live Lovart.
type DriftResult struct {
	Status               string          `json:"status"`
	Detected             bool            `json:"detected"`
	Changes              []string        `json:"changes,omitempty"`
	RecommendedActions   []string        `json:"recommended_actions,omitempty"`
	Signer               *SignerStatus   `json:"signer,omitempty"`
	Metadata             *MetadataStatus `json:"metadata,omitempty"`
	MetadataCheckSkipped string          `json:"metadata_check_skipped,omitempty"`
}

// SignerStatus summarizes signer cache freshness.
type SignerStatus struct {
	LocalSHA256 string `json:"local_sha256,omitempty"`
	LiveSHA256  string `json:"live_sha256,omitempty"`
	SourceURL   string `json:"source_url,omitempty"`
	Stale       bool   `json:"stale"`
	Missing     bool   `json:"missing"`
}

// MetadataStatus summarizes metadata cache freshness.
type MetadataStatus struct {
	LocalListHash   string `json:"local_list_hash,omitempty"`
	LiveListHash    string `json:"live_list_hash,omitempty"`
	LocalSchemaHash string `json:"local_schema_hash,omitempty"`
	LiveSchemaHash  string `json:"live_schema_hash,omitempty"`
	Stale           bool   `json:"stale"`
	Missing         bool   `json:"missing"`
}

// SignerManifest records the signer WASM cache.
type SignerManifest struct {
	Version         int       `json:"version"`
	SourceURL       string    `json:"source_url"`
	SHA256          string    `json:"sha256"`
	SizeBytes       int       `json:"size_bytes"`
	CanvasHTMLHash  string    `json:"canvas_html_hash,omitempty"`
	StaticJSHash    string    `json:"static_js_hash,omitempty"`
	SentryReleaseID string    `json:"sentry_release_id,omitempty"`
	SyncedAt        time.Time `json:"synced_at"`
}

// SignerSyncResult describes a signer sync.
type SignerSyncResult struct {
	Written  []string        `json:"written"`
	Manifest *SignerManifest `json:"manifest"`
}

// MetadataSyncResult describes a metadata sync.
type MetadataSyncResult struct {
	Written  []string           `json:"written"`
	Manifest *metadata.Manifest `json:"manifest"`
}

// SyncAllResult describes a full runtime cache sync.
type SyncAllResult struct {
	Signer   *SignerSyncResult   `json:"signer"`
	Metadata *MetadataSyncResult `json:"metadata"`
}

type signerCandidate struct {
	URL             string
	Bytes           []byte
	SHA256          string
	CanvasHTMLHash  string
	StaticJSHash    string
	SentryReleaseID string
}

// NewService returns an update service with production defaults.
func NewService() *Service {
	return &Service{
		HTTP:      &nethttp.Client{Timeout: 30 * time.Second},
		CanvasURL: defaultCanvasURL,
		Now:       time.Now,
	}
}

// Check compares runtime caches against live Lovart.
func Check(ctx context.Context) (*DriftResult, error) {
	return NewService().Check(ctx)
}

// SyncSigner refreshes the runtime signer WASM cache.
func SyncSigner(ctx context.Context) (*SignerSyncResult, error) {
	return NewService().SyncSigner(ctx)
}

// SyncMetadata refreshes runtime generator metadata snapshots.
func SyncMetadata(ctx context.Context) (*MetadataSyncResult, error) {
	return NewService().SyncMetadata(ctx)
}

// SyncAll refreshes signer first, then signed metadata.
func SyncAll(ctx context.Context) (*SyncAllResult, error) {
	return NewService().SyncAll(ctx)
}

// IsSignerMissing reports whether an error is caused by a missing signer cache.
func IsSignerMissing(err error) bool {
	return errors.Is(err, signing.ErrNoSigner)
}
