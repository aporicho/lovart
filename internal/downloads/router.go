package downloads

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var templateVarPattern = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_.]+)(?::([0-9]+))?\s*\}\}`)

func resolvePath(root string, opts Options, artifact Artifact, ext string) (string, string, error) {
	dirTemplate := opts.DirTemplate
	if dirTemplate == "" {
		dirTemplate = DefaultDirTemplate
	}
	fileTemplate := opts.FileTemplate
	if fileTemplate == "" {
		fileTemplate = DefaultFileTemplate
	}

	data := templateData{
		context:  opts.Context,
		taskID:   opts.TaskID,
		artifact: artifact,
		ext:      ext,
	}

	dirRel := sanitizeRelativePath(renderTemplate(dirTemplate, data))
	if dirRel == "" {
		dirRel = sanitizeRelativePath(data.jobFolder())
	}
	if dirRel == "" {
		dirRel = "download"
	}

	filename := safeFilename(renderTemplate(fileTemplate, data), fmt.Sprintf("artifact-%02d.%s", artifact.Index, ext))
	dir := filepath.Join(root, dirRel)
	return dir, filename, nil
}

func renderTemplate(template string, data templateData) string {
	return templateVarPattern.ReplaceAllStringFunc(template, func(match string) string {
		parts := templateVarPattern.FindStringSubmatch(match)
		if len(parts) < 2 {
			return ""
		}
		value := data.lookup(parts[1])
		if len(parts) >= 3 && parts[2] != "" {
			width, err := strconv.Atoi(parts[2])
			if err == nil {
				if n, ok := value.(int); ok {
					return fmt.Sprintf("%0*d", width, n)
				}
				if s, ok := value.(string); ok {
					if n, err := strconv.Atoi(s); err == nil {
						return fmt.Sprintf("%0*d", width, n)
					}
				}
			}
		}
		return fmt.Sprint(value)
	})
}

func sanitizeRelativePath(rendered string) string {
	rendered = strings.ReplaceAll(rendered, "\\", "/")
	parts := strings.Split(rendered, "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		segment := sanitizeSegment(part)
		if segment == "" {
			continue
		}
		out = append(out, segment)
	}
	return filepath.Join(out...)
}

type templateData struct {
	context  JobContext
	taskID   string
	artifact Artifact
	ext      string
}

func (d templateData) lookup(key string) any {
	switch key {
	case "model":
		return d.context.Model
	case "mode":
		return d.context.Mode
	case "ext":
		return d.ext
	case "job.id":
		return d.context.JobID
	case "job.title":
		return d.context.Title
	case "job.folder":
		return d.jobFolder()
	case "task.id":
		return d.taskID
	case "artifact.index":
		return d.artifact.Index
	case "artifact.width":
		return d.artifact.Width
	case "artifact.height":
		return d.artifact.Height
	default:
		if strings.HasPrefix(key, "fields.") {
			return lookupMapPath(d.context.Fields, strings.TrimPrefix(key, "fields."))
		}
		return ""
	}
}

func (d templateData) jobFolder() string {
	if folder, _ := lookupMapPath(d.context.Fields, "folder").(string); folder != "" {
		return folder
	}
	sceneNo, _ := lookupMapPath(d.context.Fields, "scene_no").(string)
	sceneName, _ := lookupMapPath(d.context.Fields, "scene_name").(string)
	if sceneNo != "" || sceneName != "" {
		return strings.TrimSpace(sceneNo + " " + sceneName)
	}
	if d.context.Title != "" {
		return strings.TrimSpace(strings.SplitN(d.context.Title, " / ", 2)[0])
	}
	if d.context.JobID != "" {
		return d.context.JobID
	}
	if d.taskID != "" {
		return d.taskID
	}
	if d.context.Model != "" {
		return d.context.Model
	}
	return "download"
}

func lookupMapPath(values map[string]any, path string) any {
	if len(values) == 0 || path == "" {
		return ""
	}
	parts := strings.Split(path, ".")
	var current any = values
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current, ok = m[part]
		if !ok {
			return ""
		}
	}
	return current
}
