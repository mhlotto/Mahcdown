package minimark

import (
	"fmt"
	"html"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Document represents a parsed MiniMark document.
type Document struct {
	Blocks []Block
}

// Align represents column alignment for tables.
type Align int

const (
	AlignNone Align = iota
	AlignLeft
	AlignRight
	AlignCenter
)

// Block is implemented by all block node types.
type Block interface {
	blockNode()
}

// Inline is implemented by all inline node types.
type Inline interface {
	inlineNode()
}

// CodeBlock is a fenced code block.
type CodeBlock struct {
	Info string
	Text string
}

// Heading is a heading block.
type Heading struct {
	Level   int
	Inlines []Inline
}

// HorizontalRule is a horizontal rule block.
type HorizontalRule struct{}

// Table is a pipe-delimited table.
type Table struct {
	Headers [][]Inline
	Aligns  []Align
	Rows    [][][]Inline
}

// List is an ordered or unordered list.
type List struct {
	Ordered  bool
	HasStart bool
	Start    int
	Items    []ListItem
}

// ListItem represents a single list item.
type ListItem struct {
	HasCheckbox bool
	Checked     bool
	Blocks      []Block
}

// BlockQuote is a blockquote containing normal block nodes.
type BlockQuote struct {
	Blocks []Block
}

// Paragraph is a paragraph block.
type Paragraph struct {
	Inlines []Inline
}

// CodeSpan is an inline code span.
type CodeSpan struct {
	Text string
}

// Image is an inline image.
type Image struct {
	Alt string
	URL string
}

// Url is an inline URL (autolink or bare).
type Url struct {
	URL  string
	Text string
}

// Checkbox is a line-initial checkbox marker.
type Checkbox struct {
	Checked bool
}

// Strong is bold text.
type Strong struct {
	Inlines []Inline
}

// Emphasis is italic text.
type Emphasis struct {
	Inlines []Inline
}

// Text is literal inline text.
type Text struct {
	Text string
}

func (CodeBlock) blockNode()      {}
func (Heading) blockNode()        {}
func (HorizontalRule) blockNode() {}
func (Table) blockNode()          {}
func (List) blockNode()           {}
func (BlockQuote) blockNode()     {}
func (Paragraph) blockNode()      {}

func (CodeSpan) inlineNode() {}
func (Image) inlineNode()    {}
func (Url) inlineNode()      {}
func (Checkbox) inlineNode() {}
func (Strong) inlineNode()   {}
func (Emphasis) inlineNode() {}
func (Text) inlineNode()     {}

// Parse converts MiniMark text into a Document AST using the default safety limits.
func Parse(input string) (Document, error) {
	return ParseWithLimits(input, Limits{})
}

// ParseWithLimits converts MiniMark text into a Document AST using limits.
func ParseWithLimits(input string, limits Limits) (Document, error) {
	limits, err := normalizedLimits(limits)
	if err != nil {
		return Document{}, err
	}
	if len(input) > limits.MaxSourceBytes {
		return Document{}, &LimitError{Kind: ErrSourceSizeLimit, Limit: limits.MaxSourceBytes}
	}
	state := &parserState{limits: limits}
	if !state.consumeItem() { // Document root.
		return Document{}, state.err
	}
	norm := normalizeNewlines(input)
	lines := splitDocumentLines(state, norm)
	if state.err != nil {
		return Document{}, state.err
	}
	blocks, _ := parseBlocks(state, lines, 0, 0, 0)
	if state.err != nil {
		return Document{}, state.err
	}
	return Document{Blocks: blocks}, nil
}

func splitDocumentLines(state *parserState, input string) []string {
	var lines []string
	start := 0
	for {
		if !state.consumeItem() {
			return nil
		}
		relativeEnd := strings.IndexByte(input[start:], '\n')
		if relativeEnd < 0 {
			return append(lines, input[start:])
		}
		end := start + relativeEnd
		lines = append(lines, input[start:end])
		start = end + 1
	}
}

func parseBlocks(state *parserState, lines []string, start, baseIndent, depth int) ([]Block, int) {
	if !state.allowDepth(depth) {
		return nil, start
	}
	var blocks []Block
	i := start
	for i < len(lines) && state.err == nil {
		line := lines[i]
		if isBlank(line) {
			i++
			continue
		}
		if leadingSpacesCount(line) < baseIndent {
			break
		}
		rel := trimIndent(line, baseIndent)

		// 1. Fenced code block
		if ok, info := isFenceStart(rel); ok {
			if !state.consumeItem() {
				break
			}
			i, blocks = parseCodeBlock(state, lines, i, baseIndent, info, &blocks)
			continue
		}

		// 2. Heading
		if lvl, text, ok := parseHeadingLine(rel); ok {
			if !state.consumeItem() {
				break
			}
			blocks = append(blocks, Heading{
				Level:   lvl,
				Inlines: parseInlines(state, text, depth),
			})
			i++
			continue
		}

		// 3. Horizontal rule
		if isHorizontalRule(rel) {
			if state.consumeItem() {
				blocks = append(blocks, HorizontalRule{})
			}
			i++
			continue
		}

		// 4. Table
		if isTableStart(lines, i, baseIndent) {
			if !state.consumeItem() {
				break
			}
			tbl, consumed := tryParseTable(state, lines, i, baseIndent, depth)
			blocks = append(blocks, tbl)
			i += consumed
			continue
		}

		// 5. List
		if _, ok := parseListMarker(rel); ok {
			if !state.consumeItem() {
				break
			}
			lst, consumed := tryParseList(state, lines, i, baseIndent, depth)
			blocks = append(blocks, lst)
			i += consumed
			continue
		}

		// 6. Blockquote
		if isBlockQuoteLine(rel) {
			if !state.consumeItem() {
				break
			}
			i, blocks = parseBlockQuote(state, lines, i, baseIndent, depth, &blocks)
			continue
		}

		// 7. Paragraph
		i, blocks = parseParagraph(state, lines, i, baseIndent, depth, &blocks)
	}
	return blocks, i
}

func normalizeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

func trimIndent(s string, indent int) string {
	count := 0
	for i := 0; i < len(s) && count < indent; i++ {
		if s[i] != ' ' {
			return s[i:]
		}
		count++
	}
	if indent >= len(s) {
		return ""
	}
	return s[indent:]
}

func isBlank(line string) bool {
	for _, r := range line {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

func leadingSpacesCount(s string) int {
	count := 0
	for _, r := range s {
		if r == ' ' {
			count++
		} else {
			break
		}
	}
	return count
}

func isFenceStart(line string) (bool, string) {
	trimLead := leadingSpacesCount(line)
	if trimLead > 3 {
		return false, ""
	}
	if len(line[trimLead:]) < 3 || line[trimLead:trimLead+3] != "```" {
		return false, ""
	}
	rest := line[trimLead+3:]
	return true, strings.TrimSpace(rest)
}

func parseCodeBlock(state *parserState, lines []string, start int, baseIndent int, info string, blocks *[]Block) (int, []Block) {
	var content []string
	for i := start + 1; i < len(lines) && state.err == nil; i++ {
		line := trimIndent(lines[i], baseIndent)
		if trimLead := leadingSpacesCount(line); trimLead <= 3 {
			after := line[trimLead:]
			if strings.HasPrefix(after, "```") && strings.TrimSpace(after) == "```" {
				*blocks = append(*blocks, CodeBlock{
					Info: info,
					Text: strings.Join(content, "\n") + "\n",
				})
				return i + 1, *blocks
			}
		}
		if !state.consumeItem() {
			return i, *blocks
		}
		content = append(content, line)
	}

	// No closing fence; extend to EOF.
	*blocks = append(*blocks, CodeBlock{
		Info: info,
		Text: strings.Join(content, "\n"),
	})
	return len(lines), *blocks
}

func parseHeadingLine(line string) (level int, text string, ok bool) {
	trimLead := leadingSpacesCount(line)
	if trimLead > 3 {
		return 0, "", false
	}
	l := line[trimLead:]
	hashes := 0
	for hashes < len(l) && l[hashes] == '#' && hashes < 6 {
		hashes++
	}
	if hashes == 0 || hashes > 6 {
		return 0, "", false
	}
	if len(l) <= hashes || l[hashes] != ' ' {
		return 0, "", false
	}
	return hashes, l[hashes+1:], true
}

func isHorizontalRule(line string) bool {
	trimmed := strings.TrimSpace(line)
	switch trimmed {
	case "---", "***", "___":
		return true
	default:
		return false
	}
}

func tryParseTable(state *parserState, lines []string, start int, baseIndent, depth int) (Table, int) {
	if start+1 >= len(lines) {
		return Table{}, 0
	}
	headerLine := trimIndent(lines[start], baseIndent)
	sepLine := trimIndent(lines[start+1], baseIndent)
	if isBlank(headerLine) || !containsStructuralTablePipe(headerLine) {
		return Table{}, 0
	}

	headerCells := splitTableRow(state, headerLine)
	if state.err != nil {
		return Table{}, 0
	}
	aligns, ok := parseSeparatorAlignments(state, sepLine, len(headerCells))
	if !ok {
		return Table{}, 0
	}

	rows := make([][][]Inline, 0)
	consumed := 2
	for i := start + 2; i < len(lines) && state.err == nil; i++ {
		line := trimIndent(lines[i], baseIndent)
		if isBlank(line) || !containsStructuralTablePipe(line) {
			break
		}
		if !state.consumeItem() { // Retained table row.
			break
		}
		rowCells := splitTableRow(state, line)
		if len(rowCells) == 0 {
			break
		}
		row := make([][]Inline, 0, len(rowCells))
		for _, cell := range rowCells {
			if state.err != nil {
				break
			}
			row = append(row, parseInlines(state, cell, depth))
		}
		rows = append(rows, row)
		consumed++
	}

	headers := make([][]Inline, len(headerCells))
	for idx, cell := range headerCells {
		if state.err != nil {
			break
		}
		headers[idx] = parseInlines(state, cell, depth)
	}

	return Table{
		Headers: headers,
		Aligns:  aligns,
		Rows:    rows,
	}, consumed
}

func splitTableRow(state *parserState, line string) []string {
	trimmed := trimOuterTablePipes(line)
	var cells []string
	if !state.consumeItem() { // First retained table cell.
		return nil
	}
	start := 0
	for i := 0; i < len(trimmed); i++ {
		if isStructuralTablePipe(trimmed, i) {
			cells = append(cells, strings.TrimSpace(trimmed[start:i]))
			if !state.consumeItem() { // Next retained table cell.
				return cells
			}
			start = i + 1
		}
	}
	cells = append(cells, strings.TrimSpace(trimmed[start:]))
	return cells
}

func trimOuterTablePipes(line string) string {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "|") {
		trimmed = trimmed[1:]
	}
	if len(trimmed) > 0 && isStructuralTablePipe(trimmed, len(trimmed)-1) {
		trimmed = trimmed[:len(trimmed)-1]
	}
	return trimmed
}

func isEscapableASCIIPunctuation(b byte) bool {
	return b >= '!' && b <= '~' &&
		!(b >= '0' && b <= '9') &&
		!(b >= 'A' && b <= 'Z') &&
		!(b >= 'a' && b <= 'z')
}

func isEscapedAt(text string, index int) bool {
	backslashes := 0
	for index > 0 && text[index-1] == '\\' {
		backslashes++
		index--
	}
	return backslashes%2 == 1
}

func isStructuralTablePipe(text string, index int) bool {
	return index >= 0 && index < len(text) && text[index] == '|' && !isEscapedAt(text, index)
}

func containsStructuralTablePipe(text string) bool {
	for i := 0; i < len(text); i++ {
		if isStructuralTablePipe(text, i) {
			return true
		}
	}
	return false
}

func findUnescapedByte(text string, target byte, start int) int {
	for i := start; i < len(text); i++ {
		if text[i] == target && !isEscapedAt(text, i) {
			return i
		}
	}
	return -1
}

func unescapeBackslashPunctuation(text string) string {
	first := -1
	for i := 0; i+1 < len(text); i++ {
		if text[i] == '\\' && isEscapableASCIIPunctuation(text[i+1]) {
			first = i
			break
		}
	}
	if first < 0 {
		return text
	}
	var result strings.Builder
	result.Grow(len(text) - 1)
	result.WriteString(text[:first])
	for i := first; i < len(text); i++ {
		if text[i] == '\\' && i+1 < len(text) && isEscapableASCIIPunctuation(text[i+1]) {
			i++
		}
		result.WriteByte(text[i])
	}
	return result.String()
}

func parseSeparatorAlignments(state *parserState, line string, expected int) ([]Align, bool) {
	trimmed := trimOuterTablePipes(line)
	aligns := make([]Align, 0, expected)
	start := 0
	for i := 0; i <= len(trimmed); i++ {
		if i < len(trimmed) && !isStructuralTablePipe(trimmed, i) {
			continue
		}
		align, ok := parseSeparatorCell(trimmed[start:i])
		if !ok || !state.consumeItem() { // Retained column alignment.
			return nil, false
		}
		aligns = append(aligns, align)
		start = i + 1
	}
	return aligns, len(aligns) == expected
}

func parseSeparatorCell(cell string) (Align, bool) {
	trimmed := strings.TrimSpace(cell)
	if trimmed == "" {
		return AlignNone, false
	}
	for _, r := range trimmed {
		if r != '-' && r != ':' {
			return AlignNone, false
		}
	}
	if strings.Count(trimmed, "-") < 3 {
		return AlignNone, false
	}
	left := strings.HasPrefix(trimmed, ":")
	right := strings.HasSuffix(trimmed, ":")
	switch {
	case left && right:
		return AlignCenter, true
	case left:
		return AlignLeft, true
	case right:
		return AlignRight, true
	default:
		return AlignNone, true
	}
}

func tryParseList(state *parserState, lines []string, start int, baseIndent, depth int) (List, int) {
	line := lines[start]
	if leadingSpacesCount(line) < baseIndent {
		return List{}, 0
	}
	rel := trimIndent(line, baseIndent)
	marker, ok := parseListMarker(rel)
	if !ok {
		return List{}, 0
	}

	list := List{
		Ordered:  marker.ordered,
		HasStart: marker.ordered,
		Start:    marker.start,
	}

	i := start
	for i < len(lines) && state.err == nil {
		// End conditions: dedent or non-marker at same level.
		if isBlank(lines[i]) {
			i++
			continue
		}
		if leadingSpacesCount(lines[i]) < baseIndent {
			break
		}
		relLine := trimIndent(lines[i], baseIndent)
		if m, ok := parseListMarker(relLine); ok && m.ordered == list.Ordered {
			if !state.consumeItem() { // ListItem.
				break
			}
			item, next := parseListItem(state, lines, i, baseIndent, depth, m)
			list.Items = append(list.Items, item)
			i = next
			continue
		}
		break
	}

	if len(list.Items) == 0 {
		return List{}, 0
	}
	return list, i - start
}

type listMarker struct {
	ordered bool
	start   int
	indent  int
	length  int // marker length including trailing space and leading whitespace already trimmed? length from line start after base?
}

func parseListMarker(line string) (listMarker, bool) {
	ws := leadingSpacesCount(line)
	if ws > 3 {
		return listMarker{}, false
	}
	rest := line[ws:]
	if rest == "" {
		return listMarker{}, false
	}
	// Unordered
	if (rest[0] == '-' || rest[0] == '*' || rest[0] == '+') && len(rest) > 1 && rest[1] == ' ' {
		return listMarker{ordered: false, start: 0, indent: ws, length: ws + 2}, true
	}
	// Ordered
	numEnd := 0
	for numEnd < len(rest) && rest[numEnd] >= '0' && rest[numEnd] <= '9' {
		numEnd++
	}
	if numEnd > 0 && numEnd+1 < len(rest) && rest[numEnd] == '.' && rest[numEnd+1] == ' ' {
		val := 0
		for i := 0; i < numEnd; i++ {
			val = val*10 + int(rest[i]-'0')
		}
		return listMarker{ordered: true, start: val, indent: ws, length: ws + numEnd + 2}, true
	}
	return listMarker{}, false
}

func parseListItem(state *parserState, lines []string, start int, baseIndent int, depth int, marker listMarker) (ListItem, int) {
	contentIndent := baseIndent + marker.length
	nestedMinIndent := baseIndent + marker.indent + 2
	rel := trimIndent(lines[start], baseIndent)
	if marker.length > len(rel) {
		marker.length = len(rel)
	}
	content := rel[marker.length:]
	hasCheckbox, checked := false, false
	if cChecked, consumed, ok := parseCheckbox(content); ok {
		hasCheckbox = true
		checked = cChecked
		content = content[consumed:]
	}

	var itemLines []string
	if !state.consumeItem() {
		return ListItem{}, start + 1
	}
	itemLines = append(itemLines, content)

	i := start + 1
	for i < len(lines) && state.err == nil {
		line := lines[i]
		if isBlank(line) {
			if !state.consumeItem() {
				break
			}
			itemLines = append(itemLines, "")
			i++
			continue
		}
		indent := leadingSpacesCount(line)
		if indent < baseIndent {
			break
		}
		relLine := trimIndent(line, baseIndent)
		if indent >= baseIndent && indent < nestedMinIndent {
			if _, ok := parseListMarker(relLine); ok {
				break
			}
		}
		if indent < contentIndent && !isBlank(line) {
			if indent >= nestedMinIndent {
				candidate := trimIndent(line, nestedMinIndent)
				if _, ok := parseListMarker(candidate); ok {
					if !state.consumeItem() {
						break
					}
					itemLines = append(itemLines, candidate)
					i++
					continue
				}
			}
			break
		}
		if indent >= contentIndent {
			candidate := trimIndent(line, nestedMinIndent)
			if _, ok := parseListMarker(candidate); ok {
				if !state.consumeItem() {
					break
				}
				itemLines = append(itemLines, candidate)
			} else {
				if !state.consumeItem() {
					break
				}
				itemLines = append(itemLines, trimIndent(line, contentIndent))
			}
		} else {
			if !state.consumeItem() {
				break
			}
			itemLines = append(itemLines, "")
		}
		i++
	}

	blocks, _ := parseBlocks(state, itemLines, 0, 0, depth+1)
	return ListItem{
		HasCheckbox: hasCheckbox,
		Checked:     checked,
		Blocks:      blocks,
	}, i
}

func isBlockQuoteLine(line string) bool {
	trimLead := leadingSpacesCount(line)
	if trimLead > 3 {
		return false
	}
	l := line[trimLead:]
	return strings.HasPrefix(l, ">")
}

func parseBlockQuote(state *parserState, lines []string, start int, baseIndent int, depth int, blocks *[]Block) (int, []Block) {
	var quoteLines []string
	i := start
	for ; i < len(lines) && state.err == nil; i++ {
		line := trimIndent(lines[i], baseIndent)
		if !isBlockQuoteLine(line) {
			break
		}
		trimLead := leadingSpacesCount(line)
		content := line[trimLead+1:]
		if strings.HasPrefix(content, " ") {
			content = content[1:]
		}
		if !state.consumeItem() { // Retained blockquote line.
			return i, *blocks
		}
		quoteLines = append(quoteLines, content)
	}
	childBlocks, next := parseBlocks(state, quoteLines, 0, 0, depth+1)
	if state.err != nil {
		return i, *blocks
	}
	if next != len(quoteLines) {
		state.err = fmt.Errorf("parse blockquote: child parser stopped at line %d of %d", next, len(quoteLines))
		return i, *blocks
	}
	return i, appendBlockQuote(*blocks, childBlocks)
}

func appendBlockQuote(blocks []Block, children []Block) []Block {
	return append(blocks, BlockQuote{Blocks: children})
}

func parseParagraph(state *parserState, lines []string, start int, baseIndent int, depth int, blocks *[]Block) (int, []Block) {
	var paraLines []string
	for i := start; i < len(lines) && state.err == nil; i++ {
		line := lines[i]
		if isBlank(line) {
			i++
			return i, appendParagraph(state, *blocks, strings.Join(paraLines, "\n"), depth)
		}

		if leadingSpacesCount(line) < baseIndent {
			return i, appendParagraph(state, *blocks, strings.Join(paraLines, "\n"), depth)
		}

		rel := trimIndent(line, baseIndent)

		// Check if a higher-precedence block would start here.
		if ok, _ := isFenceStart(rel); ok {
			return i, appendParagraph(state, *blocks, strings.Join(paraLines, "\n"), depth)
		}
		if _, _, ok := parseHeadingLine(rel); ok {
			return i, appendParagraph(state, *blocks, strings.Join(paraLines, "\n"), depth)
		}
		if isHorizontalRule(rel) {
			return i, appendParagraph(state, *blocks, strings.Join(paraLines, "\n"), depth)
		}
		if isTableStart(lines, i, baseIndent) {
			return i, appendParagraph(state, *blocks, strings.Join(paraLines, "\n"), depth)
		}
		if isBlockQuoteLine(rel) {
			return i, appendParagraph(state, *blocks, strings.Join(paraLines, "\n"), depth)
		}
		if _, ok := parseListMarker(rel); ok {
			return i, appendParagraph(state, *blocks, strings.Join(paraLines, "\n"), depth)
		}

		if !state.consumeItem() {
			return i, *blocks
		}
		paraLines = append(paraLines, trimIndent(line, baseIndent))
	}
	return len(lines), appendParagraph(state, *blocks, strings.Join(paraLines, "\n"), depth)
}

func isTableStart(lines []string, start, baseIndent int) bool {
	if start+1 >= len(lines) {
		return false
	}
	header := trimIndent(lines[start], baseIndent)
	if isBlank(header) || !containsStructuralTablePipe(header) {
		return false
	}
	headerCount := tableCellCount(header)
	return headerCount > 0 && validSeparatorLine(trimIndent(lines[start+1], baseIndent), headerCount)
}

func tableCellCount(line string) int {
	trimmed := trimOuterTablePipes(line)
	count := 1
	for i := 0; i < len(trimmed); i++ {
		if isStructuralTablePipe(trimmed, i) {
			count++
		}
	}
	return count
}

func validSeparatorLine(line string, expected int) bool {
	trimmed := trimOuterTablePipes(line)
	count := 0
	start := 0
	for i := 0; i <= len(trimmed); i++ {
		if i < len(trimmed) && !isStructuralTablePipe(trimmed, i) {
			continue
		}
		if _, ok := parseSeparatorCell(trimmed[start:i]); !ok {
			return false
		}
		count++
		start = i + 1
	}
	return count == expected
}

func appendParagraph(state *parserState, blocks []Block, text string, depth int) []Block {
	if !state.consumeItem() {
		return blocks
	}
	return append(blocks, Paragraph{Inlines: parseInlines(state, text, depth)})
}

func parseInlines(state *parserState, text string, depth int) []Inline {
	if !state.allowDepth(depth) {
		return nil
	}
	backtickRuns := indexBacktickRuns(state, text)
	if state.err != nil {
		return nil
	}
	backtickCursor := 0
	sequence := &inlineSequence{}
	var firstDelimiter, lastDelimiter *asteriskDelimiter
	i := 0
	textStart := 0
	var pendingText strings.Builder
	atLineStart := true
	suppressedBareURLAt := -1
	flushText := func(end int) bool {
		if end > textStart {
			pendingText.WriteString(text[textStart:end])
		}
		if pendingText.Len() == 0 {
			return true
		}
		value := pendingText.String()
		pendingText.Reset()
		return sequence.append(state, Text{Text: value}, 0) != nil
	}
	appendInline := func(inline Inline, consumed int) bool {
		if !flushText(i) || sequence.append(state, inline, 0) == nil {
			return false
		}
		i += consumed
		textStart = i
		atLineStart = false
		return true
	}
	for i < len(text) && state.err == nil {
		if text[i] == '\\' && i+1 < len(text) && isEscapableASCIIPunctuation(text[i+1]) {
			pendingText.WriteString(text[textStart:i])
			escaped := text[i+1]
			pendingText.WriteByte(escaped)
			i += 2
			if escaped == '*' || escaped == '`' {
				for i < len(text) && text[i] == escaped {
					pendingText.WriteByte(text[i])
					i++
				}
			}
			if escaped == '<' {
				suppressedBareURLAt = i
			}
			textStart = i
			atLineStart = false
			continue
		}
		if atLineStart {
			if checked, consumed, ok := parseCheckbox(text[i:]); ok {
				appendInline(Checkbox{Checked: checked}, consumed)
				continue
			}
		}

		// Code span
		if text[i] == '`' {
			for backtickCursor < len(backtickRuns) && backtickRuns[backtickCursor].start < i {
				backtickCursor++
			}
			if backtickCursor < len(backtickRuns) && backtickRuns[backtickCursor].start == i {
				run := backtickRuns[backtickCursor]
				if content, consumed, ok := parseCodeSpan(text, backtickRuns, backtickCursor); ok {
					appendInline(CodeSpan{Text: content}, consumed)
					continue
				}
				// An unmatched maximal run remains literal. Advance over the whole
				// run so no subset can become a delimiter.
				i += run.length
				atLineStart = false
				continue
			}
		}

		// Image
		if strings.HasPrefix(text[i:], "![") {
			if alt, url, consumed, ok := parseImage(text[i:]); ok {
				appendInline(Image{Alt: alt, URL: url}, consumed)
				continue
			}
		}

		// Autolink
		if text[i] == '<' {
			if url, consumed, ok := parseAutoLink(text[i:]); ok {
				appendInline(Url{URL: url, Text: url}, consumed)
				continue
			}
		}

		if text[i] == '*' {
			if !flushText(i) {
				break
			}
			runLength := asteriskRunLength(text, i)
			node := sequence.append(state, Text{Text: text[i : i+runLength]}, 0)
			if node == nil || !state.consumeItem() {
				break
			}
			canOpen, canClose := classifyAsteriskRun(text, i, i+runLength)
			delimiter := &asteriskDelimiter{
				node: node, originalLength: runLength, length: runLength,
				canOpen: canOpen, canClose: canClose, prev: lastDelimiter,
			}
			if lastDelimiter != nil {
				lastDelimiter.next = delimiter
			} else {
				firstDelimiter = delimiter
			}
			lastDelimiter = delimiter
			i += runLength
			textStart = i
			atLineStart = false
			continue
		}

		// Bare URL
		if i != suppressedBareURLAt && (strings.HasPrefix(text[i:], "http://") || strings.HasPrefix(text[i:], "https://")) {
			url, consumed := parseBareURL(text[i:])
			appendInline(Url{URL: url, Text: url}, consumed)
			continue
		}

		// Plain text
		atLineStart = text[i] == '\n'
		i++
	}
	flushText(i)
	if state.err == nil {
		processAsteriskDelimiters(state, sequence, firstDelimiter, depth)
	}
	if state.err != nil {
		return nil
	}
	return sequence.inlines()
}

type backtickRun struct {
	start    int
	length   int
	nextSame int
}

func indexBacktickRuns(state *parserState, text string) []backtickRun {
	lastByLength := make(map[int]int)
	var runs []backtickRun
	for i := 0; i < len(text); {
		if text[i] != '`' {
			i++
			continue
		}
		length := backtickRunLength(text, i)
		if !state.consumeItem() {
			return nil
		}
		index := len(runs)
		runs = append(runs, backtickRun{start: i, length: length, nextSame: -1})
		if previous, ok := lastByLength[length]; ok {
			runs[previous].nextSame = index
		}
		lastByLength[length] = index
		i += length
	}
	return runs
}

func backtickRunLength(text string, start int) int {
	end := start
	for end < len(text) && text[end] == '`' {
		end++
	}
	return end - start
}

func parseCodeSpan(text string, runs []backtickRun, openerIndex int) (content string, consumed int, ok bool) {
	opener := runs[openerIndex]
	if opener.nextSame < 0 {
		return "", 0, false
	}
	closer := runs[opener.nextSame]
	contentStart := opener.start + opener.length
	content = normalizeCodeSpanContent(text[contentStart:closer.start])
	return content, closer.start + closer.length - opener.start, true
}

func normalizeCodeSpanContent(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.NewReplacer("\r", " ", "\n", " ").Replace(content)
	if len(content) >= 2 && content[0] == ' ' && content[len(content)-1] == ' ' && strings.Trim(content, " ") != "" {
		return content[1 : len(content)-1]
	}
	return content
}

type inlineNode struct {
	inline Inline
	depth  int
	prev   *inlineNode
	next   *inlineNode
}

type inlineSequence struct {
	first *inlineNode
	last  *inlineNode
}

func (sequence *inlineSequence) append(state *parserState, inline Inline, depth int) *inlineNode {
	if !state.consumeItem() {
		return nil
	}
	node := &inlineNode{inline: inline, depth: depth, prev: sequence.last}
	if sequence.last != nil {
		sequence.last.next = node
	} else {
		sequence.first = node
	}
	sequence.last = node
	return node
}

func (sequence *inlineSequence) inlines() []Inline {
	var inlines []Inline
	for node := sequence.first; node != nil; node = node.next {
		if text, ok := node.inline.(Text); ok && text.Text == "" {
			continue
		}
		inlines = appendMergedInline(inlines, node.inline)
	}
	return inlines
}

func appendMergedInline(inlines []Inline, inline Inline) []Inline {
	text, isText := inline.(Text)
	if !isText || len(inlines) == 0 {
		return append(inlines, inline)
	}
	previous, previousIsText := inlines[len(inlines)-1].(Text)
	if !previousIsText {
		return append(inlines, inline)
	}
	previous.Text += text.Text
	inlines[len(inlines)-1] = previous
	return inlines
}

type asteriskDelimiter struct {
	node           *inlineNode
	originalLength int
	length         int
	canOpen        bool
	canClose       bool
	prev           *asteriskDelimiter
	next           *asteriskDelimiter
}

func asteriskRunLength(text string, start int) int {
	end := start
	for end < len(text) && text[end] == '*' {
		end++
	}
	return end - start
}

func classifyAsteriskRun(text string, start, end int) (canOpen, canClose bool) {
	beforeWhitespace, beforePunctuation := inlineRuneClassBefore(text, start)
	afterWhitespace, afterPunctuation := inlineRuneClassAfter(text, end)
	leftFlanking := !afterWhitespace && (!afterPunctuation || beforeWhitespace || beforePunctuation)
	rightFlanking := !beforeWhitespace && (!beforePunctuation || afterWhitespace || afterPunctuation)
	return leftFlanking, rightFlanking
}

func inlineRuneClassBefore(text string, offset int) (whitespace, punctuation bool) {
	if offset == 0 {
		return true, false
	}
	r, _ := utf8.DecodeLastRuneInString(text[:offset])
	return unicode.IsSpace(r), unicode.IsPunct(r) || unicode.IsSymbol(r)
}

func inlineRuneClassAfter(text string, offset int) (whitespace, punctuation bool) {
	if offset == len(text) {
		return true, false
	}
	r, _ := utf8.DecodeRuneInString(text[offset:])
	return unicode.IsSpace(r), unicode.IsPunct(r) || unicode.IsSymbol(r)
}

func processAsteriskDelimiters(state *parserState, sequence *inlineSequence, first *asteriskDelimiter, baseDepth int) {
	var openersBottom [6]*asteriskDelimiter
	for closer := first; closer != nil && state.err == nil; {
		next := closer.next
		if !closer.canClose {
			closer = next
			continue
		}
		bottomIndex := closer.length % 3
		if closer.canOpen {
			bottomIndex += 3
		}
		opener := closer.prev
		for opener != nil && opener != openersBottom[bottomIndex] {
			if opener.canOpen && !violatesRuleOfThree(opener, closer) {
				break
			}
			opener = opener.prev
		}
		if opener == nil || opener == openersBottom[bottomIndex] {
			openersBottom[bottomIndex] = closer.prev
			if !closer.canOpen {
				removeAsteriskDelimiter(closer)
			}
			closer = next
			continue
		}

		use := 1
		if opener.length >= 2 && closer.length >= 2 {
			use = 2
		}
		if !wrapAsteriskSpan(state, sequence, opener, closer, use, baseDepth) {
			return
		}
		for delimiter := opener.next; delimiter != closer; {
			remove := delimiter
			delimiter = delimiter.next
			removeAsteriskDelimiter(remove)
		}
		if opener.length == 0 {
			removeAsteriskDelimiter(opener)
		}
		if closer.length == 0 {
			removeAsteriskDelimiter(closer)
			closer = next
		}
	}
}

func violatesRuleOfThree(opener, closer *asteriskDelimiter) bool {
	if !closer.canOpen && !opener.canClose {
		return false
	}
	sum := opener.originalLength + closer.originalLength
	return sum%3 == 0 && (opener.originalLength%3 != 0 || closer.originalLength%3 != 0)
}

func wrapAsteriskSpan(state *parserState, sequence *inlineSequence, opener, closer *asteriskDelimiter, use, baseDepth int) bool {
	childCount := 0
	childDepth := 0
	for node := opener.node.next; node != closer.node; node = node.next {
		if text, ok := node.inline.(Text); ok && text.Text == "" {
			continue
		}
		childCount++
		if node.depth > childDepth {
			childDepth = node.depth
		}
	}
	containerDepth := childDepth + 1
	if !state.allowDepth(baseDepth+containerDepth) || !state.consumeItem() {
		return false
	}
	children := make([]Inline, 0, childCount)
	for node := opener.node.next; node != closer.node; node = node.next {
		if text, ok := node.inline.(Text); ok && text.Text == "" {
			continue
		}
		children = appendMergedInline(children, node.inline)
	}
	var inline Inline = Emphasis{Inlines: children}
	if use == 2 {
		inline = Strong{Inlines: children}
	}
	container := &inlineNode{inline: inline, depth: containerDepth, prev: opener.node, next: closer.node}
	opener.node.next = container
	closer.node.prev = container
	setDelimiterText(opener, opener.length-use, false)
	setDelimiterText(closer, closer.length-use, true)
	if opener.node.prev == nil {
		sequence.first = opener.node
	}
	if closer.node.next == nil {
		sequence.last = closer.node
	}
	return true
}

func setDelimiterText(delimiter *asteriskDelimiter, remaining int, consumeFromStart bool) {
	text := delimiter.node.inline.(Text).Text
	if consumeFromStart {
		text = text[len(text)-remaining:]
	} else {
		text = text[:remaining]
	}
	delimiter.node.inline = Text{Text: text}
	delimiter.length = remaining
}

func removeAsteriskDelimiter(delimiter *asteriskDelimiter) {
	if delimiter.prev != nil {
		delimiter.prev.next = delimiter.next
	}
	if delimiter.next != nil {
		delimiter.next.prev = delimiter.prev
	}
	delimiter.prev = nil
	delimiter.next = nil
}

func parseImage(s string) (alt, url string, consumed int, ok bool) {
	// s expected to start with "!["
	endAlt := findUnescapedByte(s, ']', 2)
	if endAlt < 2 || s[0:2] != "![" {
		return "", "", 0, false
	}
	altContent := s[2:endAlt]
	rest := s[endAlt+1:]
	if len(rest) < 2 || rest[0] != '(' {
		return "", "", 0, false
	}
	endURL := findUnescapedByte(rest, ')', 1)
	if endURL < 0 {
		return "", "", 0, false
	}
	urlContent := rest[1:endURL]
	consumed = (endAlt + 1) + (endURL + 1)
	return unescapeBackslashPunctuation(altContent), unescapeBackslashPunctuation(urlContent), consumed, true
}

func parseAutoLink(s string) (url string, consumed int, ok bool) {
	if len(s) < 3 || s[0] != '<' {
		return "", 0, false
	}
	end := strings.IndexByte(s, '>')
	if end < 0 {
		return "", 0, false
	}
	content := s[1:end]
	if strings.HasPrefix(content, "http://") || strings.HasPrefix(content, "https://") {
		return content, end + 1, true
	}
	return "", 0, false
}

func parseBareURL(s string) (string, int) {
	stopChars := " \t\n\r)]}>\"'"
	end := 0
	for end < len(s) {
		if strings.ContainsRune(stopChars, rune(s[end])) {
			break
		}
		end++
	}
	url := s[:end]
	// Strip trailing punctuation .,;:!?
	for len(url) > 0 {
		last := url[len(url)-1]
		if strings.ContainsRune(".,;:!?", rune(last)) {
			url = url[:len(url)-1]
			end--
		} else {
			break
		}
	}

	return url, end
}

func parseCheckbox(s string) (checked bool, consumed int, ok bool) {
	if len(s) < 4 || s[0] != '[' || s[2] != ']' || s[3] != ' ' {
		return false, 0, false
	}
	switch s[1] {
	case ' ':
		return false, 4, true
	case 'x', 'X':
		return true, 4, true
	default:
		return false, 0, false
	}
}

// RenderHTML converts a Document AST into HTML.
func RenderHTML(doc Document) string {
	return RenderHTMLWithImagePolicy(doc, ImagePolicy{})
}

// RenderHTMLWithImagePolicy converts a Document AST into HTML using imagePolicy.
func RenderHTMLWithImagePolicy(doc Document, imagePolicy ImagePolicy) string {
	var buf strings.Builder
	buf.WriteString(`<!DOCTYPE html><html><head><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="`)
	buf.WriteString(contentSecurityPolicy)
	buf.WriteString(`"><style>`)
	buf.WriteString(baseCSS)
	buf.WriteString(`</style></head><body>`)
	for _, blk := range doc.Blocks {
		renderBlock(&buf, blk, imagePolicy)
	}
	buf.WriteString(`</body></html>`)
	return buf.String()
}

const contentSecurityPolicy = "default-src 'none'; base-uri file:; img-src file:; style-src 'unsafe-inline'; script-src 'none'; connect-src 'none'; font-src 'none'; media-src 'none'; object-src 'none'; frame-src 'none'; child-src 'none'; worker-src 'none'; manifest-src 'none'; form-action 'none'"

const baseCSS = `
body{font-family:-apple-system,BlinkMacSystemFont,"Helvetica Neue",Arial,sans-serif;line-height:1.6;margin:16px;color:#111;background:#fff;}
p,blockquote p{white-space:pre-wrap;}
pre{background:#e8ebf0;padding:12px;border-radius:6px;overflow:auto;}
code{font-family:Menlo,Monaco,Consolas,"Courier New",monospace;}
:not(pre)>code{white-space:pre-wrap;}
pre code{white-space:pre;}
table{border-collapse:collapse;margin:12px 0;}
th,td{border:1px solid #ddd;padding:6px 10px;}
blockquote{border-left:4px solid #ddd;margin:12px 0;padding-left:12px;color:#444;}
.image-blocked{color:#c00;font-style:italic;}
.mahcdown-link{color:#06c;text-decoration:underline;cursor:pointer;}
.checkbox{vertical-align:middle;margin-right:6px;}
ul,ol{margin:2px 0;padding-left:18px;}
li{margin:0;padding:1px 0;}
li p{margin:0;}
li > ul,li > ol{margin:2px 0 0 18px;}
`

func renderBlock(buf *strings.Builder, blk Block, imagePolicy ImagePolicy) {
	switch b := blk.(type) {
	case Paragraph:
		buf.WriteString("<p>")
		renderInlines(buf, b.Inlines, true, imagePolicy)
		buf.WriteString("</p>")
	case Heading:
		level := b.Level
		if level < 1 || level > 6 {
			level = 1
		}
		fmt.Fprintf(buf, "<h%d>", level)
		renderInlines(buf, b.Inlines, false, imagePolicy)
		fmt.Fprintf(buf, "</h%d>", level)
	case HorizontalRule:
		buf.WriteString("<hr/>")
	case CodeBlock:
		buf.WriteString("<pre><code")
		if b.Info != "" {
			buf.WriteString(` class="language-`)
			buf.WriteString(html.EscapeString(b.Info))
			buf.WriteString(`"`)
		}
		buf.WriteString(">")
		buf.WriteString(html.EscapeString(b.Text))
		buf.WriteString("</code></pre>")
	case Table:
		buf.WriteString("<table><thead><tr>")
		for i, cell := range b.Headers {
			buf.WriteString("<th")
			writeAlign(buf, b.Aligns, i)
			buf.WriteString(">")
			renderInlines(buf, cell, false, imagePolicy)
			buf.WriteString("</th>")
		}
		buf.WriteString("</tr></thead><tbody>")
		for _, row := range b.Rows {
			buf.WriteString("<tr>")
			for i, cell := range row {
				buf.WriteString("<td")
				writeAlign(buf, b.Aligns, i)
				buf.WriteString(">")
				renderInlines(buf, cell, false, imagePolicy)
				buf.WriteString("</td>")
			}
			buf.WriteString("</tr>")
		}
		buf.WriteString("</tbody></table>")
	case BlockQuote:
		buf.WriteString("<blockquote>")
		for _, child := range b.Blocks {
			renderBlock(buf, child, imagePolicy)
		}
		buf.WriteString("</blockquote>")
	case List:
		if b.Ordered {
			buf.WriteString("<ol")
			if b.HasStart {
				fmt.Fprintf(buf, ` start="%d"`, b.Start)
			}
			buf.WriteString(">")
		} else {
			buf.WriteString("<ul>")
		}
		for _, item := range b.Items {
			buf.WriteString("<li>")
			renderListItem(buf, item, imagePolicy)
			buf.WriteString("</li>")
		}
		if b.Ordered {
			buf.WriteString("</ol>")
		} else {
			buf.WriteString("</ul>")
		}
	default:
		// Unknown block: ignore.
	}
}

func renderListItem(buf *strings.Builder, item ListItem, imagePolicy ImagePolicy) {
	if len(item.Blocks) == 0 {
		if item.HasCheckbox {
			buf.WriteString(`<input type="checkbox" disabled class="checkbox"`)
			if item.Checked {
				buf.WriteString(` checked`)
			}
			buf.WriteString(`/>`)
		}
		return
	}

	if para, ok := item.Blocks[0].(Paragraph); ok {
		buf.WriteString("<p>")
		if item.HasCheckbox {
			buf.WriteString(`<input type="checkbox" disabled class="checkbox"`)
			if item.Checked {
				buf.WriteString(` checked`)
			}
			buf.WriteString(`/>`)
		}
		renderInlines(buf, para.Inlines, true, imagePolicy)
		buf.WriteString("</p>")
		for _, child := range item.Blocks[1:] {
			renderBlock(buf, child, imagePolicy)
		}
		return
	}

	if item.HasCheckbox {
		buf.WriteString(`<input type="checkbox" disabled class="checkbox"`)
		if item.Checked {
			buf.WriteString(` checked`)
		}
		buf.WriteString(`/>`)
	}
	for _, child := range item.Blocks {
		renderBlock(buf, child, imagePolicy)
	}
}

func writeAlign(buf *strings.Builder, aligns []Align, idx int) {
	if idx >= len(aligns) {
		return
	}
	switch aligns[idx] {
	case AlignLeft:
		buf.WriteString(` align="left"`)
	case AlignRight:
		buf.WriteString(` align="right"`)
	case AlignCenter:
		buf.WriteString(` align="center"`)
	}
}

func renderInlines(buf *strings.Builder, inlines []Inline, convertNewlines bool, imagePolicy ImagePolicy) {
	for _, inl := range inlines {
		switch v := inl.(type) {
		case Text:
			writeEscapedWithNewlines(buf, v.Text, convertNewlines)
		case CodeSpan:
			buf.WriteString("<code>")
			buf.WriteString(html.EscapeString(v.Text))
			buf.WriteString("</code>")
		case Image:
			parsed, allowed := parseAllowedLocalImageURL(v.URL)
			if !allowed || !imagePolicy.allows(parsed) {
				buf.WriteString(`<span class="image-blocked" data-src="`)
				buf.WriteString(html.EscapeString(v.URL))
				buf.WriteString(`">[image blocked: `)
				buf.WriteString(html.EscapeString(v.Alt))
				buf.WriteString("]</span>")
			} else {
				buf.WriteString(`<img src="`)
				buf.WriteString(html.EscapeString(v.URL))
				buf.WriteString(`" alt="`)
				buf.WriteString(html.EscapeString(v.Alt))
				buf.WriteString(`"/>`)
			}
		case Url:
			buf.WriteString(`<a class="mahcdown-link" role="link" tabindex="0" data-mahcdown-href="`)
			buf.WriteString(html.EscapeString(v.URL))
			buf.WriteString(`">`)
			writeEscapedWithNewlines(buf, v.Text, convertNewlines)
			buf.WriteString("</a>")
		case Checkbox:
			buf.WriteString(`<input type="checkbox" disabled class="checkbox"`)
			if v.Checked {
				buf.WriteString(` checked`)
			}
			buf.WriteString(`/>`)
		case Strong:
			buf.WriteString("<strong>")
			renderInlines(buf, v.Inlines, convertNewlines, imagePolicy)
			buf.WriteString("</strong>")
		case Emphasis:
			buf.WriteString("<em>")
			renderInlines(buf, v.Inlines, convertNewlines, imagePolicy)
			buf.WriteString("</em>")
		default:
			// ignore unknown inline
		}
	}
}

func writeEscapedWithNewlines(buf *strings.Builder, text string, convertNewlines bool) {
	if !convertNewlines || !strings.Contains(text, "\n") {
		buf.WriteString(html.EscapeString(text))
		return
	}
	parts := strings.Split(text, "\n")
	for idx, part := range parts {
		if idx > 0 {
			buf.WriteString("<br/>")
		}
		buf.WriteString(html.EscapeString(part))
	}
}
