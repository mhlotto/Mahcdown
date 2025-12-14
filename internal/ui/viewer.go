package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/webview/webview_go"

	"mahcdown/internal/minimark"
)

const (
	initialWidth  = 900
	initialHeight = 700
)

// Display opens a window, renders the provided markdown text, and supports reload/quit/zoom shortcuts.
func Display(title, path, initialText string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("mahcdown currently targets macOS")
	}

	baseDir := filepath.Dir(path)
	render := func(content string) string {
		return injectBase(minimark.RenderHTML(minimark.Parse(content)), baseDir)
	}

	html := render(initialText)

	w := webview.New(false)
	defer w.Destroy()

	w.SetTitle(title)
	w.SetSize(initialWidth, initialHeight, webview.HintNone)
	w.SetHtml(html)

	// Bind reload: re-read file, re-render, and update HTML.
	_ = w.Bind("mahcdownReload", func() (string, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		newHTML := render(string(data))
		w.Dispatch(func() { w.SetHtml(newHTML) })
		return "ok", nil
	})

	// Bind quit to close the window cleanly.
	_ = w.Bind("mahcdownQuit", func() {
		w.Dispatch(func() { w.Terminate() })
	})

	// Bind external link opener (only used after user confirmation in JS).
	_ = w.Bind("mahcdownOpenLink", func(url string) {
		// Best-effort open; ignore errors to keep UI responsive.
		_ = exec.Command("open", url).Start()
	})

	// Inject JS to wire shortcuts and guarded link handling.
	w.Init(shortcutJS())

	w.Run()
	return nil
}

func injectBase(html, baseDir string) string {
	if baseDir == "" {
		return html
	}
	base := `file://` + strings.TrimRight(filepath.ToSlash(baseDir), "/") + `/`
	if strings.Contains(html, "<base ") {
		return html
	}
	const headTag = "<head>"
	if idx := strings.Index(strings.ToLower(html), headTag); idx >= 0 {
		insertAt := idx + len(headTag)
		return html[:insertAt] + `<base href="` + base + `">` + html[insertAt:]
	}
	// Fallback: prepend.
	return `<base href="` + base + `">` + html
}

func shortcutJS() string {
	return `
document.addEventListener('DOMContentLoaded', () => {
  // Intercept link clicks to prompt before opening externally.
  document.body.addEventListener('click', (e) => {
    const a = e.target.closest('a');
    if (!a || !a.href) return;
    e.preventDefault();
    const url = a.getAttribute('data-href') || a.href;
    const text = a.textContent || url;
    const ok = confirm('Open external link?\n' + text);
    if (ok && window.mahcdownOpenLink) {
      window.mahcdownOpenLink(url);
    }
  });

  // Hotkeys: r reload, Esc quit, Cmd+/- zoom via browser zoom.
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' || e.key === 'q' || e.key === 'Q') {
      if (window.mahcdownQuit) {
        window.mahcdownQuit();
      }
      return;
    }
    if (!e.metaKey && !e.ctrlKey && (e.key === 'r' || e.key === 'R')) {
      if (window.mahcdownReload) {
        window.mahcdownReload();
      }
      e.preventDefault();
      return;
    }
    if (e.metaKey && (e.key === '=' || e.key === '+')) {
      document.body.style.zoom = (parseFloat(document.body.style.zoom || '1') + 0.1).toString();
      e.preventDefault();
      return;
    }
    if (e.metaKey && e.key === '-') {
      const next = parseFloat(document.body.style.zoom || '1') - 0.1;
      document.body.style.zoom = (next > 0.5 ? next : 0.5).toString();
      e.preventDefault();
      return;
    }
  });
});
`
}
