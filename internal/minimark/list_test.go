package minimark

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestListMarkerIndentationAndMetadata(t *testing.T) {
	for indent := 0; indent <= 3; indent++ {
		input := strings.Repeat(" ", indent) + "- item"
		list := requireFirstList(t, parseListDocument(t, input))
		if list.Ordered || listText(list.Items[0]) != "item" {
			t.Errorf("marker indent %d parsed as %#v", indent, list)
		}
	}
	for _, input := range []string{"    - item", "\t- item", "-\titem", "1.\titem"} {
		doc := parseListDocument(t, input)
		if _, ok := doc.Blocks[0].(List); ok {
			t.Errorf("invalid marker indentation %q started a list", input)
		}
	}

	doc := parseListDocument(t, "- \n7. \n12. later\n")
	if len(doc.Blocks) != 2 {
		t.Fatalf("empty and mixed items produced %#v", doc.Blocks)
	}
	unordered := doc.Blocks[0].(List)
	ordered := doc.Blocks[1].(List)
	if len(unordered.Items) != 1 || len(unordered.Items[0].Blocks) != 0 {
		t.Errorf("empty unordered item = %#v", unordered)
	}
	if ordered.Start != 7 || len(ordered.Items) != 2 || len(ordered.Items[0].Blocks) != 0 {
		t.Errorf("ordered start or empty item = %#v", ordered)
	}
}

func TestListContinuationColumns(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantBlocks int
		wantText   string
	}{
		{"unordered continuation", "- item\n  continuation", 1, "item\ncontinuation"},
		{"unordered insufficient", "- item\n continuation", 2, "item"},
		{"ordered continuation", "10. item\n    continuation", 1, "item\ncontinuation"},
		{"ordered insufficient", "10. item\n   insufficient", 2, "item"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := parseListDocument(t, tt.input)
			if len(doc.Blocks) != tt.wantBlocks {
				t.Fatalf("blocks = %#v, want %d", doc.Blocks, tt.wantBlocks)
			}
			if got := listText(requireFirstList(t, doc).Items[0]); got != tt.wantText {
				t.Errorf("item text = %q, want %q", got, tt.wantText)
			}
		})
	}
}

func TestNestedListUsesMarkerColumnRatherThanContentColumn(t *testing.T) {
	for _, input := range []string{
		"- parent\n  - child",
		"10. parent\n  - child",
		" - parent\n   - child",
		"  - parent\n    - child",
		"   - parent\n     - child",
	} {
		parent := requireFirstList(t, parseListDocument(t, input)).Items[0]
		if len(parent.Blocks) != 2 {
			t.Fatalf("%q parent blocks = %#v", input, parent.Blocks)
		}
		child, ok := parent.Blocks[1].(List)
		if !ok || listText(child.Items[0]) != "child" {
			t.Fatalf("%q did not produce nested child list: %#v", input, parent.Blocks)
		}
	}
}

func TestNestedListAllowsOnlyThreeOptionalMarkerSpaces(t *testing.T) {
	for extra := 0; extra <= 3; extra++ {
		input := "- parent\n" + strings.Repeat(" ", 2+extra) + "- child"
		parent := requireFirstList(t, parseListDocument(t, input)).Items[0]
		if len(parent.Blocks) != 2 {
			t.Errorf("child marker with %d optional spaces was not nested: %#v", extra, parent.Blocks)
		}
	}
	parent := requireFirstList(t, parseListDocument(t, "- parent\n      - not nested")).Items[0]
	if len(parent.Blocks) != 1 || listText(parent) != "parent\n    - not nested" {
		t.Fatalf("over-indented marker unexpectedly nested: %#v", parent.Blocks)
	}
}

func TestOrderedListOverIndentedChildMarkerRemainsText(t *testing.T) {
	for _, marker := range []string{"1.", "10.", "100."} {
		input := marker + " parent\n      - not nested"
		parent := requireFirstList(t, parseListDocument(t, input)).Items[0]
		if len(parent.Blocks) != 1 {
			t.Fatalf("%q produced nested blocks: %#v", input, parent.Blocks)
		}
		if got, want := listText(parent), "parent\n    - not nested"; got != want {
			t.Fatalf("%q item text = %q, want %q", input, got, want)
		}
	}
}

func TestWideOrderedMarkerEndsBeforeContentColumn(t *testing.T) {
	doc := parseListDocument(t, "10000. parent\n      - outside")
	if len(doc.Blocks) != 2 {
		t.Fatalf("insufficiently indented marker did not end list item: %#v", doc.Blocks)
	}
	parent := requireFirstList(t, doc).Items[0]
	if got := listText(parent); got != "parent" {
		t.Fatalf("parent item text = %q, want parent", got)
	}
	if paragraph, ok := doc.Blocks[1].(Paragraph); !ok || !reflect.DeepEqual(paragraph.Inlines, []Inline{Text{Text: "      - outside"}}) {
		t.Fatalf("outside marker = %#v, want literal outside paragraph", doc.Blocks[1])
	}
}

func TestListTypeGrouping(t *testing.T) {
	doc := parseListDocument(t, "- one\n+ two\n* three\n")
	if len(doc.Blocks) != 1 || len(doc.Blocks[0].(List).Items) != 3 {
		t.Fatalf("unordered marker variants split the list: %#v", doc.Blocks)
	}

	for _, input := range []string{"- one\n1. two", "1. one\n- two"} {
		doc = parseListDocument(t, input)
		if len(doc.Blocks) != 2 {
			t.Fatalf("mixed same-level types for %q = %#v", input, doc.Blocks)
		}
	}

	ordered := requireFirstList(t, parseListDocument(t, "7. seven\n42. forty-two"))
	if ordered.Start != 7 || len(ordered.Items) != 2 {
		t.Fatalf("ordered values split list or changed start: %#v", ordered)
	}

	parent := requireFirstList(t, parseListDocument(t, "- parent\n  1. ordered\n  - unordered\n- sibling"))
	if len(parent.Items) != 2 || len(parent.Items[0].Blocks) != 3 {
		t.Fatalf("mixed child lists or parent sibling parsed incorrectly: %#v", parent)
	}
	if _, ok := parent.Items[0].Blocks[1].(List); !ok {
		t.Fatalf("ordered child missing: %#v", parent.Items[0].Blocks)
	}
	if child, ok := parent.Items[0].Blocks[2].(List); !ok || child.Ordered {
		t.Fatalf("unordered child missing: %#v", parent.Items[0].Blocks)
	}
}

func TestListBlankLineOwnership(t *testing.T) {
	for _, input := range []string{"- one\n\n- two", "- one\n\n\n- two"} {
		list := requireFirstList(t, parseListDocument(t, input))
		if len(list.Items) != 2 {
			t.Errorf("blank-separated siblings for %q = %#v", input, list)
		}
	}
	parent := requireFirstList(t, parseListDocument(t, "- parent\n\n  - child")).Items[0]
	if len(parent.Blocks) != 2 {
		t.Fatalf("blank before child lost nesting: %#v", parent.Blocks)
	}
	paragraphs := requireFirstList(t, parseListDocument(t, "- first paragraph\n\n  second paragraph")).Items[0]
	if len(paragraphs.Blocks) != 2 {
		t.Fatalf("blank did not separate item paragraphs: %#v", paragraphs.Blocks)
	}
	doc := parseListDocument(t, "- item\n\noutside")
	if len(doc.Blocks) != 2 {
		t.Fatalf("outside paragraph was absorbed: %#v", doc.Blocks)
	}
	list := requireFirstList(t, parseListDocument(t, "- \n\n- next\n\n"))
	if len(list.Items) != 2 || len(list.Items[0].Blocks) != 0 {
		t.Fatalf("empty item or trailing blanks changed: %#v", list)
	}
	mixed := parseListDocument(t, "- unordered\n\n1. ordered")
	if len(mixed.Blocks) != 2 {
		t.Fatalf("blank before mixed-type sibling changed grouping: %#v", mixed.Blocks)
	}
}

func TestListItemsReuseNormalBlockParser(t *testing.T) {
	tests := []struct {
		name  string
		input string
		kind  any
	}{
		{"blockquote", "- item\n  > quote", BlockQuote{}},
		{"heading", "- item\n  # heading", Heading{}},
		{"fenced code", "- item\n  ```\n  code\n  ```", CodeBlock{}},
		{"table", "- item\n  | a | b |\n  | --- | --- |\n  | 1 | 2 |", Table{}},
		{"horizontal rule", "- item\n  ---", HorizontalRule{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks := requireFirstList(t, parseListDocument(t, tt.input)).Items[0].Blocks
			if len(blocks) != 2 || reflect.TypeOf(blocks[1]) != reflect.TypeOf(tt.kind) {
				t.Fatalf("nested block = %#v, want %T", blocks, tt.kind)
			}
		})
	}

	parent := requireFirstList(t, parseListDocument(t, "- parent\n  - child\n    > quote"))
	child := parent.Items[0].Blocks[1].(List)
	if len(child.Items[0].Blocks) != 2 {
		t.Fatalf("nested list did not retain nested block: %#v", child.Items[0].Blocks)
	}
	if _, ok := child.Items[0].Blocks[1].(BlockQuote); !ok {
		t.Fatalf("nested list block = %T, want BlockQuote", child.Items[0].Blocks[1])
	}
}

func TestListDepthUsesSharedNestingLimit(t *testing.T) {
	if _, err := ParseWithLimits(nestedList(4), Limits{MaxNestingDepth: 4}); err != nil {
		t.Fatalf("exact list depth failed: %v", err)
	}
	for _, input := range []string{nestedList(5), nestedList(200)} {
		doc, err := ParseWithLimits(input, Limits{MaxNestingDepth: 4})
		if !errors.Is(err, ErrNestingDepthLimit) || len(doc.Blocks) != 0 {
			t.Fatalf("deep list = (%#v, %v), want empty document and depth error", doc, err)
		}
	}
	mixed := "> - parent\n>   - child"
	if _, err := ParseWithLimits(mixed, Limits{MaxNestingDepth: 3}); err != nil {
		t.Fatalf("mixed containers at limit failed: %v", err)
	}
	doc, err := ParseWithLimits(mixed, Limits{MaxNestingDepth: 2})
	if !errors.Is(err, ErrNestingDepthLimit) || len(doc.Blocks) != 0 {
		t.Fatalf("mixed containers over limit = (%#v, %v)", doc, err)
	}
}

func parseListDocument(t *testing.T, input string) Document {
	t.Helper()
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse(%q): %v", input, err)
	}
	return doc
}

func requireFirstList(t *testing.T, doc Document) List {
	t.Helper()
	if len(doc.Blocks) == 0 {
		t.Fatal("document has no blocks")
	}
	list, ok := doc.Blocks[0].(List)
	if !ok {
		t.Fatalf("first block = %T, want List: %#v", doc.Blocks[0], doc)
	}
	return list
}

func listText(item ListItem) string {
	var parts []string
	for _, block := range item.Blocks {
		if paragraph, ok := block.(Paragraph); ok {
			for _, inline := range paragraph.Inlines {
				if text, ok := inline.(Text); ok {
					parts = append(parts, text.Text)
				}
			}
		}
	}
	return strings.Join(parts, "\n")
}
