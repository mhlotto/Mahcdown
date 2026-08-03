package minimark

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestCodeSpanDelimiterRuns(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []Inline
	}{
		{name: "single", input: "`code`", want: []Inline{CodeSpan{Text: "code"}}},
		{name: "double", input: "``code``", want: []Inline{CodeSpan{Text: "code"}}},
		{name: "triple", input: "```code```", want: []Inline{CodeSpan{Text: "code"}}},
		{name: "embedded single", input: "``foo ` bar``", want: []Inline{CodeSpan{Text: "foo ` bar"}}},
		{name: "embedded double", input: "```foo``bar```", want: []Inline{CodeSpan{Text: "foo``bar"}}},
		{name: "longer interior run", input: "`foo``bar`", want: []Inline{CodeSpan{Text: "foo``bar"}}},
		{name: "longer does not close", input: "``foo```", want: []Inline{Text{Text: "``foo```"}}},
		{name: "shorter does not close", input: "```foo``", want: []Inline{Text{Text: "```foo``"}}},
		{name: "first equal closer", input: "``one`` two ``three``", want: []Inline{CodeSpan{Text: "one"}, Text{Text: " two "}, CodeSpan{Text: "three"}}},
		{name: "multiple independent", input: "`one` and ``two``", want: []Inline{CodeSpan{Text: "one"}, Text{Text: " and "}, CodeSpan{Text: "two"}}},
		{name: "adjacent backticks form one maximal run", input: "`one``two`", want: []Inline{CodeSpan{Text: "one``two"}}},
		{name: "neighboring spans", input: "`one` `two`", want: []Inline{CodeSpan{Text: "one"}, Text{Text: " "}, CodeSpan{Text: "two"}}},
		{name: "unmatched before valid", input: "```unclosed ``ok``", want: []Inline{Text{Text: "```unclosed "}, CodeSpan{Text: "ok"}}},
		{name: "empty surrounding prose", input: "before `code` after", want: []Inline{Text{Text: "before "}, CodeSpan{Text: "code"}, Text{Text: " after"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { assertCodeSpanInlines(t, tt.input, tt.want) })
	}
}

func TestNormalizeCodeSpanContent(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: " foo ", want: "foo"},
		{input: "  foo  ", want: " foo "},
		{input: "   ", want: "   "},
		{input: "foo  bar", want: "foo  bar"},
		{input: "foo\nbar", want: "foo bar"},
		{input: "foo\n\nbar", want: "foo  bar"},
		{input: "foo\r\nbar\rbaz", want: "foo bar baz"},
		{input: "foo\tbar", want: "foo\tbar"},
		{input: "\tfoo\t", want: "\tfoo\t"},
		{input: "\u00a0foo\u00a0", want: "\u00a0foo\u00a0"},
		{input: " ` ", want: "`"},
	}
	for _, tt := range tests {
		if got := normalizeCodeSpanContent(tt.input); got != tt.want {
			t.Errorf("normalizeCodeSpanContent(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCodeSpanIsOpaqueAndInlineParsingResumes(t *testing.T) {
	input := "``**not strong** `x` https://example.com ![alt](image.png) \\`` and **strong**"
	want := []Inline{
		CodeSpan{Text: "**not strong** `x` https://example.com ![alt](image.png) \\"},
		Text{Text: " and "},
		Strong{Inlines: []Inline{Text{Text: "strong"}}},
	}
	assertCodeSpanInlines(t, input, want)
}

func TestCodeSpanRenderingAndCSS(t *testing.T) {
	html := RenderHTML(mustParseCodeSpan(t, "``< > & \" '  spaces``"))
	for _, escaped := range []string{"&lt;", "&gt;", "&amp;", "&#34;", "&#39;", " spaces"} {
		if !strings.Contains(html, escaped) {
			t.Errorf("rendered code span missing %q: %s", escaped, html)
		}
	}
	if !strings.Contains(baseCSS, ":not(pre)>code{white-space:pre-wrap;}") {
		t.Fatalf("baseCSS does not preserve inline-code spaces: %s", baseCSS)
	}
}

func TestFencedCodeKeepsPreformattedHorizontalScrolling(t *testing.T) {
	html := RenderHTML(mustParseCodeSpan(t, "```\na very long fenced code line\n```"))
	if !strings.Contains(html, "<pre><code>") {
		t.Fatalf("fenced code did not render as pre/code: %s", html)
	}
	for _, rule := range []string{
		"pre{background:#e8ebf0;padding:12px;border-radius:6px;overflow:auto;}",
		"pre code{white-space:pre;}",
	} {
		if !strings.Contains(baseCSS, rule) {
			t.Fatalf("baseCSS missing non-wrapping fenced-code rule %q: %s", rule, baseCSS)
		}
	}
}

func TestCodeSpanRenderingAcrossBlocks(t *testing.T) {
	input := "# `heading`\n\n> `quote`\n\n- `item`\n\n| `head` |\n|---|\n| `cell` |"
	html := RenderHTML(mustParseCodeSpan(t, input))
	for _, fragment := range []string{
		"<h1><code>heading</code></h1>",
		"<blockquote><p><code>quote</code></p></blockquote>",
		"<li><p><code>item</code></p>",
		"<td><code>cell</code></td>",
	} {
		if !strings.Contains(html, fragment) {
			t.Errorf("rendered blocks missing %q: %s", fragment, html)
		}
	}
}

func TestCodeSpanParseItemBudget(t *testing.T) {
	if _, err := ParseWithLimits("`code`", Limits{MaxParseItems: 7}); err != nil {
		t.Fatalf("code span at exact parse-item budget failed: %v", err)
	}
	doc, err := ParseWithLimits("`code`", Limits{MaxParseItems: 6})
	if !errors.Is(err, ErrParseItemLimit) || len(doc.Blocks) != 0 {
		t.Fatalf("code span budget failure = (%#v, %v), want empty document and ErrParseItemLimit", doc, err)
	}
}

func TestCodeSpanLargeRobustnessInputs(t *testing.T) {
	manyMatched := "plain " + strings.Repeat("`x` ", 10_000)
	if _, err := Parse(manyMatched); err != nil {
		t.Fatalf("many matched code spans failed: %v", err)
	}

	var unmatched strings.Builder
	unmatched.WriteString("plain ")
	for length := 1; length <= 500; length++ {
		unmatched.WriteString(strings.Repeat("`", length))
		unmatched.WriteByte(' ')
	}
	if _, err := Parse(unmatched.String()); err != nil {
		t.Fatalf("differently-sized unmatched runs failed: %v", err)
	}

	largeFinalUnmatched := "plain " + strings.Repeat("`", 100_000)
	if _, err := Parse(largeFinalUnmatched); err != nil {
		t.Fatalf("large final unmatched run failed: %v", err)
	}
}

func assertCodeSpanInlines(t *testing.T, input string, want []Inline) {
	t.Helper()
	limits, err := normalizedLimits(Limits{})
	if err != nil {
		t.Fatal(err)
	}
	state := &parserState{limits: limits}
	got := parseInlines(state, input, 0)
	if state.err != nil {
		t.Fatalf("parseInlines(%q): %v", input, state.err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("inlines for %q\nwant: %#v\ngot:  %#v", input, want, got)
	}
}

func mustParseCodeSpan(t *testing.T, input string) Document {
	t.Helper()
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse(%q): %v", input, err)
	}
	return doc
}
