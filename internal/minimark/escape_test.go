package minimark

import (
	"html"
	"reflect"
	"strings"
	"testing"
)

func TestBackslashEscapableASCIIPunctuation(t *testing.T) {
	const punctuation = `!"#$%&'()*+,-./:;<=>?@[\]^_` + "`" + `{|}~`
	var input strings.Builder
	for i := range len(punctuation) {
		input.WriteByte('\\')
		input.WriteByte(punctuation[i])
	}
	assertEscapeInlines(t, input.String(), []Inline{Text{Text: punctuation}})

	doc, err := Parse(input.String())
	if err != nil {
		t.Fatal(err)
	}
	if got := html.UnescapeString(RenderHTML(doc)); !strings.Contains(got, "<p>"+punctuation+"</p>") {
		t.Fatalf("rendered punctuation missing or changed: %q", got)
	}
}

func TestBackslashNonEscapesRemainLiteral(t *testing.T) {
	for _, input := range []string{`\a`, `\1`, "\\ ", "\\\t", `\é`, `\→`, `trailing\`} {
		assertEscapeInlines(t, input, []Inline{Text{Text: input}})
	}
}

func TestBackslashRunsAndInlinePrecedence(t *testing.T) {
	tests := []struct {
		input string
		want  []Inline
	}{
		{`\*literal*`, []Inline{Text{Text: "*literal*"}}},
		{`\\*emphasis*`, []Inline{Text{Text: `\`}, Emphasis{Inlines: []Inline{Text{Text: "emphasis"}}}}},
		{`\\\*literal*`, []Inline{Text{Text: `\*literal*`}}},
		{`\\\\*emphasis*`, []Inline{Text{Text: `\\`}, Emphasis{Inlines: []Inline{Text{Text: "emphasis"}}}}},
		{`\*literal* and **strong**`, []Inline{Text{Text: "*literal* and "}, Strong{Inlines: []Inline{Text{Text: "strong"}}}}},
		{`\**literal**`, []Inline{Text{Text: "**literal**"}}},
		{"\\`literal`", []Inline{Text{Text: "`literal`"}}},
		{`\![alt](image.png)`, []Inline{Text{Text: "![alt](image.png)"}}},
		{`\<https://example.com>`, []Inline{Text{Text: "<https://example.com>"}}},
		{`\[ ] task`, []Inline{Text{Text: "[ ] task"}}},
	}
	for _, tt := range tests {
		assertEscapeInlines(t, tt.input, tt.want)
	}
}

func TestEscapesDoNotEnterOpaqueConstructs(t *testing.T) {
	tests := []struct {
		input string
		want  []Inline
	}{
		{"``\\*\\[\\`]``", []Inline{CodeSpan{Text: "\\*\\[\\`]"}}},
		{`<https://example.com/\*>`, []Inline{Url{URL: `https://example.com/\*`, Text: `https://example.com/\*`}}},
		{"`![alt](image.png) **strong**`", []Inline{CodeSpan{Text: "![alt](image.png) **strong**"}}},
	}
	for _, tt := range tests {
		assertEscapeInlines(t, tt.input, tt.want)
	}

	doc, err := Parse("```\n\\*\\[\\`]\n```\n")
	if err != nil {
		t.Fatal(err)
	}
	want := Document{Blocks: []Block{CodeBlock{Text: "\\*\\[\\`]\n"}}}
	if !reflect.DeepEqual(doc, want) {
		t.Fatalf("fenced code escapes changed\nwant: %#v\n got: %#v", want, doc)
	}
}

func TestImageFieldsUseBackslashEscapes(t *testing.T) {
	tests := []struct {
		input string
		want  []Inline
	}{
		{`![a \] b](image.png)`, []Inline{Image{Alt: "a ] b", URL: "image.png"}}},
		{`![alt](image\)name.png)`, []Inline{Image{Alt: "alt", URL: "image)name.png"}}},
		{`![a \\ b](images/a\#b\?c.png)`, []Inline{Image{Alt: `a \ b`, URL: "images/a#b?c.png"}}},
		{`![a \q b](image\q.png)`, []Inline{Image{Alt: `a \q b`, URL: `image\q.png`}}},
	}
	for _, tt := range tests {
		assertEscapeInlines(t, tt.input, tt.want)
	}

	malformed := `![unclosed \]](image.png`
	got := parseEscapeInlines(t, malformed)
	for _, inline := range got {
		if _, ok := inline.(Image); ok {
			t.Fatalf("malformed image became active: %#v", got)
		}
	}
}

func TestTablePipesUseSharedEscapeRule(t *testing.T) {
	input := "| a \\| b | c |\n| --- | --- |\n| x \\| y | tail\\ |\n"
	doc, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	want := Table{
		Headers: [][]Inline{{Text{Text: "a | b"}}, {Text{Text: "c"}}},
		Aligns:  []Align{AlignNone, AlignNone},
		Rows:    [][][]Inline{{{Text{Text: "x | y"}}, {Text{Text: `tail\`}}}},
	}
	if len(doc.Blocks) != 1 || !reflect.DeepEqual(doc.Blocks[0], want) {
		t.Fatalf("table\nwant: %#v\n got: %#v", want, doc.Blocks)
	}
}

func TestTablePipeOddAndEvenBackslashes(t *testing.T) {
	odd := splitTableRowForTest(t, `a \\\| b | c`)
	if want := []string{`a \\\| b`, "c"}; !reflect.DeepEqual(odd, want) {
		t.Fatalf("odd split = %#v, want %#v", odd, want)
	}
	even := splitTableRowForTest(t, `a \\| b | c`)
	if want := []string{`a \\`, "b", "c"}; !reflect.DeepEqual(even, want) {
		t.Fatalf("even split = %#v, want %#v", even, want)
	}

	doc, err := Parse("a \\| b\n--- | ---\n")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := doc.Blocks[0].(Table); ok {
		t.Fatalf("escaped header pipe created a table: %#v", doc)
	}

	for _, tt := range []struct {
		input string
		want  string
	}{
		{"| a \\|\n| --- |\n", "a |"},
		{"\\| a | b\n| --- | --- |\n", "| a"},
	} {
		doc, err := Parse(tt.input)
		if err != nil {
			t.Fatal(err)
		}
		table, ok := doc.Blocks[0].(Table)
		if !ok || !reflect.DeepEqual(table.Headers[0], []Inline{Text{Text: tt.want}}) {
			t.Errorf("table %q did not preserve escaped outer pipe: %#v", tt.input, doc)
		}
	}
}

func TestEscapesMergeIntoOneTextNode(t *testing.T) {
	assertEscapeInlines(t, `a\*b\#c\&d`, []Inline{Text{Text: "a*b#c&d"}})
}

func TestEscapedBlockMarkersRemainParagraphText(t *testing.T) {
	tests := map[string]string{
		`\# not a heading`:    "# not a heading",
		`\> not a blockquote`: "> not a blockquote",
		`\- not a list`:       "- not a list",
		`\* not a list`:       "* not a list",
		`1\. not a list`:      "1. not a list",
		`\---`:                "---",
		"\\``` not a fence":   "``` not a fence",
	}
	for input, wantText := range tests {
		doc, err := Parse(input)
		if err != nil {
			t.Fatalf("Parse(%q): %v", input, err)
		}
		want := Document{Blocks: []Block{Paragraph{Inlines: []Inline{Text{Text: wantText}}}}}
		if !reflect.DeepEqual(doc, want) {
			t.Errorf("Parse(%q)\nwant: %#v\n got: %#v", input, want, doc)
		}
	}
}

func TestEscapeRobustness(t *testing.T) {
	inputs := []string{
		strings.Repeat(`\*`, 50_000),
		strings.Repeat(`\\\\\\\*`, 20_000),
		"| " + strings.Repeat(`a \| `, 20_000) + "| b |\n| --- | --- |\n",
	}
	for _, input := range inputs {
		if _, err := Parse(input); err != nil {
			t.Fatalf("large escape input returned unexpected error: %v", err)
		}
	}
}

func assertEscapeInlines(t *testing.T, input string, want []Inline) {
	t.Helper()
	got := parseEscapeInlines(t, input)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parse %q\nwant: %#v\n got: %#v", input, want, got)
	}
}

func parseEscapeInlines(t *testing.T, input string) []Inline {
	t.Helper()
	state := &parserState{limits: Limits{MaxSourceBytes: DefaultMaxSourceBytes, MaxNestingDepth: DefaultMaxNestingDepth, MaxParseItems: DefaultMaxParseItems}}
	got := parseInlines(state, input, 0)
	if state.err != nil {
		t.Fatalf("parseInlines(%q): %v", input, state.err)
	}
	return got
}

func splitTableRowForTest(t *testing.T, input string) []string {
	t.Helper()
	state := &parserState{limits: Limits{MaxParseItems: DefaultMaxParseItems}}
	got := splitTableRow(state, input)
	if state.err != nil {
		t.Fatal(state.err)
	}
	return got
}
