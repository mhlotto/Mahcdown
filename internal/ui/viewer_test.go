package ui

import (
	"html"
	"strings"
	"testing"
)

func TestInjectBaseEscapesAttributeValue(t *testing.T) {
	baseDir := `/tmp/a "quote" 'single' & <script data-injected="true"> spaces`
	input := `<!DOCTYPE html><html><head><meta charset="utf-8"></head><body></body></html>`

	got := injectBase(input, baseDir)
	wantBase := `file:///tmp/a &#34;quote&#34; &#39;single&#39; &amp; &lt;script data-injected=&#34;true&#34;&gt; spaces/`
	wantTag := `<base href="` + wantBase + `">`

	if !strings.Contains(got, `<head>`+wantTag+`<meta charset="utf-8">`) {
		t.Fatalf("base element was not safely inserted into head:\n%s", got)
	}
	if strings.Count(got, `<base `) != 1 {
		t.Fatalf("expected exactly one base element, got:\n%s", got)
	}
	if strings.Contains(got, `<script`) || strings.Contains(got, `data-injected="true"`) {
		t.Fatalf("path injected markup or an attribute:\n%s", got)
	}

	start := strings.Index(got, `<base href="`)
	if start < 0 {
		t.Fatalf("base href is not a quoted attribute:\n%s", got)
	}
	start += len(`<base href="`)
	end := strings.Index(got[start:], `">`)
	if end < 0 {
		t.Fatalf("base href attribute is not structurally closed:\n%s", got)
	}
	if decoded := html.UnescapeString(got[start : start+end]); decoded != `file://`+baseDir+`/` {
		t.Fatalf("decoded base href = %q, want %q", decoded, `file://`+baseDir+`/`)
	}
}

func TestInjectBasePreservesOrdinaryPath(t *testing.T) {
	input := `<!DOCTYPE html><html><head></head><body></body></html>`
	got := injectBase(input, `/Users/example/My Notes`)
	want := `<head><base href="file:///Users/example/My Notes/">`

	if !strings.Contains(got, want) {
		t.Fatalf("injectBase() = %q, want it to contain %q", got, want)
	}
}
