package ui

import (
	"html"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

const testHTML = `<!DOCTYPE html><html><head><meta charset="utf-8"></head><body></body></html>`

func TestInjectBaseBuildsAbsoluteEncodedFileURL(t *testing.T) {
	relativeDir := filepath.Join("relative base", "100% #1?", "café")
	absoluteDir, err := filepath.Abs(relativeDir)
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}

	got, err := injectBase(testHTML, relativeDir)
	if err != nil {
		t.Fatalf("injectBase() error = %v", err)
	}
	base := parseBaseURL(t, got)

	if base.Scheme != "file" {
		t.Errorf("base scheme = %q, want file", base.Scheme)
	}
	if base.RawQuery != "" || base.Fragment != "" {
		t.Errorf("base URL has unintended query %q or fragment %q", base.RawQuery, base.Fragment)
	}
	wantPath := filepath.ToSlash(absoluteDir) + "/"
	if base.Path != wantPath {
		t.Errorf("decoded base path = %q, want %q", base.Path, wantPath)
	}
	if !strings.HasSuffix(base.Path, "/") {
		t.Errorf("base path %q does not retain directory semantics", base.Path)
	}
	for _, encoded := range []string{"%20", "%25", "%23", "%3F", "%C3%A9"} {
		if !strings.Contains(strings.ToUpper(base.EscapedPath()), strings.ToUpper(encoded)) {
			t.Errorf("escaped path %q does not contain %q", base.EscapedPath(), encoded)
		}
	}
}

func TestInjectBasePreservesOrdinaryAbsolutePathAndTrailingSlash(t *testing.T) {
	absoluteDir := filepath.Join(t.TempDir(), "ordinary") + string(filepath.Separator)

	got, err := injectBase(testHTML, absoluteDir)
	if err != nil {
		t.Fatalf("injectBase() error = %v", err)
	}
	base := parseBaseURL(t, got)
	wantPath := filepath.ToSlash(strings.TrimRight(absoluteDir, string(filepath.Separator))) + "/"
	if base.Path != wantPath {
		t.Errorf("decoded base path = %q, want %q", base.Path, wantPath)
	}
}

func TestInjectBaseKeepsAttributeSafe(t *testing.T) {
	baseDir := filepath.Join(t.TempDir(), `a "quote" 'single' & <script data-injected="true">`)

	got, err := injectBase(testHTML, baseDir)
	if err != nil {
		t.Fatalf("injectBase() error = %v", err)
	}
	if strings.Count(got, `<base `) != 1 {
		t.Fatalf("expected exactly one base element, got:\n%s", got)
	}
	if strings.Contains(got, `<script`) || strings.Contains(got, `data-injected="true"`) {
		t.Fatalf("path injected markup or an attribute:\n%s", got)
	}
	if !strings.Contains(got, "&amp;") {
		t.Fatalf("base href did not retain HTML attribute escaping:\n%s", got)
	}
	base := parseBaseURL(t, got)
	want, err := filepath.Abs(baseDir)
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}
	if base.Path != filepath.ToSlash(want)+"/" {
		t.Errorf("decoded base path = %q, want %q", base.Path, filepath.ToSlash(want)+"/")
	}
}

func TestInjectBaseDoesNotInsertSecondBaseElement(t *testing.T) {
	input := `<html><head><base href="file:///existing/"></head></html>`
	got, err := injectBase(input, "relative")
	if err != nil {
		t.Fatalf("injectBase() error = %v", err)
	}
	if got != input {
		t.Fatalf("injectBase() changed HTML with an existing base element: %q", got)
	}
}

func parseBaseURL(t *testing.T, document string) *url.URL {
	t.Helper()
	const prefix = `<base href="`
	start := strings.Index(document, prefix)
	if start < 0 {
		t.Fatalf("document has no quoted base href:\n%s", document)
	}
	start += len(prefix)
	end := strings.Index(document[start:], `">`)
	if end < 0 {
		t.Fatalf("base href attribute is not structurally closed:\n%s", document)
	}
	value := html.UnescapeString(document[start : start+end])
	parsed, err := url.Parse(value)
	if err != nil {
		t.Fatalf("url.Parse(%q) error = %v", value, err)
	}
	return parsed
}
