package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuild(t *testing.T) {
	t.Chdir("../..")
	if err := build(); err != nil {
		t.Fatalf("build: %v", err)
	}

	for _, name := range []string{"index.html", "privacy/index.html", "404.html", "_redirects", "robots.txt", "sitemap.xml"} {
		contents, err := os.ReadFile(filepath.Join("dist/site", name))
		if err != nil {
			t.Fatalf("missing output %s: %v", name, err)
		}
		if strings.Contains(string(contents), "{{") {
			t.Errorf("%s contains unrendered template syntax", name)
		}
	}
}

func TestBuildOutsideRepoRoot(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := build(); err == nil || !strings.Contains(err.Error(), "repository root") {
		t.Fatalf("expected repository root error, got %v", err)
	}
}
