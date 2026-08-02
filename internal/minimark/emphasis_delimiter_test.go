package minimark

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestAsteriskDelimiterCentralRegression(t *testing.T) {
	want := []Inline{
		Strong{Inlines: []Inline{Text{Text: "one"}}},
		Text{Text: " and "},
		Strong{Inlines: []Inline{Text{Text: "two"}}},
	}
	assertEmphasisInlines(t, "**one** and **two**", want)
	if html := RenderHTML(mustParseEmphasis(t, "**one** and **two**")); !strings.Contains(html, `<strong>one</strong> and <strong>two</strong>`) {
		t.Fatalf("central regression HTML mismatch: %s", html)
	}
}

func TestAsteriskDelimiterRuns(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []Inline
	}{
		{name: "basic emphasis", input: "*foo*", want: []Inline{Emphasis{Inlines: []Inline{Text{Text: "foo"}}}}},
		{name: "basic strong", input: "**foo**", want: []Inline{Strong{Inlines: []Inline{Text{Text: "foo"}}}}},
		{name: "independent with surrounding text", input: "before **one** middle **two** after", want: []Inline{Text{Text: "before "}, Strong{Inlines: []Inline{Text{Text: "one"}}}, Text{Text: " middle "}, Strong{Inlines: []Inline{Text{Text: "two"}}}, Text{Text: " after"}}},
		{name: "adjacent ambiguous rule of three", input: "**one****two**", want: []Inline{Strong{Inlines: []Inline{Text{Text: "one****two"}}}}},
		{name: "nested emphasis", input: "**bold *italic* text**", want: []Inline{Strong{Inlines: []Inline{Text{Text: "bold "}, Emphasis{Inlines: []Inline{Text{Text: "italic"}}}, Text{Text: " text"}}}}},
		{name: "nested strong", input: "*italic **bold** text*", want: []Inline{Emphasis{Inlines: []Inline{Text{Text: "italic "}, Strong{Inlines: []Inline{Text{Text: "bold"}}}, Text{Text: " text"}}}}},
		{name: "triple run", input: "***text***", want: []Inline{Emphasis{Inlines: []Inline{Strong{Inlines: []Inline{Text{Text: "text"}}}}}}},
		{name: "nested middle", input: "**foo *bar* baz**", want: []Inline{Strong{Inlines: []Inline{Text{Text: "foo "}, Emphasis{Inlines: []Inline{Text{Text: "bar"}}}, Text{Text: " baz"}}}}},
		{name: "four star nesting", input: "****text****", want: []Inline{Strong{Inlines: []Inline{Strong{Inlines: []Inline{Text{Text: "text"}}}}}}},
		{name: "strong with extra closer", input: "**foo***", want: []Inline{Strong{Inlines: []Inline{Text{Text: "foo"}}}, Text{Text: "*"}}},
		// CommonMark examples 412-414 exercise the rule-of-three restriction.
		{name: "rule of three blocks interior pair", input: "*foo**bar*", want: []Inline{Emphasis{Inlines: []Inline{Text{Text: "foo**bar"}}}}},
		{name: "rule of three nested strong", input: "***foo** bar*", want: []Inline{Emphasis{Inlines: []Inline{Strong{Inlines: []Inline{Text{Text: "foo"}}}, Text{Text: " bar"}}}}},
		{name: "rule of three closing run", input: "*foo **bar***", want: []Inline{Emphasis{Inlines: []Inline{Text{Text: "foo "}, Strong{Inlines: []Inline{Text{Text: "bar"}}}}}}},
		// CommonMark example 416 permits matching when both runs are multiples of three.
		{name: "both runs divisible by three", input: "foo***bar***baz", want: []Inline{Text{Text: "foo"}, Emphasis{Inlines: []Inline{Strong{Inlines: []Inline{Text{Text: "bar"}}}}}, Text{Text: "baz"}}},
		{name: "intraword emphasis", input: "foo*bar*baz", want: []Inline{Text{Text: "foo"}, Emphasis{Inlines: []Inline{Text{Text: "bar"}}}, Text{Text: "baz"}}},
		{name: "intraword strong", input: "foo**bar**baz", want: []Inline{Text{Text: "foo"}, Strong{Inlines: []Inline{Text{Text: "bar"}}}, Text{Text: "baz"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { assertEmphasisInlines(t, tt.input, tt.want) })
	}
	for input, fragment := range map[string]string{
		"before **one** middle **two** after": "before <strong>one</strong> middle <strong>two</strong> after",
		"**one****two**":                      "<strong>one****two</strong>",
	} {
		if html := RenderHTML(mustParseEmphasis(t, input)); !strings.Contains(html, fragment) {
			t.Errorf("RenderHTML(%q) does not contain %q: %s", input, fragment, html)
		}
	}
}

func TestAsteriskDelimiterAmbiguousRunsFollowAlgorithm(t *testing.T) {
	tests := []struct {
		input string
		want  []Inline
		html  string
	}{
		{
			input: "**one****two**",
			want:  []Inline{Strong{Inlines: []Inline{Text{Text: "one****two"}}}},
			html:  "<strong>one****two</strong>",
		},
		{
			input: "***foo***",
			want:  []Inline{Emphasis{Inlines: []Inline{Strong{Inlines: []Inline{Text{Text: "foo"}}}}}},
			html:  "<em><strong>foo</strong></em>",
		},
		{
			input: "******foo******",
			want:  []Inline{Strong{Inlines: []Inline{Strong{Inlines: []Inline{Strong{Inlines: []Inline{Text{Text: "foo"}}}}}}}},
			html:  "<strong><strong><strong>foo</strong></strong></strong>",
		},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assertEmphasisInlines(t, tt.input, tt.want)
			if html := RenderHTML(mustParseEmphasis(t, tt.input)); !strings.Contains(html, tt.html) {
				t.Fatalf("rendered HTML does not contain %q: %s", tt.html, html)
			}
		})
	}
}

func TestAsteriskDelimiterUsesUnicodeFlanking(t *testing.T) {
	assertRawInlineParse(t, "“*κόσμος*”", []Inline{Text{Text: "“"}, Emphasis{Inlines: []Inline{Text{Text: "κόσμος"}}}, Text{Text: "”"}})
	assertRawInlineParse(t, "α*β*γ", []Inline{Text{Text: "α"}, Emphasis{Inlines: []Inline{Text{Text: "β"}}}, Text{Text: "γ"}})
	assertRawInlineParse(t, "*\u00a0foo*", []Inline{Text{Text: "*\u00a0foo*"}})
}

func TestAsteriskDelimiterFlankingAndFallback(t *testing.T) {
	for _, input := range []string{
		"* foo*", "*foo *", "** foo**", "**foo **", "**unclosed", "*unclosed", "plain * text", "foo *****",
	} {
		assertRawInlineParse(t, input, []Inline{Text{Text: input}})
	}
}

func TestAsteriskDelimiterCrossesOnlySoftLineBreaks(t *testing.T) {
	assertEmphasisInlines(t, "**first\nsecond**", []Inline{Strong{Inlines: []Inline{Text{Text: "first\nsecond"}}}})
	assertEmphasisInlines(t, "*first\nsecond*", []Inline{Emphasis{Inlines: []Inline{Text{Text: "first\nsecond"}}}})

	doc := mustParseEmphasis(t, "**first\n\nsecond**")
	if len(doc.Blocks) != 2 {
		t.Fatalf("blank line produced %d blocks, want 2", len(doc.Blocks))
	}
	for _, block := range doc.Blocks {
		paragraph := block.(Paragraph)
		for _, inline := range paragraph.Inlines {
			if _, ok := inline.(Strong); ok {
				t.Fatalf("strong delimiter crossed a paragraph boundary: %#v", doc)
			}
		}
	}
}

func TestAsteriskDelimiterAtomicInlineOpacity(t *testing.T) {
	tests := []struct {
		input string
		want  []Inline
	}{
		{"`**not strong**` and **strong**", []Inline{CodeSpan{Text: "**not strong**"}, Text{Text: " and "}, Strong{Inlines: []Inline{Text{Text: "strong"}}}}},
		{"**code `*` remains code**", []Inline{Strong{Inlines: []Inline{Text{Text: "code "}, CodeSpan{Text: "*"}, Text{Text: " remains code"}}}}},
		{"![*alt*](image.png) **strong**", []Inline{Image{Alt: "*alt*", URL: "image.png"}, Text{Text: " "}, Strong{Inlines: []Inline{Text{Text: "strong"}}}}},
		{"<https://example.com/*path*> **strong**", []Inline{Url{URL: "https://example.com/*path*", Text: "https://example.com/*path*"}, Text{Text: " "}, Strong{Inlines: []Inline{Text{Text: "strong"}}}}},
		{"https://example.com/*path* **strong**", []Inline{Url{URL: "https://example.com/*path*", Text: "https://example.com/*path*"}, Text{Text: " "}, Strong{Inlines: []Inline{Text{Text: "strong"}}}}},
		{"[ ] *task*", []Inline{Checkbox{Checked: false}, Emphasis{Inlines: []Inline{Text{Text: "task"}}}}},
	}
	for _, tt := range tests {
		assertEmphasisInlines(t, tt.input, tt.want)
	}
}

func TestAsteriskDelimiterLimits(t *testing.T) {
	if _, err := ParseWithLimits("*", Limits{MaxParseItems: 6}); err != nil {
		t.Fatalf("one unmatched delimiter at exact budget failed: %v", err)
	}
	doc, err := ParseWithLimits("*", Limits{MaxParseItems: 5})
	if !errors.Is(err, ErrParseItemLimit) || len(doc.Blocks) != 0 {
		t.Fatalf("delimiter budget failure = (%#v, %v), want empty document and ErrParseItemLimit", doc, err)
	}

	nested := nestedAsteriskEmphasis(8)
	if _, err := ParseWithLimits(nested, Limits{MaxNestingDepth: 8}); err != nil {
		t.Fatalf("nesting exactly at limit failed: %v", err)
	}
	doc, err = ParseWithLimits(nested, Limits{MaxNestingDepth: 7})
	if !errors.Is(err, ErrNestingDepthLimit) || len(doc.Blocks) != 0 {
		t.Fatalf("nesting failure = (%#v, %v), want empty document and ErrNestingDepthLimit", doc, err)
	}

	largeUnmatched := "plain " + strings.Repeat("* ", 2_000)
	if _, err := Parse(largeUnmatched); err != nil {
		t.Fatalf("large unmatched delimiter input failed: %v", err)
	}
	alternatingUnmatched := "plain " + strings.Repeat("a***b ", 2_000)
	if _, err := Parse(alternatingUnmatched); err != nil {
		t.Fatalf("large alternating delimiter input failed: %v", err)
	}
}

func TestAsteriskDelimiterDoesNotCrossTableCells(t *testing.T) {
	html := RenderHTML(mustParseEmphasis(t, "| *left | right* |\n|---|---|"))
	if strings.Contains(html, "<em>") {
		t.Fatalf("emphasis crossed a table-cell boundary: %s", html)
	}
}

func TestAsteriskDelimiterBlockIntegration(t *testing.T) {
	input := "# **heading**\n\n> *quote*\n\n- **item**\n\n| *cell* | **cell** |\n|---|---|"
	html := RenderHTML(mustParseEmphasis(t, input))
	for _, fragment := range []string{"<h1><strong>heading</strong></h1>", "<blockquote><p><em>quote</em></p></blockquote>", "<li><p><strong>item</strong></p>", "<em>cell</em>", "<strong>cell</strong>"} {
		if !strings.Contains(html, fragment) {
			t.Errorf("integration HTML missing %q: %s", fragment, html)
		}
	}
}

func nestedAsteriskEmphasis(depth int) string {
	text := "x"
	for level := depth - 1; level >= 0; level-- {
		if level%2 == 0 {
			text = "**s " + text + " s**"
		} else {
			text = "*e " + text + " e*"
		}
	}
	return text
}

func assertEmphasisInlines(t *testing.T, input string, want []Inline) {
	t.Helper()
	doc := mustParseEmphasis(t, input)
	paragraph, ok := doc.Blocks[0].(Paragraph)
	if !ok {
		t.Fatalf("first block for %q = %T, want Paragraph", input, doc.Blocks[0])
	}
	if !reflect.DeepEqual(paragraph.Inlines, want) {
		t.Fatalf("inlines for %q\nwant: %#v\ngot:  %#v", input, want, paragraph.Inlines)
	}
}

func assertRawInlineParse(t *testing.T, input string, want []Inline) {
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

func mustParseEmphasis(t *testing.T, input string) Document {
	t.Helper()
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse(%q): %v", input, err)
	}
	return doc
}
