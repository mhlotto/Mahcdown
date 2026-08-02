package minimark

import (
	"html"
	"strings"
	"testing"
)

func TestRenderHTML(t *testing.T) {
	tests := []struct {
		name string
		doc  Document
		want string
	}{
		{
			name: "paragraph with bold and italic",
			doc: Document{Blocks: []Block{
				Paragraph{Inlines: []Inline{
					Text{Text: "Hello "},
					Strong{Inlines: []Inline{Text{Text: "bold"}}},
					Text{Text: " and "},
					Emphasis{Inlines: []Inline{Text{Text: "italics"}}},
				}},
			}},
			want: `<p>Hello <strong>bold</strong> and <em>italics</em></p>`,
		},
		{
			name: "heading and code block",
			doc: Document{Blocks: []Block{
				Heading{Level: 2, Inlines: []Inline{Text{Text: "Title"}}},
				CodeBlock{Info: "go", Text: "fmt.Println(\"hi\")\n"},
			}},
			want: `<h2>Title</h2><pre><code class="language-go">fmt.Println(&#34;hi&#34;)
</code></pre>`,
		},
		{
			name: "table with alignment",
			doc: Document{Blocks: []Block{
				Table{
					Headers: [][]Inline{{Text{Text: "A"}}, {Text{Text: "B"}}},
					Aligns:  []Align{AlignLeft, AlignRight},
					Rows: [][][]Inline{
						{{Text{Text: "1"}}, {Text{Text: "2"}}},
					},
				},
			}},
			want: `<table><thead><tr><th align="left">A</th><th align="right">B</th></tr></thead><tbody><tr><td align="left">1</td><td align="right">2</td></tr></tbody></table>`,
		},
		{
			name: "blockquote joins lines",
			doc: Document{Blocks: []Block{
				BlockQuote{Lines: [][]Inline{
					{Text{Text: "first"}},
					{Text{Text: "second"}},
				}},
			}},
			want: `<blockquote><p>first<br/>second</p></blockquote>`,
		},
		{
			name: "external image is blocked",
			doc: Document{Blocks: []Block{
				Paragraph{Inlines: []Inline{
					Image{Alt: "alt", URL: "https://example.com/img.png"},
				}},
			}},
			want: `<p><span class="image-blocked" data-src="https://example.com/img.png">[image blocked: alt]</span></p>`,
		},
		{
			name: "link rendered with prompt-friendly attributes",
			doc: Document{Blocks: []Block{
				Paragraph{Inlines: []Inline{
					Url{URL: "https://example.com", Text: "https://example.com"},
				}},
			}},
			want: `<p><a href="https://example.com" data-href="https://example.com" rel="noreferrer noopener">https://example.com</a></p>`,
		},
		{
			name: "checkbox rendering",
			doc: Document{Blocks: []Block{
				Paragraph{Inlines: []Inline{
					Checkbox{Checked: false},
					Text{Text: "Task"},
				}},
			}},
			want: `<p><input type="checkbox" disabled class="checkbox"/>Task</p>`,
		},
		{
			name: "list rendering with checkbox and nested",
			doc: Document{Blocks: []Block{
				List{
					Ordered: false,
					Items: []ListItem{
						{HasCheckbox: true, Checked: false, Blocks: []Block{
							Paragraph{Inlines: []Inline{Text{Text: "task"}}},
							List{
								Ordered: true, HasStart: true, Start: 2,
								Items: []ListItem{
									{Blocks: []Block{
										Paragraph{Inlines: []Inline{Text{Text: "inner"}}},
									}},
								},
							},
						}},
					},
				},
			}},
			want: `<ul><li><p><input type="checkbox" disabled class="checkbox"/>task</p><ol start="2"><li><p>inner</p></li></ol></li></ul>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderHTML(tt.doc)
			if got != htmlWrap(tt.want) {
				t.Fatalf("RenderHTML() mismatch.\nexpected: %q\ngot:      %q", htmlWrap(tt.want), got)
			}
		})
	}
}

func htmlWrap(body string) string {
	return `<!DOCTYPE html><html><head><meta charset="utf-8"><style>` + baseCSS + `</style></head><body>` + body + `</body></html>`
}

func TestRenderImageSourceAllowlist(t *testing.T) {
	rejected := []string{
		"http://example.com/image.png",
		"https://example.com/image.png",
		"HtTp://example.com/image.png",
		"hTTps://example.com/image.png",
		"//example.com/image.png",
		"ftp://example.com/image.png",
		"data:image/png;base64,AAAA",
		"javascript:alert(1)",
		"file:///tmp/image.png",
		"file://remote-host/path/image.png",
		"custom-scheme:image.png",
		"/images/photo.png",
		`\\example.com\image.png`,
		"  https://example.com/image.png",
		"?query",
		"#fragment",
		"?query#fragment",
	}
	for _, source := range rejected {
		t.Run("reject "+source, func(t *testing.T) {
			got := renderImage(source, `<unsafe & alt>`)
			if strings.Contains(got, `<img src=`) {
				t.Fatalf("rejected source appeared in an active image: %s", got)
			}
			if !strings.Contains(got, `<span class="image-blocked"`) {
				t.Fatalf("rejected source did not use blocked-image output: %s", got)
			}
			if !strings.Contains(got, `[image blocked: &lt;unsafe &amp; alt&gt;]`) {
				t.Fatalf("blocked image did not preserve escaped alt text: %s", got)
			}
		})
	}

	allowed := []string{
		"image.png",
		"images/photo.png",
		"../images/photo.png",
		"photo with spaces.png",
		"photo%20encoded.png",
		"写真.png",
		"./images/./photo.png",
		"image.png?version=2",
		"image.svg#icon",
		"images/photo.png?version=2#preview",
	}
	for _, source := range allowed {
		t.Run("allow "+source, func(t *testing.T) {
			got := renderImage(source, `<safe & alt>`)
			want := `<img src="` + html.EscapeString(source) + `" alt="&lt;safe &amp; alt&gt;"/>`
			if !strings.Contains(got, want) {
				t.Fatalf("allowed source was not rendered as expected:\nwant: %s\ngot:  %s", want, got)
			}
			if strings.Contains(got, `<span class="image-blocked"`) {
				t.Fatalf("allowed source was blocked: %s", got)
			}
		})
	}
}

func renderImage(source, alt string) string {
	return RenderHTML(Document{Blocks: []Block{
		Paragraph{Inlines: []Inline{Image{URL: source, Alt: alt}}},
	}})
}
