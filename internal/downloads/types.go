package downloads

import nethttp "net/http"

const (
	// DefaultDirTemplate groups batch downloads by the derived job folder.
	DefaultDirTemplate = "{{job.folder}}"
	// DefaultFileTemplate preserves stable artifact ordering within a job.
	DefaultFileTemplate = "artifact-{{artifact.index:02}}.{{ext}}"
)

// Artifact is one remote generation output to download.
type Artifact struct {
	URL    string `json:"url"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
	Index  int    `json:"index,omitempty"`
}

// JobContext describes the user-facing generation context used for routing and
// effect metadata. Fields are organizational only and are not embedded into the
// image effect metadata.
type JobContext struct {
	Model  string         `json:"model,omitempty"`
	Mode   string         `json:"mode,omitempty"`
	JobID  string         `json:"job_id,omitempty"`
	Title  string         `json:"title,omitempty"`
	Fields map[string]any `json:"fields,omitempty"`
	Body   map[string]any `json:"body,omitempty"`
}

// Options configures a download batch.
type Options struct {
	RootDir      string
	DirTemplate  string
	FileTemplate string
	TaskID       string
	Context      JobContext
	HTTPClient   *nethttp.Client
	Overwrite    bool
}

// Result summarizes a download batch.
type Result struct {
	RootDir    string       `json:"root_dir"`
	IndexPath  string       `json:"index_path,omitempty"`
	IndexError string       `json:"index_error,omitempty"`
	Files      []FileResult `json:"files"`
}

// FileResult is the persisted result for one artifact.
type FileResult struct {
	ArtifactIndex    int    `json:"artifact_index"`
	URL              string `json:"url"`
	Path             string `json:"path,omitempty"`
	Directory        string `json:"directory,omitempty"`
	Filename         string `json:"filename,omitempty"`
	ContentType      string `json:"content_type,omitempty"`
	Bytes            int64  `json:"bytes,omitempty"`
	Existing         bool   `json:"existing,omitempty"`
	EmbeddedMetadata bool   `json:"embedded_metadata,omitempty"`
	MetadataFormat   string `json:"metadata_format,omitempty"`
	MetadataError    string `json:"metadata_error,omitempty"`
	Error            string `json:"error,omitempty"`
}

// EffectMetadata is embedded into image files. It intentionally contains only
// fields that can affect image appearance.
type EffectMetadata struct {
	SchemaVersion int               `json:"schema_version"`
	Model         string            `json:"model,omitempty"`
	Mode          string            `json:"mode,omitempty"`
	InputArgs     map[string]any    `json:"input_args,omitempty"`
	Artifact      EffectArtifactRef `json:"artifact"`
}

// EffectArtifactRef identifies the generated image within the request.
type EffectArtifactRef struct {
	Index  int `json:"index,omitempty"`
	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`
}
