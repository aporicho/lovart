package update

import (
	"context"
	"fmt"

	"github.com/aporicho/lovart/internal/metadata"
	"github.com/aporicho/lovart/internal/paths"
	"github.com/aporicho/lovart/internal/signing"
)

// SyncSigner refreshes the runtime signer WASM cache.
func (s *Service) SyncSigner(ctx context.Context) (*SignerSyncResult, error) {
	candidate, err := s.discoverSignerCandidate(ctx)
	if err != nil {
		return nil, err
	}
	if err := signing.ValidateWASMBytes(ctx, candidate.Bytes); err != nil {
		return nil, fmt.Errorf("update: downloaded signer failed validation: %w", err)
	}

	if err := metadata.WriteFileAtomic(paths.SignerWASMFile, candidate.Bytes, 0644); err != nil {
		return nil, err
	}
	manifest := &SignerManifest{
		Version:         1,
		SourceURL:       candidate.URL,
		SHA256:          candidate.SHA256,
		SizeBytes:       len(candidate.Bytes),
		CanvasHTMLHash:  candidate.CanvasHTMLHash,
		StaticJSHash:    candidate.StaticJSHash,
		SentryReleaseID: candidate.SentryReleaseID,
		SyncedAt:        s.now(),
	}
	if err := WriteSignerManifest(manifest); err != nil {
		return nil, err
	}
	return &SignerSyncResult{
		Written:  []string{paths.SignerWASMFile, paths.SignerManifestFile},
		Manifest: manifest,
	}, nil
}
