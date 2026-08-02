package source

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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

func TestDecodeText(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  string
	}{
		{name: "ASCII apostrophe", input: []byte("It's working"), want: "It's working"},
		{name: "UTF-8 curly apostrophe", input: []byte("It’s working"), want: "It’s working"},
		{name: "UTF-8 BOM", input: append([]byte{0xef, 0xbb, 0xbf}, []byte("It’s working")...), want: "It’s working"},
		{name: "Windows-1252 apostrophe", input: []byte{'I', 't', 0x92, 's', ' ', 'w', 'o', 'r', 'k', 'i', 'n', 'g'}, want: "It’s working"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeText(tt.input)
			if err != nil {
				t.Fatalf("DecodeText() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("DecodeText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReadTextFileInitialAndReloadUseSameDecoding(t *testing.T) {
	path := filepath.Join(t.TempDir(), "document.md")
	write := func(data []byte) {
		t.Helper()
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write([]byte("It's initial"))
	initial, err := ReadTextFile(path, minimark.DefaultMaxSourceBytes)
	if err != nil || initial != "It's initial" {
		t.Fatalf("initial ReadTextFile() = (%q, %v)", initial, err)
	}

	write([]byte{'I', 't', 0x92, 's', ' ', 'r', 'e', 'l', 'o', 'a', 'd', 'e', 'd'})
	reloaded, err := ReadTextFile(path, minimark.DefaultMaxSourceBytes)
	if err != nil || reloaded != "It’s reloaded" {
		t.Fatalf("reload ReadTextFile() = (%q, %v)", reloaded, err)
	}
}

func TestReadTextFileEnforcesByteLimitBeforeDecoding(t *testing.T) {
	path := filepath.Join(t.TempDir(), "document.md")
	if err := os.WriteFile(path, []byte{0x92, 0x92, 0x92}, 0o644); err != nil {
		t.Fatal(err)
	}
	text, err := ReadTextFile(path, 2)
	if !errors.Is(err, minimark.ErrSourceSizeLimit) {
		t.Fatalf("ReadTextFile() error = %v, want ErrSourceSizeLimit", err)
	}
	if text != "" {
		t.Fatalf("ReadTextFile() returned partial text %q", text)
	}
	if strings.Contains(text, "’") {
		t.Fatal("oversized source was decoded before rejection")
	}
}
