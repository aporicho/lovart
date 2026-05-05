// Package update handles signer and metadata drift detection and sync.
package update

import (
	"context"
	"sort"

	"github.com/aporicho/lovart/internal/metadata"
	"github.com/aporicho/lovart/internal/signing"
)

// Check compares runtime caches against live Lovart.
func (s *Service) Check(ctx context.Context) (*DriftResult, error) {
	candidate, err := s.discoverSignerCandidate(ctx)
	if err != nil {
		return nil, err
	}

	result := &DriftResult{Status: "fresh"}
	result.Signer = s.checkSigner(candidate, result)
	result.Metadata = s.checkMetadata(ctx, result)

	sort.Strings(result.Changes)
	result.Detected = len(result.Changes) > 0
	if result.Detected {
		result.Status = "stale"
		result.RecommendedActions = []string{"run `lovart update sync --all`"}
	}
	return result, nil
}

// SyncAll refreshes signer first, then signed metadata.
func (s *Service) SyncAll(ctx context.Context) (*SyncAllResult, error) {
	signer, err := s.SyncSigner(ctx)
	if err != nil {
		return nil, err
	}
	meta, err := s.SyncMetadata(ctx)
	if err != nil {
		return nil, err
	}
	return &SyncAllResult{Signer: signer, Metadata: meta}, nil
}

func (s *Service) checkSigner(candidate *signerCandidate, result *DriftResult) *SignerStatus {
	localSigner, signerErr := ReadSignerManifest()
	status := &SignerStatus{
		LiveSHA256: candidate.SHA256,
		SourceURL:  candidate.URL,
	}
	if signerErr != nil {
		status.Missing = true
		status.Stale = true
		result.Changes = append(result.Changes, "signer_wasm")
		return status
	}
	status.LocalSHA256 = localSigner.SHA256
	status.Stale = localSigner.SHA256 != candidate.SHA256 || localSigner.SourceURL != candidate.URL
	if status.Stale {
		result.Changes = append(result.Changes, "signer_wasm")
	}
	return status
}

func (s *Service) checkMetadata(ctx context.Context, result *DriftResult) *MetadataStatus {
	localMetadata, metadataErr := metadata.ReadManifest()
	if metadataErr != nil {
		result.Changes = append(result.Changes, "generator_metadata")
		return &MetadataStatus{Missing: true, Stale: true}
	}
	status := &MetadataStatus{
		LocalListHash:   localMetadata.GeneratorListHash,
		LocalSchemaHash: localMetadata.GeneratorSchemaHash,
	}
	if _, err := signing.NewSigner(); err != nil {
		result.MetadataCheckSkipped = err.Error()
		return status
	}
	live, err := s.fetchMetadata(ctx)
	if err != nil {
		result.MetadataCheckSkipped = err.Error()
		return status
	}
	listHash, schemaHash, err := metadataHashes(live.list, live.schema)
	if err != nil {
		result.MetadataCheckSkipped = err.Error()
		return status
	}
	status.LiveListHash = listHash
	status.LiveSchemaHash = schemaHash
	status.Stale = localMetadata.GeneratorListHash != listHash || localMetadata.GeneratorSchemaHash != schemaHash
	if status.Stale {
		result.Changes = append(result.Changes, "generator_metadata")
	}
	return status
}
