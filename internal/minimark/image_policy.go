package minimark

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// ImagePolicy controls which local image paths may be rendered.
type ImagePolicy struct {
	allowOutside bool
	root         string
	resolvedRoot string
}

// AllowOutsideImagePolicy permits local images outside the document directory.
func AllowOutsideImagePolicy() ImagePolicy {
	return ImagePolicy{allowOutside: true}
}

// NewRestrictedImagePolicy restricts local images to root and its descendants.
func NewRestrictedImagePolicy(root string) (ImagePolicy, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return ImagePolicy{}, fmt.Errorf("resolve image root: %w", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(absoluteRoot)
	if err != nil {
		return ImagePolicy{}, fmt.Errorf("resolve image root symlinks: %w", err)
	}
	return ImagePolicy{
		root:         filepath.Clean(absoluteRoot),
		resolvedRoot: filepath.Clean(resolvedRoot),
	}, nil
}

func parseAllowedLocalImageURL(source string) (*url.URL, bool) {
	if source == "" || source != strings.TrimSpace(source) || strings.Contains(source, `\`) {
		return nil, false
	}
	parsed, err := url.Parse(source)
	if err != nil {
		return nil, false
	}
	if parsed.Scheme != "" || parsed.Host != "" || parsed.User != nil || parsed.Opaque != "" {
		return nil, false
	}
	if parsed.Path == "" || strings.HasPrefix(parsed.Path, "/") || strings.Contains(parsed.Path, `\`) {
		return nil, false
	}
	return parsed, true
}

func (policy ImagePolicy) allows(parsed *url.URL) bool {
	if policy.allowOutside {
		return true
	}
	if policy.root == "" || policy.resolvedRoot == "" {
		return false
	}
	candidate := filepath.Clean(filepath.Join(policy.root, filepath.FromSlash(parsed.Path)))
	if !pathWithin(policy.root, candidate) {
		return false
	}

	existing := candidate
	for {
		_, err := os.Lstat(existing)
		switch {
		case err == nil:
			resolved, err := filepath.EvalSymlinks(existing)
			return err == nil && pathWithin(policy.resolvedRoot, resolved)
		case !os.IsNotExist(err):
			return false
		}
		parent := filepath.Dir(existing)
		if parent == existing || !pathWithin(policy.root, parent) {
			return false
		}
		existing = parent
	}
}

func pathWithin(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil || filepath.IsAbs(relative) {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
