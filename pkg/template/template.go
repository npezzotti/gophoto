package template

import (
	"bytes"
	"errors"
	"html/template"
	"io"
	"path/filepath"
)

type TemplateCache map[string]*template.Template

func (tc *TemplateCache) RenderTemplate(w io.Writer, tmpl string, data any) error {
	t, ok := (*tc)[tmpl]
	if !ok {
		return errors.New("can't get template from cache")
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

func NewTemplateCache() (TemplateCache, error) {
	cache := make(TemplateCache)

	pages, err := filepath.Glob("./templates/pages/*.html")
	if err != nil {
		return nil, err
	}

	partials, err := filepath.Glob("./templates/partials/*.html")
	if err != nil {
		return nil, err
	}

	for _, page := range pages {
		name := filepath.Base(page)

		patterns := append([]string{"./templates/base.html"}, partials...)
		patterns = append(patterns, page)

		ts, err := template.New(name).ParseFiles(patterns...)
		if err != nil {
			return nil, err
		}

		cache[name] = ts
	}

	return cache, nil
}
