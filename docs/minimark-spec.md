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

Identical to MiniMark 1.1.

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
including arbitrarily nested lists.

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

- A list item marker must be preceded by **0–3 spaces** at the current nesting level.
- Nested lists are created when a list item is indented **at least 2 spaces more**
  than its parent list item marker.
- Tabs are not allowed for indentation.

Indentation is measured from the start of the line to the first non-whitespace character.

### 2.5.3 List Item Content

A list item consists of:

- The marker
- A required single space
- Item content

Item content may span multiple lines.

Continuation lines must be indented **at least to the column of the first content character**
after the marker.

Blank lines are allowed inside list items and are preserved.

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

Asterisk delimiter runs provide `*emphasis*` and `**strong emphasis**`. MiniMark classifies runs
with Unicode left- and right-flanking rules and resolves them with a delimiter stack following the
CommonMark asterisk emphasis algorithm. Underscore emphasis is not supported, and MiniMark does
not claim complete CommonMark compatibility. Unmatched delimiters remain literal.

Emphasis may cross a soft line break inside one inline-bearing block. Delimiter matching resets at
paragraph, heading, list-item, blockquote-child, and table-cell boundaries. Successfully parsed
code spans, images, URLs, checkboxes, and other atomic inline constructs are opaque to delimiter
processing.

---

## 4. Escapes

Identical to MiniMark 1.1.

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
