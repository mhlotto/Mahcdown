package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"mahcdown/internal/minimark"
	"mahcdown/internal/source"
	"mahcdown/internal/ui"
)

// version is overridden at build time with -ldflags "-X main.version=..."
var version = "0.1.0"

type cliOptions struct {
	allowOutsideLocalImages bool
}

func main() {
	if runtime.GOOS != "darwin" {
		exitErr(errors.New("mahcdown currently targets macOS"))
	}
	runtime.LockOSThread()

	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println(version)
		return
	}

	options, path, err := parseCLI(os.Args[1:], os.Stderr)
	if err != nil {
		exitErr(err)
	}

	content, err := source.ReadTextFile(filepath.Clean(path), minimark.DefaultMaxSourceBytes)
	if err != nil {
		exitErr(fmt.Errorf("read file: %w", err))
	}

	title := fmt.Sprintf("Mahcdown - %s", filepath.Base(path))

	if err := ui.Display(title, path, content, ui.Options{
		AllowOutsideLocalImages: options.allowOutsideLocalImages,
	}); err != nil {
		exitErr(err)
	}
}

func parseCLI(args []string, output io.Writer) (cliOptions, string, error) {
	var options cliOptions
	flags := flag.NewFlagSet("mahcdown", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.BoolVar(&options.allowOutsideLocalImages, "allow-outside-local-images", false,
		"Allow local images that resolve outside the Markdown document's directory.")
	flags.Usage = func() {
		fmt.Fprintln(output, "Usage: mahcdown [--allow-outside-local-images] <path-to-markdown-file>")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return cliOptions{}, "", err
	}
	if flags.NArg() != 1 {
		flags.Usage()
		return cliOptions{}, "", fmt.Errorf("expected one Markdown file path")
	}
	return options, flags.Arg(0), nil
}

func exitErr(err error) {
	fmt.Fprintf(os.Stderr, "mahcdown: %v\n", err)
	os.Exit(1)
}
