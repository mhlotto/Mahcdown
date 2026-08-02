package source

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"mahcdown/internal/minimark"
)

func TestReadFileSourceLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "document.md")
	if err := os.WriteFile(path, []byte("1234"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := ReadFile(path, 4)
	if err != nil || string(data) != "1234" {
		t.Fatalf("ReadFile(exact) = (%q, %v), want 1234, nil", data, err)
	}

	if err := os.WriteFile(path, []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err = ReadFile(path, 4)
	if !errors.Is(err, minimark.ErrSourceSizeLimit) {
		t.Fatalf("ReadFile(over) error = %v, want ErrSourceSizeLimit", err)
	}
	if data != nil {
		t.Fatalf("ReadFile(over) returned partial data %q", data)
	}
}
