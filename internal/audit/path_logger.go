package audit

import (
	"os"
	"path/filepath"
)

// NewPath opens path for append and returns a JSONL logger. An empty path
// returns a disabled logger without creating files or directories.
func NewPath(path string) (*Logger, error) {
	if path == "" {
		return Disabled(), nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	logger := NewWriter(file)
	logger.closer = file
	return logger, nil
}
