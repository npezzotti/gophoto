package web

import (
	"bytes"
	"errors"
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/justinas/nosurf"
	"github.com/npezzotti/gophoto/db"
	"github.com/npezzotti/gophoto/pagination"
)

type templateData struct {
	Form      Form
	Flash     *Flash
	User      *UserResponse
	Albums    []*AlbumResponse
	Album     db.GetAlbumRow
	Images    []*UserImageResponse
	Paginator *pagination.Pagination
	CSRFToken string
}

func (a *application) newTemplateData(r *http.Request) *templateData {
	td := &templateData{
		CSRFToken: nosurf.Token(r),
	}

	td.User = a.newUserResponse(r.Context(), a.getUserFromRequest(r))

	flash, ok := a.sessionManager.Pop(r.Context(), "__flash").(Flash)
	if ok {
		td.Flash = &flash
	}

	return td
}

func (a *application) renderTemplate(w http.ResponseWriter, data *templateData, tmpl string) error {
	var tc map[string]*template.Template

	if a.config.UseTemplateCache {
		tc = a.templateCache
	} else {
		tc, _ = NewTemplateCache()
	}

	t, ok := tc[tmpl]
	if !ok {
		return errors.New("can't get template from cache")
	}

	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "base", data); err != nil {
		return err
	}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	_, err := buf.WriteTo(w)
	if err != nil {
		return err
	}

	return nil
}

func NewTemplateCache() (map[string]*template.Template, error) {
	cache := make(map[string]*template.Template)

	pages, err := filepath.Glob("./templates/pages/*.html")
	if err != nil {
		return nil, err
	}

	for _, page := range pages {
		name := filepath.Base(page)
		patterns := []string{
			"./templates/base.html",
			"./templates/partials/header.html",
			"./templates/partials/footer.html",
			page,
		}

		ts, err := template.New(name).ParseFiles(patterns...)
		if err != nil {
			return nil, err
		}

		cache[name] = ts
	}

	return cache, nil
}
