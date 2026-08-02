package minimark

import (
	"errors"
	"strings"
	"testing"
)

func TestSourceSizeLimit(t *testing.T) {
	limits := Limits{MaxSourceBytes: 4}
	doc, err := ParseWithLimits("éé", limits)
	if err != nil {
		t.Fatalf("input exactly at byte limit failed: %v", err)
	}
	if len(doc.Blocks) != 1 {
		t.Fatalf("input exactly at limit produced %d blocks, want 1", len(doc.Blocks))
	}

	doc, err = ParseWithLimits("ééx", limits)
	if !errors.Is(err, ErrSourceSizeLimit) {
		t.Fatalf("one byte over limit error = %v, want ErrSourceSizeLimit", err)
	}
	if len(doc.Blocks) != 0 {
		t.Fatalf("oversized input returned a partial document: %#v", doc)
	}
}

func TestParseItemLimitNodeBoundaries(t *testing.T) {
	tests := []struct {
		name  string
		input string
		nodes int
	}{
		{name: "plain text", input: "text", nodes: 5},
		{name: "code inline", input: "`code`", nodes: 5},
		{name: "image inline", input: "![alt](image.png)", nodes: 5},
		{name: "link inline", input: "https://example.com", nodes: 5},
		{name: "checkbox inline", input: "[ ] task", nodes: 6},
		{name: "strong inline", input: "**bold**", nodes: 10},
		{name: "emphasis inline", input: "*italic*", nodes: 10},
		{name: "code block", input: "```\ncode\n```", nodes: 6},
		{name: "block heavy", input: "---\n---\n", nodes: 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := ParseWithLimits(tt.input, Limits{MaxParseItems: tt.nodes})
			if err != nil {
				t.Fatalf("exact node budget failed: %v", err)
			}
			if len(doc.Blocks) == 0 {
				t.Fatal("exact node budget returned an empty document")
			}

			doc, err = ParseWithLimits(tt.input, Limits{MaxParseItems: tt.nodes - 1})
			if !errors.Is(err, ErrParseItemLimit) {
				t.Fatalf("one item under budget error = %v, want ErrParseItemLimit", err)
			}
			if len(doc.Blocks) != 0 {
				t.Fatalf("node-limit failure returned a partial document: %#v", doc)
			}
		})
	}

	_, err := ParseWithLimits("**bold**", Limits{MaxParseItems: 3})
	if !errors.Is(err, ErrParseItemLimit) {
		t.Fatalf("nested inline node error = %v, want ErrParseItemLimit", err)
	}
}

func TestStructuralParseItemBudget(t *testing.T) {
	tests := []struct {
		name  string
		input string
		items int
	}{
		{name: "empty list items", input: "- \n- ", items: 8},
		{name: "empty table cells", input: "| | |\n|---|---|", items: 8},
		{name: "table row and wide empty cells", input: "| h |\n|---|\n|||||", items: 13},
		{name: "empty blockquote lines", input: ">\n>", items: 6},
		{name: "empty code lines", input: "```\n\n```", items: 6},
		{name: "document line index", input: "\n\n", items: 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := ParseWithLimits(tt.input, Limits{MaxParseItems: tt.items})
			if err != nil {
				t.Fatalf("exact parse-item budget failed: %v", err)
			}
			if doc.Blocks == nil && strings.TrimSpace(tt.input) != "" {
				t.Fatal("exact parse-item budget returned no blocks")
			}

			doc, err = ParseWithLimits(tt.input, Limits{MaxParseItems: tt.items - 1})
			if !errors.Is(err, ErrParseItemLimit) {
				t.Fatalf("one item under budget error = %v, want ErrParseItemLimit", err)
			}
			if len(doc.Blocks) != 0 {
				t.Fatalf("parse-item failure returned a partial document: %#v", doc)
			}
		})
	}
}

func TestLineIndexBudgetAndLongLine(t *testing.T) {
	if _, err := ParseWithLimits("\n\n", Limits{MaxParseItems: 4}); err != nil {
		t.Fatalf("line structure exactly at boundary failed: %v", err)
	}
	if _, err := ParseWithLimits("\n\n\n", Limits{MaxParseItems: 4}); !errors.Is(err, ErrParseItemLimit) {
		t.Fatalf("one additional line error = %v, want ErrParseItemLimit", err)
	}

	longLine := strings.Repeat("x", 4096)
	if _, err := ParseWithLimits(longLine, Limits{MaxSourceBytes: len(longLine), MaxParseItems: 5}); err != nil {
		t.Fatalf("single long line at source limit failed: %v", err)
	}
}

func TestNestingDepthLimitInline(t *testing.T) {
	atLimit := nestedStrong(4)
	if _, err := ParseWithLimits(atLimit, Limits{MaxNestingDepth: 4}); err != nil {
		t.Fatalf("inline nesting exactly at limit failed: %v", err)
	}
	if _, err := ParseWithLimits(nestedStrong(5), Limits{MaxNestingDepth: 4}); !errors.Is(err, ErrNestingDepthLimit) {
		t.Fatalf("inline nesting over limit error = %v, want ErrNestingDepthLimit", err)
	}
	if _, err := ParseWithLimits(nestedStrong(200), Limits{MaxNestingDepth: 4}); !errors.Is(err, ErrNestingDepthLimit) {
		t.Fatalf("deep malicious inline error = %v, want ErrNestingDepthLimit", err)
	}
}

func TestNestingDepthLimitLists(t *testing.T) {
	if _, err := ParseWithLimits(nestedList(4), Limits{MaxNestingDepth: 4}); err != nil {
		t.Fatalf("list nesting exactly at limit failed: %v", err)
	}
	if _, err := ParseWithLimits(nestedList(5), Limits{MaxNestingDepth: 4}); !errors.Is(err, ErrNestingDepthLimit) {
		t.Fatalf("list nesting over limit error = %v, want ErrNestingDepthLimit", err)
	}
}

func TestLimitsConfiguration(t *testing.T) {
	normalized, err := normalizedLimits(Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if normalized.MaxSourceBytes != DefaultMaxSourceBytes ||
		normalized.MaxNestingDepth != DefaultMaxNestingDepth ||
		normalized.MaxParseItems != DefaultMaxParseItems {
		t.Fatalf("zero limits did not select defaults: %#v", normalized)
	}
	if _, err := Parse("ordinary"); err != nil {
		t.Fatalf("production Parse() failed ordinary input: %v", err)
	}
	invalid := []Limits{
		{MaxSourceBytes: -1},
		{MaxNestingDepth: -1},
		{MaxParseItems: -1},
	}
	for _, limits := range invalid {
		if _, err := ParseWithLimits("text", limits); !errors.Is(err, ErrInvalidLimits) {
			t.Errorf("ParseWithLimits(%#v) error = %v, want ErrInvalidLimits", limits, err)
		}
	}
}

func nestedStrong(depth int) string {
	text := "x"
	for range depth {
		text = "**" + text + "**"
	}
	return text
}

func nestedList(depth int) string {
	var lines []string
	for level := 0; level < depth; level++ {
		lines = append(lines, strings.Repeat("  ", level)+"- level")
	}
	return strings.Join(lines, "\n")
}
