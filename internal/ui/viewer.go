package ui

import (
	"fmt"
	stdhtml "html"
	"net/url"
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

// Options configures viewer behavior.
type Options struct {
	AllowOutsideLocalImages bool
}

// Display opens a window, renders the provided markdown text, and supports reload/quit/zoom shortcuts.
func Display(title, path, initialText string, options Options) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("mahcdown currently targets macOS")
	}

	baseDir := filepath.Dir(path)
	imagePolicy, err := imagePolicyFor(baseDir, options)
	if err != nil {
		return err
	}
	render := func(content string) (string, error) {
		return renderDocument(content, baseDir, imagePolicy)
	}

	html, err := render(initialText)
	if err != nil {
		return err
	}

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
		newHTML, err := render(string(data))
		if err != nil {
			return "", err
		}
		w.Dispatch(func() { w.SetHtml(newHTML) })
		return "ok", nil
	})

	// Bind quit to close the window cleanly.
	_ = w.Bind("mahcdownQuit", func() {
		w.Dispatch(func() { w.Terminate() })
	})

	// Bind external link opener (only used after user confirmation in JS).
	_ = w.Bind("mahcdownOpenLink", func(destination string) error {
		return openExternalURL(destination, func(validatedURL string) error {
			return exec.Command("open", validatedURL).Start()
		})
	})

	// Inject JS to wire shortcuts and guarded link handling.
	w.Init(shortcutJS())

	w.Run()
	return nil
}

func imagePolicyFor(baseDir string, options Options) (minimark.ImagePolicy, error) {
	if options.AllowOutsideLocalImages {
		return minimark.AllowOutsideImagePolicy(), nil
	}
	return minimark.NewRestrictedImagePolicy(baseDir)
}

func renderDocument(content, baseDir string, imagePolicy minimark.ImagePolicy) (string, error) {
	return injectBase(minimark.RenderHTMLWithImagePolicy(minimark.Parse(content), imagePolicy), baseDir)
}

func openExternalURL(destination string, opener func(string) error) error {
	if err := validateExternalURL(destination); err != nil {
		return err
	}
	if err := opener(destination); err != nil {
		return fmt.Errorf("open external link: %w", err)
	}
	return nil
}

func validateExternalURL(destination string) error {
	if destination == "" || destination != strings.TrimSpace(destination) {
		return fmt.Errorf("invalid link destination: URL must not be empty or surrounded by whitespace")
	}
	parsed, err := url.Parse(destination)
	if err != nil {
		return fmt.Errorf("invalid link destination: malformed URL")
	}
	if !strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https") {
		return fmt.Errorf("invalid link destination: unsupported URL scheme")
	}
	if parsed.Opaque != "" {
		return fmt.Errorf("invalid link destination: opaque URLs are not allowed")
	}
	if parsed.User != nil {
		return fmt.Errorf("invalid link destination: user information is not allowed")
	}
	if parsed.Hostname() == "" {
		return fmt.Errorf("invalid link destination: URL must include a host")
	}
	if strings.Contains(destination, `\`) {
		return fmt.Errorf("invalid link destination: backslashes are not allowed")
	}
	return nil
}

func injectBase(html, baseDir string) (string, error) {
	if baseDir == "" {
		return html, nil
	}
	if strings.Contains(html, "<base ") {
		return html, nil
	}
	absoluteDir, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("resolve base directory: %w", err)
	}
	urlPath := strings.TrimRight(filepath.ToSlash(absoluteDir), "/") + "/"
	base := stdhtml.EscapeString((&url.URL{Scheme: "file", Path: urlPath}).String())
	baseElement := `<base href="` + base + `">`
	if styleAt := strings.Index(strings.ToLower(html), "<style>"); styleAt >= 0 {
		return html[:styleAt] + baseElement + html[styleAt:], nil
	}
	const headTag = "<head>"
	if idx := strings.Index(strings.ToLower(html), headTag); idx >= 0 {
		insertAt := idx + len(headTag)
		return html[:insertAt] + baseElement + html[insertAt:], nil
	}
	// Fallback: prepend.
	return baseElement + html, nil
}

func shortcutJS() string {
	return `
document.addEventListener('DOMContentLoaded', () => {
  // Inject search UI and styling.
  const search = document.createElement('div');
  search.id = 'mahcdown-search';
  search.setAttribute('aria-hidden', 'true');
  search.style.display = 'none';

  const inner = document.createElement('div');
  inner.className = 'mahcdown-search-inner';

  const label = document.createElement('span');
  label.className = 'mahcdown-search-label';
  label.textContent = 'Find';

  const input = document.createElement('input');
  input.type = 'text';
  input.className = 'mahcdown-search-input';
  input.setAttribute('placeholder', 'Search text');

  const prevBtn = document.createElement('button');
  prevBtn.type = 'button';
  prevBtn.className = 'mahcdown-search-prev';
  prevBtn.textContent = 'Prev';

  const nextBtn = document.createElement('button');
  nextBtn.type = 'button';
  nextBtn.className = 'mahcdown-search-next';
  nextBtn.textContent = 'Next';

  const count = document.createElement('span');
  count.className = 'mahcdown-search-count';
  count.textContent = '0 / 0';

  const closeBtn = document.createElement('button');
  closeBtn.type = 'button';
  closeBtn.className = 'mahcdown-search-close';
  closeBtn.textContent = 'Close';

  inner.appendChild(label);
  inner.appendChild(input);
  inner.appendChild(prevBtn);
  inner.appendChild(nextBtn);
  inner.appendChild(count);
  inner.appendChild(closeBtn);
  search.appendChild(inner);
  document.body.appendChild(search);

  const style = document.createElement('style');
  style.textContent =
    '#mahcdown-search{position:fixed;left:50%;bottom:12px;transform:translateX(-50%);background:#f6f6f8;border:1px solid #c8c8cc;border-radius:8px;padding:8px 10px;box-shadow:0 10px 24px rgba(0,0,0,0.18);z-index:9999;max-width:calc(100% - 24px);font-family:-apple-system,BlinkMacSystemFont,"Helvetica Neue",Arial,sans-serif;}' +
    '#mahcdown-search .mahcdown-search-inner{display:flex;align-items:center;gap:6px;}' +
    '#mahcdown-search .mahcdown-search-label{font-size:11px;letter-spacing:0.08em;text-transform:uppercase;color:#666;margin-right:2px;}' +
    '#mahcdown-search .mahcdown-search-input{flex:1;min-width:180px;font-size:14px;padding:4px 6px;border:1px solid #a8a8ad;border-radius:4px;}' +
    '#mahcdown-search button{font-size:12px;padding:4px 8px;border:1px solid #b6b6bb;border-radius:4px;background:#fff;cursor:pointer;}' +
    '#mahcdown-search button:hover{background:#f0f0f2;}' +
    '#mahcdown-search .mahcdown-search-count{font-size:12px;color:#444;min-width:52px;text-align:right;}' +
    'mark.mahcdown-search-hit{background:#ffe16a;color:#111;padding:0 1px;}' +
    'mark.mahcdown-search-current{background:#ffb347;}';
  document.head.appendChild(style);

  let matches = [];
  let currentIndex = -1;

  const updateCount = () => {
    if (!matches.length) {
      count.textContent = '0 / 0';
      return;
    }
    count.textContent = (currentIndex + 1) + ' / ' + matches.length;
  };

  const clearHighlights = () => {
    const existing = document.querySelectorAll('mark.mahcdown-search-hit');
    existing.forEach((mark) => {
      const parent = mark.parentNode;
      if (!parent) return;
      parent.replaceChild(document.createTextNode(mark.textContent || ''), mark);
      parent.normalize();
    });
  };

  const collectTextNodes = () => {
    const nodes = [];
    const walker = document.createTreeWalker(
      document.body,
      NodeFilter.SHOW_TEXT,
      {
        acceptNode: (node) => {
          if (!node.nodeValue || !node.nodeValue.trim()) {
            return NodeFilter.FILTER_REJECT;
          }
          const parent = node.parentNode;
          if (!parent || !parent.closest) {
            return NodeFilter.FILTER_REJECT;
          }
          if (parent.closest('#mahcdown-search')) {
            return NodeFilter.FILTER_REJECT;
          }
          const tag = parent.nodeName.toLowerCase();
          if (tag === 'script' || tag === 'style' || tag === 'textarea' || tag === 'input') {
            return NodeFilter.FILTER_REJECT;
          }
          return NodeFilter.FILTER_ACCEPT;
        },
      }
    );
    while (walker.nextNode()) {
      nodes.push(walker.currentNode);
    }
    return nodes;
  };

  const findMatches = (query) => {
    clearHighlights();
    if (!query) {
      return [];
    }
    const lowerQuery = query.toLowerCase();
    const hits = [];
    const nodes = collectTextNodes();
    nodes.forEach((node) => {
      const text = node.nodeValue;
      const lowerText = text.toLowerCase();
      let index = 0;
      let lastIndex = 0;
      let hadMatch = false;
      const frag = document.createDocumentFragment();
      while ((index = lowerText.indexOf(lowerQuery, lastIndex)) !== -1) {
        hadMatch = true;
        if (index > lastIndex) {
          frag.appendChild(document.createTextNode(text.slice(lastIndex, index)));
        }
        const mark = document.createElement('mark');
        mark.className = 'mahcdown-search-hit';
        mark.textContent = text.slice(index, index + query.length);
        frag.appendChild(mark);
        hits.push(mark);
        lastIndex = index + query.length;
      }
      if (!hadMatch) {
        return;
      }
      if (lastIndex < text.length) {
        frag.appendChild(document.createTextNode(text.slice(lastIndex)));
      }
      node.parentNode.replaceChild(frag, node);
    });
    return hits;
  };

  const focusMatch = (index) => {
    if (!matches.length) {
      updateCount();
      return;
    }
    if (currentIndex >= 0 && matches[currentIndex]) {
      matches[currentIndex].classList.remove('mahcdown-search-current');
    }
    const nextIndex = ((index % matches.length) + matches.length) % matches.length;
    currentIndex = nextIndex;
    const current = matches[currentIndex];
    current.classList.add('mahcdown-search-current');
    current.scrollIntoView({ block: 'center' });
    updateCount();
  };

  const runSearch = () => {
    const query = input.value;
    const trimmed = query.trim();
    if (!trimmed) {
      clearHighlights();
      matches = [];
      currentIndex = -1;
      updateCount();
      return;
    }
    matches = findMatches(trimmed);
    currentIndex = -1;
    if (matches.length) {
      focusMatch(0);
    } else {
      updateCount();
    }
  };

  const setSearchOpen = (open) => {
    search.style.display = open ? 'block' : 'none';
    search.setAttribute('aria-hidden', open ? 'false' : 'true');
    document.body.style.paddingBottom = open ? '64px' : '';
  };

  const openSearch = () => {
    setSearchOpen(true);
    input.focus();
    input.select();
    runSearch();
  };

  const closeSearch = () => {
    setSearchOpen(false);
    input.blur();
    clearHighlights();
    matches = [];
    currentIndex = -1;
    updateCount();
  };

  input.addEventListener('input', () => {
    runSearch();
  });

  input.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') {
      if (matches.length) {
        focusMatch(currentIndex + (e.shiftKey ? -1 : 1));
      }
      e.preventDefault();
      return;
    }
    if (e.key === 'Escape') {
      closeSearch();
      e.preventDefault();
      e.stopPropagation();
    }
  });

  prevBtn.addEventListener('click', () => {
    if (matches.length) {
      focusMatch(currentIndex - 1);
    }
  });

  nextBtn.addEventListener('click', () => {
    if (matches.length) {
      focusMatch(currentIndex + 1);
    }
  });

  closeBtn.addEventListener('click', () => {
    closeSearch();
  });

  const isSearchOpen = () => search.style.display !== 'none';
  const isSearchFocused = () => search.contains(document.activeElement);

  const findRenderedLink = (target) => {
    if (!target || !target.closest) return null;
    return target.closest('a.mahcdown-link[data-mahcdown-href]');
  };

  const activateRenderedLink = (e, link) => {
    e.preventDefault();
    const url = link.getAttribute('data-mahcdown-href');
    if (!url) return;
    const text = link.textContent || url;
    const ok = confirm('Open external link?\n' + text);
    if (ok && window.mahcdownOpenLink) {
      window.mahcdownOpenLink(url);
    }
  };

  // Renderer-produced links have no href, so they remain inert without this handler.
  document.body.addEventListener('click', (e) => {
    const link = findRenderedLink(e.target);
    if (!link) return;
    activateRenderedLink(e, link);
  });

  document.body.addEventListener('keydown', (e) => {
    if (e.key !== 'Enter' && e.key !== ' ') return;
    const link = findRenderedLink(e.target);
    if (!link) return;
    activateRenderedLink(e, link);
  });

  // Hotkeys: r reload, Esc quit, Cmd+F search, Cmd+/- zoom via browser zoom.
  document.addEventListener('keydown', (e) => {
    if ((e.metaKey || e.ctrlKey) && (e.key === 'f' || e.key === 'F')) {
      openSearch();
      e.preventDefault();
      return;
    }
    if ((e.metaKey || e.ctrlKey) && (e.key === 'c' || e.key === 'C')) {
      if (!isSearchFocused()) {
        const selection = window.getSelection();
        const text = selection ? selection.toString() : '';
        if (text) {
          try {
            document.execCommand('copy');
          } catch (err) {
            // Ignore copy errors and let default behavior proceed.
          }
          e.preventDefault();
          return;
        }
      }
    }
    if (e.key === 'Escape') {
      if (isSearchOpen()) {
        closeSearch();
        e.preventDefault();
        return;
      }
      if (window.mahcdownQuit) {
        window.mahcdownQuit();
      }
      return;
    }
    if (isSearchFocused()) {
      return;
    }
    if (e.key === 'q' || e.key === 'Q') {
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
