package minimark

import (
	"html"
	"reflect"
	"strings"
	"testing"
)

func TestApostrophesRemainUnchanged(t *testing.T) {
	for _, input := range []string{"It's working", "It’s working"} {
		assertApostropheInlines(t, input, []Inline{Text{Text: input}})
		rendered := html.UnescapeString(RenderHTML(mustParseApostrophe(t, input)))
		if !strings.Contains(rendered, input) {
			t.Fatalf("rendered HTML for %q lost punctuation: %s", input, rendered)
		}
	}
}

func TestBareURLPreservesFollowingTerminators(t *testing.T) {
	tests := []struct {
		input      string
		terminator string
	}{
		{input: "Visit https://example.com/path'.", terminator: "'."},
		{input: "Visit https://example.com/path\".", terminator: "\"."},
		{input: "See https://example.com/path).", terminator: ")."},
	}
	for _, tt := range tests {
		t.Run(tt.terminator, func(t *testing.T) {
			assertApostropheInlines(t, tt.input, []Inline{
				Text{Text: strings.Split(tt.input, "https://")[0]},
				Url{URL: "https://example.com/path", Text: "https://example.com/path"},
				Text{Text: tt.terminator},
			})
			rendered := html.UnescapeString(RenderHTML(mustParseApostrophe(t, tt.input)))
			if !strings.Contains(rendered, `data-mahcdown-href="https://example.com/path"`) || !strings.Contains(rendered, tt.terminator) {
				t.Fatalf("rendered HTML lost URL terminator %q: %s", tt.terminator, rendered)
			}
		})
	}
}

func TestBareURLPreservesStrippedTrailingPunctuation(t *testing.T) {
	for _, punctuation := range []string{".", ",", ";", ":", "!", "?", ".,;:!?"} {
		input := "Visit https://example.com/path" + punctuation
		assertApostropheInlines(t, input, []Inline{
			Text{Text: "Visit "},
			Url{URL: "https://example.com/path", Text: "https://example.com/path"},
			Text{Text: punctuation},
		})
		rendered := html.UnescapeString(RenderHTML(mustParseApostrophe(t, input)))
		if !strings.Contains(rendered, punctuation+"</p>") {
			t.Fatalf("rendered HTML lost trailing punctuation %q: %s", punctuation, rendered)
		}
	}
}

func assertApostropheInlines(t *testing.T, input string, want []Inline) {
	t.Helper()
	doc := mustParseApostrophe(t, input)
	paragraph, ok := doc.Blocks[0].(Paragraph)
	if !ok {
		t.Fatalf("first block = %T, want Paragraph", doc.Blocks[0])
	}
	if !reflect.DeepEqual(paragraph.Inlines, want) {
		t.Fatalf("inlines for %q\nwant: %#v\ngot:  %#v", input, want, paragraph.Inlines)
	}
}

func mustParseApostrophe(t *testing.T, input string) Document {
	t.Helper()
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse(%q): %v", input, err)
	}
	return doc
}
