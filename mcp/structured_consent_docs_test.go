package mcp

import (
	"os"
	"strings"
	"testing"
)

var structuredConsentPhase1Fields = []string{
	"confirm_paid",
	"confirmed_max_credits",
	"confirmed_model",
	"confirmed_mode",
	"confirmed_outputs",
	"confirmed_remote_requests",
	"confirmed_project_id",
	"confirm_current_project",
	"confirmed_canonical_path",
}

var structuredConsentPhase1Tools = []string{
	"lovart_generate",
	"lovart_jobs_run",
	"lovart_jobs_resume",
}

var structuredConsentPhase2Fields = []string{
	"confirm_project_id",
	"confirm_task_ids",
	"confirm_new_name",
	"confirmed_project_name",
	"confirmed_select_after_create",
	"confirm_overwrite",
	"confirm_outside_default_root",
	"confirmed_file_count",
	"confirmed_canvas_write",
	"confirm_canvas_shape_ids",
}

var structuredConsentPhase2Risks = []string{
	"destructive",
	"remote_write",
	"local_write",
	"canvas_write",
}

var structuredConsentPhase2Tools = []string{
	"lovart_project_delete",
	"lovart_project_rename",
	"lovart_task_cancel",
	"lovart_project_create",
	"lovart_project_select",
	"lovart_task_download",
	"lovart_canvas_download",
	"lovart_task_canvas",
	"lovart_jobs_finalize",
}

func TestStructuredConsentPhase1ReviewMatrix(t *testing.T) {
	design := readStructuredConsentDoc(t, "../docs/architecture/structured-consent.md")
	matrix := readStructuredConsentDoc(t, "../docs/architecture/structured-consent-test-matrix.md")

	for _, field := range structuredConsentPhase1Fields {
		assertDocMentions(t, "structured consent design", design, "`"+field+"`")
		row := structuredConsentMatrixRow(t, matrix, field)
		assertDocMentions(t, "structured consent matrix row for "+field, row, "schema property named `"+field+"`")
		assertDocMentions(t, "structured consent matrix row for "+field, row, "equivalent command docs/help")
	}
	for _, tool := range structuredConsentPhase1Tools {
		assertDocMentions(t, "structured consent design", design, "`"+tool+"`")
		assertDocMentions(t, "structured consent test matrix", matrix, "`"+tool+"`")
	}
}

func TestStructuredConsentPhase2ReviewMatrix(t *testing.T) {
	design := readStructuredConsentDoc(t, "../docs/architecture/structured-consent.md")
	matrix := readStructuredConsentDoc(t, "../docs/architecture/structured-consent-test-matrix.md")

	for _, risk := range structuredConsentPhase2Risks {
		assertDocMentions(t, "structured consent design", design, "`"+risk+"`")
		assertDocMentions(t, "structured consent test matrix", matrix, "`"+risk+"`")
	}
	for _, field := range structuredConsentPhase2Fields {
		assertDocMentions(t, "structured consent design", design, "`"+field+"`")
		row := structuredConsentMatrixRow(t, matrix, field)
		if !strings.Contains(row, "tool input schema property named `"+field+"`") && !strings.Contains(row, "schema/reserved field named `"+field+"`") {
			t.Fatalf("structured consent matrix row for %s must name the MCP schema property or reserved schema field; row=%s", field, row)
		}
	}
	for _, tool := range structuredConsentPhase2Tools {
		assertDocMentions(t, "structured consent design", design, "`"+tool+"`")
		assertDocMentions(t, "structured consent test matrix", matrix, "`"+tool+"`")
	}
	assertDocMentions(t, "structured consent design", design, "schema/reserved")
}

func TestStructuredConsentAuditReviewMatrix(t *testing.T) {
	design := readStructuredConsentDoc(t, "../docs/architecture/structured-consent.md")
	matrix := readStructuredConsentDoc(t, "../docs/architecture/structured-consent-test-matrix.md")

	for _, required := range []string{
		"correlation_id",
		"structured_consent",
		"structured-consent/v1",
		"present",
		"matched",
		"mismatched",
		"not_required",
	} {
		assertDocMentions(t, "structured consent audit matrix", matrix, required)
	}
	for _, required := range []string{
		"correlation_id",
		"structured-consent/v1",
		"present",
		"matched",
		"mismatched",
		"not_required",
	} {
		assertDocMentions(t, "structured consent design", design, required)
	}
	for _, forbidden := range []string{
		"prompt body",
		"cookie",
		"token",
		"auth header",
		"file contents",
	} {
		assertDocMentions(t, "structured consent audit matrix", matrix, forbidden)
	}
}

func readStructuredConsentDoc(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func structuredConsentMatrixRow(t *testing.T, matrix, field string) string {
	t.Helper()
	needle := "| `" + field + "` |"
	for _, line := range strings.Split(matrix, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	t.Fatalf("structured consent test matrix must contain a table row for %q", field)
	return ""
}

func assertDocMentions(t *testing.T, label, doc, needle string) {
	t.Helper()
	if !strings.Contains(doc, needle) {
		t.Fatalf("%s must mention %q", label, needle)
	}
}
