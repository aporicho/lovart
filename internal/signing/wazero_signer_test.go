package signing

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/aporicho/lovart/internal/paths"
)

const testSignerWASM = "testdata/26bd3a5bd74c3c92.wasm"

// TestSignerConsistency verifies that the Go wazero signer produces
// identical output to the v1 Node.js signer for known inputs.
// These test vectors were validated against the v1 signature.js output.
func TestSignerConsistency(t *testing.T) {
	signer, err := NewSignerFromPath(testSignerWASM)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	ctx := context.Background()

	tests := []struct {
		name      string
		timestamp string
		reqUUID   string
		want      string
	}{
		{
			name:      "v1 verified vector 1",
			timestamp: "1746600000000",
			reqUUID:   "test1234567890abcdef1234567890ab",
			want:      "1:75438f6c5c2d4a8b23e28abcc9740ff32244436e83b491c2b0b88b07b386f0f4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := signer.Sign(ctx, SigningPayload{
				Timestamp: tt.timestamp,
				ReqUUID:   tt.reqUUID,
			})
			if err != nil {
				t.Fatalf("Sign: %v", err)
			}
			if result.Signature != tt.want {
				t.Errorf("signature mismatch:\n  got  %s\n  want %s", result.Signature, tt.want)
			}
		})
	}
}

// TestSignerHealth verifies the WASM module is loaded and healthy.
func TestSignerHealth(t *testing.T) {
	signer, err := NewSignerFromPath(testSignerWASM)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	if err := signer.Health(); err != nil {
		t.Errorf("Health: %v", err)
	}
}

// TestSignerBlank produces unique signatures for different inputs.
func TestSignerUniqueOutput(t *testing.T) {
	signer, err := NewSignerFromPath(testSignerWASM)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	ctx := context.Background()

	r1, err := signer.Sign(ctx, SigningPayload{Timestamp: "1000000000000", ReqUUID: "aaaa"})
	if err != nil {
		t.Fatal(err)
	}
	r2, err := signer.Sign(ctx, SigningPayload{Timestamp: "2000000000000", ReqUUID: "bbbb"})
	if err != nil {
		t.Fatal(err)
	}

	if r1.Signature == "" || r2.Signature == "" {
		t.Error("empty signature")
	}
	if r1.Signature == r2.Signature {
		t.Error("different inputs produced identical signatures")
	}
}

func TestNewSignerLoadsRuntimeCache(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOVART_HOME", dir)
	paths.Reset()

	data, err := os.ReadFile(testSignerWASM)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(paths.SignerWASMFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.SignerWASMFile, data, 0644); err != nil {
		t.Fatal(err)
	}

	signer, err := NewSigner()
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	if err := signer.Health(); err != nil {
		t.Fatalf("Health: %v", err)
	}
}

func TestNewSignerMissingRuntimeCache(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOVART_HOME", dir)
	paths.Reset()

	_, err := NewSigner()
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrNoSigner) {
		t.Fatalf("expected ErrNoSigner, got %v", err)
	}
}
