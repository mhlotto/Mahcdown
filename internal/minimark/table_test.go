package minimark

import (
	"reflect"
	"testing"
)

func TestTableRowsNormalizeToHeaderWidth(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  [][][]Inline
	}{
		{
			name:  "pad missing body cells",
			input: "| a | b | c |\n|---|---|---|\n| 1 | 2 |",
			want:  [][][]Inline{{{Text{Text: "1"}}, {Text{Text: "2"}}, nil}},
		},
		{
			name:  "ignore extra body cells",
			input: "| a | b |\n|---|---|\n| 1 | 2 | 3 |",
			want:  [][][]Inline{{{Text{Text: "1"}}, {Text{Text: "2"}}}},
		},
		{
			name:  "retain explicit empty cells",
			input: "| a | b | c |\n|---|---|---|\n| 1 || 3 |",
			want:  [][][]Inline{{{Text{Text: "1"}}, nil, {Text{Text: "3"}}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table := requireTable(t, tt.input)
			if !reflect.DeepEqual(table.Rows, tt.want) {
				t.Fatalf("rows\nwant: %#v\n got: %#v", tt.want, table.Rows)
			}
			for _, row := range table.Rows {
				if len(row) != len(table.Headers) {
					t.Fatalf("row width = %d, header width = %d", len(row), len(table.Headers))
				}
			}
		})
	}
}

func TestTableStructuralPipesIgnoreMatchedCodeSpans(t *testing.T) {
	tests := []struct {
		name string
		cell string
		want Inline
	}{
		{"single backticks", "`a | b`", CodeSpan{Text: "a | b"}},
		{"multiple backticks", "``a ` | b``", CodeSpan{Text: "a ` | b"}},
		{"different run inside", "```a `` | b```", CodeSpan{Text: "a `` | b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table := requireTable(t, "| code | value |\n|---|---|\n| "+tt.cell+" | x |")
			if len(table.Rows) != 1 || len(table.Rows[0]) != 2 {
				t.Fatalf("code pipe changed row width: %#v", table.Rows)
			}
			if !reflect.DeepEqual(table.Rows[0][0], []Inline{tt.want}) {
				t.Fatalf("code cell = %#v, want %#v", table.Rows[0][0], tt.want)
			}
		})
	}
}

func TestUnmatchedBackticksDoNotHideStructuralTablePipes(t *testing.T) {
	line := "`unmatched | first | ``code | pipe`` | last"
	pipes := structuralTablePipes(line)
	want := []int{11, 19, 37}
	if !reflect.DeepEqual(pipes, want) {
		t.Fatalf("structural pipes for %q = %v, want %v", line, pipes, want)
	}

	table := requireTable(t, "| a | b |\n|---|---|\n| `unmatched | value |")
	if got := len(table.Rows[0]); got != 2 {
		t.Fatalf("unmatched run hid body separator; row width = %d", got)
	}

	escaped := structuralTablePipes(`\` + "`a | b` | c")
	if !reflect.DeepEqual(escaped, []int{4, 9}) {
		t.Fatalf("escaped backtick opener hid structural pipes: %v", escaped)
	}
}

func TestTableEscapesAndOuterPipesUseSharedScanner(t *testing.T) {
	input := `| a \| b | c |
|---|---|
| x \\\| y | tail\ |`
	table := requireTable(t, input)
	wantHeaders := [][]Inline{{Text{Text: "a | b"}}, {Text{Text: "c"}}}
	wantRows := [][][]Inline{{{Text{Text: `x \| y`}}, {Text{Text: `tail\`}}}}
	if !reflect.DeepEqual(table.Headers, wantHeaders) || !reflect.DeepEqual(table.Rows, wantRows) {
		t.Fatalf("escaped table content\nwant headers: %#v rows: %#v\n got headers: %#v rows: %#v", wantHeaders, wantRows, table.Headers, table.Rows)
	}

	odd := structuralTablePipes(`a \\\| b | c`)
	even := structuralTablePipes(`a \\| b | c`)
	if !reflect.DeepEqual(odd, []int{9}) {
		t.Errorf("odd-backslash pipes = %v, want only final structural pipe", odd)
	}
	if !reflect.DeepEqual(even, []int{4, 8}) {
		t.Errorf("even-backslash pipes = %v, want both structural pipes", even)
	}
}

func TestTableHeaderAndSeparatorWidthsMustMatch(t *testing.T) {
	for _, input := range []string{
		"| a | b |\n|---|",
		"| a |\n|---|---|",
		"| `a | b` | c |\n|---|---|---|",
	} {
		doc, err := Parse(input)
		if err != nil {
			t.Fatalf("Parse(%q): %v", input, err)
		}
		if _, ok := doc.Blocks[0].(Table); ok {
			t.Fatalf("mismatched separator width created table for %q: %#v", input, doc)
		}
	}
}

func requireTable(t *testing.T, input string) Table {
	t.Helper()
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse(%q): %v", input, err)
	}
	if len(doc.Blocks) == 0 {
		t.Fatalf("Parse(%q) returned no blocks", input)
	}
	table, ok := doc.Blocks[0].(Table)
	if !ok {
		t.Fatalf("first block = %T, want Table: %#v", doc.Blocks[0], doc)
	}
	return table
}
