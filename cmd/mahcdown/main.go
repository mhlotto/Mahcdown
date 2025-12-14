package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"mahcdown/internal/ui"
)

// version is overridden at build time with -ldflags "-X main.version=..."
var version = "0.1.0"

func main() {
	if runtime.GOOS != "darwin" {
		exitErr(errors.New("mahcdown currently targets macOS"))
	}
	runtime.LockOSThread()

	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println(version)
		return
	}

	if len(os.Args) != 2 {
		fmt.Println("Usage: mahcdown <path-to-markdown-file>")
		os.Exit(1)
	}

	path := os.Args[1]
	content, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		exitErr(fmt.Errorf("read file: %w", err))
	}

	title := fmt.Sprintf("Mahcdown - %s", filepath.Base(path))

	if err := ui.Display(title, path, string(content)); err != nil {
		exitErr(err)
	}
}

func exitErr(err error) {
	fmt.Fprintf(os.Stderr, "mahcdown: %v\n", err)
	os.Exit(1)
}
