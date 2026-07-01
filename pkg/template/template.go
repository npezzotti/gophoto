package template

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

var ErrMissingTemplate = errors.New("template not found in cache")

type TemplateCache map[string]*template.Template

func (tc *TemplateCache) RenderTemplate(w io.Writer, tmpl string, data any) error {
	t, ok := (*tc)[tmpl]
	if !ok {
		return ErrMissingTemplate
	}

	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "base", data); err != nil {
		return fmt.Errorf("error executing template: %w", err)
	}

	_, err := buf.WriteTo(w)
	if err != nil {
		return fmt.Errorf("error writing template to response: %w", err)
	}

	return nil
}

func NewTemplateCache(pagesGlob, partialsGlob, baseTemplate string) (TemplateCache, error) {
	return NewTemplateCacheFromFS(os.DirFS("."), pagesGlob, partialsGlob, baseTemplate)
}

func NewTemplateCacheFromFS(filesystem fs.FS, pagesGlob, partialsGlob, baseTemplate string) (TemplateCache, error) {
	cache := make(TemplateCache)

	pages, err := fs.Glob(filesystem, pagesGlob)
	if err != nil {
		return nil, fmt.Errorf("error globbing pages: %w", err)
	}
	if len(pages) == 0 {
		return nil, fmt.Errorf("no page templates found for glob %q", pagesGlob)
	}

	partials, err := fs.Glob(filesystem, partialsGlob)
	if err != nil {
		return nil, fmt.Errorf("error globbing partials: %w", err)
	}
	if len(partials) == 0 {
		return nil, fmt.Errorf("no partial templates found for glob %q", partialsGlob)
	}

	for _, page := range pages {
		patterns := append([]string{baseTemplate, page}, partials...)

		name := filepath.Base(page)
		ts, parseErr := template.New(name).ParseFS(filesystem, patterns...)
		if parseErr != nil {
			return nil, fmt.Errorf("error parsing template %s: %w", name, parseErr)
		}

		cache[name] = ts
	}

	return cache, nil
}
