package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseCLIImagePolicy(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		allowOutside bool
		path         string
	}{
		{name: "default is restricted", args: []string{"README.md"}, path: "README.md"},
		{name: "outside images allowed", args: []string{"--allow-outside-local-images", "README.md"}, allowOutside: true, path: "README.md"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			options, path, err := parseCLI(tt.args, &output)
			if err != nil {
				t.Fatalf("parseCLI() error = %v, output = %q", err, output.String())
			}
			if options.allowOutsideLocalImages != tt.allowOutside || path != tt.path {
				t.Fatalf("parseCLI() = (%v, %q), want (%v, %q)", options.allowOutsideLocalImages, path, tt.allowOutside, tt.path)
			}
		})
	}
}

func TestParseCLIHelpAndErrors(t *testing.T) {
	var help bytes.Buffer
	_, _, err := parseCLI([]string{"--help"}, &help)
	if err == nil {
		t.Fatal("parseCLI(--help) error = nil")
	}
	if !strings.Contains(help.String(), "--allow-outside-local-images") ||
		!strings.Contains(help.String(), "Allow local images that resolve outside") {
		t.Fatalf("help output does not document image policy: %q", help.String())
	}

	var unknown bytes.Buffer
	_, _, err = parseCLI([]string{"--unknown", "README.md"}, &unknown)
	if err == nil {
		t.Fatal("parseCLI(--unknown) error = nil")
	}

	var obsolete bytes.Buffer
	_, _, err = parseCLI([]string{"--restrict-local-images", "README.md"}, &obsolete)
	if err == nil {
		t.Fatal("obsolete --restrict-local-images flag was accepted")
	}
}
