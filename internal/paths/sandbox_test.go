package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveWithinBaseAllowsRelativePathInsideBase(t *testing.T) {
	base := t.TempDir()
	got, err := ResolveWithinBase(base, filepath.Join("nested", "artifact.png"))
	if err != nil {
		t.Fatalf("ResolveWithinBase: %v", err)
	}
	want := filepath.Join(base, "nested", "artifact.png")
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestResolveWithinBaseRejectsRelativeEscape(t *testing.T) {
	base := t.TempDir()
	if _, err := ResolveWithinBase(base, filepath.Join("..", "escape")); err == nil {
		t.Fatalf("expected escape to be rejected")
	}
}

func TestResolveWithinBaseAllowsExplicitAbsolutePath(t *testing.T) {
	base := t.TempDir()
	explicit := filepath.Join(t.TempDir(), "chosen", "artifact.png")
	got, err := ResolveWithinBase(base, explicit)
	if err != nil {
		t.Fatalf("ResolveWithinBase: %v", err)
	}
	if got != explicit {
		t.Fatalf("path = %q, want %q", got, explicit)
	}
}

func TestResolveWithinBaseRejectsSymlinkEscape(t *testing.T) {
	base := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(base, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if _, err := ResolveWithinBase(base, filepath.Join("link", "artifact.png")); err == nil {
		t.Fatalf("expected symlink escape to be rejected")
	}
}

func TestResolveWithinBaseRejectsEmptyPath(t *testing.T) {
	if _, err := ResolveWithinBase(t.TempDir(), ""); err == nil {
		t.Fatalf("expected empty path error")
	}
}
