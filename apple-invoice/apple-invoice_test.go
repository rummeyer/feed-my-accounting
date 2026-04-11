package apple

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// sanitizeFilename
// ---------------------------------------------------------------------------

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"normal text", "Deine Rechnung von Apple", "Deine Rechnung von Apple"},
		{"special chars", "Invoice #123 (2024)", "Invoice _123 _2024_"},
		{"umlauts preserved", "Rechnungsübersicht für März", "Rechnungsübersicht für März"},
		{"only special chars", "!!!", "_"},
		{"empty string", "", "invoice"},
		{"hyphens and underscores", "my-file_name", "my-file_name"},
		{"slashes removed", "path/to/file", "path_to_file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeFilename(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractOrderNumber
// ---------------------------------------------------------------------------

func TestExtractOrderNumber(t *testing.T) {
	tests := []struct {
		name string
		html string
		want string
	}{
		{
			"found in span",
			`<html><body><span>Bestellnummer: W123456789</span></body></html>`,
			"W123456789",
		},
		{
			"found in td",
			`<html><body><table><tr><td>Bestellnummer: MHJT12345</td></tr></table></body></html>`,
			"MHJT12345",
		},
		{
			"not found",
			`<html><body><p>No order number here</p></body></html>`,
			"",
		},
		{
			"with extra whitespace",
			`<html><body><p>Bestellnummer:   ABC99  </p></body></html>`,
			"ABC99",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractOrderNumber(tt.html)
			if got != tt.want {
				t.Errorf("extractOrderNumber() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// cleanHTML
// ---------------------------------------------------------------------------

func TestCleanHTML_RemovesActionButton(t *testing.T) {
	html := `<html><body>
		<div class="action-button-cell">Click here</div>
		<p>Keep this</p>
	</body></html>`

	result, err := cleanHTML(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(result, "action-button-cell") {
		t.Error("expected action-button-cell to be removed")
	}
	if !strings.Contains(result, "Keep this") {
		t.Error("expected other content to be preserved")
	}
}

func TestCleanHTML_RemovesInlineLinkGroup(t *testing.T) {
	html := `<html><body>
		<div class="inline-link-group">Privacy | Terms</div>
		<p>Content</p>
	</body></html>`

	result, err := cleanHTML(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(result, "inline-link-group") {
		t.Error("expected inline-link-group to be removed")
	}
}

func TestCleanHTML_BoldsUIDNr(t *testing.T) {
	html := `<html><body>
		<div class="footer-copy"><p>UID-Nr: ATU12345</p></div>
	</body></html>`

	result, err := cleanHTML(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "font-weight:600") {
		t.Error("expected UID-Nr paragraph to be bolded")
	}
}

func TestCleanHTML_PreservesContent(t *testing.T) {
	html := `<html><body>
		<h1>Invoice</h1>
		<p>Amount: €9.99</p>
	</body></html>`

	result, err := cleanHTML(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Invoice") || !strings.Contains(result, "€9.99") {
		t.Error("expected content to be preserved")
	}
}
