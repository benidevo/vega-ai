// Command build-site renders the public landing and privacy pages without
// starting the Vega application.
package main

import (
	"fmt"
	commonrender "github.com/benidevo/vega/internal/common/render"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
)

const siteURL = "https://vega.benidevo.com"

func main() {
	if err := build(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func build() error {
	tmpl := template.New("site").Funcs(template.FuncMap{
		"dict":   dict,
		"jsonLD": commonrender.JSONLD,
	})
	if err := filepath.WalkDir("templates/landing", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".html" {
			return nil
		}
		_, err = tmpl.ParseFiles(path)
		return err
	}); err != nil {
		return fmt.Errorf("parse landing templates: %w", err)
	}
	for _, path := range []string{"templates/pages/privacy.html", "templates/partials/logo_component.html"} {
		if _, err := tmpl.ParseFiles(path); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
	}

	if err := os.RemoveAll("dist/site"); err != nil {
		return fmt.Errorf("clear output directory: %w", err)
	}
	if err := os.MkdirAll("dist/site", 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := render(tmpl, "landing/index.html", "dist/site/index.html"); err != nil {
		return err
	}
	if err := os.MkdirAll("dist/site/privacy", 0o755); err != nil {
		return fmt.Errorf("create privacy output directory: %w", err)
	}
	if err := render(tmpl, "pages/privacy.html", "dist/site/privacy/index.html"); err != nil {
		return err
	}

	for _, path := range []string{"static/landing", "static/images"} {
		if err := copyDir(path, filepath.Join("dist/site", path)); err != nil {
			return fmt.Errorf("copy %s: %w", path, err)
		}
	}
	robots, err := os.ReadFile("static/landing/robots.txt")
	if err != nil {
		return fmt.Errorf("read robots.txt: %w", err)
	}
	if err := os.WriteFile("dist/site/robots.txt", robots, 0o644); err != nil {
		return fmt.Errorf("write robots.txt: %w", err)
	}
	if err := os.WriteFile("dist/site/sitemap.xml", []byte(sitemap), 0o644); err != nil {
		return fmt.Errorf("write sitemap: %w", err)
	}
	return nil
}

func dict(values ...any) (map[string]any, error) {
	if len(values)%2 != 0 {
		return nil, fmt.Errorf("dict expects an even number of arguments")
	}
	result := make(map[string]any, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict key %d must be a string", i/2)
		}
		result[key] = values[i+1]
	}
	return result, nil
}

func render(tmpl *template.Template, name, output string) error {
	file, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("create %s: %w", output, err)
	}
	defer file.Close()
	if err := tmpl.ExecuteTemplate(file, name, nil); err != nil {
		return fmt.Errorf("render %s: %w", name, err)
	}
	return nil
}

func copyDir(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, contents, 0o644)
	})
}

const sitemap = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>` + siteURL + `/</loc></url>
  <url><loc>` + siteURL + `/privacy/</loc></url>
</urlset>
`
