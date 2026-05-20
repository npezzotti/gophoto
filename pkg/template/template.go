package template

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
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
		return err
	}

	_, err := buf.WriteTo(w)
	if err != nil {
		return err
	}

	return nil
}

func NewTemplateCache(pagesGlob, partialsGlob, baseTemplate string) (TemplateCache, error) {
	cache := make(TemplateCache)

	pages, err := filepath.Glob(pagesGlob)
	if err != nil {
		return nil, fmt.Errorf("error globbing pages: %w", err)
	}

	partials, err := filepath.Glob(partialsGlob)
	if err != nil {
		return nil, fmt.Errorf("error globbing partials: %w", err)
	}

	for _, page := range pages {
		patterns := append([]string{baseTemplate, page}, partials...)

		name := filepath.Base(page)
		ts, err := template.New(name).ParseFiles(patterns...)
		if err != nil {
			return nil, fmt.Errorf("error parsing template %s: %w", name, err)
		}

		cache[name] = ts
	}

	return cache, nil
}
