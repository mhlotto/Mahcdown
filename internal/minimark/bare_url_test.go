package minimark

import (
	"html"
	"strings"
	"testing"
)

func TestBareURLPreservesEveryExcludedClosingDelimiter(t *testing.T) {
	for _, closing := range []string{")", "]", "}", ">", `"`, "'"} {
		input := "https://example.com/path" + closing + "."
		assertApostropheInlines(t, input, []Inline{
			Url{URL: "https://example.com/path", Text: "https://example.com/path"},
			Text{Text: closing + "."},
		})
	}
}

func TestBareURLPreservesSurroundingQuotes(t *testing.T) {
	tests := []struct {
		input string
		quote string
	}{
		{`"https://example.com/path"`, `"`},
		{"'https://example.com/path'", "'"},
	}
	for _, tt := range tests {
		assertApostropheInlines(t, tt.input, []Inline{
			Text{Text: tt.quote},
			Url{URL: "https://example.com/path", Text: "https://example.com/path"},
			Text{Text: tt.quote},
		})
	}
}

func TestBareURLIncludesBalancedParentheses(t *testing.T) {
	tests := []struct {
		input string
		url   string
		left  string
		right string
	}{
		{"https://example.com/a_(b)", "https://example.com/a_(b)", "", ""},
		{"https://example.com/a_(b_(c))", "https://example.com/a_(b_(c))", "", ""},
		{"(https://example.com/a_(b))", "https://example.com/a_(b)", "(", ")"},
		{"https://example.com/a_(b)).", "https://example.com/a_(b)", "", ")."},
		{"https://example.com/path))).", "https://example.com/path", "", ")))."},
		{"https://example.com/a_(b).", "https://example.com/a_(b)", "", "."},
	}
	for _, tt := range tests {
		var want []Inline
		if tt.left != "" {
			want = append(want, Text{Text: tt.left})
		}
		want = append(want, Url{URL: tt.url, Text: tt.url})
		if tt.right != "" {
			want = append(want, Text{Text: tt.right})
		}
		assertApostropheInlines(t, tt.input, want)
	}
}

func TestBareURLMultipleIndependentURLsPreservePunctuation(t *testing.T) {
	input := "https://one.example/a_(b)), then https://two.example/path]."
	assertApostropheInlines(t, input, []Inline{
		Url{URL: "https://one.example/a_(b)", Text: "https://one.example/a_(b)"},
		Text{Text: "), then "},
		Url{URL: "https://two.example/path", Text: "https://two.example/path"},
		Text{Text: "]."},
	})
}

func TestBareURLRenderedHTMLPreservesSurroundingPunctuation(t *testing.T) {
	input := `(https://example.com/a_(b)). "https://example.com/quoted"`
	rendered := html.UnescapeString(RenderHTML(mustParseApostrophe(t, input)))
	for _, fragment := range []string{
		`(<a class="mahcdown-link" role="link" tabindex="0" data-mahcdown-href="https://example.com/a_(b)">https://example.com/a_(b)</a>).`,
		`"<a class="mahcdown-link" role="link" tabindex="0" data-mahcdown-href="https://example.com/quoted">https://example.com/quoted</a>"`,
	} {
		if !strings.Contains(rendered, fragment) {
			t.Fatalf("rendered HTML lost URL punctuation %q: %s", fragment, rendered)
		}
	}
}

func TestBareURLLargeBalancedParenthesesIsLinearAndSafe(t *testing.T) {
	const depth = 50_000
	input := "https://example.com/" + strings.Repeat("(", depth) + "x" + strings.Repeat(")", depth) + ")."
	url, consumed := parseBareURL(input)
	if consumed != len(input)-2 {
		t.Fatalf("consumed = %d, want %d", consumed, len(input)-2)
	}
	if url != input[:consumed] {
		t.Fatalf("large URL was changed")
	}
}
