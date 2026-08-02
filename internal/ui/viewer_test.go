package ui

import (
	"errors"
	"html"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"mahcdown/internal/minimark"
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

func TestGeneratedPolicyPrecedesBaseAndStyle(t *testing.T) {
	generated := minimark.RenderHTML(minimark.Document{Blocks: []minimark.Block{
		minimark.Paragraph{Inlines: []minimark.Inline{
			minimark.Image{URL: "images/local.png", Alt: "local"},
			minimark.Image{URL: "https://example.com/remote.png", Alt: "remote"},
		}},
	}})
	got, err := injectBase(generated, filepath.Join(t.TempDir(), `notes & "drafts"`))
	if err != nil {
		t.Fatalf("injectBase() error = %v", err)
	}

	cspAt := strings.Index(got, `<meta http-equiv="Content-Security-Policy"`)
	baseAt := strings.Index(got, `<base href="`)
	styleAt := strings.Index(got, `<style>`)
	if cspAt < 0 || baseAt < 0 || styleAt < 0 || !(cspAt < baseAt && baseAt < styleAt) {
		t.Fatalf("expected CSP before base before style, indexes: CSP=%d base=%d style=%d", cspAt, baseAt, styleAt)
	}
	if strings.Count(got, `http-equiv="Content-Security-Policy"`) != 1 {
		t.Fatalf("expected exactly one CSP meta element: %s", got)
	}
	if !strings.Contains(got, `notes%20&amp;%20%22drafts%22/`) {
		t.Fatalf("base URL lost URL encoding or HTML escaping: %s", got)
	}
	if !strings.Contains(got, `<img src="images/local.png" alt="local"/>`) {
		t.Fatalf("ordinary local image was not preserved: %s", got)
	}
	if strings.Contains(got, `<img src="https://example.com/remote.png"`) || !strings.Contains(got, `[image blocked: remote]`) {
		t.Fatalf("remote image was not blocked: %s", got)
	}

	second, err := injectBase(got, filepath.Join(t.TempDir(), "other"))
	if err != nil {
		t.Fatalf("second injectBase() error = %v", err)
	}
	if strings.Count(second, `http-equiv="Content-Security-Policy"`) != 1 {
		t.Fatalf("reprocessing duplicated CSP: %s", second)
	}
}

func TestShortcutJSHandlesOnlyInertRenderedLinks(t *testing.T) {
	js := shortcutJS()
	required := []string{
		`a.mahcdown-link[data-mahcdown-href]`,
		`link.getAttribute('data-mahcdown-href')`,
		`e.preventDefault()`,
		`document.body.addEventListener('click'`,
		`document.body.addEventListener('keydown'`,
		`e.key !== 'Enter' && e.key !== ' '`,
		`if (!url) return`,
		`window.mahcdownOpenLink(url)`,
	}
	for _, fragment := range required {
		if !strings.Contains(js, fragment) {
			t.Errorf("shortcutJS() is missing %q", fragment)
		}
	}
	for _, forbidden := range []string{`getAttribute('href')`, `.href`, `closest('a')`} {
		if strings.Contains(js, forbidden) {
			t.Errorf("shortcutJS() still reads or handles an active generic href via %q", forbidden)
		}
	}
}

func TestValidateExternalURL(t *testing.T) {
	accepted := []string{
		"http://example.com",
		"https://example.com",
		"HtTp://example.com",
		"hTTps://example.com",
		"https://example.com/path",
		"https://example.com:8443/path",
		"https://example.com/path?query=value#fragment",
		"http://192.0.2.1/path",
		"https://[2001:db8::1]/path",
		"https://example.com/a%20path",
		"https://例え.テスト/path",
	}
	for _, destination := range accepted {
		t.Run("accept "+destination, func(t *testing.T) {
			if err := validateExternalURL(destination); err != nil {
				t.Fatalf("validateExternalURL() error = %v", err)
			}
		})
	}

	rejected := []string{
		"",
		"   ",
		" https://example.com",
		"https://example.com ",
		"example.com",
		"/relative/path",
		"./relative",
		"../relative",
		"//example.com/path",
		"file:///tmp/example",
		"ftp://example.com/file",
		"mailto:user@example.com",
		"javascript:alert(1)",
		"data:text/plain,test",
		"blob:https://example.com/id",
		"custom:example",
		"http:///missing-host",
		"https://",
		"https://user:pass@example.com",
		"http:example.com",
		"https:example.com/path",
		"https://example.com:bad/path",
		`https:\\example.com\path`,
		`https://example.com\path`,
	}
	for _, destination := range rejected {
		t.Run("reject "+destination, func(t *testing.T) {
			if err := validateExternalURL(destination); err == nil {
				t.Fatal("validateExternalURL() error = nil")
			}
		})
	}
}

func TestOpenExternalURLValidatesBeforeOpening(t *testing.T) {
	var opened []string
	opener := func(destination string) error {
		opened = append(opened, destination)
		return nil
	}

	valid := "https://example.com/path?query=value#fragment"
	if err := openExternalURL(valid, opener); err != nil {
		t.Fatalf("openExternalURL(valid) error = %v", err)
	}
	if len(opened) != 1 || opened[0] != valid {
		t.Fatalf("opener calls = %q, want original URL %q", opened, valid)
	}

	if err := openExternalURL("file:///tmp/secret", opener); err == nil {
		t.Fatal("openExternalURL(rejected) error = nil")
	}
	if len(opened) != 1 {
		t.Fatalf("opener was called for rejected URL: %q", opened)
	}
}

func TestOpenExternalURLPropagatesOpenerError(t *testing.T) {
	wantErr := errors.New("opener failed")
	err := openExternalURL("https://example.com", func(string) error { return wantErr })
	if !errors.Is(err, wantErr) {
		t.Fatalf("openExternalURL() error = %v, want wrapped %v", err, wantErr)
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
