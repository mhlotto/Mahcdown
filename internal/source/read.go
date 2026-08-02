package source

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"

	"mahcdown/internal/minimark"
)

// ReadFile reads at most maxBytes and returns the parser's source-size error if exceeded.
func ReadFile(path string, maxBytes int) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, int64(maxBytes)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxBytes {
		return nil, &minimark.LimitError{Kind: minimark.ErrSourceSizeLimit, Limit: maxBytes}
	}
	return data, nil
}

// ReadTextFile reads a size-bounded Markdown file and decodes it as UTF-8.
// Invalid UTF-8 is decoded as Windows-1252 for compatibility with common
// Markdown files produced by Windows editors.
func ReadTextFile(path string, maxBytes int) (string, error) {
	data, err := ReadFile(path, maxBytes)
	if err != nil {
		return "", err
	}
	return DecodeText(data)
}

// DecodeText returns UTF-8 text without changing already-valid UTF-8 input.
func DecodeText(data []byte) (string, error) {
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	if utf8.Valid(data) {
		return string(data), nil
	}
	decoded, err := charmap.Windows1252.NewDecoder().Bytes(data)
	if err != nil {
		return "", fmt.Errorf("decode source as Windows-1252: %w", err)
	}
	return string(decoded), nil
}
