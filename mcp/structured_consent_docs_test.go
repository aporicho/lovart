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
