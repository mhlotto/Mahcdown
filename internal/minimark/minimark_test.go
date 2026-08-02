package minimark

import (
	"reflect"
	"testing"
)

func TestParseBlocks(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Document
	}{
		{
			name:  "fenced code block with info",
			input: "```go\nfmt.Println(\"hi\")\n```\n",
			expected: Document{Blocks: []Block{
				CodeBlock{Info: "go", Text: "fmt.Println(\"hi\")\n"},
			}},
		},
		{
			name:  "unclosed fenced code extends to EOF",
			input: "```\nopen\n",
			expected: Document{Blocks: []Block{
				CodeBlock{Info: "", Text: "open\n"},
			}},
		},
		{
			name:  "heading with inline strong",
			input: "# Hello **world**\n",
			expected: Document{Blocks: []Block{
				Heading{Level: 1, Inlines: []Inline{
					Text{Text: "Hello "},
					Strong{Inlines: []Inline{Text{Text: "world"}}},
				}},
			}},
		},
		{
			name:  "horizontal rule then paragraph",
			input: "---\ntext\n",
			expected: Document{Blocks: []Block{
				HorizontalRule{},
				Paragraph{Inlines: []Inline{Text{Text: "text"}}},
			}},
		},
		{
			name:  "table with alignment and escaped pipe",
			input: "| a \\| b | c |\n|:---|---:|\n| 1 | 2 |\n",
			expected: Document{Blocks: []Block{
				Table{
					Headers: [][]Inline{
						{Text{Text: "a | b"}},
						{Text{Text: "c"}},
					},
					Aligns: []Align{AlignLeft, AlignRight},
					Rows: [][][]Inline{
						{{Text{Text: "1"}}, {Text{Text: "2"}}},
					},
				},
			}},
		},
		{
			name:  "blockquote stops at blank line",
			input: "> first\n> second\n\nthird\n",
			expected: Document{Blocks: []Block{
				BlockQuote{Blocks: []Block{
					Paragraph{Inlines: []Inline{Text{Text: "first\nsecond"}}},
				}},
				Paragraph{Inlines: []Inline{Text{Text: "third"}}},
			}},
		},
		{
			name:  "paragraph joins lines with newline",
			input: "a\nb\n\nc\n",
			expected: Document{Blocks: []Block{
				Paragraph{Inlines: []Inline{Text{Text: "a\nb"}}},
				Paragraph{Inlines: []Inline{Text{Text: "c"}}},
			}},
		},
		{
			name:  "not a heading without space after hashes",
			input: "#Not heading\n",
			expected: Document{Blocks: []Block{
				Paragraph{Inlines: []Inline{Text{Text: "#Not heading"}}},
			}},
		},
		{
			name:  "checkbox at paragraph start and after newline",
			input: "[ ] first line\n[x] second line\nplain\n",
			expected: Document{Blocks: []Block{
				Paragraph{Inlines: []Inline{
					Checkbox{Checked: false},
					Text{Text: "first line\n"},
					Checkbox{Checked: true},
					Text{Text: "second line\nplain"},
				}},
			}},
		},
		{
			name:  "unordered and ordered lists with nesting and checkbox",
			input: "- [ ] task one\n- [x] task two\n  1. sub numbered\n    - deeper\n- final\n",
			expected: Document{Blocks: []Block{
				List{
					Ordered: false,
					Items: []ListItem{
						{HasCheckbox: true, Checked: false, Blocks: []Block{
							Paragraph{Inlines: []Inline{Text{Text: "task one"}}},
						}},
						{HasCheckbox: true, Checked: true, Blocks: []Block{
							Paragraph{Inlines: []Inline{Text{Text: "task two"}}},
							List{
								Ordered:  true,
								HasStart: true,
								Start:    1,
								Items: []ListItem{
									{Blocks: []Block{
										Paragraph{Inlines: []Inline{Text{Text: "sub numbered"}}},
										List{
											Ordered: false,
											Items: []ListItem{
												{Blocks: []Block{
													Paragraph{Inlines: []Inline{Text{Text: "deeper"}}},
												}},
											},
										},
									}},
								},
							},
						}},
						{HasCheckbox: false, Blocks: []Block{
							Paragraph{Inlines: []Inline{Text{Text: "final"}}},
						}},
					},
				},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.expected) {
				t.Fatalf("Parse() mismatch.\ninput: %q\nexpected: %#v\ngot: %#v", tt.input, tt.expected, got)
			}
		})
	}
}

func TestInlinePrecedence(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Inline
	}{
		{
			name:  "code span wins over bold",
			input: "before `**bold**` after",
			expected: []Inline{
				Text{Text: "before "},
				CodeSpan{Text: "**bold**"},
				Text{Text: " after"},
			},
		},
		{
			name:  "image parsed and malformed stays text",
			input: "![alt](url) [broken!]",
			expected: []Inline{
				Image{Alt: "alt", URL: "url"},
				Text{Text: " [broken!]"},
			},
		},
		{
			name:  "autolink and bare url with trailing punctuation",
			input: "see <https://ex.com> and https://ex.com/path).",
			expected: []Inline{
				Text{Text: "see "},
				Url{URL: "https://ex.com", Text: "https://ex.com"},
				Text{Text: " and "},
				Url{URL: "https://ex.com/path", Text: "https://ex.com/path"},
				Text{Text: ")."},
			},
		},
		{
			name:  "strong containing italic",
			input: "**strong *not italic***",
			expected: []Inline{
				Strong{Inlines: []Inline{
					Text{Text: "strong "},
					Emphasis{Inlines: []Inline{Text{Text: "not italic"}}},
				}},
			},
		},
		{
			name:  "dangling markers are literal",
			input: "*dangling **bold",
			expected: []Inline{
				Text{Text: "*dangling **bold"},
			},
		},
		{
			name:  "code span missing close becomes text",
			input: "hello `world",
			expected: []Inline{
				Text{Text: "hello `world"},
			},
		},
		{
			name:  "checkbox only at line starts",
			input: "prefix [ ] no checkbox\n[ ] yes checkbox\nmid [x] no\n[x] yes",
			expected: []Inline{
				Text{Text: "prefix [ ] no checkbox\n"},
				Checkbox{Checked: false},
				Text{Text: "yes checkbox\nmid [x] no\n"},
				Checkbox{Checked: true},
				Text{Text: "yes"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if len(doc.Blocks) != 1 {
				t.Fatalf("expected single block, got %d", len(doc.Blocks))
			}
			para, ok := doc.Blocks[0].(Paragraph)
			if !ok {
				t.Fatalf("expected Paragraph, got %T", doc.Blocks[0])
			}
			if !reflect.DeepEqual(para.Inlines, tt.expected) {
				t.Fatalf("inline mismatch.\ninput: %q\nexpected: %#v\ngot: %#v", tt.input, tt.expected, para.Inlines)
			}
		})
	}
}
