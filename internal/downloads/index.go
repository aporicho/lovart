package downloads

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const indexDirName = ".lovart-downloads"
const indexFileName = "index.jsonl"

type indexRecord struct {
	RecordedAt time.Time      `json:"recorded_at"`
	TaskID     string         `json:"task_id,omitempty"`
	JobID      string         `json:"job_id,omitempty"`
	Title      string         `json:"title,omitempty"`
	Model      string         `json:"model,omitempty"`
	Mode       string         `json:"mode,omitempty"`
	Fields     map[string]any `json:"fields,omitempty"`
	File       FileResult     `json:"file"`
}

func appendIndex(root string, result *Result, opts Options) error {
	if result == nil || len(result.Files) == 0 {
		return nil
	}
	indexDir := filepath.Join(root, indexDirName)
	if err := os.MkdirAll(indexDir, 0755); err != nil {
		return fmt.Errorf("downloads: create index dir: %w", err)
	}
	indexPath := filepath.Join(indexDir, indexFileName)
	file, err := os.OpenFile(indexPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("downloads: open index: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetEscapeHTML(false)
	for _, item := range result.Files {
		record := indexRecord{
			RecordedAt: time.Now().UTC(),
			TaskID:     opts.TaskID,
			JobID:      opts.Context.JobID,
			Title:      opts.Context.Title,
			Model:      opts.Context.Model,
			Mode:       opts.Context.Mode,
			Fields:     copyMap(opts.Context.Fields),
			File:       item,
		}
		if err := encoder.Encode(record); err != nil {
			return fmt.Errorf("downloads: write index: %w", err)
		}
	}
	result.IndexPath = indexPath
	return nil
}
