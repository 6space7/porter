package services

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Catalog struct {
	templates []Template
	bySlug    map[string]Template
}

type Template struct {
	Slug         string            `json:"slug" yaml:"slug"`
	Name         string            `json:"name" yaml:"name"`
	Description  string            `json:"description" yaml:"description"`
	Category     string            `json:"category" yaml:"category"`
	DocsURL      string            `json:"docs_url" yaml:"docs_url"`
	Logo         string            `json:"logo" yaml:"logo"`
	Image        string            `json:"image" yaml:"image"`
	Command      []string          `json:"command,omitempty" yaml:"command"`
	InternalPort int64             `json:"internal_port" yaml:"internal_port"`
	Exposed      bool              `json:"exposed" yaml:"exposed"`
	Variables    map[string]string `json:"variables" yaml:"variables"`
	Provides     map[string]string `json:"provides" yaml:"provides"`
	Volumes      []VolumeSpec      `json:"volumes" yaml:"volumes"`
	Healthcheck  HealthcheckSpec   `json:"healthcheck" yaml:"healthcheck"`
}

type VolumeSpec struct {
	Name string `json:"name" yaml:"name"`
	Path string `json:"path" yaml:"path"`
}

type HealthcheckSpec struct {
	Command string `json:"command" yaml:"command"`
}

func LoadCatalog(fsys fs.FS) (Catalog, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return Catalog{}, fmt.Errorf("read templates: %w", err)
	}

	catalog := Catalog{bySlug: map[string]Template{}}
	for _, entry := range entries {
		if entry.IsDir() || !isTemplateFile(entry.Name()) {
			continue
		}
		body, err := fs.ReadFile(fsys, entry.Name())
		if err != nil {
			return Catalog{}, fmt.Errorf("read template %s: %w", entry.Name(), err)
		}
		var tmpl Template
		if err := yaml.Unmarshal(body, &tmpl); err != nil {
			return Catalog{}, fmt.Errorf("parse template %s: %w", entry.Name(), err)
		}
		if err := validateTemplate(tmpl); err != nil {
			return Catalog{}, fmt.Errorf("template %s: %w", entry.Name(), err)
		}
		if _, exists := catalog.bySlug[tmpl.Slug]; exists {
			return Catalog{}, fmt.Errorf("duplicate template slug %q", tmpl.Slug)
		}
		catalog.bySlug[tmpl.Slug] = tmpl
		catalog.templates = append(catalog.templates, tmpl)
	}

	sort.Slice(catalog.templates, func(i, j int) bool {
		return catalog.templates[i].Name < catalog.templates[j].Name
	})
	return catalog, nil
}

func (catalog Catalog) All() []Template {
	return append([]Template(nil), catalog.templates...)
}

func (catalog Catalog) Get(slug string) (Template, bool) {
	tmpl, ok := catalog.bySlug[slug]
	return tmpl, ok
}

func (catalog Catalog) Search(query string) []Template {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return catalog.All()
	}

	matches := make([]Template, 0)
	for _, tmpl := range catalog.templates {
		haystack := strings.ToLower(strings.Join([]string{
			tmpl.Slug,
			tmpl.Name,
			tmpl.Description,
			tmpl.Category,
		}, " "))
		if strings.Contains(haystack, query) {
			matches = append(matches, tmpl)
		}
	}
	return matches
}

func isTemplateFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".yaml" || ext == ".yml"
}

func validateTemplate(tmpl Template) error {
	switch {
	case strings.TrimSpace(tmpl.Slug) == "":
		return fmt.Errorf("slug is required")
	case strings.TrimSpace(tmpl.Name) == "":
		return fmt.Errorf("name is required")
	case strings.TrimSpace(tmpl.Image) == "":
		return fmt.Errorf("image is required")
	case tmpl.InternalPort < 0 || tmpl.InternalPort > 65535:
		return fmt.Errorf("internal_port is invalid")
	}
	for _, volume := range tmpl.Volumes {
		if strings.TrimSpace(volume.Name) == "" || !strings.HasPrefix(volume.Path, "/") {
			return fmt.Errorf("volume %q has invalid path %q", volume.Name, volume.Path)
		}
	}
	return nil
}
