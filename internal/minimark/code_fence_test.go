package minimark

import (
	"errors"
	"reflect"
	"testing"
)

func TestFencedCodeDelimiterMatching(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  CodeBlock
	}{
		{"three backticks", "```\ncode\n```", CodeBlock{Text: "code\n"}},
		{"three tildes", "~~~\ntilde block\n~~~", CodeBlock{Text: "tilde block\n"}},
		{"long backtick opener", "````go\n``` remains content\n````", CodeBlock{Info: "go", Text: "``` remains content\n"}},
		{"long tilde opener", "~~~~ text\n~~~ remains content\n~~~~", CodeBlock{Info: "text", Text: "~~~ remains content\n"}},
		{"longer closing fence", "```\ncode\n`````", CodeBlock{Text: "code\n"}},
		{"shorter fence remains content", "````\n```\n````", CodeBlock{Text: "```\n"}},
		{"different fence remains content", "```\n~~~\n```", CodeBlock{Text: "~~~\n"}},
		{"fence-like content", "```\n``` trailing\ntext\n```", CodeBlock{Text: "``` trailing\ntext\n"}},
		{"indented close", "~~~\ncode\n   ~~~   ", CodeBlock{Text: "code\n"}},
		{"four-space close remains content", "~~~\n    ~~~\n~~~", CodeBlock{Text: "    ~~~\n"}},
		{"trailing non-whitespace prevents close", "~~~\n~~~ no\n~~~", CodeBlock{Text: "~~~ no\n"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := mustParseFence(t, tt.input)
			if len(doc.Blocks) != 1 || !reflect.DeepEqual(doc.Blocks[0], tt.want) {
				t.Fatalf("Parse(%q)\nwant: %#v\n got: %#v", tt.input, tt.want, doc.Blocks)
			}
		})
	}
}

func TestFenceOpeningUsesCompleteMaximalRun(t *testing.T) {
	for _, short := range []string{"``", "~~"} {
		if _, ok := parseFenceStart(short); ok {
			t.Errorf("short run %q opened a fence", short)
		}
	}
	fence, ok := parseFenceStart("   `````go")
	if !ok || fence.character != '`' || fence.length != 5 || fence.info != "go" {
		t.Fatalf("maximal opening fence = %#v, %v", fence, ok)
	}
}

func TestFencedCodeInfoStrings(t *testing.T) {
	tests := []struct {
		input string
		info  string
	}{
		{"```   go test   \n```", "go test"},
		{"~~~   language ~ value   \n~~~", "language ~ value"},
	}
	for _, tt := range tests {
		doc := mustParseFence(t, tt.input)
		if got := doc.Blocks[0].(CodeBlock).Info; got != tt.info {
			t.Errorf("info for %q = %q, want %q", tt.input, got, tt.info)
		}
	}

	doc := mustParseFence(t, "``` bad`info")
	if _, ok := doc.Blocks[0].(CodeBlock); ok {
		t.Fatalf("backtick-containing info opened a code block: %#v", doc)
	}
}

func TestFencedCodeEmptyAndUnterminatedNewlines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		text  string
	}{
		{"empty terminated", "```\n```", ""},
		{"one blank line", "```\n\n```", "\n"},
		{"unterminated without final newline", "```\nunterminated", "unterminated"},
		{"unterminated with final newline", "~~~\nunterminated\n", "unterminated\n"},
		{"empty unterminated", "```", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := mustParseFence(t, tt.input)
			block, ok := doc.Blocks[0].(CodeBlock)
			if !ok || block.Text != tt.text {
				t.Fatalf("Parse(%q) block = %#v, want text %q", tt.input, doc.Blocks[0], tt.text)
			}
		})
	}
}

func TestFencedCodeContentIsLiteral(t *testing.T) {
	doc := mustParseFence(t, "~~~~\n**not strong** `not code` ![not](image.png)\n~~~~")
	want := "**not strong** `not code` ![not](image.png)\n"
	if block := doc.Blocks[0].(CodeBlock); block.Text != want {
		t.Fatalf("code content = %q, want literal %q", block.Text, want)
	}
}

func TestFencedCodeInsideListsAndBlockquotes(t *testing.T) {
	listDoc := mustParseFence(t, "- item\n  ~~~~go\n  code\n  ~~~~")
	list := listDoc.Blocks[0].(List)
	if len(list.Items[0].Blocks) != 2 {
		t.Fatalf("list code blocks = %#v", list.Items[0].Blocks)
	}
	if block, ok := list.Items[0].Blocks[1].(CodeBlock); !ok || block.Info != "go" || block.Text != "code\n" {
		t.Fatalf("list code block = %#v", list.Items[0].Blocks[1])
	}

	quoteDoc := mustParseFence(t, "> ~~~\n> quoted\n> ~~~")
	quote := quoteDoc.Blocks[0].(BlockQuote)
	if len(quote.Blocks) != 1 {
		t.Fatalf("blockquote code blocks = %#v", quote.Blocks)
	}
	if block, ok := quote.Blocks[0].(CodeBlock); !ok || block.Text != "quoted\n" {
		t.Fatalf("blockquote code block = %#v", quote.Blocks[0])
	}
}

func TestTildeFenceUsesExistingParseItemLimit(t *testing.T) {
	input := "~~~\n\n~~~"
	if _, err := ParseWithLimits(input, Limits{MaxParseItems: 6}); err != nil {
		t.Fatalf("tilde fence at parse-item boundary failed: %v", err)
	}
	doc, err := ParseWithLimits(input, Limits{MaxParseItems: 5})
	if !errors.Is(err, ErrParseItemLimit) || len(doc.Blocks) != 0 {
		t.Fatalf("tilde fence over budget = (%#v, %v)", doc, err)
	}
}

func mustParseFence(t *testing.T, input string) Document {
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
