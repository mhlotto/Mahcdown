package minimark

import (
	"fmt"
	"html"
	"net/url"
	"strings"
	"unicode"
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

// BlockQuote is a blockquote containing inline-parsed lines.
type BlockQuote struct {
	Lines [][]Inline
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

// Parse converts MiniMark text into a Document AST.
func Parse(input string) Document {
	norm := normalizeNewlines(input)
	lines := strings.Split(norm, "\n")
	blocks, _ := parseBlocks(lines, 0, 0)
	return Document{Blocks: blocks}
}

func parseBlocks(lines []string, start, baseIndent int) ([]Block, int) {
	var blocks []Block
	i := start
	for i < len(lines) {
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
			i, blocks = parseCodeBlock(lines, i, baseIndent, info, &blocks)
			continue
		}

		// 2. Heading
		if lvl, text, ok := parseHeadingLine(rel); ok {
			blocks = append(blocks, Heading{
				Level:   lvl,
				Inlines: parseInlines(text),
			})
			i++
			continue
		}

		// 3. Horizontal rule
		if isHorizontalRule(rel) {
			blocks = append(blocks, HorizontalRule{})
			i++
			continue
		}

		// 4. Table
		if tbl, consumed := tryParseTable(lines, i, baseIndent); consumed > 0 {
			blocks = append(blocks, tbl)
			i += consumed
			continue
		}

		// 5. List
		if lst, consumed := tryParseList(lines, i, baseIndent); consumed > 0 {
			blocks = append(blocks, lst)
			i += consumed
			continue
		}

		// 6. Blockquote
		if isBlockQuoteLine(rel) {
			i, blocks = parseBlockQuote(lines, i, baseIndent, &blocks)
			continue
		}

		// 7. Paragraph
		i, blocks = parseParagraph(lines, i, baseIndent, &blocks)
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

func parseCodeBlock(lines []string, start int, baseIndent int, info string, blocks *[]Block) (int, []Block) {
	var content []string
	for i := start + 1; i < len(lines); i++ {
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

func tryParseTable(lines []string, start int, baseIndent int) (Table, int) {
	if start+1 >= len(lines) {
		return Table{}, 0
	}
	headerLine := trimIndent(lines[start], baseIndent)
	sepLine := trimIndent(lines[start+1], baseIndent)
	if isBlank(headerLine) || !strings.Contains(headerLine, "|") {
		return Table{}, 0
	}

	headerCells := splitTableRow(headerLine)
	sepCells := splitTableRow(sepLine)
	if len(headerCells) == 0 || len(headerCells) != len(sepCells) {
		return Table{}, 0
	}
	aligns, ok := parseSeparatorAlignments(sepCells)
	if !ok {
		return Table{}, 0
	}

	rows := make([][][]Inline, 0)
	consumed := 2
	for i := start + 2; i < len(lines); i++ {
		line := trimIndent(lines[i], baseIndent)
		if isBlank(line) || !strings.Contains(line, "|") {
			break
		}
		rowCells := splitTableRow(line)
		if len(rowCells) == 0 {
			break
		}
		row := make([][]Inline, 0, len(rowCells))
		for _, cell := range rowCells {
			row = append(row, parseInlines(cell))
		}
		rows = append(rows, row)
		consumed++
	}

	headers := make([][]Inline, len(headerCells))
	for idx, cell := range headerCells {
		headers[idx] = parseInlines(cell)
	}

	return Table{
		Headers: headers,
		Aligns:  aligns,
		Rows:    rows,
	}, consumed
}

func splitTableRow(line string) []string {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "|") {
		trimmed = trimmed[1:]
	}
	if strings.HasSuffix(trimmed, "|") {
		trimmed = trimmed[:len(trimmed)-1]
	}
	var cells []string
	var current strings.Builder
	escaped := false
	for i := 0; i < len(trimmed); i++ {
		ch := trimmed[i]
		if escaped {
			if ch == '|' {
				current.WriteByte('|')
			} else {
				current.WriteByte('\\')
				current.WriteByte(ch)
			}
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '|' {
			cells = append(cells, strings.TrimSpace(current.String()))
			current.Reset()
			continue
		}
		current.WriteByte(ch)
	}
	cells = append(cells, strings.TrimSpace(current.String()))
	return cells
}

func parseSeparatorAlignments(cells []string) ([]Align, bool) {
	aligns := make([]Align, 0, len(cells))
	for _, cell := range cells {
		trimmed := strings.TrimSpace(cell)
		if trimmed == "" {
			return nil, false
		}
		for _, r := range trimmed {
			if r != '-' && r != ':' {
				return nil, false
			}
		}
		dashes := strings.Count(trimmed, "-")
		if dashes < 3 {
			return nil, false
		}
		left := strings.HasPrefix(trimmed, ":")
		right := strings.HasSuffix(trimmed, ":")
		switch {
		case left && right:
			aligns = append(aligns, AlignCenter)
		case left:
			aligns = append(aligns, AlignLeft)
		case right:
			aligns = append(aligns, AlignRight)
		default:
			aligns = append(aligns, AlignNone)
		}
	}
	return aligns, true
}

func tryParseList(lines []string, start int, baseIndent int) (List, int) {
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
	for i < len(lines) {
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
			item, next := parseListItem(lines, i, baseIndent, m)
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

func parseListItem(lines []string, start int, baseIndent int, marker listMarker) (ListItem, int) {
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
	itemLines = append(itemLines, content)

	i := start + 1
	for i < len(lines) {
		line := lines[i]
		if isBlank(line) {
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
				itemLines = append(itemLines, candidate)
			} else {
				itemLines = append(itemLines, trimIndent(line, contentIndent))
			}
		} else {
			itemLines = append(itemLines, "")
		}
		i++
	}

	blocks, _ := parseBlocks(itemLines, 0, 0)
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

func parseBlockQuote(lines []string, start int, baseIndent int, blocks *[]Block) (int, []Block) {
	var quoteLines [][]Inline
	for i := start; i < len(lines); i++ {
		line := trimIndent(lines[i], baseIndent)
		if isBlank(line) {
			return i + 1, append(*blocks, BlockQuote{Lines: quoteLines})
		}
		if !isBlockQuoteLine(line) {
			return i, append(*blocks, BlockQuote{Lines: quoteLines})
		}
		trimLead := leadingSpacesCount(line)
		content := line[trimLead+1:]
		if strings.HasPrefix(content, " ") {
			content = content[1:]
		}
		quoteLines = append(quoteLines, parseInlines(content))
	}
	return len(lines), append(*blocks, BlockQuote{Lines: quoteLines})
}

func parseParagraph(lines []string, start int, baseIndent int, blocks *[]Block) (int, []Block) {
	var paraLines []string
	for i := start; i < len(lines); i++ {
		line := lines[i]
		if isBlank(line) {
			i++
			return i, append(*blocks, Paragraph{Inlines: parseInlines(strings.Join(paraLines, "\n"))})
		}

		if leadingSpacesCount(line) < baseIndent {
			return i, append(*blocks, Paragraph{Inlines: parseInlines(strings.Join(paraLines, "\n"))})
		}

		rel := trimIndent(line, baseIndent)

		// Check if a higher-precedence block would start here.
		if ok, _ := isFenceStart(rel); ok {
			return i, append(*blocks, Paragraph{Inlines: parseInlines(strings.Join(paraLines, "\n"))})
		}
		if _, _, ok := parseHeadingLine(rel); ok {
			return i, append(*blocks, Paragraph{Inlines: parseInlines(strings.Join(paraLines, "\n"))})
		}
		if isHorizontalRule(rel) {
			return i, append(*blocks, Paragraph{Inlines: parseInlines(strings.Join(paraLines, "\n"))})
		}
		if tbl, _ := tryParseTable(lines, i, baseIndent); tbl.Aligns != nil && tbl.Headers != nil {
			return i, append(*blocks, Paragraph{Inlines: parseInlines(strings.Join(paraLines, "\n"))})
		}
		if isBlockQuoteLine(rel) {
			return i, append(*blocks, Paragraph{Inlines: parseInlines(strings.Join(paraLines, "\n"))})
		}
		if lst, _ := tryParseList(lines, i, baseIndent); lst.Items != nil {
			return i, append(*blocks, Paragraph{Inlines: parseInlines(strings.Join(paraLines, "\n"))})
		}

		paraLines = append(paraLines, trimIndent(line, baseIndent))
	}
	return len(lines), append(*blocks, Paragraph{Inlines: parseInlines(strings.Join(paraLines, "\n"))})
}

func parseInlines(text string) []Inline {
	var out []Inline
	i := 0
	atLineStart := true
	for i < len(text) {
		if atLineStart {
			if checked, consumed, ok := parseCheckbox(text[i:]); ok {
				out = append(out, Checkbox{Checked: checked})
				i += consumed
				atLineStart = false
				continue
			}
		}

		// Code span
		if text[i] == '`' {
			end := strings.IndexByte(text[i+1:], '`')
			if end >= 0 {
				content := text[i+1 : i+1+end]
				out = append(out, CodeSpan{Text: content})
				i += end + 2
				atLineStart = false
				continue
			}
			// No closing backtick; treat as text.
		}

		// Image
		if strings.HasPrefix(text[i:], "![") {
			if alt, url, consumed, ok := parseImage(text[i:]); ok {
				out = append(out, Image{Alt: alt, URL: url})
				i += consumed
				atLineStart = false
				continue
			}
		}

		// Autolink
		if text[i] == '<' {
			if url, consumed, ok := parseAutoLink(text[i:]); ok {
				out = append(out, Url{URL: url, Text: url})
				i += consumed
				atLineStart = false
				continue
			}
		}

		// Strong
		if strings.HasPrefix(text[i:], "**") {
			if content, consumed, ok := parseDelimited(text[i+2:], "**"); ok && content != "" && !strings.ContainsRune(content, '\n') {
				out = append(out, Strong{Inlines: parseInlines(content)})
				i += 2 + consumed
				atLineStart = false
				continue
			}
		}

		// Emphasis
		if text[i] == '*' {
			if content, consumed, ok := parseDelimitedSingleStar(text[i+1:]); ok && content != "" && !strings.ContainsRune(content, '\n') {
				out = append(out, Emphasis{Inlines: parseInlines(content)})
				i += 1 + consumed
				atLineStart = false
				continue
			}
			// Unmatched '*' becomes text.
		}

		// Bare URL
		if strings.HasPrefix(text[i:], "http://") || strings.HasPrefix(text[i:], "https://") {
			url, consumed := parseBareURL(text[i:])
			out = append(out, Url{URL: url, Text: url})
			i += consumed
			atLineStart = false
			continue
		}

		// Plain text
		out = append(out, Text{Text: string(text[i])})
		atLineStart = text[i] == '\n'
		i++
	}

	out = mergeText(out)
	return out
}

func parseImage(s string) (alt, url string, consumed int, ok bool) {
	// s expected to start with "!["
	endAlt := strings.IndexByte(s, ']')
	if endAlt < 2 || s[0:2] != "![" {
		return "", "", 0, false
	}
	altContent := s[2:endAlt]
	rest := s[endAlt+1:]
	if len(rest) < 2 || rest[0] != '(' {
		return "", "", 0, false
	}
	endURL := strings.IndexByte(rest, ')')
	if endURL < 0 {
		return "", "", 0, false
	}
	urlContent := rest[1:endURL]
	if strings.Contains(altContent, "]") || strings.Contains(urlContent, ")") {
		return "", "", 0, false
	}
	consumed = (endAlt + 1) + (endURL + 1)
	return altContent, urlContent, consumed, true
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

func parseDelimited(s string, delim string) (content string, consumed int, ok bool) {
	end := strings.LastIndex(s, delim)
	if end < 0 {
		return "", 0, false
	}
	return s[:end], end + len(delim), true
}

func parseDelimitedSingleStar(s string) (content string, consumed int, ok bool) {
	for idx := 0; idx < len(s); idx++ {
		if s[idx] != '*' {
			continue
		}
		// closing delimiter must be a single star, not part of "**" (either side)
		if (idx+1 < len(s) && s[idx+1] == '*') || (idx > 0 && s[idx-1] == '*') {
			continue
		}
		return s[:idx], idx + 1, true
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

	// Consume the stop character if it was a closing delimiter to avoid emitting it as text.
	if end < len(s) && strings.ContainsRune(")]}>\"'", rune(s[end])) {
		end++
	}
	return url, end
}

func mergeText(inlines []Inline) []Inline {
	if len(inlines) == 0 {
		return inlines
	}
	var merged []Inline
	var builder strings.Builder
	flush := func() {
		if builder.Len() > 0 {
			merged = append(merged, Text{Text: builder.String()})
			builder.Reset()
		}
	}
	for _, inl := range inlines {
		if t, ok := inl.(Text); ok {
			builder.WriteString(t.Text)
			continue
		}
		flush()
		merged = append(merged, inl)
	}
	flush()
	return merged
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
	var buf strings.Builder
	buf.WriteString(`<!DOCTYPE html><html><head><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="`)
	buf.WriteString(contentSecurityPolicy)
	buf.WriteString(`"><style>`)
	buf.WriteString(baseCSS)
	buf.WriteString(`</style></head><body>`)
	for _, blk := range doc.Blocks {
		renderBlock(&buf, blk)
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
table{border-collapse:collapse;margin:12px 0;}
th,td{border:1px solid #ddd;padding:6px 10px;}
blockquote{border-left:4px solid #ddd;margin:12px 0;padding-left:12px;color:#444;}
.image-blocked{color:#c00;font-style:italic;}
.checkbox{vertical-align:middle;margin-right:6px;}
ul,ol{margin:2px 0;padding-left:18px;}
li{margin:0;padding:1px 0;}
li p{margin:0;}
li > ul,li > ol{margin:2px 0 0 18px;}
`

func renderBlock(buf *strings.Builder, blk Block) {
	switch b := blk.(type) {
	case Paragraph:
		buf.WriteString("<p>")
		renderInlines(buf, b.Inlines, true)
		buf.WriteString("</p>")
	case Heading:
		level := b.Level
		if level < 1 || level > 6 {
			level = 1
		}
		fmt.Fprintf(buf, "<h%d>", level)
		renderInlines(buf, b.Inlines, false)
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
			renderInlines(buf, cell, false)
			buf.WriteString("</th>")
		}
		buf.WriteString("</tr></thead><tbody>")
		for _, row := range b.Rows {
			buf.WriteString("<tr>")
			for i, cell := range row {
				buf.WriteString("<td")
				writeAlign(buf, b.Aligns, i)
				buf.WriteString(">")
				renderInlines(buf, cell, false)
				buf.WriteString("</td>")
			}
			buf.WriteString("</tr>")
		}
		buf.WriteString("</tbody></table>")
	case BlockQuote:
		buf.WriteString("<blockquote><p>")
		for i, line := range b.Lines {
			if i > 0 {
				buf.WriteString("<br/>")
			}
			renderInlines(buf, line, false)
		}
		buf.WriteString("</p></blockquote>")
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
			renderListItem(buf, item)
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

func renderListItem(buf *strings.Builder, item ListItem) {
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
		renderInlines(buf, para.Inlines, true)
		buf.WriteString("</p>")
		for _, child := range item.Blocks[1:] {
			renderBlock(buf, child)
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
		renderBlock(buf, child)
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

func renderInlines(buf *strings.Builder, inlines []Inline, convertNewlines bool) {
	for _, inl := range inlines {
		switch v := inl.(type) {
		case Text:
			writeEscapedWithNewlines(buf, v.Text, convertNewlines)
		case CodeSpan:
			buf.WriteString("<code>")
			buf.WriteString(html.EscapeString(v.Text))
			buf.WriteString("</code>")
		case Image:
			if !isAllowedLocalImageURL(v.URL) {
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
			buf.WriteString(`<a href="`)
			buf.WriteString(html.EscapeString(v.URL))
			buf.WriteString(`" data-href="`)
			buf.WriteString(html.EscapeString(v.URL))
			buf.WriteString(`" rel="noreferrer noopener">`)
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
			renderInlines(buf, v.Inlines, convertNewlines)
			buf.WriteString("</strong>")
		case Emphasis:
			buf.WriteString("<em>")
			renderInlines(buf, v.Inlines, convertNewlines)
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

func isAllowedLocalImageURL(source string) bool {
	if source == "" || source != strings.TrimSpace(source) || strings.Contains(source, `\`) {
		return false
	}
	parsed, err := url.Parse(source)
	if err != nil {
		return false
	}
	if parsed.Scheme != "" || parsed.Host != "" || parsed.User != nil || parsed.Opaque != "" {
		return false
	}
	if parsed.Path == "" || strings.HasPrefix(parsed.Path, "/") || strings.Contains(parsed.Path, `\`) {
		return false
	}
	return true
}
