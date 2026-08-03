# MiniMark 1.2 Specification

## 0. Scope

MiniMark is a lightweight, deterministic Markdown-style language.
Version 1.2 extends MiniMark with **proper list support**, including nesting, while preserving
a well-defined and parser-friendly rule set.

MiniMark 1.2 supports:

- Inline code spans using backticks: `code`
- Fenced code blocks using triple backticks
- Blockquotes using `>`
- URLs (autolinks and bare URLs)
- Images
- Tables
- Bold text
- Italic text
- Horizontal rules
- Headings (`#` through `######`)
- Checkbox markers: `[ ]` and `[x]`
- Bulleted lists
- Numbered lists
- Nested lists

MiniMark intentionally omits HTML blocks, reference-style links, footnotes, definition lists,
and other complex extensions.

---

## 1. Document Model

A document is parsed into a sequence of **block nodes**.
Some block nodes contain **inline nodes**, and some block nodes may recursively contain other block nodes.

---

## 2. Block Parsing

Parsing proceeds top-to-bottom, line-by-line.
At each line boundary, the parser attempts to start a block using the following precedence order:

1. Fenced code block
2. Heading
3. Horizontal rule
4. Table
5. List
6. Blockquote
7. Paragraph

Indentation is significant for list parsing (see §2.5).

---

## 2.1 Fenced Code Block

A fenced code block opens with a maximal run of at least three identical backticks (`` ` ``) or
tildes (`~`), optionally preceded by zero to three ASCII spaces. Text after the opening run is the
info string; surrounding whitespace is removed before it is stored. A backtick-fence info string
must not contain a backtick. Tilde-fence info strings have no corresponding backtick restriction.

A closing fence:

- Uses the same character as the opener.
- Has a maximal run at least as long as the opening run.
- May have zero to three leading ASCII spaces.
- Contains only whitespace after its fence run.

A shorter run, the other fence character, or a fence followed by non-whitespace remains literal
code content. Code content receives no inline parsing. A terminated block preserves the newline
before its closing fence when it has content; an empty block stores empty text. An unterminated
block extends to end-of-file without error and preserves whether its final content line ended in a
newline.

---

## 2.2 Headings

Identical to MiniMark 1.1.

---

## 2.3 Horizontal Rule

Identical to MiniMark 1.1.

---

## 2.4 Tables

Identical to MiniMark 1.1.

---

## 2.5 Lists

MiniMark supports **unordered (bulleted)** and **ordered (numbered)** lists,
including nested lists up to the configured parser nesting-depth limit.

### 2.5.1 List Item Markers

An unordered list item begins with one of:

- `-`
- `*`
- `+`

An ordered list item begins with:

- One or more digits
- A literal `.` (period)
- A single space

Examples:

```
- item
* item
+ item

1. item
42. item
```

### 2.5.2 Indentation Rules

List indentation uses these columns:

- `listBase` is the indentation base of the current list parser.
- `markerIndent` is the 0–3 optional spaces before a marker, relative to `listBase`.
- `markerColumn` is `listBase + markerIndent`, the absolute column where the marker begins.
- `contentColumn` is the absolute column after the marker and its required literal space.
- `childBase` is `markerColumn + 2`, the minimum base for a nested list.

A child marker may have another 0–3 optional spaces after `childBase`. Ordinary continuation text
must begin at or after `contentColumn`, while a nested list marker may begin at or after
`childBase`. Consequently, a child below a wide ordered marker can begin before the parent's text
column, as in `10. parent` followed by `  - child`.

Only ASCII spaces count as indentation. Tabs do not count as indentation, and the required byte
after `-`, `+`, `*`, or an ordered `.` marker is a literal ASCII space.

### 2.5.3 List Item Content

A list item consists of:

- The marker
- A required single space
- Item content

Item content may span multiple lines.

Continuation lines must be indented **at least to the column of the first content character**
after the marker. That many spaces are stripped; additional indentation remains in the retained
content. Nested list lines instead strip `childBase`, preserving any optional child marker indent.

Blank lines may separate blocks within an item or appear before a nested list. They are retained
for block separation but do not produce empty paragraphs or literal blank-text nodes. Blank lines
before a same-level sibling keep that sibling in the same list. Unindented non-list content after
a blank line ends the item and list.

### 2.5.4 Nested Blocks in List Items

Each list item contains a sequence of block nodes, parsed using the same rules as top-level blocks,
with indentation stripped relative to the list item.

Allowed inside list items:
- Paragraphs
- Nested lists
- Blockquotes
- Tables
- Headings
- Fenced code blocks
- Horizontal rules

Changing among `-`, `+`, and `*` does not split an unordered list. Ordered items remain in one
ordered list even when later marker numbers differ; the list start is the first marker's number.
Changing between ordered and unordered markers at the same level creates adjacent list blocks.
The same switch within an item creates adjacent child list blocks.

List-item block parsing uses the normal parser recursively and increments the shared parser depth
once. `MaxNestingDepth` is the sole list-nesting boundary; exceeding it is an error rather than
silent truncation or partial rendering.

---

### 2.5.5 Checkbox List Items

Checkbox markers (`[ ]`, `[x]`, `[X]`) may appear **immediately after a list item marker**.

Examples:

```
- [ ] todo
- [x] done
1. [ ] numbered todo
```

When present, the checkbox applies to the list item itself.

---

### 2.5.6 List Node Model

```
List {
  ordered: boolean,
  start: number | null,
  items: ListItem[]
}

ListItem {
  checkbox: boolean | null,
  checked: boolean | null,
  blocks: Block[]
}
```

- `start` is the numeric value of the first ordered list item.
- For unordered lists, `start` is `null`.

---

## 2.6 Blockquote

A blockquote is a contiguous sequence of lines that each contain a quote marker. A marker consists
of zero to three spaces followed by `>`, with one optional space removed after the marker. Both
`> text` and `>text` are accepted.

Exactly one quote level is removed from each line, and the resulting lines are parsed recursively
using the normal block parser. Blockquotes may therefore contain paragraphs, headings, lists,
fenced code blocks, tables, horizontal rules, and nested blockquotes.

`>` and `> ` are quoted blank lines and separate paragraphs inside the same blockquote. An
unquoted line, including an unquoted blank line, terminates the contiguous blockquote region. Lazy
continuation lines are not supported. Nested quotes may be written as either `>> text` or
`> > text`.

Parser nesting and parse-item safety limits apply recursively inside blockquotes.

---

## 2.7 Paragraph

Identical to MiniMark 1.1.

---

## 3. Inline Parsing

Inline parsing rules are identical to MiniMark 1.1.

Inline parsing applies inside:
- Paragraphs
- Headings
- Inline-bearing blocks nested inside blockquotes
- Table cells
- List item paragraph content

Inline parsing does **not** apply inside fenced code blocks.

Code spans use maximal backtick delimiter runs. An opening run closes at the first later maximal
run of exactly the same length; differently sized runs inside the span remain literal. This permits
embedded backticks by using a longer surrounding delimiter. Unmatched runs remain literal text.

Inside a matched code span, line endings become ASCII spaces. When the normalized content begins
and ends with an ASCII space and is not entirely ASCII spaces, exactly one space is removed from
each edge. Tabs, Unicode whitespace, interior spaces, backslashes, and other content remain
literal. Code spans are opaque to all other inline parsing. These are MiniMark's defined code-span
rules and do not imply complete CommonMark compatibility.

Asterisk delimiter runs provide `*emphasis*` and `**strong emphasis**`. MiniMark classifies runs
with Unicode left- and right-flanking rules and resolves them with a delimiter stack following the
CommonMark asterisk emphasis algorithm. Underscore emphasis is not supported, and MiniMark does
not claim complete CommonMark compatibility. Unmatched delimiters remain literal.

Emphasis may cross a soft line break inside one inline-bearing block. Delimiter matching resets at
paragraph, heading, list-item, blockquote-child, and table-cell boundaries. Successfully parsed
code spans, images, URLs, checkboxes, and other atomic inline constructs are opaque to delimiter
processing.

Bare `http://` and `https://` URLs end at whitespace, quotes, apostrophes, `]`, `}`, `>`, or an
unmatched closing `)`. Parentheses inside a URL are included while balanced, including nested
pairs. A depth-zero closing parenthesis remains surrounding text. Trailing `.`, `,`, `;`, `:`,
`!`, and `?` are removed from the URL and likewise remain ordinary text. URL parsing never consumes
an excluded delimiter or trailing punctuation byte.

Standard inline links use `[label](destination)`. Labels are parsed as inline content and may
contain emphasis, strong emphasis, code spans, images, and backslash escapes. Successfully closed
code spans are opaque while locating the label boundary. Link-producing syntax inside a link label,
including standard links, autolinks, and bare URLs, remains literal so links cannot nest.

Backslash-escaped `]` bytes do not close labels, and escaped parentheses do not affect destination
balancing. Destinations may contain balanced and nested parentheses; the first unescaped closing
parenthesis at depth zero ends the destination. Escapable punctuation in destinations is unescaped
once. Empty labels and destinations are allowed. Malformed or unclosed link-shaped input remains
literal text. Images retain precedence over links. Rendered links store their escaped destination
in inert `data-mahcdown-href` metadata rather than an active `href` attribute.

---

## 4. Escapes

Backslash escapes apply to printable ASCII punctuation only. The escapable set is:

```text
!"#$%&'()*+,-./:;<=>?@[\]^_`{|}~
```

An escaped punctuation byte is emitted literally and loses its MiniMark meaning. ASCII letters,
digits, spaces, tabs, Unicode letters and punctuation, and other non-ASCII bytes are not escapable;
the backslash remains literal before them. A trailing backslash is also literal. In a run of
backslashes, pairs produce literal backslashes, so an odd run escapes following ASCII punctuation
and an even run leaves it available to MiniMark parsing.

Matched inline code spans, fenced code blocks, and recognized autolinks are opaque: their contents
do not undergo backslash unescaping. Images use escape-aware closing delimiters, and escapable
punctuation in image alt text and destinations is unescaped once.

Table structure uses the same odd/even rule for pipes. A pipe preceded by an odd backslash run is
cell content; a pipe preceded by an even run separates cells. Table scanning retains the
backslashes, and normal inline parsing performs the final unescape exactly once. Pipes inside code
spans are not treated specially by table structure in this version.

These are MiniMark's defined escape rules and do not imply complete CommonMark compatibility.

---

## 5. Error Handling

MiniMark is forgiving:

- Malformed constructs are emitted as literal text
- Unclosed fenced code blocks extend to end-of-file
- List indentation errors terminate the current list but do not fail parsing

---

## 6. Conformance

A conforming MiniMark 1.2 parser must:

- Implement all rules in this specification
- Correctly parse nested lists using indentation
- Preserve list item ordering and checkbox state
- Avoid introducing implicit HTML or CommonMark-only behaviors

---

## 7. Notes on Compatibility

MiniMark list behavior is intentionally similar to CommonMark but simpler:

- No lazy continuation lines
- No list tight/loose distinction
- Indentation rules are explicit and deterministic
- Checkbox handling is fully specified

This makes MiniMark suitable for reliable parsing, transformation, and rendering.
