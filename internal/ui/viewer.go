package ui

import (
	"fmt"
	stdhtml "html"
	"net/url"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/webview/webview_go"

	"mahcdown/internal/minimark"
	"mahcdown/internal/source"
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

	reload := func() (string, error) {
		err := reloadDocument(
			func() (string, error) {
				return source.ReadTextFile(path, minimark.DefaultMaxSourceBytes)
			},
			render,
			func(newHTML string) error {
				w.Dispatch(func() { w.SetHtml(newHTML) })
				return nil
			},
		)
		if err != nil {
			return "", err
		}
		return "ok", nil
	}
	quit := func() {
		w.Dispatch(func() { w.Terminate() })
	}
	openLink := func(destination string) error {
		return openExternalURL(destination, func(validatedURL string) error {
			return exec.Command("open", validatedURL).Start()
		})
	}
	if err := registerViewerBindings(w.Bind, reload, quit, openLink); err != nil {
		return err
	}

	// Inject JS to wire shortcuts and guarded link handling.
	w.Init(shortcutJS())

	w.Run()
	return nil
}

type bindFunction func(name string, callback interface{}) error

func registerViewerBindings(bind bindFunction, reload func() (string, error), quit func(), openLink func(string) error) error {
	bindings := []struct {
		name     string
		purpose  string
		callback interface{}
	}{
		{name: "mahcdownReload", purpose: "reload action", callback: reload},
		{name: "mahcdownQuit", purpose: "quit action", callback: quit},
		{name: "mahcdownOpenLink", purpose: "external-link action", callback: openLink},
	}
	for _, binding := range bindings {
		if err := bind(binding.name, binding.callback); err != nil {
			return fmt.Errorf("bind %s: %w", binding.purpose, err)
		}
	}
	return nil
}

func reloadDocument(read func() (string, error), render func(string) (string, error), update func(string) error) error {
	content, err := read()
	if err != nil {
		return fmt.Errorf("reload document: read source: %w", err)
	}
	html, err := render(content)
	if err != nil {
		return fmt.Errorf("reload document: render source: %w", err)
	}
	if err := update(html); err != nil {
		return fmt.Errorf("reload document: update view: %w", err)
	}
	return nil
}

func imagePolicyFor(baseDir string, options Options) (minimark.ImagePolicy, error) {
	if options.AllowOutsideLocalImages {
		return minimark.AllowOutsideImagePolicy(), nil
	}
	return minimark.NewRestrictedImagePolicy(baseDir)
}

func renderDocument(content, baseDir string, imagePolicy minimark.ImagePolicy) (string, error) {
	return renderDocumentWithLimits(content, baseDir, imagePolicy, minimark.Limits{})
}

func renderDocumentWithLimits(content, baseDir string, imagePolicy minimark.ImagePolicy, limits minimark.Limits) (string, error) {
	document, err := minimark.ParseWithLimits(content, limits)
	if err != nil {
		return "", err
	}
	return injectBase(minimark.RenderHTMLWithImagePolicy(document, imagePolicy), baseDir)
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
  const actionError = document.createElement('div');
  actionError.id = 'mahcdown-action-error';
  actionError.setAttribute('role', 'alert');
  actionError.setAttribute('aria-live', 'assertive');
  actionError.style.display = 'none';
  document.body.appendChild(actionError);

  const errorMessage = (error) => {
    if (error instanceof Error && error.message) return error.message;
    if (typeof error === 'string' && error) return error;
    return 'Unknown error';
  };

  const showActionError = (label, error) => {
    actionError.textContent = label + ' failed: ' + errorMessage(error);
    actionError.style.display = 'block';
  };

  const clearActionError = () => {
    actionError.textContent = '';
    actionError.style.display = 'none';
  };

  const invokeNativeAction = (label, action, options = {}, ...args) => {
    if (typeof action !== 'function') {
      showActionError(label, new Error('native action unavailable'));
      return Promise.resolve(false);
    }
    try {
      return Promise.resolve(action(...args))
        .then(() => {
          if (options.clearOnSuccess) clearActionError();
          return true;
        })
        .catch((error) => {
          showActionError(label, error);
          return false;
        });
    } catch (error) {
      showActionError(label, error);
      return Promise.resolve(false);
    }
  };

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
    '#mahcdown-action-error{position:fixed;top:12px;right:12px;max-width:calc(100% - 24px);box-sizing:border-box;background:#fff2f2;color:#8a1111;border:1px solid #d99;padding:8px 10px;border-radius:6px;box-shadow:0 6px 18px rgba(0,0,0,0.16);z-index:10000;font:13px/1.4 -apple-system,BlinkMacSystemFont,"Helvetica Neue",Arial,sans-serif;white-space:pre-wrap;}' +
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
  let searchTruncated = false;
  let searchTimer = null;
  const searchDebounceMilliseconds = 150;
  const maxSearchMatches = 500;

  const updateCount = () => {
    if (!matches.length) {
      count.textContent = '0 / 0';
      return;
    }
    const total = matches.length + (searchTruncated ? '+' : '');
    count.textContent = (currentIndex + 1) + ' / ' + total;
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

  const createSearchWalker = () => document.createTreeWalker(
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
          if (parent.closest('#mahcdown-search, #mahcdown-action-error')) {
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

  const findMatches = (query) => {
    clearHighlights();
    if (!query) {
      return {hits: [], truncated: false};
    }
    const lowerQuery = query.toLowerCase();
    const hits = [];
    let truncated = false;
    const walker = createSearchWalker();
    let node = walker.nextNode();
    while (node && !truncated) {
      // Advance before replacing node so the walker retains a live current node.
      const nextNode = walker.nextNode();
      const text = node.nodeValue;
      const lowerText = text.toLowerCase();
      let index = 0;
      let lastIndex = 0;
      let hadMatch = false;
      const frag = document.createDocumentFragment();
      while ((index = lowerText.indexOf(lowerQuery, lastIndex)) !== -1) {
        if (hits.length >= maxSearchMatches) {
          truncated = true;
          break;
        }
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
        node = nextNode;
        continue;
      }
      if (lastIndex < text.length) {
        frag.appendChild(document.createTextNode(text.slice(lastIndex)));
      }
      node.parentNode.replaceChild(frag, node);
      if (truncated) {
        break;
      }
      node = nextNode;
    }
    return {hits, truncated};
  };

  const cancelPendingSearch = () => {
    if (searchTimer !== null) {
      clearTimeout(searchTimer);
      searchTimer = null;
    }
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
    cancelPendingSearch();
    const query = input.value;
    const trimmed = query.trim();
    if (!trimmed) {
      clearHighlights();
      matches = [];
      currentIndex = -1;
      searchTruncated = false;
      updateCount();
      return;
    }
    const result = findMatches(trimmed);
    matches = result.hits;
    searchTruncated = result.truncated;
    currentIndex = -1;
    if (matches.length) {
      focusMatch(0);
    } else {
      updateCount();
    }
  };

  const scheduleSearch = () => {
    cancelPendingSearch();
    if (!input.value.trim()) {
      runSearch();
      return;
    }
    searchTimer = setTimeout(() => {
      searchTimer = null;
      runSearch();
    }, searchDebounceMilliseconds);
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
    cancelPendingSearch();
    setSearchOpen(false);
    input.blur();
    clearHighlights();
    matches = [];
    currentIndex = -1;
    searchTruncated = false;
    updateCount();
  };

  input.addEventListener('input', () => {
    scheduleSearch();
  });

  input.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') {
      if (searchTimer !== null) {
        runSearch();
      }
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
    if (ok) {
      invokeNativeAction('Open link', window.mahcdownOpenLink, {}, url);
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
    if (e.key === 'Escape') {
      if (isSearchOpen()) {
        closeSearch();
        e.preventDefault();
        return;
      }
      invokeNativeAction('Quit', window.mahcdownQuit);
      return;
    }
    if (isSearchFocused()) {
      return;
    }
    if (e.key === 'q' || e.key === 'Q') {
      invokeNativeAction('Quit', window.mahcdownQuit);
      return;
    }
    if (!e.metaKey && !e.ctrlKey && (e.key === 'r' || e.key === 'R')) {
      invokeNativeAction('Reload', window.mahcdownReload, {clearOnSuccess: true});
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
