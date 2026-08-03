package minimark

import (
	"errors"
	"html"
	"reflect"
	"strings"
	"testing"
)

func TestInlineMarkdownLinks(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []Inline
	}{
		{"basic", "[label](https://example.com)", []Inline{Link{URL: "https://example.com", Inlines: []Inline{Text{Text: "label"}}}}},
		{"formatted label", "[**formatted** label](https://example.com)", []Inline{Link{URL: "https://example.com", Inlines: []Inline{Strong{Inlines: []Inline{Text{Text: "formatted"}}}, Text{Text: " label"}}}}},
		{"escaped label bracket", `[a \] b](https://example.com)`, []Inline{Link{URL: "https://example.com", Inlines: []Inline{Text{Text: "a ] b"}}}}},
		{"escaped destination parentheses", `[label](https://example.com/a\(b\))`, []Inline{Link{URL: "https://example.com/a(b)", Inlines: []Inline{Text{Text: "label"}}}}},
		{"balanced destination", "[label](https://example.com/a_(b_(c)))", []Inline{Link{URL: "https://example.com/a_(b_(c))", Inlines: []Inline{Text{Text: "label"}}}}},
		{"empty label and destination", "[]()", []Inline{Link{URL: "", Inlines: nil}}},
		{"code label is opaque", "[`**not strong**`](https://example.com)", []Inline{Link{URL: "https://example.com", Inlines: []Inline{CodeSpan{Text: "**not strong**"}}}}},
		{"code label shields bracket", "[before `]` after](https://example.com)", []Inline{Link{URL: "https://example.com", Inlines: []Inline{Text{Text: "before "}, CodeSpan{Text: "]"}, Text{Text: " after"}}}}},
		{"escaped link opener", `\[label](https://example.com)`, []Inline{Text{Text: "[label](https://example.com)"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertLinkInlines(t, tt.input, tt.want)
		})
	}
}

func TestInlineLinkUsesExistingParseItemLimit(t *testing.T) {
	if _, err := ParseWithLimits("[x](u)", Limits{MaxParseItems: 6}); err != nil {
		t.Fatalf("link at parse-item boundary failed: %v", err)
	}
	doc, err := ParseWithLimits("[x](u)", Limits{MaxParseItems: 5})
	if !errors.Is(err, ErrParseItemLimit) || len(doc.Blocks) != 0 {
		t.Fatalf("link over parse-item budget = (%#v, %v)", doc, err)
	}
}

func TestMalformedInlineLinksRemainLiteral(t *testing.T) {
	for _, input := range []string{
		"[label](https://example.com",
		"[label](https://example.com/a_(b)",
		"[unclosed label",
		"[label] not a link",
	} {
		assertLinkInlines(t, input, []Inline{Text{Text: input}})
	}
}

func TestImagePrecedesInlineLink(t *testing.T) {
	assertLinkInlines(t, "![alt](image.png) [label](https://example.com)", []Inline{
		Image{Alt: "alt", URL: "image.png"},
		Text{Text: " "},
		Link{URL: "https://example.com", Inlines: []Inline{Text{Text: "label"}}},
	})
}

func TestNestedLinksRemainLiteralInsideLabel(t *testing.T) {
	tests := []struct {
		input string
		label string
	}{
		{"[outer [inner](https://inner.example)](https://outer.example)", "outer [inner](https://inner.example)"},
		{"[outer <https://inner.example>](https://outer.example)", "outer <https://inner.example>"},
		{"[outer https://inner.example](https://outer.example)", "outer https://inner.example"},
	}
	for _, tt := range tests {
		want := []Inline{Link{URL: "https://outer.example", Inlines: []Inline{Text{Text: tt.label}}}}
		assertLinkInlines(t, tt.input, want)
		rendered := RenderHTML(mustParseLink(t, tt.input))
		if count := strings.Count(rendered, `<a class="mahcdown-link"`); count != 1 {
			t.Fatalf("nested link rendered %d anchors: %s", count, rendered)
		}
	}
}

func TestInlineLinksAcrossBlockContainers(t *testing.T) {
	input := "# [heading](https://heading.example)\n\n- [item](https://item.example)\n\n> [quote](https://quote.example)\n\n| [cell](https://cell.example) |\n| --- |"
	doc := mustParseLink(t, input)
	if len(doc.Blocks) != 4 {
		t.Fatalf("container blocks = %#v", doc.Blocks)
	}
	assertOnlyLink(t, doc.Blocks[0].(Heading).Inlines, "https://heading.example")
	assertOnlyLink(t, doc.Blocks[1].(List).Items[0].Blocks[0].(Paragraph).Inlines, "https://item.example")
	assertOnlyLink(t, doc.Blocks[2].(BlockQuote).Blocks[0].(Paragraph).Inlines, "https://quote.example")
	assertOnlyLink(t, doc.Blocks[3].(Table).Headers[0], "https://cell.example")
}

func TestInlineLinkRenderingIsInertAndEscaped(t *testing.T) {
	input := `[<label> & "quote"](https://example.com/?a="x"&b=<tag>)`
	rendered := RenderHTML(mustParseLink(t, input))
	want := `<a class="mahcdown-link" role="link" tabindex="0" data-mahcdown-href="https://example.com/?a=&#34;x&#34;&amp;b=&lt;tag&gt;">&lt;label&gt; &amp; &#34;quote&#34;</a>`
	if !strings.Contains(rendered, want) {
		t.Fatalf("safe inert link missing\nwant: %s\n got: %s", want, rendered)
	}
	if strings.Contains(rendered, ` href=`) {
		t.Fatalf("link destination appeared in active href: %s", rendered)
	}
	if unescaped := html.UnescapeString(rendered); !strings.Contains(unescaped, `<label> & "quote"`) {
		t.Fatalf("rendered label text changed after HTML unescaping: %s", unescaped)
	}
}

func assertLinkInlines(t *testing.T, input string, want []Inline) {
	t.Helper()
	doc := mustParseLink(t, input)
	paragraph, ok := doc.Blocks[0].(Paragraph)
	if !ok {
		t.Fatalf("first block = %T, want Paragraph", doc.Blocks[0])
	}
	if !reflect.DeepEqual(paragraph.Inlines, want) {
		t.Fatalf("inlines for %q\nwant: %#v\n got: %#v", input, want, paragraph.Inlines)
	}
}

func mustParseLink(t *testing.T, input string) Document {
	t.Helper()
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse(%q): %v", input, err)
	}
	if len(doc.Blocks) == 0 {
		t.Fatalf("Parse(%q) returned no blocks", input)
	}
	return doc
}

func assertOnlyLink(t *testing.T, inlines []Inline, destination string) {
	t.Helper()
	if len(inlines) != 1 {
		t.Fatalf("inlines = %#v, want one Link", inlines)
	}
	link, ok := inlines[0].(Link)
	if !ok || link.URL != destination {
		t.Fatalf("inline = %#v, want Link to %q", inlines[0], destination)
	}
}
