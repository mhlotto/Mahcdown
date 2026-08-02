package minimark

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImagePolicyModes(t *testing.T) {
	temp := t.TempDir()
	root := filepath.Join(temp, "document")
	outside := filepath.Join(temp, "document-other")
	for _, dir := range []string{
		filepath.Join(root, "images", "nested"),
		filepath.Join(outside, "shared"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(outside, "shared", "photo.png"), []byte("image"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "shared"), filepath.Join(root, "images", "outside")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("second", filepath.Join(root, "images", "first")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "shared"), filepath.Join(root, "images", "second")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(temp, "missing-target"), filepath.Join(root, "images", "broken")); err != nil {
		t.Fatal(err)
	}

	restricted, err := NewRestrictedImagePolicy(root)
	if err != nil {
		t.Fatalf("NewRestrictedImagePolicy() error = %v", err)
	}

	permissiveAllowed := []string{
		"image.png",
		"images/photo.png",
		"../document-other/shared/photo.png",
		"images/outside/photo.png",
	}
	for _, source := range permissiveAllowed {
		t.Run("permissive "+source, func(t *testing.T) {
			assertImageRendered(t, source, AllowOutsideImagePolicy(), true)
		})
	}

	restrictedAllowed := []string{
		"image.png",
		"./image.png",
		"images/photo.png",
		"images/nested/photo.png",
		"image with spaces.png",
		"image%20with%20spaces.png",
		"写真.png",
		"images/./nested/../photo.png",
	}
	for _, source := range restrictedAllowed {
		t.Run("restricted allow "+source, func(t *testing.T) {
			assertImageRendered(t, source, restricted, true)
		})
	}

	restrictedRejected := []string{
		"../photo.png",
		"../../photo.png",
		"images/../../../photo.png",
		"%2e%2e/photo.png",
		"%2E%2E/photo.png",
		"%2e%2e%2Fdocument-other/shared/photo.png",
		filepath.ToSlash(filepath.Join(temp, "absolute.png")),
		"/root-relative.png",
		"images/outside/photo.png",
		"images/outside/missing.png",
		"images/first/photo.png",
		"images/broken/photo.png",
		"../document-other/shared/photo.png",
		"https://example.com/photo.png",
		"file:///tmp/photo.png",
	}
	for _, source := range restrictedRejected {
		t.Run("restricted reject "+source, func(t *testing.T) {
			got := renderImageWithPolicy(source, restricted)
			if strings.Contains(got, `<img src=`) {
				t.Fatalf("blocked source emitted an active image: %s", got)
			}
			if !strings.Contains(got, `[image blocked: alt]`) {
				t.Fatalf("blocked source did not use placeholder: %s", got)
			}
			if strings.Contains(got, outside) {
				t.Fatalf("blocked output disclosed resolved outside path: %s", got)
			}
		})
	}
}

func TestZeroValueImagePolicyFailsClosed(t *testing.T) {
	assertImageRendered(t, "image.png", ImagePolicy{}, false)
	assertImageRendered(t, "../outside.png", ImagePolicy{}, false)
}

func TestRestrictedImagePolicyResolvesSymlinkedRoot(t *testing.T) {
	temp := t.TempDir()
	realRoot := filepath.Join(temp, "real-document")
	if err := os.MkdirAll(realRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	linkedRoot := filepath.Join(temp, "linked-document")
	if err := os.Symlink(realRoot, linkedRoot); err != nil {
		t.Fatal(err)
	}
	policy, err := NewRestrictedImagePolicy(linkedRoot)
	if err != nil {
		t.Fatalf("NewRestrictedImagePolicy() error = %v", err)
	}
	assertImageRendered(t, "image.png", policy, true)
}

func assertImageRendered(t *testing.T, source string, policy ImagePolicy, want bool) {
	t.Helper()
	got := renderImageWithPolicy(source, policy)
	rendered := strings.Contains(got, `<img src=`)
	if rendered != want {
		t.Fatalf("active image rendered = %v, want %v: %s", rendered, want, got)
	}
}

func renderImageWithPolicy(source string, policy ImagePolicy) string {
	return RenderHTMLWithImagePolicy(Document{Blocks: []Block{
		Paragraph{Inlines: []Inline{Image{URL: source, Alt: "alt"}}},
	}}, policy)
}
