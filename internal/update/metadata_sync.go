package update

import (
	"context"
	"fmt"

	"github.com/aporicho/lovart/internal/auth"
	"github.com/aporicho/lovart/internal/http"
	"github.com/aporicho/lovart/internal/metadata"
	"github.com/aporicho/lovart/internal/paths"
	"github.com/aporicho/lovart/internal/signing"
)

type liveMetadata struct {
	list   map[string]any
	schema map[string]any
}

// SyncMetadata refreshes runtime generator metadata snapshots.
func (s *Service) SyncMetadata(ctx context.Context) (*MetadataSyncResult, error) {
	live, err := s.fetchMetadata(ctx)
	if err != nil {
		return nil, err
	}
	listHash, schemaHash, err := metadataHashes(live.list, live.schema)
	if err != nil {
		return nil, err
	}
	signerSHA := ""
	if signerManifest, err := ReadSignerManifest(); err == nil {
		signerSHA = signerManifest.SHA256
	}

	if err := metadata.WriteJSONAtomic(paths.GeneratorListFile, live.list, 0644); err != nil {
		return nil, err
	}
	if err := metadata.WriteJSONAtomic(paths.GeneratorSchemaFile, live.schema, 0644); err != nil {
		return nil, err
	}
	manifest := &metadata.Manifest{
		Version:             1,
		Source:              http.LGWBase,
		GeneratorListHash:   listHash,
		GeneratorSchemaHash: schemaHash,
		SignerSHA256:        signerSHA,
		SyncedAt:            s.now(),
	}
	if err := metadata.WriteManifest(manifest); err != nil {
		return nil, err
	}

	return &MetadataSyncResult{
		Written:  []string{paths.GeneratorListFile, paths.GeneratorSchemaFile, paths.MetadataManifestFile},
		Manifest: manifest,
	}, nil
}

func (s *Service) fetchMetadata(ctx context.Context) (*liveMetadata, error) {
	creds, err := auth.Load()
	if err != nil {
		return nil, err
	}
	signer, err := signing.NewSigner()
	if err != nil {
		return nil, err
	}
	client := http.NewClient(creds, signer)
	if err := client.SyncTime(ctx); err != nil {
		return nil, err
	}

	list, err := fetchGeneratorList(ctx, client)
	if err != nil {
		return nil, err
	}
	schema, err := fetchGeneratorSchema(ctx, client)
	if err != nil {
		return nil, err
	}
	return &liveMetadata{list: list, schema: schema}, nil
}

func fetchGeneratorList(ctx context.Context, client *http.Client) (map[string]any, error) {
	var resp struct {
		Code    int            `json:"code"`
		Message string         `json:"message"`
		Data    map[string]any `json:"data"`
	}
	if err := client.GetJSON(ctx, http.LGWBase, "/v1/generator/list?biz_type=16", &resp); err != nil {
		return nil, fmt.Errorf("update: fetch generator list: %w", err)
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("update: generator list returned code %d: %s", resp.Code, resp.Message)
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("update: generator list returned empty data")
	}
	return resp.Data, nil
}

func fetchGeneratorSchema(ctx context.Context, client *http.Client) (map[string]any, error) {
	var resp struct {
		Code    int            `json:"code"`
		Message string         `json:"message"`
		Data    map[string]any `json:"data"`
	}
	if err := client.GetJSON(ctx, http.LGWBase, "/v1/generator/schema?biz_type=16", &resp); err != nil {
		return nil, fmt.Errorf("update: fetch generator schema: %w", err)
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("update: generator schema returned code %d: %s", resp.Code, resp.Message)
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("update: generator schema returned empty data")
	}
	return resp.Data, nil
}

func metadataHashes(list, schema map[string]any) (string, string, error) {
	listHash, err := metadata.StableHash(list)
	if err != nil {
		return "", "", err
	}
	schemaHash, err := metadata.StableHash(schema)
	if err != nil {
		return "", "", err
	}
	return listHash, schemaHash, nil
}
