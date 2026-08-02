package minimark

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestBlockQuoteBasicStructureAndTermination(t *testing.T) {
	input := "before\n\n>first line\n> second line\n>\n> second paragraph\nafter\n"
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(doc.Blocks) != 3 {
		t.Fatalf("top-level blocks = %d, want 3: %#v", len(doc.Blocks), doc)
	}
	quote, ok := doc.Blocks[1].(BlockQuote)
	if !ok {
		t.Fatalf("middle block = %T, want BlockQuote", doc.Blocks[1])
	}
	if len(quote.Blocks) != 2 {
		t.Fatalf("quoted blocks = %d, want 2: %#v", len(quote.Blocks), quote)
	}
	first := quote.Blocks[0].(Paragraph)
	second := quote.Blocks[1].(Paragraph)
	if first.Inlines[0].(Text).Text != "first line\nsecond line" || second.Inlines[0].(Text).Text != "second paragraph" {
		t.Fatalf("quoted paragraphs were not preserved: %#v", quote.Blocks)
	}
	if got := doc.Blocks[2].(Paragraph).Inlines[0].(Text).Text; got != "after" {
		t.Fatalf("unquoted termination text = %q, want after", got)
	}
}

func TestBlockQuoteContainsNormalBlockTypes(t *testing.T) {
	input := strings.Join([]string{
		"> # Heading",
		">",
		"> - one",
		">   - nested",
		">",
		"> 2. ordered",
		">",
		"> ```go",
		"> code",
		"> ```",
		">",
		"> | A |",
		"> |---|",
		"> | x |",
		">",
		"> ---",
	}, "\n")
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	quote := doc.Blocks[0].(BlockQuote)
	wantTypes := []any{Heading{}, List{}, List{}, CodeBlock{}, Table{}, HorizontalRule{}}
	if len(quote.Blocks) != len(wantTypes) {
		t.Fatalf("quoted blocks = %d, want %d: %#v", len(quote.Blocks), len(wantTypes), quote.Blocks)
	}
	for i, want := range wantTypes {
		if reflect.TypeOf(quote.Blocks[i]) != reflect.TypeOf(want) {
			t.Errorf("quoted block %d = %T, want %T", i, quote.Blocks[i], want)
		}
	}
	list := quote.Blocks[1].(List)
	if len(list.Items) != 1 || len(list.Items[0].Blocks) != 2 {
		t.Fatalf("nested quoted list was not parsed structurally: %#v", list)
	}
}

func TestNestedBlockQuotesASTAndRendering(t *testing.T) {
	input := "> Outer\n>\n>> Inner\n"
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	outer := doc.Blocks[0].(BlockQuote)
	if len(outer.Blocks) != 2 {
		t.Fatalf("outer blocks = %d, want 2: %#v", len(outer.Blocks), outer)
	}
	inner, ok := outer.Blocks[1].(BlockQuote)
	if !ok || len(inner.Blocks) != 1 {
		t.Fatalf("nested quote structure = %#v", outer.Blocks[1])
	}
	html := RenderHTML(doc)
	if !strings.Contains(html, `<blockquote><p>Outer</p><blockquote><p>Inner</p></blockquote></blockquote>`) {
		t.Fatalf("nested quote HTML mismatch: %s", html)
	}
}

func TestBlockQuoteMarkerForms(t *testing.T) {
	for _, input := range []string{">> Inner", "> > Inner"} {
		doc, err := Parse(input)
		if err != nil {
			t.Fatalf("Parse(%q) error = %v", input, err)
		}
		outer := doc.Blocks[0].(BlockQuote)
		if _, ok := outer.Blocks[0].(BlockQuote); !ok {
			t.Fatalf("Parse(%q) did not produce a nested quote: %#v", input, outer)
		}
	}

	doc, err := Parse("    > literal")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := doc.Blocks[0].(Paragraph); !ok {
		t.Fatalf("four-space quote-like input = %T, want literal Paragraph", doc.Blocks[0])
	}
}

func TestBlockQuoteRenderingPreservesNestedBlocksAndInlineBehavior(t *testing.T) {
	input := "> **bold** and `code` https://example.com ![local](image.png)\n>\n> - item\n>\n> ```\n> quoted code\n> ```\n"
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	html := RenderHTMLWithImagePolicy(doc, AllowOutsideImagePolicy())
	for _, fragment := range []string{
		`<blockquote><p><strong>bold</strong> and <code>code</code>`,
		`class="mahcdown-link"`,
		`<img src="image.png" alt="local"/>`,
		`<ul><li><p>item</p></li></ul>`,
		`<pre><code>quoted code`,
	} {
		if !strings.Contains(html, fragment) {
			t.Errorf("quoted HTML is missing %q: %s", fragment, html)
		}
	}
}

func TestBlockQuoteNestingDepthLimit(t *testing.T) {
	if _, err := ParseWithLimits(nestedQuote(4), Limits{MaxNestingDepth: 4}); err != nil {
		t.Fatalf("quote nesting exactly at limit failed: %v", err)
	}
	if _, err := ParseWithLimits(nestedQuote(5), Limits{MaxNestingDepth: 4}); !errors.Is(err, ErrNestingDepthLimit) {
		t.Fatalf("quote nesting over limit error = %v, want ErrNestingDepthLimit", err)
	}
	if _, err := ParseWithLimits(nestedQuote(200), Limits{MaxNestingDepth: 4}); !errors.Is(err, ErrNestingDepthLimit) {
		t.Fatalf("deep malicious quote error = %v, want ErrNestingDepthLimit", err)
	}
}

func TestBlockQuoteParseItemBudget(t *testing.T) {
	input := ">\n>"
	if _, err := ParseWithLimits(input, Limits{MaxParseItems: 6}); err != nil {
		t.Fatalf("quote structure exactly at budget failed: %v", err)
	}
	doc, err := ParseWithLimits(input, Limits{MaxParseItems: 5})
	if !errors.Is(err, ErrParseItemLimit) {
		t.Fatalf("quote structure over budget error = %v, want ErrParseItemLimit", err)
	}
	if len(doc.Blocks) != 0 {
		t.Fatalf("quote budget failure returned partial document: %#v", doc)
	}
}

func nestedQuote(depth int) string {
	return strings.Repeat(">", depth) + " text"
}
