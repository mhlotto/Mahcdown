package ui

import (
	"errors"
	"html"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mahcdown/internal/minimark"
	"mahcdown/internal/source"
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
	generated := minimark.RenderHTMLWithImagePolicy(minimark.Document{Blocks: []minimark.Block{
		minimark.Paragraph{Inlines: []minimark.Inline{
			minimark.Image{URL: "images/local.png", Alt: "local"},
			minimark.Image{URL: "https://example.com/remote.png", Alt: "remote"},
		}},
	}}, minimark.AllowOutsideImagePolicy())
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
		`invokeNativeAction('Open link', window.mahcdownOpenLink, {}, url)`,
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

func TestRegisterViewerBindings(t *testing.T) {
	wantNames := []string{"mahcdownReload", "mahcdownQuit", "mahcdownOpenLink"}
	var gotNames []string
	err := registerViewerBindings(func(name string, callback interface{}) error {
		gotNames = append(gotNames, name)
		if callback == nil {
			t.Fatalf("binding %q received nil callback", name)
		}
		return nil
	}, func() (string, error) { return "ok", nil }, func() {}, func(string) error { return nil })
	if err != nil {
		t.Fatalf("registerViewerBindings() error = %v", err)
	}
	if strings.Join(gotNames, ",") != strings.Join(wantNames, ",") {
		t.Fatalf("registered names = %v, want %v", gotNames, wantNames)
	}
}

func TestRegisterViewerBindingsStopsAtFirstFailure(t *testing.T) {
	sentinel := errors.New("bind failed")
	tests := []struct {
		name        string
		failName    string
		wantContext string
		wantCalls   []string
	}{
		{name: "reload", failName: "mahcdownReload", wantContext: "bind reload action", wantCalls: []string{"mahcdownReload"}},
		{name: "quit", failName: "mahcdownQuit", wantContext: "bind quit action", wantCalls: []string{"mahcdownReload", "mahcdownQuit"}},
		{name: "external link", failName: "mahcdownOpenLink", wantContext: "bind external-link action", wantCalls: []string{"mahcdownReload", "mahcdownQuit", "mahcdownOpenLink"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls []string
			err := registerViewerBindings(func(name string, _ interface{}) error {
				calls = append(calls, name)
				if name == tt.failName {
					return sentinel
				}
				return nil
			}, func() (string, error) { return "ok", nil }, func() {}, func(string) error { return nil })
			if !errors.Is(err, sentinel) || !strings.Contains(err.Error(), tt.wantContext) {
				t.Fatalf("registration error = %v, want wrapped sentinel with %q", err, tt.wantContext)
			}
			if strings.Join(calls, ",") != strings.Join(tt.wantCalls, ",") {
				t.Fatalf("binding calls = %v, want %v", calls, tt.wantCalls)
			}
		})
	}
}

func TestReloadDocumentIsTransactionalAndContextual(t *testing.T) {
	readErr := errors.New("read failed")
	renderErr := errors.New("render failed")
	updateErr := errors.New("update failed")
	tests := []struct {
		name        string
		read        func() (string, error)
		render      func(string) (string, error)
		update      func(string) error
		wantErr     error
		wantContext string
		wantUpdates int
	}{
		{name: "read", read: func() (string, error) { return "", readErr }, render: func(string) (string, error) { t.Fatal("render called after read failure"); return "", nil }, update: func(string) error { t.Fatal("update called after read failure"); return nil }, wantErr: readErr, wantContext: "reload document: read source"},
		{name: "render", read: func() (string, error) { return "source", nil }, render: func(string) (string, error) { return "partial", renderErr }, update: func(string) error { t.Fatal("update called after render failure"); return nil }, wantErr: renderErr, wantContext: "reload document: render source"},
		{name: "update", read: func() (string, error) { return "source", nil }, render: func(string) (string, error) { return "complete HTML", nil }, update: func(string) error { return updateErr }, wantErr: updateErr, wantContext: "reload document: update view", wantUpdates: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updates := 0
			update := func(html string) error {
				updates++
				return tt.update(html)
			}
			err := reloadDocument(tt.read, tt.render, update)
			if !errors.Is(err, tt.wantErr) || !strings.Contains(err.Error(), tt.wantContext) {
				t.Fatalf("reloadDocument() error = %v, want wrapped error with %q", err, tt.wantContext)
			}
			if updates != tt.wantUpdates {
				t.Fatalf("update calls = %d, want %d", updates, tt.wantUpdates)
			}
		})
	}
}

func TestReloadDocumentSuccessUsesDecodedCompleteHTML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "document.md")
	if err := os.WriteFile(path, []byte{'I', 't', 0x92, 's'}, 0o644); err != nil {
		t.Fatal(err)
	}
	updates := 0
	var updatedHTML string
	err := reloadDocument(
		func() (string, error) { return source.ReadTextFile(path, minimark.DefaultMaxSourceBytes) },
		func(content string) (string, error) {
			if content != "It’s" {
				t.Fatalf("render content = %q, want decoded Windows-1252", content)
			}
			return "<html>complete</html>", nil
		},
		func(html string) error {
			updates++
			updatedHTML = html
			return nil
		},
	)
	if err != nil {
		t.Fatalf("reloadDocument() error = %v", err)
	}
	if updates != 1 || updatedHTML != "<html>complete</html>" {
		t.Fatalf("reload update = (%d, %q), want one complete update", updates, updatedHTML)
	}
}

func TestReloadDocumentPreservesLimitErrors(t *testing.T) {
	readLimit := &minimark.LimitError{Kind: minimark.ErrSourceSizeLimit, Limit: 4}
	err := reloadDocument(
		func() (string, error) { return "", readLimit },
		func(string) (string, error) { t.Fatal("render called after source-size failure"); return "", nil },
		func(string) error { t.Fatal("update called after source-size failure"); return nil },
	)
	if !errors.Is(err, minimark.ErrSourceSizeLimit) {
		t.Fatalf("wrapped read error = %v, want ErrSourceSizeLimit", err)
	}

	parseLimit := &minimark.LimitError{Kind: minimark.ErrParseItemLimit, Limit: 4}
	err = reloadDocument(
		func() (string, error) { return "source", nil },
		func(string) (string, error) { return "", parseLimit },
		func(string) error { t.Fatal("update called after parser failure"); return nil },
	)
	if !errors.Is(err, minimark.ErrParseItemLimit) {
		t.Fatalf("wrapped render error = %v, want ErrParseItemLimit", err)
	}
}

func TestShortcutJSReportsNativeActionFailures(t *testing.T) {
	js := shortcutJS()
	required := []string{
		`actionError.id = 'mahcdown-action-error'`,
		`actionError.setAttribute('role', 'alert')`,
		`actionError.setAttribute('aria-live', 'assertive')`,
		`actionError.textContent = label + ' failed: ' + errorMessage(error)`,
		`const invokeNativeAction =`,
		`typeof action !== 'function'`,
		`new Error('native action unavailable')`,
		`Promise.resolve(action(...args))`,
		`if (options.clearOnSuccess) clearActionError()`,
		`.catch((error) =>`,
		`} catch (error) {`,
		`invokeNativeAction('Reload', window.mahcdownReload, {clearOnSuccess: true})`,
		`invokeNativeAction('Quit', window.mahcdownQuit)`,
		`invokeNativeAction('Open link', window.mahcdownOpenLink, {}, url)`,
		`if (ok) {`,
		`if (isSearchOpen()) {`,
		`if (isSearchFocused()) {`,
		`!e.metaKey && !e.ctrlKey`,
	}
	for _, fragment := range required {
		if !strings.Contains(js, fragment) {
			t.Errorf("shortcutJS() is missing %q", fragment)
		}
	}
	if count := strings.Count(js, `invokeNativeAction('Quit', window.mahcdownQuit)`); count != 2 {
		t.Errorf("quit helper call count = %d, want 2", count)
	}
	for _, direct := range []string{
		`window.mahcdownReload();`,
		`window.mahcdownQuit();`,
		`window.mahcdownOpenLink(url);`,
		`actionError.innerHTML`,
	} {
		if strings.Contains(js, direct) {
			t.Errorf("shortcutJS() contains unsafe/unhandled action %q", direct)
		}
	}
}

func TestShortcutJSLeavesCopyToNativeBrowser(t *testing.T) {
	js := shortcutJS()
	for _, forbidden := range []string{
		`document.execCommand`,
		`execCommand('copy')`,
		`navigator.clipboard`,
		`writeText(`,
		`e.key === 'c'`,
		`e.key === 'C'`,
	} {
		if strings.Contains(js, forbidden) {
			t.Errorf("shortcutJS() still intercepts native copying via %q", forbidden)
		}
	}
	for _, preserved := range []string{
		`(e.key === 'f' || e.key === 'F')`,
		`if (e.key === 'Escape')`,
		`(e.key === 'r' || e.key === 'R')`,
		`if (e.key === 'q' || e.key === 'Q')`,
		`openSearch()`,
		`invokeNativeAction('Reload', window.mahcdownReload, {clearOnSuccess: true})`,
		`invokeNativeAction('Quit', window.mahcdownQuit)`,
	} {
		if !strings.Contains(js, preserved) {
			t.Errorf("shortcutJS() lost surrounding shortcut behavior %q", preserved)
		}
	}
}

func TestShortcutJSSearchExcludesOnlyViewerUI(t *testing.T) {
	js := shortcutJS()
	for _, fragment := range []string{
		`document.createTreeWalker(`,
		`document.body,`,
		`parent.closest('#mahcdown-search, #mahcdown-action-error')`,
		`return NodeFilter.FILTER_ACCEPT`,
	} {
		if !strings.Contains(js, fragment) {
			t.Errorf("shortcutJS() search logic is missing %q", fragment)
		}
	}
	for _, broadExclusion := range []string{
		`parent.closest('[style*=fixed]')`,
		`parent.closest('[id]')`,
	} {
		if strings.Contains(js, broadExclusion) {
			t.Errorf("shortcutJS() broadly excludes document text via %q", broadExclusion)
		}
	}
}

func TestRenderDocumentAppliesSelectedImagePolicyOnEveryRender(t *testing.T) {
	temp := t.TempDir()
	root := filepath.Join(temp, "document")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(temp, "outside-image.png")
	if err := os.WriteFile(outside, []byte("image"), 0o644); err != nil {
		t.Fatal(err)
	}
	restricted, err := imagePolicyFor(root, Options{})
	if err != nil {
		t.Fatalf("imagePolicyFor(default) error = %v", err)
	}
	for _, content := range []string{
		`![initial](../outside-image.png)`,
		`![reloaded](../outside-image.png)`,
	} {
		got, err := renderDocument(content, root, restricted)
		if err != nil {
			t.Fatalf("renderDocument() error = %v", err)
		}
		if strings.Contains(got, `<img src=`) || !strings.Contains(got, `image-blocked`) {
			t.Fatalf("restricted render allowed outside image: %s", got)
		}
	}

	permissive, err := imagePolicyFor(root, Options{AllowOutsideLocalImages: true})
	if err != nil {
		t.Fatalf("imagePolicyFor(override) error = %v", err)
	}
	for _, content := range []string{
		`![initial](../outside-image.png)`,
		`![reloaded](../outside-image.png)`,
	} {
		got, err := renderDocument(content, root, permissive)
		if err != nil {
			t.Fatalf("renderDocument() error = %v", err)
		}
		if !strings.Contains(got, `<img src="../outside-image.png"`) {
			t.Fatalf("outside-image override was not applied: %s", got)
		}
	}
}

func TestRenderDocumentPropagatesParserLimitsWithoutPartialHTML(t *testing.T) {
	root := t.TempDir()
	policy, err := minimark.NewRestrictedImagePolicy(root)
	if err != nil {
		t.Fatal(err)
	}
	limits := minimark.Limits{MaxSourceBytes: 4}
	for _, phase := range []string{"initial", "reload"} {
		html, err := renderDocumentWithLimits("12345", root, policy, limits)
		if !errors.Is(err, minimark.ErrSourceSizeLimit) {
			t.Fatalf("%s render error = %v, want ErrSourceSizeLimit", phase, err)
		}
		if html != "" {
			t.Fatalf("%s limit failure returned partial HTML: %q", phase, html)
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
