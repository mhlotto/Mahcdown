package source

import (
	"io"
	"os"

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
